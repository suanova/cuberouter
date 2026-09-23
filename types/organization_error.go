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
package types

// 组织相关的错误码。它们同时出现在管理接口的错误载荷里，前端按 code 做分支
// （例如 410 的 organization_dissolved 触发"组织已解散"引导），因此取值是对外契约，
// 不要重命名。
const (
	ErrorCodeOrganizationContextMismatch          ErrorCode = "organization_context_mismatch"
	ErrorCodeOrganizationAccessDenied             ErrorCode = "organization_access_denied"
	ErrorCodeOrganizationDisabled                 ErrorCode = "organization_disabled"
	ErrorCodeOrganizationDissolved                ErrorCode = "organization_dissolved"
	ErrorCodeOrganizationLimitExceeded            ErrorCode = "organization_limit_exceeded"
	ErrorCodeOrganizationNameConflict             ErrorCode = "organization_name_conflict"
	ErrorCodeInsufficientOrganizationQuota        ErrorCode = "insufficient_organization_quota"
	ErrorCodeOrganizationIdempotencyConflict      ErrorCode = "organization_idempotency_conflict"
	ErrorCodeOrganizationIdempotencyKeyRequired   ErrorCode = "organization_idempotency_key_required"
	ErrorCodeOrganizationConfirmationMismatch     ErrorCode = "organization_confirmation_mismatch"
	ErrorCodeOrganizationBillingSessionConflict   ErrorCode = "organization_billing_session_conflict"
	ErrorCodeOrganizationMemberOperationForbidden ErrorCode = "organization_member_operation_forbidden"
	ErrorCodeOrganizationOperationBlocked         ErrorCode = "organization_operation_blocked"
	ErrorCodeOrganizationInviteUnavailable        ErrorCode = "organization_invite_unavailable"
	ErrorCodeOrganizationInviteDeliveryFailed     ErrorCode = "organization_invite_delivery_failed"
	ErrorCodeOrganizationInviteDeliveryInProgress ErrorCode = "organization_invite_delivery_in_progress"
	ErrorCodeOrganizationInviteRecipientRejected  ErrorCode = "organization_invite_recipient_rejected"
	ErrorCodeOrganizationInviteAlreadySent        ErrorCode = "organization_invite_already_sent"
	ErrorCodeOrganizationJoinRuleInvalid          ErrorCode = "organization_join_rule_invalid"
)

const (
	ErrorCodeOrganizationTokenResponsibleMemberDisabled ErrorCode = "organization_token_responsible_member_disabled"
	ErrorCodeOrganizationTokenResponsibleUserDisabled   ErrorCode = "organization_token_responsible_user_disabled"
	ErrorCodeOrganizationTokenEnableForbidden           ErrorCode = "organization_token_enable_forbidden"
)
