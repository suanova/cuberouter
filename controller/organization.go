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
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type createOrganizationRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateOrganizationRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Group       *string `json:"group"`
	Reason      string  `json:"reason"`
}

type updateOrganizationStatusRequest struct {
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	ConfirmName string `json:"confirm_name"`
}

type dissolveOrganizationRequest struct {
	ConfirmName string `json:"confirm_name"`
	Reason      string `json:"reason"`
}

type transferOrganizationOwnerRequest struct {
	OwnerUserId int    `json:"owner_user_id" binding:"required"`
	Reason      string `json:"reason"`
}

type adjustOrganizationQuotaRequest struct {
	QuotaDelta int    `json:"quota_delta"`
	Reason     string `json:"reason"`
}

// AdjustOrganizationQuota 平台管理员调整组织配额(增减)
//
// @Summary      调整组织配额
// @Description  高风险操作:为组织增加/扣减原生额度并记审计。需要请求头 Idempotency-Key(幂等键,重复提交返回冲突),quota_delta 为本次变更量
// @Tags         组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复调整)"
// @Param        body body adjustOrganizationQuotaRequest true "配额变更量与原因"
// @Success      200 {object} dto.APIResponse
// @Router       /admin/organizations/{id}/quota-adjustments [post]
func AdjustOrganizationQuota(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req adjustOrganizationQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	adjustment, err := service.AdjustOrganizationQuota(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationQuotaAdjustmentRequest{QuotaDelta: req.QuotaDelta, Reason: req.Reason, IdempotencyKey: c.GetHeader("Idempotency-Key")}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, adjustment)
}

// ListOrganizations 平台管理员分页检索全部组织
//
// @Summary      组织管理列表
// @Description  平台侧全量组织列表,支持关键字、状态与分组过滤
// @Tags         组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Param        keyword query string false "关键字(名称模糊匹配)"
// @Param        status query string false "状态过滤(active/disabled)"
// @Param        group query string false "分组过滤"
// @Success      200 {object} dto.APIResponse
// @Router       /admin/organizations [get]
func ListOrganizations(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	organizations, total, err := service.ListOrganizationsForManagement(c.GetInt("id"), service.OrganizationManagementListRequest{Keyword: c.Query("keyword"), Status: c.Query("status"), Group: c.Query("group")}, pageInfo)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(organizations)
	common.ApiSuccess(c, pageInfo)
}

// ListSelfOrganizations 获取当前登录用户可访问的组织列表
//
// @Summary      获取我的组织列表
// @Description  返回当前登录用户可访问的 active/disabled 组织列表
// @Tags         组织
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} dto.APIResponse
// @Router       /organizations [get]
// @Router       /organizations/self [get]
func ListSelfOrganizations(c *gin.Context) {
	organizations, err := service.ListUserOrganizations(c.GetInt("id"), false)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, organizations)
}

// CreateOrganization 创建组织(当前用户成为所有者)
//
// @Summary      创建组织
// @Description  创建组织并将当前用户设为所有者;受平台组织数量上限约束
// @Tags         组织
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        body body createOrganizationRequest true "组织名称与描述"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations [post]
func CreateOrganization(c *gin.Context) {
	var req createOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	organization, err := service.CreateOrganization(c.GetInt("id"), service.CreateOrganizationRequest{Name: req.Name, Description: req.Description}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, organization)
}

// GetOrganization 获取组织详情
//
// @Summary      获取组织详情
// @Description  返回组织详情(含成员数等概要);普通入口按成员权限访问,管理入口为平台视角
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id} [get]
// @Router       /admin/organizations/{id} [get]
func GetOrganization(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	detail, err := service.GetOrganizationDetailForUserWithAccessMode(c.GetInt("id"), organizationId, organizationAccessMode(c), true)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

// UpdateOrganization 更新组织资料(名称/描述/分组)
//
// @Summary      更新组织资料
// @Description  更新组织名称、描述与分组;reason 会记入审计日志
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        body body updateOrganizationRequest true "组织资料变更字段"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id} [patch]
// @Router       /admin/organizations/{id} [patch]
func UpdateOrganization(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req updateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	organization, err := service.UpdateOrganization(c.GetInt("id"), organizationId, organizationAccessMode(c), service.UpdateOrganizationRequest{Name: req.Name, Description: req.Description, Group: req.Group, Reason: req.Reason}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, organization)
}

// UpdateOrganizationStatus 组织内管理者启用/停用组织
//
// @Summary      启用/停用组织(组织侧)
// @Description  高风险操作:status 取 active/disabled,停用时必须携带二次确认字段 confirm_name(须与组织名称一致)方可生效
// @Tags         组织
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        body body updateOrganizationStatusRequest true "目标状态、二次确认与原因"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/status [patch]
func UpdateOrganizationStatus(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req updateOrganizationStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	var err error
	switch req.Status {
	case "active":
		err = service.EnableOrganization(c.GetInt("id"), organizationId, organizationAccessMode(c), req.ConfirmName, req.Reason, organizationAuditRequestMetadata(c))
	case "disabled":
		err = service.DisableOrganization(c.GetInt("id"), organizationId, organizationAccessMode(c), req.ConfirmName, req.Reason, organizationAuditRequestMetadata(c))
	default:
		err = strconv.ErrSyntax
	}
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// UpdateOrganizationPlatformStatus 平台管理员启用/停用组织
//
// @Summary      启用/停用组织(平台侧)
// @Description  高风险操作:平台视角的状态变更,status 取 active/disabled,停用时必须携带二次确认字段 confirm_name(须与组织名称一致)方可生效
// @Tags         组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        body body updateOrganizationStatusRequest true "目标状态、二次确认与原因"
// @Success      200 {object} dto.APIResponse
// @Router       /admin/organizations/{id}/status [patch]
func UpdateOrganizationPlatformStatus(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req updateOrganizationStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	var err error
	switch req.Status {
	case "active":
		err = service.EnableOrganizationByPlatform(c.GetInt("id"), organizationId, organizationAccessMode(c), req.ConfirmName, req.Reason, organizationAuditRequestMetadata(c))
	case "disabled":
		err = service.DisableOrganizationByPlatform(c.GetInt("id"), organizationId, organizationAccessMode(c), req.ConfirmName, req.Reason, organizationAuditRequestMetadata(c))
	default:
		err = strconv.ErrSyntax
	}
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// DissolveOrganization 解散组织(不可恢复)
//
// @Summary      解散组织
// @Description  高风险操作:解散组织及其成员关系,不可恢复。必须携带二次确认字段 confirm_name(须与组织名称一致),且需要请求头 Idempotency-Key(幂等键,重复提交返回冲突)
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复解散)"
// @Param        body body dissolveOrganizationRequest true "二次确认与原因"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id} [delete]
// @Router       /admin/organizations/{id} [delete]
func DissolveOrganization(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req dissolveOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := service.DissolveOrganization(c.GetInt("id"), organizationId, organizationAccessMode(c), service.DissolveOrganizationRequest{ConfirmName: req.ConfirmName, Reason: req.Reason, IdempotencyKey: c.GetHeader("Idempotency-Key")}, organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// TransferOrganizationOwner 转让组织所有权
//
// @Summary      转让组织所有权
// @Description  高风险操作:将组织所有者变更为指定成员(owner_user_id 必填),原所有者降级为普通成员。需要请求头 Idempotency-Key(幂等键,重复提交返回冲突)
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复转让)"
// @Param        body body transferOrganizationOwnerRequest true "新所有者用户 ID 与原因"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/owner [put]
// @Router       /admin/organizations/{id}/owner [put]
func TransferOrganizationOwner(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req transferOrganizationOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := service.TransferOrganizationOwner(c.GetInt("id"), organizationId, organizationAccessMode(c), service.TransferOrganizationOwnerRequest{OwnerUserId: req.OwnerUserId, Reason: req.Reason, IdempotencyKey: c.GetHeader("Idempotency-Key")}, organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func parseOrganizationId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid organization id"})
		return 0, false
	}
	return id, true
}

func organizationAccessMode(c *gin.Context) string {
	return common.GetContextKeyString(c, constant.ContextKeyOrganizationAccessMode)
}

func organizationAuditRequestMetadata(c *gin.Context) service.OrganizationAuditRequestMetadata {
	return service.OrganizationAuditRequestMetadata{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

func writeOrganizationError(c *gin.Context, err error) {
	message := err.Error()
	status := http.StatusBadRequest
	var code types.ErrorCode
	response := gin.H{"success": false, "message": message}
	var deliveryErr *service.OrganizationInviteDeliveryError
	var blockedErr *service.OrganizationOperationBlockedError
	if errors.As(err, &deliveryErr) {
		status = http.StatusBadGateway
		code = types.ErrorCodeOrganizationInviteDeliveryFailed
		response["message"] = "organization invite email delivery failed"
		response["invite_id"] = deliveryErr.InviteID
		response["delivery_status"] = deliveryErr.DeliveryStatus
	} else if errors.Is(err, service.ErrOrganizationInviteDeliveryInProgress) {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationInviteDeliveryInProgress
	} else if errors.Is(err, service.ErrOrganizationInviteRecipientRejected) {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationInviteRecipientRejected
	} else if errors.Is(err, service.ErrOrganizationInviteAlreadySent) {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationInviteAlreadySent
	} else if errors.As(err, &blockedErr) {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationOperationBlocked
		response["blockers"] = blockedErr.Blockers
	} else if errors.Is(err, service.ErrOrganizationTokenResponsibleMemberDisabled) {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationTokenResponsibleMemberDisabled
	} else if errors.Is(err, service.ErrOrganizationTokenResponsibleUserDisabled) {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationTokenResponsibleUserDisabled
	} else if errors.Is(err, service.ErrOrganizationTokenEnableForbidden) {
		status = http.StatusForbidden
		code = types.ErrorCodeOrganizationTokenEnableForbidden
	} else if errors.Is(err, service.ErrOrganizationNameConflict) {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationNameConflict
		response["message"] = service.ErrOrganizationNameConflict.Error()
	} else if strings.Contains(message, "organization limit exceeded") {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationLimitExceeded
	} else if strings.Contains(message, "organization member operation forbidden") {
		status = http.StatusForbidden
		code = types.ErrorCodeOrganizationMemberOperationForbidden
	} else if strings.Contains(message, "organization dissolved") {
		status = http.StatusGone
		code = types.ErrorCodeOrganizationDissolved
	} else if strings.Contains(message, "organization disabled") {
		status = http.StatusForbidden
		code = types.ErrorCodeOrganizationDisabled
	} else if strings.Contains(message, "permission denied") || strings.Contains(message, "organization access denied") || strings.Contains(message, "organization is not active") {
		status = http.StatusForbidden
		code = types.ErrorCodeOrganizationAccessDenied
	}
	if strings.Contains(message, "record not found") {
		status = http.StatusNotFound
	}
	if strings.Contains(message, "organization idempotency conflict") {
		status = http.StatusConflict
		code = types.ErrorCodeOrganizationIdempotencyConflict
	}
	if strings.Contains(message, "organization idempotency key is required") {
		status = http.StatusBadRequest
		code = types.ErrorCodeOrganizationIdempotencyKeyRequired
	}
	if strings.Contains(message, "organization confirmation mismatch") {
		status = http.StatusBadRequest
		code = types.ErrorCodeOrganizationConfirmationMismatch
	}
	if strings.Contains(message, "already joined organization") {
		status = http.StatusConflict
	}
	if strings.Contains(message, "invite is not pending") || strings.Contains(message, "invite expired") {
		code = types.ErrorCodeOrganizationInviteUnavailable
	}
	if code != "" {
		response["code"] = code
	}
	c.JSON(status, response)
}
