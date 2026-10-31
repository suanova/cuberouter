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
package studioqueue

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioQueueIdempotencyAndOwnership(t *testing.T) {
	q := New(func(Request, string) *Reservation { return nil })
	r := Request{Owner: 1, Key: strings.Repeat("a", 32), Body: []byte(`{"prompt":"cat"}`)}
	now := time.Unix(100, 0)
	s, err := q.Submit(r, q.Epoch(), now)
	require.NoError(t, err)
	again, err := q.Submit(r, q.Epoch(), now)
	require.NoError(t, err)
	assert.Equal(t, s.ID, again.ID)
	r.Body = []byte(`{"prompt":"dog"}`)
	_, err = q.Submit(r, q.Epoch(), now)
	require.EqualError(t, err, "idempotency_conflict")
	_, found := q.Lookup(2, s.ID)
	assert.False(t, found)
	_, found = q.Cancel(2, s.ID, now)
	assert.False(t, found)
	_, found = q.Result(2, s.ID)
	assert.False(t, found)
	assert.False(t, q.Ack(2, s.ID))
	_, err = q.Submit(Request{Owner: 2, Key: r.Key, Body: r.Body}, "previous-boot", now)
	require.EqualError(t, err, "studio_epoch_changed")
}

func TestStudioQueueCancellationKeepsSlotAndDiscardsLateOutput(t *testing.T) {
	started, finish, released := make(chan struct{}), make(chan struct{}), make(chan struct{})
	q := New(func(Request, string) *Reservation {
		return &Reservation{
			Run: func(context.Context, <-chan struct{}) Result {
				close(started)
				<-finish
				return Result{PNG: []byte("late image")}
			},
			Release: func() { close(released) },
		}
	})
	now := time.Unix(100, 0)
	r := Request{Owner: 1, Key: strings.Repeat("a", 32), Body: []byte("request")}
	s, err := q.Submit(r, q.Epoch(), now)
	require.NoError(t, err)
	q.Tick(context.Background(), now)
	<-started
	status, found := q.Cancel(1, s.ID, now)
	require.True(t, found)
	assert.Equal(t, "cancelling", status.State)
	q.Tick(context.Background(), now.Add(31*time.Second))
	status, found = q.Lookup(1, s.ID)
	require.True(t, found)
	assert.Equal(t, "cancelled", status.State)
	assert.True(t, status.RecoveryPending)
	assert.Equal(t, len(r.Body)+MaxResultBytes, q.bytes, "in-flight bytes must remain charged")
	r.Key = strings.Repeat("b", 32)
	_, err = q.Submit(r, q.Epoch(), now)
	require.EqualError(t, err, "studio_account_busy")
	close(finish)
	<-released
	_, found = q.Result(1, s.ID)
	assert.False(t, found)
	assert.Zero(t, q.bytes)
	status, found = q.Lookup(1, s.ID)
	require.True(t, found)
	assert.False(t, status.RecoveryPending)
}

func TestStudioQueueWaitingExpiryAndAggregateBudget(t *testing.T) {
	q := New(func(Request, string) *Reservation { return nil })
	now := time.Unix(100, 0)
	var first Status
	for i := 1; i <= 11; i++ {
		s, err := q.Submit(Request{Owner: i, Key: strings.Repeat("a", 32), Body: []byte("input")}, q.Epoch(), now)
		if i == 11 {
			require.EqualError(t, err, "studio_queue_full")
			break
		}
		require.NoError(t, err)
		assert.Equal(t, i-1, s.Ahead)
		if i == 1 {
			first = s
		}
	}
	q.Tick(context.Background(), now.Add(15*time.Minute))
	s, found := q.Lookup(1, first.ID)
	require.True(t, found)
	assert.Equal(t, "expired", s.State)
	assert.Zero(t, q.bytes)
	q.Tick(context.Background(), now.Add(26*time.Minute))
	_, found = q.Lookup(1, first.ID)
	assert.False(t, found)
	large := []byte(strings.Repeat("a", MaxRequestBytes))
	for owner := 1; owner <= 10; owner++ {
		_, err := q.Submit(Request{Owner: owner, Key: strings.Repeat("b", 32), Body: large}, q.Epoch(), now)
		if owner == 10 {
			require.EqualError(t, err, "studio_queue_full")
		} else {
			require.NoError(t, err)
		}
	}
}

func TestStudioQueueAcknowledgementReleasesResultAndPreservesKey(t *testing.T) {
	released := make(chan struct{})
	q := New(func(Request, string) *Reservation {
		return &Reservation{
			Run: func(context.Context, <-chan struct{}) Result { return Result{PNG: []byte("reviewed PNG")} }, Release: func() { close(released) },
		}
	})
	r := Request{Owner: 1, Key: strings.Repeat("a", 32), Body: []byte("input")}
	s, err := q.Submit(r, q.Epoch(), time.Now())
	require.NoError(t, err)
	q.Tick(context.Background(), time.Now())
	<-released
	image, found := q.Result(1, s.ID)
	require.True(t, found)
	assert.Equal(t, []byte("reviewed PNG"), image)
	require.True(t, q.Ack(1, s.ID))
	assert.Zero(t, q.bytes)
	again, err := q.Submit(r, q.Epoch(), time.Now())
	require.NoError(t, err)
	assert.Equal(t, s.ID, again.ID)
	assert.Equal(t, "released", again.State)
}

func TestStudioQueuePublishesFailureForAuditBeforeReleasingCapacity(t *testing.T) {
	released := make(chan struct{})
	var observed Status
	q := New(func(Request, string) *Reservation {
		return &Reservation{
			Run:     func(context.Context, <-chan struct{}) Result { return Result{Error: "studio_access_denied"} },
			Release: func() { close(released) },
		}
	}, func(status Status) { observed = status })
	r := Request{Owner: 1, Key: strings.Repeat("a", 32), Body: []byte("input")}
	s, err := q.Submit(r, q.Epoch(), time.Now())
	require.NoError(t, err)
	q.Tick(context.Background(), time.Now())
	<-released
	assert.Equal(t, s.ID, observed.ID)
	assert.Equal(t, "failed", observed.State)
	assert.Equal(t, "studio_access_denied", observed.Error)
}
