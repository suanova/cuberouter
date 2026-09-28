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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestImageSlotsNeverShareAnActiveGPU(t *testing.T) {
	pool := newImageChannelSlots()
	ids := []int{1, 2, 3, 4, 5, 6, 7, 8}
	var wg sync.WaitGroup
	var mu sync.Mutex
	active := map[int]bool{}
	ready, release := make(chan int, 8), make(chan struct{})
	for range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, done := pool.reserve(ids)
			mu.Lock()
			duplicate := active[id]
			active[id] = true
			mu.Unlock()
			if id == 0 || duplicate {
				t.Errorf("overlapping GPU reservation: %d", id)
				return
			}
			ready <- id
			<-release
			done()
			done() // duplicate cleanup must not corrupt the next lease
		}()
	}
	for range ids {
		select {
		case <-ready:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("eight reservations did not become ready")
		}
	}
	id, done := pool.reserve(ids)
	require.Zero(t, id)
	require.Nil(t, done)
	close(release)
	wg.Wait()
	id, done = pool.reserve(ids)
	require.NotZero(t, id)
	done()
}

func TestImagePoolEightChannelsAndNinthWaits(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	oldDB, oldCache, oldSlots := model.DB, common.MemoryCacheEnabled, imageSlots
	model.DB, common.MemoryCacheEnabled, imageSlots = db, true, newImageChannelSlots()
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { model.DB, common.MemoryCacheEnabled, imageSlots = oldDB, oldCache, oldSlots; sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" || r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"state":"ready","gpu_count":1}`))
	}))
	defer health.Close()
	for id := 1; id <= 8; id++ {
		require.NoError(t, db.Create(&model.Channel{Id: id, Type: 1, Status: common.ChannelStatusEnabled, Key: "test-key", BaseURL: &health.URL, Models: "qwen-image-2.1", Group: "trial"}).Error)
		require.NoError(t, db.Create(&model.Ability{Group: "trial", Model: "qwen-image-2.1", ChannelId: id, Enabled: true}).Error)
	}
	model.InitChannelCache()
	t.Setenv("IMAGE_STUDIO_CHANNEL_IDS", "1,2,3,4,5,6,7,8")
	t.Setenv("IMAGE_STUDIO_CHANNEL_MODEL", "qwen-image-2.1")
	ctx := func(timeout time.Duration) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		requestCtx, cancel := context.WithTimeout(context.Background(), timeout)
		t.Cleanup(cancel)
		c.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{}`)).WithContext(requestCtx)
		return c
	}
	releases := []func(){}
	seen := map[int]bool{}
	for i := 0; i < 8; i++ {
		ch, g, done, handled, err := reserveConfiguredImageChannel(ctx(time.Second), "qwen-image-2.1", "trial", 0)
		require.NoError(t, err)
		require.True(t, handled)
		require.Equal(t, "trial", g)
		require.False(t, seen[ch.Id])
		seen[ch.Id] = true
		releases = append(releases, done)
	}
	_, _, _, handled, err := reserveConfiguredImageChannel(ctx(25*time.Millisecond), "qwen-image-2.1", "trial", 0)
	require.True(t, handled)
	require.Error(t, err)
	releases[3]()
	ch, _, done, _, err := reserveConfiguredImageChannel(ctx(time.Second), "qwen-image-2.1", "trial", 0)
	require.NoError(t, err)
	require.NotNil(t, ch)
	done()
	for _, f := range releases {
		f()
	}
	// An enabled channel from another account group must never be borrowed.
	_, _, _, _, err = reserveConfiguredImageChannel(ctx(25*time.Millisecond), "qwen-image-2.1", "other-group", 0)
	require.Error(t, err)
	// Disabled channels remain excluded even when their worker is healthy.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 1).Update("status", common.ChannelStatusManuallyDisabled).Error)
	_, _, _, _, err = reserveConfiguredImageChannel(ctx(25*time.Millisecond), "qwen-image-2.1", "trial", 1)
	require.Error(t, err)
	// An unhealthy high-priority worker must release its reservation and allow
	// a lower-priority healthy worker to serve the request before any inference.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 2).Updates(map[string]any{
		"base_url": health.URL + "/unavailable", "priority": 10,
	}).Error)
	t.Setenv("IMAGE_STUDIO_CHANNEL_IDS", "2,3")
	ch, _, done, handled, err = reserveConfiguredImageChannel(ctx(time.Second), "qwen-image-2.1", "trial", 0)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, 3, ch.Id)
	done()
	id, release := imageSlots.reserve([]int{2})
	require.Equal(t, 2, id)
	require.NotNil(t, release)
	release()
}

func TestImageHealthRequiresReadySingleGPUAndNeverRedirectsKey(t *testing.T) {
	for _, tc := range []struct {
		state string
		count int
		ready bool
	}{{"ready", 1, true}, {"generating", 1, false}, {"failed", 1, false}, {"ready", 2, false}} {
		t.Run(fmt.Sprintf("%s-%d", tc.state, tc.count), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"state":%q,"gpu_count":%d}`, tc.state, tc.count)
			}))
			defer s.Close()
			require.Equal(t, tc.ready, imageChannelReady(context.Background(), &model.Channel{BaseURL: &s.URL, Key: "secret"}))
		})
	}
	visited := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { visited = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	require.False(t, imageChannelReady(context.Background(), &model.Channel{BaseURL: &source.URL, Key: "secret"}))
	require.False(t, visited)
}
