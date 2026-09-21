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
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归：开启邮箱验证注册时，验证过的邮箱必须随用户记录入库。
// 邀请分组继承重构把插入移入事务后，Email 赋值被留在插入之后，
// 只落在内存结构体上，从未写入数据库。
func TestRegisterSavesVerifiedEmail(t *testing.T) {
	setupCampaignTestDB(t)
	gin.SetMode(gin.TestMode)

	oldRegister, oldPwdReg := common.RegisterEnabled, common.PasswordRegisterEnabled
	oldEmailVerify := common.EmailVerificationEnabled
	oldQuota, oldGenToken := common.QuotaForNewUser, constant.GenerateDefaultToken
	common.RegisterEnabled, common.PasswordRegisterEnabled = true, true
	common.EmailVerificationEnabled = true
	common.QuotaForNewUser, constant.GenerateDefaultToken = 0, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled = oldRegister, oldPwdReg
		common.EmailVerificationEnabled = oldEmailVerify
		common.QuotaForNewUser, constant.GenerateDefaultToken = oldQuota, oldGenToken
	})

	const email = "new-user@example.com"
	const code = "123456"
	require.NoError(t, common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose))

	engine := gin.New()
	engine.POST("/api/user/register", Register)
	body, err := common.Marshal(map[string]any{
		"username":          "email-reg-user",
		"password":          "Secret-password1",
		"email":             email,
		"verification_code": code,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/user/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	var regResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &regResp))
	require.True(t, regResp.Success, regResp.Message)

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "email-reg-user").First(&registered).Error)
	assert.Equal(t, email, registered.Email, "verified email must be persisted on the new user")
}
