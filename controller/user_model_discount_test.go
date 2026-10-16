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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// performUserModelDiscountsRequest 直调 handler：补齐路由层提供的 :id 参数与
// 鉴权中间件写入的操作者身份，让测试只覆盖 handler 自身的行为。
func performUserModelDiscountsRequest(t *testing.T, handler gin.HandlerFunc, method, userId, body string, operatorId, role int) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, "/api/user/"+userId+"/model-discounts", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: userId}}
	c.Set("id", operatorId)
	c.Set("role", role)
	c.Set("username", "operator")
	handler(c)
	return recorder
}

// storedUserModelDiscounts 读回列原值（与 handler 相同的结构体扫描，NULL 列安全）。
func storedUserModelDiscounts(t *testing.T, db *gorm.DB, userId int) string {
	t.Helper()
	var row model.User
	require.NoError(t, db.Select("id", "model_discounts").Where("id = ?", userId).First(&row).Error)
	return row.ModelDiscounts
}

type userModelDiscountsBody struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Discounts map[string]float64 `json:"discounts"`
	} `json:"data"`
}

func TestUpdateUserModelDiscountsRoundTrip(t *testing.T) {
	db := setupManageUserTestDB(t)
	target := model.User{
		Username: "discount-roundtrip", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "discount-roundtrip-aff",
	}
	require.NoError(t, db.Create(&target).Error)
	id := strconv.Itoa(target.Id)

	recorder := performUserModelDiscountsRequest(t, UpdateUserModelDiscounts, http.MethodPut, id, `{"discounts":{"m":0.8}}`, 9999, common.RoleRootUser)
	var body userModelDiscountsBody
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success, recorder.Body.String())
	assert.Equal(t, `{"m":0.8}`, storedUserModelDiscounts(t, db, target.Id))

	// 全量替换：第二次 PUT 覆盖而不是合并上一次的表。
	recorder = performUserModelDiscountsRequest(t, UpdateUserModelDiscounts, http.MethodPut, id, `{"discounts":{"x":0.5,"y":0.9}}`, 9999, common.RoleRootUser)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success, recorder.Body.String())
	assert.Equal(t, `{"x":0.5,"y":0.9}`, storedUserModelDiscounts(t, db, target.Id))

	var auditLogs []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).Order("id asc").Find(&auditLogs).Error)
	require.Len(t, auditLogs, 2)
	for _, auditLog := range auditLogs {
		assert.EqualValues(t, 9999, auditLog.UserId, "audit log belongs to the operator")
		assert.Contains(t, auditLog.Other, `"action":"user.model_discounts"`)
		assert.Contains(t, auditLog.Other, fmt.Sprintf(`"target_user_id":%d`, target.Id))
	}
	assert.Contains(t, auditLogs[0].Other, `"before":null`)
	assert.Contains(t, auditLogs[0].Other, `"after":{"m":0.8}`)
	assert.Contains(t, auditLogs[1].Other, `"before":{"m":0.8}`)
	assert.Contains(t, auditLogs[1].Other, `"after":{"x":0.5,"y":0.9}`)
}

func TestUpdateUserModelDiscountsRejectsInvalid(t *testing.T) {
	db := setupManageUserTestDB(t)
	target := model.User{
		Username: "discount-invalid", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "discount-invalid-aff",
	}
	require.NoError(t, db.Create(&target).Error)
	id := strconv.Itoa(target.Id)

	tooMany := make(map[string]float64, model.MaxUserModelDiscountEntries+1)
	for i := 0; i <= model.MaxUserModelDiscountEntries; i++ {
		tooMany[fmt.Sprintf("model-%d", i)] = 0.5
	}
	tooManyPayload, err := common.Marshal(map[string]any{"discounts": tooMany})
	require.NoError(t, err)

	// 500 条、名字合法但每条 ~260 字节 → 序列化超过 64KiB，触发体积上限。
	tooLarge := make(map[string]float64, model.MaxUserModelDiscountEntries)
	longName := strings.Repeat("a", model.MaxUserModelDiscountNameLen-8)
	for i := 0; i < model.MaxUserModelDiscountEntries; i++ {
		tooLarge[fmt.Sprintf("%s-%d", longName, i)] = 0.5
	}
	tooLargePayload, err := common.Marshal(map[string]any{"discounts": tooLarge})
	require.NoError(t, err)

	cases := []struct {
		name string
		body string
		key  string
	}{
		{name: "zero discount", body: `{"discounts":{"m":0}}`, key: i18n.MsgModelDiscountOutOfRange},
		{name: "discount above one", body: `{"discounts":{"m":1.5}}`, key: i18n.MsgModelDiscountOutOfRange},
		{name: "empty model name", body: `{"discounts":{"":0.5}}`, key: i18n.MsgModelDiscountNameInvalid},
		{name: "too many entries", body: string(tooManyPayload), key: i18n.MsgModelDiscountTooMany},
		{name: "oversized payload", body: string(tooLargePayload), key: i18n.MsgModelDiscountTooLarge},
		{name: "malformed body", body: `{"discounts":`, key: i18n.MsgInvalidParams},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performUserModelDiscountsRequest(t, UpdateUserModelDiscounts, http.MethodPut, id, tc.body, 9999, common.RoleRootUser)
			var body userModelDiscountsBody
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
			assert.False(t, body.Success)
			// 与 handler 的 ApiErrorI18n 比对同一渲染路径，bundle 是否初始化都成立。
			assert.Equal(t, common.TranslateMessage(expectCtx(), tc.key), body.Message)
			assert.Equal(t, "", storedUserModelDiscounts(t, db, target.Id), "rejected write must not touch the column")
		})
	}
}

func TestUpdateUserModelDiscountsRoleGuard(t *testing.T) {
	db := setupManageUserTestDB(t)
	target := model.User{
		Username: "discount-role-guard", Password: "password", Role: common.RoleRootUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "discount-role-guard-aff",
	}
	require.NoError(t, db.Create(&target).Error)

	recorder := performUserModelDiscountsRequest(t, UpdateUserModelDiscounts, http.MethodPut, strconv.Itoa(target.Id), `{"discounts":{"m":0.8}}`, 8888, common.RoleAdminUser)
	var body userModelDiscountsBody
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, common.TranslateMessage(expectCtx(), i18n.MsgUserNoPermissionHigherLevel), body.Message)
	assert.Equal(t, "", storedUserModelDiscounts(t, db, target.Id))
}

func TestGetUserModelDiscountsReturnsStoredMap(t *testing.T) {
	db := setupManageUserTestDB(t)

	t.Run("returns the stored map", func(t *testing.T) {
		target := model.User{
			Username: "discount-get", Password: "password", Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, Group: "default", AffCode: "discount-get-aff",
		}
		require.NoError(t, db.Create(&target).Error)
		require.NoError(t, db.Model(&model.User{}).Where("id = ?", target.Id).Update("model_discounts", `{"m":0.75,"n":0.5}`).Error)

		recorder := performUserModelDiscountsRequest(t, GetUserModelDiscounts, http.MethodGet, strconv.Itoa(target.Id), "", 9999, common.RoleRootUser)
		var body userModelDiscountsBody
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		assert.True(t, body.Success, recorder.Body.String())
		assert.Equal(t, map[string]float64{"m": 0.75, "n": 0.5}, body.Data.Discounts)
	})

	t.Run("no discounts renders an empty object", func(t *testing.T) {
		target := model.User{
			Username: "discount-get-empty", Password: "password", Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, Group: "default", AffCode: "discount-get-empty-aff",
		}
		require.NoError(t, db.Create(&target).Error)

		recorder := performUserModelDiscountsRequest(t, GetUserModelDiscounts, http.MethodGet, strconv.Itoa(target.Id), "", 9999, common.RoleRootUser)
		var body userModelDiscountsBody
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		assert.True(t, body.Success, recorder.Body.String())
		assert.NotNil(t, body.Data.Discounts, "empty table must render as {}, not null")
		assert.Empty(t, body.Data.Discounts)
		assert.Contains(t, recorder.Body.String(), `"discounts":{}`)
	})

	t.Run("missing user is rejected", func(t *testing.T) {
		recorder := performUserModelDiscountsRequest(t, GetUserModelDiscounts, http.MethodGet, "999999", "", 9999, common.RoleRootUser)
		var body userModelDiscountsBody
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		assert.False(t, body.Success)
		assert.Equal(t, common.TranslateMessage(expectCtx(), i18n.MsgUserNotExists), body.Message)
	})

	t.Run("non-numeric id is rejected", func(t *testing.T) {
		recorder := performUserModelDiscountsRequest(t, GetUserModelDiscounts, http.MethodGet, "abc", "", 9999, common.RoleRootUser)
		var body userModelDiscountsBody
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		assert.False(t, body.Success)
		assert.Equal(t, common.TranslateMessage(expectCtx(), i18n.MsgInvalidId), body.Message)
	})
}
