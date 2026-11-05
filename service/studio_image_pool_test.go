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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioImageOversizedPoolDeniesAdmissionWithoutUnreservingWorkers(t *testing.T) {
	configured := make([]string, 65)
	for i := range configured {
		configured[i] = strconv.Itoa(i + 1)
	}
	t.Setenv("IMAGE_STUDIO_CHANNEL_IDS", strings.Join(configured, ","))
	reserved := StudioImageChannelIDs()
	require.Len(t, reserved, 65, "ordinary relay must still exclude every configured worker")
	assert.Equal(t, 65, reserved[64])
	// Invalid capacity configuration is rejected before any DB/probe access.
	assert.Empty(t, ReadyStudioImageChannels(context.Background(), "image-studio", "qwen-image-2.1", false))
}

func TestStudioImageWorkerHealthCoalescesAndReusesConnection(t *testing.T) {
	var calls, connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `{"state":"ready","gpu_count":1,"control_protocol":"qwen-execution-v1"}`)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	channel := &model.Channel{Id: 92, BaseURL: &server.URL, Key: strings.Repeat("a", 32)}
	var probes sync.WaitGroup
	for range 20 {
		probes.Go(func() {
			assert.True(t, cachedStudioImageWorkerHealth(context.Background(), channel))
		})
	}
	probes.Wait()
	assert.EqualValues(t, 1, calls.Load())
	assert.True(t, ProbeStudioImageWorker(context.Background(), channel), "leased dispatch rechecks live health")
	assert.EqualValues(t, 2, calls.Load())
	assert.EqualValues(t, 1, connections.Load())
}
