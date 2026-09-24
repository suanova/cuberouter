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
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStudioUploadValidationAndOwnership(t *testing.T) {
	t.Setenv("MEDIA_STUDIO_S3_ENDPOINT", "https://objects.example")
	t.Setenv("MEDIA_STUDIO_S3_BUCKET", "studio-test")
	t.Setenv("MEDIA_STUDIO_S3_REGION", "us-east-1")
	t.Setenv("MEDIA_STUDIO_S3_ACCESS_KEY", "test-access")
	t.Setenv("MEDIA_STUDIO_S3_SECRET_KEY", "test-secret")
	for _, tc := range []struct {
		name, body    string
		owner, status int
	}{
		{"signed upload", `{"content_type":"image/png","size":42,"owner":999,"key":"other-account"}`, 17, 200},
		{"unauthenticated", `{"content_type":"image/png","size":42}`, 0, 401},
		{"invalid content", `{"content_type":"text/html","size":42}`, 17, 400},
		{"unbounded size", `{"content_type":"image/png","size":999999999}`, 17, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("id", tc.owner)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/media-studio/uploads/presign", bytes.NewBufferString(tc.body))
			MediaStudioUpload(c)
			require.Equal(t, tc.status, recorder.Code)
			assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			if tc.status == 200 {
				var body map[string]interface{}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
				assert.Contains(t, body["upload_url"], "/uploads/17/")
				assert.NotContains(t, recorder.Body.String(), "test-secret")
				assert.NotContains(t, recorder.Body.String(), "other-account")
			}
		})
	}
}
func TestStudioConfigDisablesUploadsWithoutCredentials(t *testing.T) {
	t.Setenv("MEDIA_STUDIO_S3_ENDPOINT", "")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	MediaStudioConfig(c)
	var body struct {
		UploadEnabled bool `json:"upload_enabled"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.False(t, body.UploadEnabled)
	// 编辑能力改由模型元数据的 image-to-image 标签声明（前端读 /api/pricing 的
	// tags），这里不再返回第二份模型名单。
	assert.NotContains(t, recorder.Body.String(), "edit_models")
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}
