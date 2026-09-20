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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestWriteOrganizationErrorReturnsStableCodes 锁定错误到 HTTP 的映射表。
//
// 这张表就是组织接口对外的契约：前端按 code 分支、PRD 的验收标准按状态码判定
// （410 已解散、409 冲突、403 无权限）。映射本身靠字符串匹配，改一个错误文案
// 就可能悄悄把状态码或 code 换掉，所以这里逐条钉死。
func TestWriteOrganizationErrorReturnsStableCodes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		err        error
		statusCode int
		code       string
	}{
		{
			name:       "organization limit",
			err:        errors.New("organization limit exceeded"),
			statusCode: http.StatusConflict,
			code:       "organization_limit_exceeded",
		},
		{
			name:       "organization name conflict",
			err:        service.ErrOrganizationNameConflict,
			statusCode: http.StatusConflict,
			code:       "organization_name_conflict",
		},
		{
			name:       "operation blocked",
			err:        &service.OrganizationOperationBlockedError{Message: "active transfer target required", Blockers: []string{"active transfer target required"}},
			statusCode: http.StatusConflict,
			code:       "organization_operation_blocked",
		},
		{
			name:       "idempotency conflict",
			err:        errors.New("organization idempotency conflict"),
			statusCode: http.StatusConflict,
			code:       "organization_idempotency_conflict",
		},
		{
			name:       "idempotency key required",
			err:        errors.New("organization idempotency key is required"),
			statusCode: http.StatusBadRequest,
			code:       "organization_idempotency_key_required",
		},
		{
			name:       "confirmation mismatch",
			err:        errors.New("organization confirmation mismatch"),
			statusCode: http.StatusBadRequest,
			code:       "organization_confirmation_mismatch",
		},
		{
			name:       "organization dissolved",
			err:        errors.New("organization dissolved"),
			statusCode: http.StatusGone,
			code:       "organization_dissolved",
		},
		{
			name:       "organization access denied",
			err:        errors.New("organization access denied"),
			statusCode: http.StatusForbidden,
			code:       "organization_access_denied",
		},
		{
			name:       "organization token responsible member disabled",
			err:        service.ErrOrganizationTokenResponsibleMemberDisabled,
			statusCode: http.StatusConflict,
			code:       "organization_token_responsible_member_disabled",
		},
		{
			name:       "organization token responsible user disabled",
			err:        service.ErrOrganizationTokenResponsibleUserDisabled,
			statusCode: http.StatusConflict,
			code:       "organization_token_responsible_user_disabled",
		},
		{
			name:       "organization token enable forbidden",
			err:        service.ErrOrganizationTokenEnableForbidden,
			statusCode: http.StatusForbidden,
			code:       "organization_token_enable_forbidden",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			writeOrganizationError(c, tc.err)

			require.Equal(t, tc.statusCode, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+tc.code+`"`)
		})
	}
}

// TestWriteOrganizationErrorReturnsTypedInviteDeliveryFailure 覆盖邀请投递失败这支：
// 对外只能暴露收件人被拒这一事实，SMTP 的原始报错里带着收件人地址，泄出去等于
// 把邀请对象的邮箱回显给调用方。
func TestWriteOrganizationErrorReturnsTypedInviteDeliveryFailure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeOrganizationError(c, &service.OrganizationInviteDeliveryError{InviteID: 42, DeliveryStatus: "rejected", Cause: errors.New("550 User not found: secret@example.com")})

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, `"code":"organization_invite_delivery_failed"`)
	require.Contains(t, body, `"invite_id":42`)
	require.Contains(t, body, `"delivery_status":"rejected"`)
	require.Contains(t, body, `"message":"organization invite email delivery failed"`)
	require.NotContains(t, body, "User not found")
	require.NotContains(t, body, "secret@example.com")
}

func TestWriteOrganizationErrorReturnsStableInviteConflictCodes(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		err  error
		code string
	}{
		{name: "delivery in progress", err: service.ErrOrganizationInviteDeliveryInProgress, code: "organization_invite_delivery_in_progress"},
		{name: "recipient rejected", err: service.ErrOrganizationInviteRecipientRejected, code: "organization_invite_recipient_rejected"},
		{name: "already sent", err: service.ErrOrganizationInviteAlreadySent, code: "organization_invite_already_sent"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			writeOrganizationError(c, tc.err)

			require.Equal(t, http.StatusConflict, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+tc.code+`"`)
		})
	}
}
