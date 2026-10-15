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
package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// A pool is deliberately local to ONE CubeRouter process. Workers also reject
// overlapping inference. Multiple gateway replicas require shared leases first.
type imageChannelSlots struct {
	sync.Mutex
	busy     map[int]bool
	last     map[int]uint64
	sequence uint64
}

func newImageChannelSlots() *imageChannelSlots {
	return &imageChannelSlots{busy: map[int]bool{}, last: map[int]uint64{}}
}

func (p *imageChannelSlots) reserve(ids []int) (int, func()) {
	p.Lock()
	defer p.Unlock()
	chosen := 0
	for _, id := range ids {
		if !p.busy[id] && (chosen == 0 || p.last[id] < p.last[chosen]) {
			chosen = id
		}
	}
	if chosen == 0 {
		return 0, nil
	}
	p.sequence++
	p.busy[chosen], p.last[chosen] = true, p.sequence
	var once sync.Once
	return chosen, func() { once.Do(func() { p.Lock(); delete(p.busy, chosen); p.Unlock() }) }
}

var imageSlots = newImageChannelSlots()
var imageHealthHTTP = &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}

func imagePoolIDs() ([]int, error) {
	raw := strings.TrimSpace(os.Getenv("IMAGE_STUDIO_CHANNEL_IDS"))
	if raw == "" || raw == "auto" {
		return nil, nil
	}
	ids, seen := []int{}, map[int]bool{}
	for _, value := range strings.Split(raw, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || id < 1 || seen[id] {
			return nil, errors.New("invalid image channel pool")
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) > 64 {
		return nil, errors.New("image channel pool too large")
	}
	return ids, nil
}

var ErrImageChannelBusy = errors.New("image channel busy before inference")

// Count current configured channels only; never expose their keys or URLs.
func ImageChannelPoolCapacity() (int, error) {
	ids, err := imagePoolIDs()
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(os.Getenv("IMAGE_STUDIO_CHANNEL_IDS")) != "auto" && len(ids) == 0 {
		return 0, errors.New("image pool not configured")
	}
	channels, err := model.GetImageChannelCandidates(os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL"), nil, ids)
	if err != nil {
		return 0, err
	}
	if len(channels) > 64 {
		return 0, errors.New("image channel pool too large")
	}
	return len(channels), nil
}

func imageChannelReady(ctx context.Context, ch *model.Channel) bool {
	if ch.BaseURL == nil || ch.ChannelInfo.IsMultiKey {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(*ch.BaseURL, "/")+"/health", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+ch.Key)
	resp, err := imageHealthHTTP.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16385))
	if err != nil || len(data) > 16384 {
		return false
	}
	var health struct {
		State    string `json:"state"`
		GPUCount int    `json:"gpu_count"`
	}
	return common.Unmarshal(data, &health) == nil && health.State == "ready" && health.GPUCount == 1
}

// Reserve only configured image channels that are still enabled for the user's
// group and model. Health checks happen BEFORE forwarding any generation body.
func reserveConfiguredImageChannel(c *gin.Context, requestedModel, group string, specific int) (*model.Channel, string, func(), bool, error) {
	if requestedModel != os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL") ||
		(c.Request.URL.Path != "/v1/images/generations" && c.Request.URL.Path != "/v1/images/edits") {
		return nil, "", nil, false, nil
	}
	ids, err := imagePoolIDs()
	if err != nil {
		return nil, "", nil, true, err
	}
	dynamic := strings.TrimSpace(os.Getenv("IMAGE_STUDIO_CHANNEL_IDS")) == "auto"
	if len(ids) == 0 && !dynamic {
		return nil, "", nil, false, nil
	}
	if specific > 0 {
		found := dynamic
		for _, id := range ids {
			if id == specific {
				found = true
			}
		}
		if !found {
			return nil, "", nil, false, nil
		}
		ids = []int{specific}
	}
	groups := []string{group}
	if group == "auto" {
		groups = service.GetRequestAutoGroups(c, common.GetContextKeyString(c, constant.ContextKeyUserGroup))
	}
	wait := 30 * time.Second
	studio := common.GetContextKeyBool(c, constant.ContextKeyImageStudio)
	if studio {
		wait = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), wait)
	defer cancel()
	cooldown := map[int]time.Time{}
	for {
		channels, selectedGroups := map[int]*model.Channel{}, map[int]string{}
		eligible := []int{}
		candidates, lookupErr := model.GetImageChannelCandidates(requestedModel, groups, ids)
		if lookupErr != nil {
			return nil, "", nil, true, lookupErr
		}
		if len(candidates) > 64 {
			return nil, "", nil, true, errors.New("image channel pool too large")
		}
		if len(candidates) == 0 && studio {
			if dynamic {
				// A zero-capacity refresh can lag behind a last-channel removal.
				// Preserve queued work until channels return (or queue TTL expires).
				capacity, err := ImageChannelPoolCapacity()
				if err == nil && capacity == 0 {
					return nil, "", nil, true, ErrImageChannelBusy
				}
			}
			return nil, "", nil, true, errors.New("no eligible image channels")
		}
		for _, ch := range candidates {
			if ok, _ := model.ChannelSatisfiesFilters(ch, requestedModel, service.GetChannelConstraints(c).Filters); !ok {
				continue
			}
			id := ch.Id
			if time.Now().Before(cooldown[id]) {
				continue
			}
			// Abilities and channel fields are read fresh on every admission.
			for _, g := range groups {
				allowed, err := model.GetImageChannelCandidates(requestedModel, []string{g}, []int{id})
				if err != nil {
					return nil, "", nil, true, err
				}
				if len(allowed) > 0 {
					channels[id], selectedGroups[id] = ch, g
					eligible = append(eligible, id)
					break
				}
			}
		}
		// Honor channel priority while preferring idle channels within that tier.
		sort.SliceStable(eligible, func(i, j int) bool { return channels[eligible[i]].GetPriority() > channels[eligible[j]].GetPriority() })
		for len(eligible) > 0 {
			end := 1
			for end < len(eligible) && channels[eligible[end]].GetPriority() == channels[eligible[0]].GetPriority() {
				end++
			}
			id, release := imageSlots.reserve(eligible[:end])
			if id == 0 {
				eligible = eligible[end:]
				continue
			}
			if imageChannelReady(ctx, channels[id]) {
				return channels[id], selectedGroups[id], release, true, nil
			}
			release()
			cooldown[id] = time.Now().Add(2 * time.Second)
			for i, candidate := range eligible {
				if candidate == id {
					eligible = append(eligible[:i], eligible[i+1:]...)
					break
				}
			}
		}
		if studio {
			// No generation body has been forwarded. Queue may safely wait.
			return nil, "", nil, true, ErrImageChannelBusy
		}
		select {
		case <-ctx.Done():
			return nil, "", nil, true, errors.New("no idle healthy image channel")
		case <-time.After(150 * time.Millisecond):
		}
	}
}
