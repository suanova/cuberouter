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
package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"image"
	"image/png"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupReviewDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { model.DB = old; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ImageModerationPolicy{}, &model.ImageModerationLease{}))
	return db
}

// Opt-in GPU acceptance against operator-supplied endpoints. Configuration
// contains paths to existing service secrets, never prompts or generated images.
func TestImageModerationRealChannels(t *testing.T) {
	path := os.Getenv("STUDIO_REVIEW_CHANNEL_CONFIG")
	if path == "" {
		t.Skip("real channel integration is opt-in")
	}
	setupReviewDB(t)
	encoded, err := os.ReadFile(path)
	require.NoError(t, err)
	var config []struct{ Endpoint, TokenFile, CAFile string }
	require.NoError(t, common.Unmarshal(encoded, &config))
	require.GreaterOrEqual(t, len(config), 1)
	require.LessOrEqual(t, len(config), 2)
	var channels []*model.Channel
	for _, entry := range config {
		key, err := os.ReadFile(entry.TokenFile)
		require.NoError(t, err)
		ca, err := os.ReadFile(entry.CAFile)
		require.NoError(t, err)
		settings, err := common.Marshal(model.ImageModerationSettings{CAPEM: string(ca)})
		require.NoError(t, err)
		channel := &model.Channel{Type: constant.ChannelTypeImageModeration, Status: 1, Name: "GPU acceptance", Key: strings.TrimSpace(string(key)), BaseURL: common.GetPointer(entry.Endpoint), OtherSettings: string(settings)}
		_, _, err = model.ValidateImageModerationChannel(channel)
		require.NoError(t, err)
		require.NoError(t, channel.Insert())
		channels = append(channels, channel)
	}
	fixture := image.NewRGBA(image.Rect(0, 0, 1312, 1312))
	random := rand.New(rand.NewSource(42))
	for i := 0; i < len(fixture.Pix); i += 4 {
		fixture.Pix[i] = uint8(random.Intn(256))
		fixture.Pix[i+1] = uint8(random.Intn(256))
		fixture.Pix[i+2] = uint8(random.Intn(256))
		fixture.Pix[i+3] = 255
	}
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, fixture))
	for _, replicas := range []int{len(channels), 1} {
		if replicas == 1 && len(channels) > 1 {
			require.NoError(t, model.DB.Model(channels[1]).Update("status", 2).Error)
		}
		started := time.Now()
		var wg sync.WaitGroup
		reports := make([]studioModerationReport, 30)
		for i := range reports {
			wg.Add(1)
			go func(i int) { defer wg.Done(); reports[i] = checkStudioOutput(context.Background(), data.Bytes()) }(i)
		}
		wg.Wait()
		counts := map[int]int{}
		for _, report := range reports {
			require.Equal(t, "passed", report.State)
			require.Equal(t, "channels", report.Source)
			require.Len(t, report.Scores, 3)
			counts[report.ChannelID]++
		}
		t.Logf("replicas=%d images=30 png_bytes=%d elapsed=%s channel_counts=%v", replicas, data.Len(), time.Since(started).Round(time.Millisecond), counts)
	}
}

func addReviewChannel(t *testing.T, server *httptest.Server) *model.Channel {
	t.Helper()
	settings, err := common.Marshal(model.ImageModerationSettings{CAPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))})
	require.NoError(t, err)
	endpoint := server.URL + "/v1/moderate"
	channel := &model.Channel{Type: constant.ChannelTypeImageModeration, Key: strings.Repeat("s", 40), Status: 1, Name: "test moderator", BaseURL: &endpoint, Models: model.ImageModerationModel, Group: "default", OtherSettings: string(settings)}
	require.NoError(t, channel.Insert())
	return channel
}

func reviewServer(t *testing.T, status int, change func(map[string]any)) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		result := reviewResponse(data)
		if change != nil {
			change(result)
		}
		body, _ := common.Marshal(result)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestImageModerationThirtyOutputsShareTwoChannels(t *testing.T) {
	setupReviewDB(t)
	t.Setenv("IMAGE_STUDIO_MODERATION_ENABLED", "false") // Channel mode cannot be bypassed by legacy flag.
	var calls, active [2]atomic.Int32
	var overlap atomic.Bool
	for i := 0; i < 2; i++ {
		i := i
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if active[i].Add(1) != 1 {
				overlap.Store(true)
			}
			defer active[i].Add(-1)
			data, _ := io.ReadAll(r.Body)
			if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 40) {
				w.WriteHeader(401)
				return
			}
			time.Sleep(10 * time.Millisecond)
			calls[i].Add(1)
			body, _ := common.Marshal(reviewResponse(data))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		}))
		t.Cleanup(server.Close)
		addReviewChannel(t, server)
	}
	var wg sync.WaitGroup
	reports := make([]studioModerationReport, 30)
	for i := range reports {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reports[i] = checkStudioOutput(context.Background(), append([]byte{}, reviewPNG...))
		}(i)
	}
	wg.Wait()
	for _, report := range reports {
		require.Equal(t, "passed", report.State)
		require.Equal(t, "channels", report.Source)
		require.NotZero(t, report.ChannelID)
		require.Len(t, report.Scores, 3)
	}
	require.False(t, overlap.Load(), "shared leases must serialize each endpoint even with concurrent callers")
	require.EqualValues(t, 30, calls[0].Load()+calls[1].Load())
	require.Positive(t, calls[0].Load())
	require.Positive(t, calls[1].Load())
}

func TestImageModerationChannelChangesAreLiveAndNeverFallBack(t *testing.T) {
	db := setupReviewDB(t)
	var legacyCalls atomic.Int32
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { legacyCalls.Add(1) }))
	t.Cleanup(legacy.Close)
	t.Setenv("IMAGE_STUDIO_MODERATION_ENABLED", "true")
	t.Setenv("IMAGE_STUDIO_MODERATION_URL", legacy.URL)
	t.Setenv("IMAGE_STUDIO_MODERATION_TOKEN", strings.Repeat("l", 40))
	first := addReviewChannel(t, reviewServer(t, 200, nil))
	endpoint, _, err := model.ValidateImageModerationChannel(first)
	require.NoError(t, err)
	ok, err := model.TryImageModerationLease(context.Background(), endpoint, "occupied", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	result := make(chan studioModerationReport, 1)
	go func() { result <- checkStudioOutput(context.Background(), reviewPNG) }()
	// Add and disable channels while a review is waiting for a busy GPU.
	time.Sleep(200 * time.Millisecond)
	require.NoError(t, db.Model(first).Update("status", 2).Error)
	second := addReviewChannel(t, reviewServer(t, 200, nil))
	select {
	case report := <-result:
		require.Equal(t, "passed", report.State)
		require.Equal(t, second.Id, report.ChannelID)
	case <-time.After(3 * time.Second):
		t.Fatal("waiting review did not discover a newly added channel")
	}
	require.NoError(t, db.Model(second).Update("status", 2).Error)
	require.Equal(t, "error", checkStudioOutput(context.Background(), reviewPNG).State)
	require.NoError(t, first.Delete())
	require.NoError(t, second.Delete())
	t.Setenv("IMAGE_STUDIO_MODERATION_ENABLED", "false")
	report := checkStudioOutput(context.Background(), reviewPNG)
	require.Equal(t, "error", report.State)
	require.Equal(t, "channels", report.Source)
	require.Zero(t, legacyCalls.Load())
}

func TestImageModerationAvailabilityFailoverAndTerminalVerdicts(t *testing.T) {
	for _, test := range []string{"availability", "block", "hash mismatch", "untrusted TLS"} {
		t.Run(test, func(t *testing.T) {
			setupReviewDB(t)
			var fallbackCalls atomic.Int32
			good := addReviewChannel(t, reviewServer(t, 200, func(map[string]any) { fallbackCalls.Add(1) }))
			status := 200
			if test == "availability" {
				status = 503
			}
			bad := addReviewChannel(t, reviewServer(t, status, func(r map[string]any) {
				if test == "block" {
					r["decision"] = "block"
					r["release_allowed"] = false
					r["scores"].(map[string]float64)["violence"] = 0.9
				}
				if test == "hash mismatch" {
					r["image_sha256"] = "wrong"
				}
			}))
			bad.Priority = common.GetPointer(int64(100))
			if test == "untrusted TLS" {
				bad.OtherSettings = "{}"
			}
			require.NoError(t, bad.Update())
			report := checkStudioOutput(context.Background(), reviewPNG)
			if test == "availability" || test == "untrusted TLS" {
				require.Equal(t, "passed", report.State)
				require.Equal(t, good.Id, report.ChannelID)
				require.Equal(t, []int{bad.Id, good.Id}, report.AttemptedChannels)
			} else {
				want := "blocked"
				if test == "hash mismatch" {
					want = "error"
				}
				require.Equal(t, want, report.State)
				require.Zero(t, fallbackCalls.Load())
			}
		})
	}
}

func TestImageModerationChannelPrivacyAndDuplicateEndpointLease(t *testing.T) {
	db := setupReviewDB(t)
	server := reviewServer(t, 200, nil)
	channel := addReviewChannel(t, server)
	require.Empty(t, model.GetEnabledModels())
	var count int64
	require.NoError(t, db.Model(&model.Ability{}).Count(&count).Error)
	require.Zero(t, count)
	recorder := httptest.NewRecorder()
	ctxRelay, _ := gin.CreateTestContext(recorder)
	require.NotNil(t, middleware.SetupContextForSelectedChannel(ctxRelay, channel, model.ImageModerationModel))
	require.Empty(t, common.GetContextKeyString(ctxRelay, constant.ContextKeyChannelKey))
	endpoint, _, err := model.ValidateImageModerationChannel(channel)
	require.NoError(t, err)
	ok, err := model.TryImageModerationLease(context.Background(), endpoint, "first", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	duplicate := addReviewChannel(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.Equal(t, "error", runStudioChannelReview(ctx, reviewPNG, []*model.Channel{duplicate}, false).State)
	model.ReleaseImageModerationLease(endpoint, "wrong-owner", 0)
	ok, err = model.TryImageModerationLease(context.Background(), endpoint, "second", time.Minute)
	require.NoError(t, err)
	require.False(t, ok)
	model.ReleaseImageModerationLease(endpoint, "first", 0)
	require.Equal(t, "passed", checkStudioOutput(context.Background(), reviewPNG).State)
}

func TestImageModerationChannelValidation(t *testing.T) {
	setupReviewDB(t)
	channel := addReviewChannel(t, reviewServer(t, 200, nil))
	for _, endpoint := range []string{"http://example.com/v1/moderate", "https://example.com", "https://user:secret@example.com/v1/moderate", "https://example.com/v1/moderate?key=secret"} {
		copy := *channel
		copy.BaseURL = &endpoint
		_, _, err := model.ValidateImageModerationChannel(&copy)
		require.Error(t, err)
	}
	copy := *channel
	copy.Key = "short"
	_, _, err := model.ValidateImageModerationChannel(&copy)
	require.Error(t, err)
	copy = *channel
	copy.OtherSettings = `{"image_moderation_ca_pem":"not a certificate"}`
	_, _, err = model.ValidateImageModerationChannel(&copy)
	require.Error(t, err)
	require.NoError(t, validateChannel(channel, true))
	require.Equal(t, "image-moderation-internal", channel.Group)
	require.Equal(t, model.ImageModerationModel, channel.Models)
	require.Equal(t, 0, *channel.AutoBan)
}

func TestImageModerationChannelAuditKeepsGenerationAndReviewIdentity(t *testing.T) {
	db := setupReviewDB(t)
	t.Setenv("IMAGE_STUDIO_AUDIT_ENABLED", "true")
	require.NoError(t, db.AutoMigrate(&model.ImageStudioAudit{}))
	channel := addReviewChannel(t, reviewServer(t, 200, nil))
	require.NoError(t, model.CreateImageStudioAudit(&model.ImageStudioAudit{JobID: "channel-audit", UserID: 71, ChannelID: 42, UserPrompt: "green fixture"}))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/", nil)
	c.Set("id", 71)
	c.Set(common.RequestIdKey, "channel-audit")
	encoded := base64.StdEncoding.EncodeToString(reviewPNG)
	body, err := common.Marshal(map[string]any{"data": []map[string]string{{"b64_json": encoded}}})
	require.NoError(t, err)
	buffer := &studioRelayWriter{ResponseWriter: c.Writer, status: 200}
	buffer.body.Write(body)
	deliverStudioOutput(c, buffer)
	require.Equal(t, 200, recorder.Code)
	rows, err := model.FindImageStudioAudits("channel-audit", "", 71)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 42, rows[0].ChannelID)
	var report studioModerationReport
	require.NoError(t, common.UnmarshalJsonStr(rows[0].ModerationResult, &report))
	require.Equal(t, "channels", report.Source)
	require.Equal(t, channel.Id, report.ChannelID)
	require.Equal(t, "passed", report.State)
	require.Len(t, report.Scores, 3)
	require.NotContains(t, rows[0].ModerationResult, encoded)
	require.NotContains(t, rows[0].ModerationResult, channel.Key)
}

func TestImageModerationLeaseAtomicExpiryAndAdmission(t *testing.T) {
	db := setupReviewDB(t)
	const endpoint = "https://example.com/v1/moderate"
	var wg sync.WaitGroup
	var winners atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := model.TryImageModerationLease(context.Background(), endpoint, fmt.Sprint(i), time.Minute)
			if err == nil && ok {
				winners.Add(1)
			}
		}(i)
	}
	wg.Wait()
	require.EqualValues(t, 1, winners.Load())
	require.NoError(t, db.Model(&model.ImageModerationLease{}).Where("1 = 1").Update("expires_at", 0).Error)
	ok, err := model.TryImageModerationLease(context.Background(), endpoint, "new", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	var releases []func()
	defer func() {
		for _, f := range releases {
			f()
		}
	}()
	for i := 0; i < 64; i++ {
		f, ok := admitStudioReview(1)
		require.True(t, ok)
		releases = append(releases, f)
	}
	_, ok = admitStudioReview(1)
	require.False(t, ok)
}
