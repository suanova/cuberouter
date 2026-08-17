/*
Copyright (C) 2023-2026 QuantumNous

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
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// TestResetPasswordEmailFailureKeepsPasswordAndToken 保护"重置密码先发邮件、
// 成功后才落库新密码并消费 token"的契约：SMTP 失败时用户旧密码与重置 token
// 都必须保持原样，可重试，不会被锁在门外。
//
// 注意：fixture 不能用 setupOpsUserTestDB——它调用 i18n.Init() 初始化全局
// bundle，而本文件按字母序排在其他断言原始 i18n key 的测试之前，会改变
// 它们拿到的消息文本。
func TestResetPasswordEmailFailureKeepsPasswordAndToken(t *testing.T) {
	db := setupManageUserTestDB(t)

	// 显式强制邮件投递失败（而不是依赖环境未配置 SMTP）：若其他测试或
	// 设置项已配置 SMTP，ResetPassword 会走成功路径并改动用户状态，
	// 测试就失去确定性。保存原配置，测试结束恢复。
	prevSMTPServer, prevSMTPAccount := common.SMTPServer, common.SMTPAccount
	common.SMTPServer, common.SMTPAccount = "", ""
	t.Cleanup(func() {
		common.SMTPServer, common.SMTPAccount = prevSMTPServer, prevSMTPAccount
	})

	passwordHash, err := common.Password2Hash("OldPass123")
	require.NoError(t, err)
	user := model.User{
		Username: "reset-email-fail",
		Password: passwordHash,
		Email:    "reset-email-fail@test.com",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(&user).Error)

	// 预置有效 token（Redis 未启用，验证码走进程内存储）
	code := common.GenerateVerificationCode(0)
	common.RegisterVerificationCodeWithKey(user.Email, code, common.PasswordResetPurpose)
	require.True(t, common.VerifyCodeWithKey(user.Email, code, common.PasswordResetPurpose))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := fmt.Sprintf(`{"email":"%s","token":"%s"}`, user.Email, code)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/reset", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	ResetPassword(c)

	// SMTP 未配置 → 邮件发送失败，请求失败
	assert.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)

	// 密码未被轮换（哈希不变），token 未被消费（仍可验证）
	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Equal(t, passwordHash, stored.Password)
	assert.True(t, common.VerifyCodeWithKey(user.Email, code, common.PasswordResetPurpose))
}
