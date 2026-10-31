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
// Package studioqueue owns browser image jobs, independently of relay providers.
// Images live in bounded RAM; an epoch change requires explicit resubmission.
package studioqueue

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const MaxBytes = 256 << 20
const MaxRequestBytes = 16 << 20
const MaxResultBytes = 12 << 20

type Request struct {
	Owner int
	Key   string
	Model string
	Mode  string
	Body  []byte
}

type Status struct {
	ID              string `json:"id"`
	Key             string `json:"key"`
	Epoch           string `json:"epoch"`
	State           string `json:"state"`
	Ahead           int    `json:"ahead"`
	Error           string `json:"error,omitempty"`
	CancelRequested bool   `json:"cancel_requested"`
	RecoveryPending bool   `json:"recovery_pending"`
	CreatedAt       int64  `json:"created_at"`
}

type Result struct {
	PNG   []byte
	Error string
}

// Reservation keeps the physical slot until Run returns, even after UI cancel.
// Run must have a bounded provider timeout and must never retry inference.
type Reservation struct {
	Run     func(context.Context, <-chan struct{}) Result
	Release func()
}

type Reserve func(Request, string) *Reservation

type entry struct {
	request     Request
	status      Status
	fingerprint [32]byte
	created     time.Time
	ended       time.Time
	cancelled   time.Time
	stop        chan struct{}
	active      bool
	result      []byte
}

type Queue struct {
	mu         sync.Mutex
	dispatchMu sync.Mutex
	entries    map[string]*entry
	order      []string
	bytes      int
	epoch      string
	reserve    Reserve
	onState    func(Status)
	wake       chan struct{}
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func New(reserve Reserve, onState ...func(Status)) *Queue {
	q := &Queue{entries: make(map[string]*entry), epoch: randomID(), reserve: reserve, wake: make(chan struct{}, 1)}
	if len(onState) > 0 {
		q.onState = onState[0]
	}
	return q
}

func (q *Queue) Epoch() string { return q.epoch }

func (q *Queue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *Queue) Submit(r Request, epoch string, now time.Time) (Status, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if epoch != q.epoch {
		return Status{}, errors.New("studio_epoch_changed")
	}
	if r.Owner <= 0 || len(r.Key) < 16 || len(r.Key) > 64 || len(r.Body) == 0 || len(r.Body) > MaxRequestBytes {
		return Status{}, errors.New("invalid_studio_request")
	}
	fingerprint := sha256.Sum256(r.Body)
	waiting := 0
	for _, e := range q.entries {
		if e.request.Owner == r.Owner && e.request.Key == r.Key {
			if e.fingerprint != fingerprint {
				return Status{}, errors.New("idempotency_conflict")
			}
			return q.statusLocked(e), nil
		}
	}
	for _, e := range q.entries {
		if e.request.Owner == r.Owner && (e.active || e.status.State == "queued") {
			return Status{}, errors.New("studio_account_busy")
		}
		if e.status.State == "queued" {
			waiting++
		}
	}
	// Reserve result space before starting GPU work: a large output cannot
	// breach the aggregate budget or cause another job's images to be evicted.
	if waiting >= 10 || len(q.entries) >= 1024 || q.bytes+len(r.Body)+MaxResultBytes > MaxBytes {
		return Status{}, errors.New("studio_queue_full")
	}
	r.Body = append([]byte(nil), r.Body...)
	s := Status{ID: randomID(), Key: r.Key, Epoch: q.epoch, State: "queued", CreatedAt: now.UnixMilli()}
	e := &entry{request: r, status: s, fingerprint: fingerprint, created: now, stop: make(chan struct{})}
	q.entries[s.ID] = e
	q.order = append(q.order, s.ID)
	q.bytes += len(r.Body) + MaxResultBytes
	q.signal()
	return q.statusLocked(e), nil
}

func (q *Queue) statusLocked(e *entry) Status {
	s := e.status
	s.RecoveryPending = e.active && s.State == "cancelled"
	if s.State == "queued" {
		for _, id := range q.order {
			if id == s.ID {
				break
			}
			if other := q.entries[id]; other != nil && other.status.State == "queued" {
				s.Ahead++
			}
		}
	}
	return s
}

func (q *Queue) Lookup(owner int, idOrKey string) (Status, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, e := range q.entries {
		if e.request.Owner == owner && (e.status.ID == idOrKey || e.request.Key == idOrKey) {
			return q.statusLocked(e), true
		}
	}
	return Status{}, false
}

func (q *Queue) Session(owner int) []Status {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Status, 0)
	for _, id := range q.order {
		if e := q.entries[id]; e != nil && e.request.Owner == owner {
			out = append(out, q.statusLocked(e))
		}
	}
	return out
}

func (q *Queue) Cancel(owner int, id string, now time.Time) (Status, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.entries[id]
	if e == nil || e.request.Owner != owner {
		return Status{}, false
	}
	if e.status.State == "completed" {
		q.bytes -= len(e.result)
		e.result = nil
		e.status.State = "cancelled"
		e.status.CancelRequested = true
		return q.statusLocked(e), true
	}
	if e.status.State == "queued" || e.active {
		if !e.status.CancelRequested {
			e.status.CancelRequested = true
			e.cancelled = now
			close(e.stop)
		}
		if !e.active {
			e.status.State = "cancelled"
			e.ended = now
			q.bytes -= len(e.request.Body) + MaxResultBytes
			e.request.Body = nil
		} else if e.status.State != "cancelled" {
			e.status.State = "cancelling"
		}
	}
	return q.statusLocked(e), true
}

func (q *Queue) Result(owner int, id string) ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.entries[id]
	if e == nil || e.request.Owner != owner || e.status.State != "completed" {
		return nil, false
	}
	return append([]byte(nil), e.result...), true
}

func (q *Queue) Ack(owner int, id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.entries[id]
	if e == nil || e.request.Owner != owner || e.active || e.status.State == "queued" {
		return false
	}
	q.bytes -= len(e.result)
	e.result = nil
	e.status.State = "released"
	return true
}

func (q *Queue) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		case <-ticker.C:
		}
		q.Tick(ctx, time.Now())
	}
}

// Tick is also used with an explicit clock by lifecycle regression tests.
func (q *Queue) Tick(ctx context.Context, now time.Time) {
	q.dispatchMu.Lock()
	defer q.dispatchMu.Unlock()
	q.mu.Lock()
	var queued []*entry
	var transitions []Status
	kept := q.order[:0]
	for _, id := range q.order {
		e := q.entries[id]
		if e.active && e.status.State == "cancelling" && !e.cancelled.IsZero() && now.Sub(e.cancelled) >= 30*time.Second {
			e.status.State = "cancelled"
			transitions = append(transitions, q.statusLocked(e))
		}
		if e.status.State == "queued" && now.Sub(e.created) >= 15*time.Minute {
			e.status.State, e.ended = "expired", now
			q.bytes -= len(e.request.Body) + MaxResultBytes
			e.request.Body = nil
			transitions = append(transitions, q.statusLocked(e))
		}
		if !e.active && !e.ended.IsZero() && now.Sub(e.ended) >= 10*time.Minute {
			q.bytes -= len(e.result)
			delete(q.entries, id)
			continue
		}
		kept = append(kept, id)
		if e.status.State == "queued" {
			queued = append(queued, e)
		}
	}
	q.order = kept
	q.mu.Unlock()
	if q.onState != nil {
		for _, status := range transitions {
			q.onState(status)
		}
	}
	for _, e := range queued {
		q.mu.Lock()
		if e.status.State != "queued" {
			q.mu.Unlock()
			continue
		}
		r := e.request
		q.mu.Unlock()
		reservation := q.reserve(r, e.status.ID)
		if reservation == nil {
			continue
		}
		q.mu.Lock()
		if e.status.State != "queued" {
			q.mu.Unlock()
			reservation.Release()
			continue
		}
		e.active, e.status.State = true, "running"
		q.mu.Unlock()
		go q.execute(ctx, e, reservation)
	}
}

func (q *Queue) execute(ctx context.Context, e *entry, reservation *Reservation) {
	defer reservation.Release()
	execCtx, stop := context.WithTimeout(ctx, 11*time.Minute)
	defer stop()
	result := Result{Error: "studio_generation_failed"}
	func() {
		defer func() { _ = recover() }()
		result = reservation.Run(execCtx, e.stop)
	}()
	q.mu.Lock()
	defer func() {
		status := q.statusLocked(e)
		q.mu.Unlock()
		if q.onState != nil {
			q.onState(status)
		}
	}()
	q.bytes -= len(e.request.Body) + MaxResultBytes
	e.request.Body, e.active, e.ended = nil, false, time.Now()
	if e.status.CancelRequested {
		e.status.State = "cancelled"
		return
	}
	if len(result.PNG) == 0 || len(result.PNG) > MaxResultBytes || result.Error != "" {
		e.status.State, e.status.Error = "failed", result.Error
		if e.status.Error == "" {
			e.status.Error = "studio_generation_failed"
		}
		return
	}
	e.status.State, e.result = "completed", result.PNG
	q.bytes += len(result.PNG)
	q.signal()
}
