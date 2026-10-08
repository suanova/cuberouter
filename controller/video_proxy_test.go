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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCapabilityVideoProxyContext 构造一个只持 capability 签名的 content 请求。
// 生产链路上该上下文键由 TokenOrVideoContentAccessAuth 验签成功后写入；这里直接
// 置位，把 VideoProxy 当成已通过能力鉴权的 handler 来测。
func newCapabilityVideoProxyContext(taskID string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.TaskArtifactAccessContextKey, true)
	c.Params = gin.Params{{Key: "task_id", Value: taskID}}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+taskID+"/content", nil)
	return c, recorder
}

// pointTaskAtCubeStackUpstream 把任务所属渠道改成 CubeStack 并指向给定的上游。
func pointTaskAtCubeStackUpstream(t *testing.T, task *model.Task, baseURL, key string) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", task.ChannelId).Updates(map[string]any{
		"type":     constant.ChannelTypeCubeStack,
		"key":      key,
		"base_url": baseURL,
	}).Error)
}

// disableChannelMemoryCache 让 CacheGetChannel 直接读库，避免测试之间共用渠道缓存。
func disableChannelMemoryCache(t *testing.T) {
	t.Helper()
	previous := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previous })
}

func TestVideoProxyCapabilityFetchesCubeStackUpstreamBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	task := setupGenericTaskTest(t)
	allowPrivateTaskMediaTest(t)
	disableChannelMemoryCache(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 回源走「当前渠道 base URL + upstream task id」，不带 capability 参数。
		assert.Equal(t, "/v1/videos/upstream-video-1/content", r.URL.Path)
		assert.Equal(t, "Bearer cube-stack-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("cube-stack-mp4-bytes"))
	}))
	defer upstream.Close()

	pointTaskAtCubeStackUpstream(t, task, upstream.URL, "cube-stack-key")
	task.PrivateData.UpstreamTaskID = "upstream-video-1"
	// 落库的 ResultURL 仍是集群内地址：capability 路径不该读它。
	task.PrivateData.ResultURL = "http://minimax-h3-sglang.default.svc:30000/v1/videos/upstream-video-1/content"
	require.NoError(t, model.DB.Save(task).Error)

	c, recorder := newCapabilityVideoProxyContext(task.TaskID)
	VideoProxy(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "cube-stack-mp4-bytes", recorder.Body.String())
	assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.NotContains(t, recorder.Body.String(), ".svc:30000")
}

func TestVideoProxyCapabilityOmitsAuthorizationWhenChannelKeyEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	task := setupGenericTaskTest(t)
	allowPrivateTaskMediaTest(t)
	disableChannelMemoryCache(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SGLang 本地服务通常无鉴权：渠道 key 为空时不得发送空 Bearer。
		assert.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("mp4"))
	}))
	defer upstream.Close()

	pointTaskAtCubeStackUpstream(t, task, upstream.URL, "")
	require.NoError(t, model.DB.Save(task).Error)

	c, recorder := newCapabilityVideoProxyContext(task.TaskID)
	VideoProxy(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "mp4", recorder.Body.String())
}

func TestVideoProxyCapabilityHidesTaskExistence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	task := setupGenericTaskTest(t)
	pendingTask := &model.Task{
		TaskID: "task_pending", Platform: "64", UserId: task.UserId, ChannelId: task.ChannelId,
		Status: model.TaskStatusQueued,
	}
	model.NormalizeTaskBillingScope(pendingTask)
	require.NoError(t, model.DB.Create(pendingTask).Error)

	missingContext, missingRecorder := newCapabilityVideoProxyContext("task_absent")
	VideoProxy(missingContext)
	pendingContext, pendingRecorder := newCapabilityVideoProxyContext("task_pending")
	VideoProxy(pendingContext)

	require.Equal(t, http.StatusNotFound, missingRecorder.Code)
	require.Equal(t, http.StatusNotFound, pendingRecorder.Code)
	// 「任务不存在」与「任务存在但未完成」必须不可区分：响应体与缓存头全一致。
	assert.Equal(t, missingRecorder.Body.String(), pendingRecorder.Body.String())
	assert.Equal(t, missingRecorder.Header().Get("Cache-Control"), pendingRecorder.Header().Get("Cache-Control"))
	assert.Contains(t, missingRecorder.Body.String(), "artifact_not_found")
	// 未完成的旧行为（400 + 具体状态）不得泄露给 capability 调用方。
	assert.NotContains(t, pendingRecorder.Body.String(), string(model.TaskStatusQueued))
}

func TestVideoProxyWithoutCapabilityReportsTaskStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	task := setupGenericTaskTest(t)
	pendingTask := &model.Task{
		TaskID: "task_relay_pending", Platform: "64", UserId: task.UserId, ChannelId: task.ChannelId,
		Status: model.TaskStatusQueued,
	}
	// relay 身份走作用域查询，作用域列必须落库（与 InitTask 一致）。
	model.NormalizeTaskBillingScope(pendingTask)
	require.NoError(t, model.DB.Create(pendingTask).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	authenticateControllerTestUser(c, task.UserId)
	c.Params = gin.Params{{Key: "task_id", Value: "task_relay_pending"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_relay_pending/content", nil)

	VideoProxy(c)

	// 持 relay 身份的调用方保留既有行为：400 且带上当前状态。
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), string(model.TaskStatusQueued))
	assert.NotContains(t, recorder.Body.String(), "artifact_not_found")
}
