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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioReviewerHealthFailuresAreCounted(t *testing.T) {
	setupModelListControllerTestDB(t)
	channel := &model.Channel{Id: 1, Type: constant.ChannelTypeStudioReview, Status: common.ChannelStatusEnabled}
	result := testChannel(context.Background(), channel, 1, "", "", false)
	require.NotNil(t, result.context, "automatic health transitions need a context for the channel key")
	summary := testChannelForHealthCheck(context.Background(), channel, 1, false, 100000)
	assert.Equal(t, 1, summary.Tested)
	assert.Equal(t, 1, summary.Failed)
	assert.Zero(t, summary.Succeeded)
	assert.Zero(t, summary.Disabled)
}

func TestStudioReviewerValidationRejectsEmptyStoredServiceKey(t *testing.T) {
	base := "http://127.0.0.1:19001/moderate"
	err := validateChannel(&model.Channel{Type: constant.ChannelTypeStudioReview, BaseURL: &base}, false)
	require.ErrorContains(t, err, "one service key")
}

func TestStudioReviewerValidationChecksCredentialWhitespace(t *testing.T) {
	base := "http://127.0.0.1:19001/moderate"
	for _, tc := range []struct {
		name  string
		key   string
		valid bool
	}{
		{"spaces provide no credential", strings.Repeat(" ", 32), false},
		{"tabs provide no credential", strings.Repeat("\t", 32), false},
		{"padding cannot extend a short credential", strings.Repeat("s", 31) + " ", false},
		{"embedded space is not a single credential", strings.Repeat("s", 16) + " " + strings.Repeat("s", 16), false},
		{"embedded tab is not a single credential", strings.Repeat("s", 16) + "\t" + strings.Repeat("s", 16), false},
		{"embedded Unicode whitespace is not a single credential", strings.Repeat("s", 16) + "\u00a0" + strings.Repeat("s", 16), false},
		{"minimum length credential with trailing space remains valid", strings.Repeat("s", 32) + " ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channel := &model.Channel{Type: constant.ChannelTypeStudioReview, Key: tc.key, BaseURL: &base}
			if tc.valid {
				require.NoError(t, validateChannel(channel, true))
				require.NoError(t, validateChannel(channel, false))
				assert.Equal(t, tc.key, channel.Key, "validation preserves the accepted service key")
				return
			}
			require.ErrorContains(t, validateChannel(channel, true), "one service key")
			require.ErrorContains(t, validateChannel(channel, false), "one service key")
		})
	}
}

func TestStudioReviewerValidationRejectsInvalidHeaderBytes(t *testing.T) {
	base := "http://127.0.0.1:19001/moderate"
	for _, tc := range []struct {
		name string
		key  string
	}{
		{"trailing newline", strings.Repeat("s", 32) + "\n"},
		{"embedded carriage return", strings.Repeat("s", 16) + "\r" + strings.Repeat("s", 16)},
		{"embedded null", strings.Repeat("s", 16) + "\x00" + strings.Repeat("s", 16)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channel := &model.Channel{Type: constant.ChannelTypeStudioReview, Key: tc.key, BaseURL: &base}
			require.Error(t, validateChannel(channel, true), "creation must reject an unusable Authorization header")
			require.Error(t, validateChannel(channel, false), "editing must reject an unusable Authorization header")
		})
	}
}

func TestStudioReviewerSaveRejectsFinalMultiKeyState(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	base := "http://127.0.0.1:19001/moderate"
	channel := &model.Channel{Type: constant.ChannelTypeStudioReview, Name: "reviewer", Key: strings.Repeat("s", 32), BaseURL: &base}
	t.Run("add mode sets multi key after initial validation", func(t *testing.T) {
		payload, err := common.Marshal(AddChannelRequest{Mode: "multi_to_single", Channel: channel})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/", bytes.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		AddChannel(c)
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
		assert.False(t, result.Success)
		assert.Contains(t, result.Message, "one service key")
		var count int64
		require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
		assert.Zero(t, count)
	})
	t.Run("edit restores stored multi key state", func(t *testing.T) {
		stored := *channel
		stored.Id = 8
		stored.ChannelInfo.IsMultiKey = true
		require.NoError(t, db.Create(&stored).Error)
		payload, err := common.Marshal(map[string]any{"id": 8, "type": constant.ChannelTypeStudioReview, "base_url": base, "name": "edited"})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("id", 1)
		c.Set("role", common.RoleRootUser)
		UpdateChannel(c)
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
		assert.False(t, result.Success)
		assert.Contains(t, result.Message, "one service key")
		var unchanged model.Channel
		require.NoError(t, db.First(&unchanged, 8).Error)
		assert.Equal(t, "reviewer", unchanged.Name)
	})
}

func TestStudioReviewerPartialUpdateValidatesStoredTypeAndConnection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		patch       map[string]any
		wantSuccess bool
	}{
		{"name only preserves valid connection", map[string]any{"name": "renamed"}, true},
		{"short key without type", map[string]any{"key": "short"}, false},
		{"newline key without type", map[string]any{"key": strings.Repeat("s", 32) + "\n"}, false},
		{"whitespace key without type", map[string]any{"key": strings.Repeat(" ", 32)}, false},
		{"embedded whitespace key without type", map[string]any{"key": strings.Repeat("s", 16) + " " + strings.Repeat("s", 16)}, false},
		{"invalid CA without type", map[string]any{"other": "invalid certificate"}, false},
		{"valid key without base URL", map[string]any{"key": strings.Repeat("n", 32)}, true},
		{"valid key retains trailing padding", map[string]any{"key": strings.Repeat("n", 32) + " "}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupModelListControllerTestDB(t)
			base := "http://127.0.0.1:19001/moderate"
			stored := &model.Channel{Id: 8, Type: constant.ChannelTypeStudioReview, Name: "reviewer", Key: strings.Repeat("s", 32), BaseURL: &base}
			require.NoError(t, db.Create(stored).Error)
			tc.patch["id"] = stored.Id
			payload, err := common.Marshal(tc.patch)
			require.NoError(t, err)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(payload))
			c.Set("id", 1)
			c.Set("role", common.RoleRootUser)
			UpdateChannel(c)
			var result struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
			assert.Equal(t, tc.wantSuccess, result.Success)
			var after model.Channel
			require.NoError(t, db.First(&after, stored.Id).Error)
			assert.Equal(t, constant.ChannelTypeStudioReview, after.Type)
			assert.Equal(t, base, after.GetBaseURL())
			if name, ok := tc.patch["name"].(string); ok && tc.wantSuccess {
				assert.Equal(t, name, after.Name)
			}
			if key, ok := tc.patch["key"].(string); ok && tc.wantSuccess {
				assert.Equal(t, key, after.Key)
			}
			if !tc.wantSuccess {
				assert.Equal(t, stored.Key, after.Key)
				assert.Empty(t, after.Other)
			}
		})
	}
}

func TestStudioReviewerBatchValidatesEachKeyBeforeInserting(t *testing.T) {
	for _, tc := range []struct {
		name      string
		keys      string
		wantKeys  []string
		wantNames []string
	}{
		{"later short key rejects the entire batch", strings.Repeat("v", 32) + "\n" + strings.Repeat("s", 31), nil, nil},
		{"two minimum-length keys are saved independently", strings.Repeat("v", 32) + "\n" + strings.Repeat("s", 32), []string{strings.Repeat("v", 32), strings.Repeat("s", 32)}, nil},
		{"prefix names are independent for each key", strings.Repeat("v", 32) + "\n" + strings.Repeat("s", 32), []string{strings.Repeat("v", 32), strings.Repeat("s", 32)}, []string{"reviewer vvvvvvvv", "reviewer ssssssss"}},
		{"later invalid header key rejects the entire batch", strings.Repeat("v", 32) + "\n" + strings.Repeat("s", 32) + "\r", nil, nil},
		{"later whitespace key rejects the entire batch", strings.Repeat("v", 32) + "\n" + strings.Repeat(" ", 32), nil, nil},
		{"later embedded whitespace key rejects the entire batch", strings.Repeat("v", 32) + "\n" + strings.Repeat("s", 16) + "\t" + strings.Repeat("s", 16), nil, nil},
		{"empty batch is rejected", "\n", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupModelListControllerTestDB(t)
			base := "http://127.0.0.1:19001/moderate"
			channel := &model.Channel{Type: constant.ChannelTypeStudioReview, Name: "reviewer", Key: tc.keys, BaseURL: &base}
			payload, err := common.Marshal(AddChannelRequest{Mode: "batch", Channel: channel, BatchAddSetKeyPrefix2Name: tc.wantNames != nil})
			require.NoError(t, err)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/", bytes.NewReader(payload))
			c.Request.Header.Set("Content-Type", "application/json")
			AddChannel(c)
			var result struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
			assert.Equal(t, len(tc.wantKeys) > 0, result.Success)
			var count int64
			require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
			assert.Equal(t, int64(len(tc.wantKeys)), count, "batch validation runs before any reviewer is inserted")
			if len(tc.wantKeys) > 0 {
				var storedKeys []string
				require.NoError(t, db.Model(&model.Channel{}).Order("id").Pluck("key", &storedKeys).Error)
				assert.Equal(t, tc.wantKeys, storedKeys, "each row retains its corresponding input key")
			}
			if tc.wantNames != nil {
				var storedNames []string
				require.NoError(t, db.Model(&model.Channel{}).Order("id").Pluck("name", &storedNames).Error)
				assert.Equal(t, tc.wantNames, storedNames, "each name uses the original base name and only its own key prefix")
			}
		})
	}
}
