/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"golang.org/x/sync/singleflight"
)

type studioWorkerHealth struct {
	ready   bool
	expires time.Time
}

var studioHealthClient = studioPrivateClient(studioPrivateTransport(), 2*time.Second)
var studioHealthProbes singleflight.Group
var studioHealthCache = struct {
	sync.Mutex
	results map[string]studioWorkerHealth
}{results: make(map[string]studioWorkerHealth)}

func StudioImageEnabled() bool { return os.Getenv("IMAGE_STUDIO_ENABLED") == "true" }

func StudioImageModels() []string {
	var names []string
	for _, name := range strings.Split(os.Getenv("IMAGE_STUDIO_MODELS"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func StudioImageChannelIDs() []int {
	var ids []int
	for _, value := range strings.Split(os.Getenv("IMAGE_STUDIO_CHANNEL_IDS"), ",") {
		id, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || id <= 0 || slices.Contains(ids, id) {
			continue
		}
		ids = append(ids, id)
	}
	// Keep every configured ID reserved from ordinary traffic, even when an
	// oversized pool is rejected for Studio admission below.
	return ids
}

func StudioImageGroupAllowed(group string) bool {
	allowed, set := os.LookupEnv("IMAGE_STUDIO_ALLOWED_GROUPS")
	if !set {
		allowed = "image-studio"
	}
	for _, item := range strings.Split(allowed, ",") {
		if group != "" && group == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}

// Readiness is semantic, not HTTP liveness. Bound the entire parallel probe so
// an unavailable high-priority channel cannot hide a healthy lower replica.
// Dispatch skips workers already fenced by another execution.
func ReadyStudioImageChannels(ctx context.Context, group, modelName string, skipLeased bool) []*model.Channel {
	ids := StudioImageChannelIDs()
	if len(ids) == 0 || len(ids) > 64 {
		return nil
	}
	channels, err := model.StudioImageWorkerChannels(ids, group, modelName)
	if err != nil || len(channels) == 0 {
		return nil
	}
	var leases []model.StudioImageLease
	resources := make([]string, 0, len(channels))
	for _, channel := range channels {
		resources = append(resources, "gpu:"+strconv.Itoa(channel.Id))
	}
	if model.DB.Where("resource IN ?", resources).Find(&leases).Error != nil {
		return nil
	}
	workerLeases := make(map[int]model.StudioImageLease, len(leases))
	for _, lease := range leases {
		if id, err := strconv.Atoi(strings.TrimPrefix(lease.Resource, "gpu:")); err == nil {
			workerLeases[id] = lease
		}
	}
	now := time.Now().UnixMilli()
	probeCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	var mu sync.Mutex
	var wg sync.WaitGroup
	ready := make([]*model.Channel, 0, len(channels))
	for _, ch := range channels {
		if skipLeased && workerLeases[ch.Id].ExpiresAt > now {
			continue
		}
		if (ch.ParamOverride != nil && strings.TrimSpace(*ch.ParamOverride) != "" && strings.TrimSpace(*ch.ParamOverride) != "{}") ||
			(ch.HeaderOverride != nil && strings.TrimSpace(*ch.HeaderOverride) != "" && strings.TrimSpace(*ch.HeaderOverride) != "{}") {
			continue
		}
		if ch.Type != constant.ChannelTypeOpenAI || ch.ChannelInfo.IsMultiKey ||
			!slices.Contains(strings.Split(ch.Models, ","), modelName) {
			continue
		}
		wg.Add(1)
		go func(channel *model.Channel) {
			defer wg.Done()
			if cachedStudioImageWorkerHealth(probeCtx, channel) {
				mu.Lock()
				ready = append(ready, channel)
				mu.Unlock()
			}
		}(ch)
	}
	wg.Wait()
	sort.Slice(ready, func(i, j int) bool {
		if ready[i].GetPriority() != ready[j].GetPriority() {
			return ready[i].GetPriority() > ready[j].GetPriority()
		}
		lastI := workerLeases[ready[i].Id].LastUsed
		lastJ := workerLeases[ready[j].Id].LastUsed
		if lastI != lastJ {
			return lastI < lastJ
		}
		return ready[i].Id < ready[j].Id
	})
	return ready
}

func cachedStudioImageWorkerHealth(ctx context.Context, channel *model.Channel) bool {
	key := fmt.Sprintf("%d:%s:%x", channel.Id, channel.GetBaseURL(), sha256.Sum256([]byte(channel.Key)))
	get := func() (studioWorkerHealth, bool) {
		studioHealthCache.Lock()
		defer studioHealthCache.Unlock()
		cached, ok := studioHealthCache.results[key]
		return cached, ok && time.Now().Before(cached.expires)
	}
	if cached, ok := get(); ok {
		return cached.ready
	}
	probe := studioHealthProbes.DoChan(key, func() (any, error) {
		if cached, ok := get(); ok {
			return cached.ready, nil
		}
		ready := ProbeStudioImageWorker(context.Background(), channel)
		now := time.Now()
		studioHealthCache.Lock()
		for id, cached := range studioHealthCache.results {
			if !now.Before(cached.expires) {
				delete(studioHealthCache.results, id)
			}
		}
		studioHealthCache.results[key] = studioWorkerHealth{ready: ready, expires: now.Add(2 * time.Second)}
		studioHealthCache.Unlock()
		return ready, nil
	})
	select {
	case <-ctx.Done():
		return false
	case result := <-probe:
		return result.Val == true
	}
}

func ProbeStudioImageWorker(ctx context.Context, channel *model.Channel) bool {
	if len(channel.Key) < 32 {
		return false
	}
	endpoint := strings.TrimRight(channel.GetBaseURL(), "/") + "/health"
	if validateStudioServiceURL(endpoint) != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	resp, err := studioHealthClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	var health struct {
		State           string `json:"state"`
		GPUCount        int    `json:"gpu_count"`
		ControlProtocol string `json:"control_protocol"`
	}
	return resp.StatusCode == http.StatusOK && err == nil && len(body) <= 4096 &&
		common.Unmarshal(body, &health) == nil && health.State == "ready" && health.GPUCount == 1 && health.ControlProtocol == "qwen-execution-v1"
}
