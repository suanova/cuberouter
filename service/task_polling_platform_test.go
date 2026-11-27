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
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedPollingTaskOnPlatform(t *testing.T, channelID int, platform constant.TaskPlatform, publicID, upstreamID string) *model.Task {
	t.Helper()
	now := time.Now().Unix()
	task := &model.Task{
		TaskID:    publicID,
		Platform:  platform,
		UserId:    1,
		ChannelId: channelID,
		Action:    constant.TaskActionImageToVideo,
		Status:    model.TaskStatusInProgress,
		Progress:  "30%",
		CreatedAt: now,
		UpdatedAt: now,
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: upstreamID,
		},
	}
	require.NoError(t, model.DB.Create(task).Error)
	return task
}

// platformBarrierPollingAdaptor holds every fetch until the test releases it,
// making it observable whether two platforms started polling concurrently.
type platformBarrierPollingAdaptor struct {
	taskPollingFetchAdaptor
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (a *platformBarrierPollingAdaptor) FetchTask(baseURL, key string, task *model.Task, proxy string) (*http.Response, error) {
	if a.calls.Add(1) <= 2 {
		a.started <- struct{}{}
	}
	<-a.release
	return a.taskPollingFetchAdaptor.FetchTask(baseURL, key, task, proxy)
}

func TestRunTaskPollingOncePollsPlatformsConcurrently(t *testing.T) {
	truncate(t)

	const channelA, channelB = 211, 212
	seedTaskPollingChannel(t, channelA, true)
	seedTaskPollingChannel(t, channelB, true)
	seedPollingTaskOnPlatform(t, channelA, "platform-a", "task_a_1", "up_a_1")
	seedPollingTaskOnPlatform(t, channelB, "platform-b", "task_b_1", "up_b_1")

	previousLimit := constant.TaskQueryLimit
	constant.TaskQueryLimit = 10
	t.Cleanup(func() { constant.TaskQueryLimit = previousLimit })
	previousTimeout := constant.TaskTimeoutMinutes
	constant.TaskTimeoutMinutes = 0
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previousTimeout })

	adaptor := &platformBarrierPollingAdaptor{
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(adaptor.release) }) }
	t.Cleanup(release)
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	done := make(chan struct{})
	go func() {
		RunTaskPollingOnce(context.Background(), nil)
		close(done)
	}()

	bothStarted := true
waitForBothPlatforms:
	for i := 0; i < 2; i++ {
		select {
		case <-adaptor.started:
		case <-time.After(500 * time.Millisecond):
			bothStarted = false
			break waitForBothPlatforms
		}
	}
	release()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("polling pass did not finish")
	}

	require.True(t, bothStarted, "both platforms must start polling before either finishes")
}

func TestRunTaskPollingOnceSerializesProgressReports(t *testing.T) {
	truncate(t)

	const channelA, channelB = 221, 222
	seedTaskPollingChannel(t, channelA, true)
	seedTaskPollingChannel(t, channelB, true)
	seedPollingTaskOnPlatform(t, channelA, "platform-a", "task_a_1", "up_a_1")
	seedPollingTaskOnPlatform(t, channelB, "platform-b", "task_b_1", "up_b_1")

	previousLimit := constant.TaskQueryLimit
	constant.TaskQueryLimit = 10
	t.Cleanup(func() { constant.TaskQueryLimit = previousLimit })
	previousTimeout := constant.TaskTimeoutMinutes
	constant.TaskTimeoutMinutes = 0
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previousTimeout })

	adaptor := &platformBarrierPollingAdaptor{
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(adaptor.release) }) }
	t.Cleanup(release)
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	var active atomic.Int32
	var overlapped atomic.Bool
	var reportsMu sync.Mutex
	reports := make([]int, 0, 4)
	report := func(processed, total int) {
		if active.Add(1) > 1 {
			overlapped.Store(true)
		}
		time.Sleep(20 * time.Millisecond)
		reportsMu.Lock()
		reports = append(reports, processed)
		reportsMu.Unlock()
		active.Add(-1)
	}

	done := make(chan struct{})
	go func() {
		RunTaskPollingOnce(context.Background(), report)
		close(done)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-adaptor.started:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("platforms did not start")
		}
	}
	release()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("polling pass did not finish")
	}

	require.False(t, overlapped.Load(), "report callback must never run concurrently")
	assert.Equal(t, []int{0, 1, 2, 2}, reports)
}

// A platform whose unfinished-task count exceeds TaskQueryLimit must not hide
// another platform's newer tasks from the polling pass.
func TestRunTaskPollingOnceQueriesTasksPerPlatform(t *testing.T) {
	truncate(t)

	const channelA, channelB = 201, 202
	seedTaskPollingChannel(t, channelA, true)
	seedTaskPollingChannel(t, channelB, true)

	previousLimit := constant.TaskQueryLimit
	constant.TaskQueryLimit = 2
	t.Cleanup(func() { constant.TaskQueryLimit = previousLimit })
	previousTimeout := constant.TaskTimeoutMinutes
	constant.TaskTimeoutMinutes = 0
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previousTimeout })

	for i := 1; i <= 3; i++ {
		seedPollingTaskOnPlatform(t, channelA, "platform-a", fmt.Sprintf("task_a_%d", i), fmt.Sprintf("up_a_%d", i))
	}
	platformBTask := seedPollingTaskOnPlatform(t, channelB, "platform-b", "task_b_1", "up_b_1")

	adaptor := &taskPollingFetchAdaptor{}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	summary := RunTaskPollingOnce(context.Background(), nil)

	assert.Equal(t, 3, summary.UnfinishedTasks)
	assert.Equal(t, 2, summary.PlatformsScanned)
	assert.Len(t, adaptor.fetchedTaskIDs(), 3)
	assert.Contains(t, adaptor.fetchedTaskIDs(), platformBTask.GetUpstreamTaskID())
}
