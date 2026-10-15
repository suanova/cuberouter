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
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStudioRecoveryRegistryOwnershipExpiryAndCapacity(t *testing.T) {
	now := time.Unix(1700000000, 0)
	r := newStudioExecutionRegistry(2, time.Hour, func() time.Time { return now })
	require.NoError(t, r.claim("active", "cuberouter-1", now.Add(25*time.Minute)))
	require.ErrorIs(t, r.claim("active", "cuberouter-2", now.Add(25*time.Minute)), errStudioExecutionOwner)
	_, err := r.reconcile("active", "cuberouter-2")
	require.ErrorIs(t, err, errStudioExecutionOwner)
	r.settle("active", "cuberouter-2")
	now = now.Add(24 * time.Hour)
	state, err := r.reconcile("active", "cuberouter-1")
	require.NoError(t, err)
	require.Equal(t, "active", state, "active work must not expire or settle for another owner")
	state, err = r.reconcile("absent", "cuberouter-1")
	require.NoError(t, err)
	require.Equal(t, "fenced", state)
	require.ErrorIs(t, r.claim("absent", "cuberouter-1", now.Add(25*time.Minute)), errStudioExecutionExists)
	require.ErrorIs(t, r.claim("third", "cuberouter-1", now.Add(25*time.Minute)), errStudioExecutionCapacity)
	_, err = r.reconcile("third", "cuberouter-1")
	require.ErrorIs(t, err, errStudioExecutionCapacity)
	now = now.Add(time.Hour)
	require.NoError(t, r.claim("third", "cuberouter-1", now.Add(25*time.Minute)), "expired terminal entries may be removed")
	state, err = r.reconcile("active", "cuberouter-1")
	require.NoError(t, err)
	require.Equal(t, "active", state)
	r.settle("active", "cuberouter-1")
	state, err = r.reconcile("active", "cuberouter-1")
	require.NoError(t, err)
	require.Equal(t, "settled", state)
	now = now.Add(time.Hour)
	state, err = r.reconcile("active", "cuberouter-1")
	require.NoError(t, err)
	require.Equal(t, "fenced", state, "an expired record must be fenced again, never reported simply absent")
	require.NotEqual(t, r.epoch, newStudioExecutionRegistry(1, time.Hour, time.Now).epoch)
}

func TestStudioRecoveryClaimAndFenceRace(t *testing.T) {
	for attempt := 0; attempt < 100; attempt++ {
		r := newStudioExecutionRegistry(2, time.Hour, time.Now)
		if attempt%2 == 0 {
			require.NoError(t, r.claim("job", "cuberouter-1", time.Now().Add(25*time.Minute)))
			r.deferUnstarted("job", "cuberouter-1")
		}
		start := make(chan struct{})
		var claimErr, reconcileErr error
		var state string
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			claimErr = r.claim("job", "cuberouter-1", time.Now().Add(25*time.Minute))
		}()
		go func() {
			defer wg.Done()
			<-start
			state, reconcileErr = r.reconcile("job", "cuberouter-1")
		}()
		close(start)
		wg.Wait()
		require.NoError(t, reconcileErr)
		if claimErr == nil {
			require.Equal(t, "active", state)
		} else {
			require.ErrorIs(t, claimErr, errStudioExecutionExists)
			require.Equal(t, "fenced", state)
		}
		require.ErrorIs(t, r.claim("job", "cuberouter-1", time.Now().Add(25*time.Minute)), errStudioExecutionExists)
	}
}

func setupStudioRecoveryTest(t *testing.T) *studioExecutionRegistry {
	t.Helper()
	t.Setenv("IMAGE_STUDIO_RELAY_TOKEN", strings.Repeat("r", 40))
	t.Setenv("IMAGE_STUDIO_CHANNEL_MODEL", "qwen-image-2.1")
	t.Setenv("IMAGE_STUDIO_ALLOWED_GROUPS", "trial")
	old := studioExecutions
	studioExecutions = newStudioExecutionRegistry(16, time.Hour, time.Now)
	t.Cleanup(func() { studioExecutions = old })
	return studioExecutions
}

func recoveryRequest(method, path, job, owner, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("r", 40))
	r.Header.Set("X-Image-Owner", owner)
	r.Header.Set("X-Image-Job", job)
	r.Header.Set("X-Image-Execution-Deadline", strconv.FormatInt(time.Now().Add(25*time.Minute).Unix(), 10))
	return r
}

func TestStudioRecoveryPrivateHTTPContract(t *testing.T) {
	registry := setupStudioRecoveryTest(t)
	router := gin.New()
	router.POST("/internal/image-studio/reconcile", ReconcileImageStudioExecution)
	job := strings.Repeat("a", 32)
	for _, tc := range []struct {
		name, owner, body, auth, query string
		status                         int
	}{
		{"no credential", "cuberouter-1", "{}", "", "", http.StatusUnauthorized},
		{"bad owner", "cuberouter-01", "{}", "valid", "", http.StatusBadRequest},
		{"array body", "cuberouter-1", "[]", "valid", "", http.StatusBadRequest},
		{"null body", "cuberouter-1", "null", "valid", "", http.StatusBadRequest},
		{"unexpected body", "cuberouter-1", `{"prompt":"must not be accepted"}`, "valid", "", http.StatusBadRequest},
		{"oversized body", "cuberouter-1", "{" + strings.Repeat(" ", 1024) + "}", "valid", "", http.StatusBadRequest},
		{"query forbidden", "cuberouter-1", "{}", "valid", "?job=other", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := recoveryRequest("POST", "/internal/image-studio/reconcile"+tc.query, job, tc.owner, tc.body)
			if tc.auth == "" {
				r.Header.Del("Authorization")
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, tc.status, w.Code)
			require.Empty(t, registry.seen, "invalid requests must not create fencing entries")
		})
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, recoveryRequest("POST", "/internal/image-studio/reconcile", job, "cuberouter-1", "{}"))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var response map[string]string
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, map[string]string{
		"protocol": studioRecoveryProtocol, "epoch": registry.epoch,
		"owner": "cuberouter-1", "job_id": job, "state": "fenced",
	}, response)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, recoveryRequest("POST", "/internal/image-studio/reconcile", job, "cuberouter-2", "{}"))
	require.Equal(t, http.StatusConflict, w.Code)
	registry.limit = 1
	w = httptest.NewRecorder()
	router.ServeHTTP(w, recoveryRequest("POST", "/internal/image-studio/reconcile", strings.Repeat("b", 32), "cuberouter-1", "{}"))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func setupStudioRecoveryUser(t *testing.T) {
	t.Helper()
	t.Setenv("IMAGE_STUDIO_AUDIT_ENABLED", "false")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	oldDB, oldRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { model.DB, common.RedisEnabled = oldDB, oldRedis })
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 71, Username: "studio-recovery-test", Status: common.UserStatusEnabled, Group: "trial", Quota: 100000}).Error)
}

type delayedStudioBody struct {
	io.Reader
	started chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func (r *delayedStudioBody) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started); <-r.resume })
	return r.Reader.Read(p)
}

func TestStudioRecoveryFencesCallbackWhoseBodyIsStillArriving(t *testing.T) {
	registry := setupStudioRecoveryTest(t)
	setupStudioRecoveryUser(t)
	router := gin.New()
	var calls atomic.Int32
	router.POST("/internal/image-studio/execute", PrepareImageStudioRelay, func(c *gin.Context) {
		calls.Add(1)
		c.Status(http.StatusOK)
	})
	body, err := common.Marshal(studioTestParameters())
	require.NoError(t, err)
	blocked := &delayedStudioBody{Reader: bytes.NewReader(body), started: make(chan struct{}), resume: make(chan struct{})}
	job := strings.Repeat("c", 32)
	r := recoveryRequest("POST", "/internal/image-studio/execute", job, "cuberouter-71", "")
	r.Body = io.NopCloser(blocked)
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); router.ServeHTTP(w, r) }()
	<-blocked.started
	state, err := registry.reconcile(job, "cuberouter-71")
	require.NoError(t, err)
	require.Equal(t, "fenced", state)
	close(blocked.resume)
	<-done
	require.Equal(t, http.StatusConflict, w.Code)
	require.Zero(t, calls.Load(), "late callback must never reach channel selection or inference")
}

func TestStudioRecoveryRechecksDeadlineAfterDelayedBody(t *testing.T) {
	registry := setupStudioRecoveryTest(t)
	setupStudioRecoveryUser(t)
	var currentUnix atomic.Int64
	currentUnix.Store(time.Now().Unix())
	registry.now = func() time.Time { return time.Unix(currentUnix.Load(), 0) }
	router := gin.New()
	var calls atomic.Int32
	router.POST("/internal/image-studio/execute", PrepareImageStudioRelay, func(c *gin.Context) {
		calls.Add(1)
		c.Status(http.StatusOK)
	})
	body, err := common.Marshal(studioTestParameters())
	require.NoError(t, err)
	blocked := &delayedStudioBody{Reader: bytes.NewReader(body), started: make(chan struct{}), resume: make(chan struct{})}
	r := recoveryRequest("POST", "/internal/image-studio/execute", strings.Repeat("f", 32), "cuberouter-71", "")
	r.Body = io.NopCloser(blocked)
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); router.ServeHTTP(w, r) }()
	<-blocked.started
	currentUnix.Add(int64((2 * time.Hour).Seconds()))
	close(blocked.resume)
	<-done
	require.Equal(t, http.StatusRequestTimeout, w.Code)
	require.Zero(t, calls.Load())
	require.Empty(t, registry.seen)
}

func TestStudioRecoverySettlesOnlyAfterDownstreamChannelRelease(t *testing.T) {
	registry := setupStudioRecoveryTest(t)
	setupStudioRecoveryUser(t)
	job, owner := strings.Repeat("d", 32), "cuberouter-71"
	router := gin.New()
	released := false
	router.POST("/internal/image-studio/execute", PrepareImageStudioRelay, func(c *gin.Context) {
		defer func() {
			state, err := registry.reconcile(job, owner)
			require.NoError(t, err)
			require.Equal(t, "active", state, "channel release must precede settlement")
			released = true
		}()
		c.Next()
	}, func(c *gin.Context) { c.Status(http.StatusOK) })
	body, err := common.Marshal(studioTestParameters())
	require.NoError(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, recoveryRequest("POST", "/internal/image-studio/execute", job, owner, string(body)))
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, released)
	state, err := registry.reconcile(job, owner)
	require.NoError(t, err)
	require.Equal(t, "settled", state)
	require.ErrorIs(t, registry.claim(job, owner, time.Now().Add(25*time.Minute)), errStudioExecutionExists)
}

func TestStudioRecoveryRejectsCallbackAfterItsFenceExpires(t *testing.T) {
	now := time.Unix(1700000000, 0)
	r := newStudioExecutionRegistry(2, time.Hour, func() time.Time { return now })
	deadline := now.Add(25 * time.Minute)
	state, err := r.reconcile("delayed", "cuberouter-1")
	require.NoError(t, err)
	require.Equal(t, "fenced", state)
	now = now.Add(2 * time.Hour)
	require.ErrorIs(t, r.claim("delayed", "cuberouter-1", deadline), errStudioExecutionDeadline)
	state, err = r.reconcile("delayed", "cuberouter-1")
	require.NoError(t, err)
	require.Equal(t, "fenced", state)
}

func TestStudioRecoveryRejectsInvalidWireDeadlineBeforeReadingBody(t *testing.T) {
	registry := setupStudioRecoveryTest(t)
	router := gin.New()
	router.POST("/internal/image-studio/execute", PrepareImageStudioRelay)
	for _, deadline := range []string{"", "invalid", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10), strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)} {
		r := recoveryRequest("POST", "/internal/image-studio/execute", strings.Repeat("e", 32), "cuberouter-71", "invalid body")
		r.Header.Set("X-Image-Execution-Deadline", deadline)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		require.Contains(t, []int{http.StatusBadRequest, http.StatusRequestTimeout}, w.Code)
		require.Empty(t, registry.seen)
		require.Contains(t, w.Body.String(), "execution_deadline")
	}
}
