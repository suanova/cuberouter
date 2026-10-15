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
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const studioRecoveryProtocol = "image-studio-recovery-v1"

var (
	errStudioExecutionExists   = errors.New("execution_already_attempted")
	errStudioExecutionOwner    = errors.New("execution_owner_mismatch")
	errStudioExecutionCapacity = errors.New("execution_registry_full")
	errStudioExecutionDeadline = errors.New("execution_deadline_expired")
)

type studioExecution struct {
	owner   string
	state   string
	expires time.Time
}

// Only identifiers are retained; recovery never stores or replays image bytes.
// The registry and channel pool require one active CubeRouter process.
type studioExecutionRegistry struct {
	sync.Mutex
	seen  map[string]studioExecution
	epoch string
	limit int
	ttl   time.Duration
	now   func() time.Time
}

func newStudioExecutionRegistry(limit int, ttl time.Duration, now func() time.Time) *studioExecutionRegistry {
	epoch := make([]byte, 16)
	if _, err := rand.Read(epoch); err != nil {
		panic("image studio recovery epoch unavailable")
	}
	return &studioExecutionRegistry{
		seen: make(map[string]studioExecution), epoch: hex.EncodeToString(epoch),
		limit: limit, ttl: ttl, now: now,
	}
}

var studioExecutions = newStudioExecutionRegistry(4096, time.Hour, time.Now)

func (r *studioExecutionRegistry) cleanup(now time.Time) {
	for job, entry := range r.seen {
		// Never evict work still running, even if an upstream call hangs.
		if entry.state != "active" && !now.Before(entry.expires) {
			delete(r.seen, job)
		}
	}
}

func (r *studioExecutionRegistry) claim(job, owner string, deadline time.Time) error {
	r.Lock()
	defer r.Unlock()
	now := r.now()
	// Check atomically with claiming, after body validation. The dispatcher's
	// 25-minute wire deadline is shorter than the one-hour fencing retention,
	// so a delayed callback cannot outlive its fence and start later.
	if !now.Before(deadline) {
		return errStudioExecutionDeadline
	}
	r.cleanup(now)
	if entry, exists := r.seen[job]; exists {
		if entry.owner != owner {
			return errStudioExecutionOwner
		}
		if entry.state == "retryable" {
			r.seen[job] = studioExecution{owner: owner, state: "active"}
			return nil
		}
		return errStudioExecutionExists
	}
	if len(r.seen) >= r.limit {
		return errStudioExecutionCapacity
	}
	r.seen[job] = studioExecution{owner: owner, state: "active"}
	return nil
}

// Used only after channel selection confirms that no inference was started.
func (r *studioExecutionRegistry) deferUnstarted(job, owner string) {
	r.Lock()
	defer r.Unlock()
	entry, exists := r.seen[job]
	if exists && entry.owner == owner && entry.state == "active" {
		entry.state, entry.expires = "retryable", r.now().Add(r.ttl)
		r.seen[job] = entry
	}
}

func (r *studioExecutionRegistry) settle(job, owner string) {
	r.Lock()
	defer r.Unlock()
	entry, exists := r.seen[job]
	if !exists || entry.owner != owner || entry.state != "active" {
		return
	}
	entry.state, entry.expires = "settled", r.now().Add(r.ttl)
	r.seen[job] = entry
}

func (r *studioExecutionRegistry) reconcile(job, owner string) (string, error) {
	r.Lock()
	defer r.Unlock()
	now := r.now()
	r.cleanup(now)
	if entry, exists := r.seen[job]; exists {
		if entry.owner != owner {
			return "", errStudioExecutionOwner
		}
		// Once the dispatcher has lost certainty, prevent any delayed retry.
		if entry.state == "retryable" {
			entry.state = "fenced"
		}
		if entry.state != "active" {
			entry.expires = now.Add(r.ttl)
			r.seen[job] = entry
		}
		return entry.state, nil
	}
	if len(r.seen) >= r.limit {
		return "", errStudioExecutionCapacity
	}
	// A plain "not found" response races with an execution whose body has not
	// finished arriving. Fence the ID under the same mutex as claim so that a
	// late callback cannot start after the dispatcher releases its queue slot.
	r.seen[job] = studioExecution{owner: owner, state: "fenced", expires: now.Add(r.ttl)}
	return "fenced", nil
}

// ReconcileImageStudioExecution is a private dispatcher operation. It releases
// only logical queue capacity: it does not cancel, retry, or recover a result.
func ReconcileImageStudioExecution(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	fail := func(status int, code string) { c.AbortWithStatusJSON(status, gin.H{"error": code}) }
	secret := os.Getenv("IMAGE_STUDIO_RELAY_TOKEN")
	if len(secret) < 32 || strings.TrimSpace(os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL")) == "" {
		fail(http.StatusServiceUnavailable, "image_studio_channel_disabled")
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.GetHeader("Authorization")), []byte("Bearer "+secret)) != 1 {
		fail(http.StatusUnauthorized, "dispatcher_auth_required")
		return
	}
	owner, job := c.GetHeader("X-Image-Owner"), c.GetHeader("X-Image-Job")
	id, err := strconv.Atoi(strings.TrimPrefix(owner, "cuberouter-"))
	if err != nil || id < 1 || owner != fmt.Sprintf("cuberouter-%d", id) || !studioJobID.MatchString(job) {
		fail(http.StatusBadRequest, "invalid_execution_identity")
		return
	}
	if c.ContentType() != "application/json" || c.Request.URL.RawQuery != "" {
		fail(http.StatusBadRequest, "invalid_request")
		return
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, 1025))
	var body map[string]any
	if err != nil || len(data) > 1024 || common.Unmarshal(data, &body) != nil || body == nil || len(body) != 0 {
		fail(http.StatusBadRequest, "invalid_request")
		return
	}
	state, err := studioExecutions.reconcile(job, owner)
	if err != nil {
		if err == errStudioExecutionOwner {
			fail(http.StatusConflict, "execution_owner_mismatch")
		} else {
			fail(http.StatusServiceUnavailable, "execution_registry_full")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"protocol": studioRecoveryProtocol, "epoch": studioExecutions.epoch,
		"owner": owner, "job_id": job, "state": state,
	})
}
