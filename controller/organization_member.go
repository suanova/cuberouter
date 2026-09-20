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
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type updateOrganizationMemberRequest struct {
	Role             string `json:"role"`
	Status           string `json:"status"`
	TransferToUserId int    `json:"transfer_to_user_id"`
	Reason           string `json:"reason"`
}

type addOrganizationMemberRequest struct {
	UserId int    `json:"user_id" binding:"required"`
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

type removeOrganizationMemberRequest struct {
	TransferToUserId int    `json:"transfer_to_user_id"`
	Reason           string `json:"reason"`
}

// ListOrganizationMembers 分页查询组织成员列表
//
// @Summary      组织成员列表
// @Description  分页返回组织成员及其角色信息
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/members [get]
// @Router       /admin/organizations/{id}/members [get]
func ListOrganizationMembers(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	members, total, err := service.ListOrganizationMembersPaged(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationMemberListRequest{Offset: pageInfo.GetStartIdx(), Limit: pageInfo.GetPageSize()})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(members)
	common.ApiSuccess(c, pageInfo)
}

// AddOrganizationMember 平台管理员直接添加成员加入组织
//
// @Summary      添加组织成员
// @Description  平台侧将指定用户直接加入组织(user_id 必填);reason 记入审计日志
// @Tags         组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        body body addOrganizationMemberRequest true "用户 ID 与角色"
// @Success      200 {object} dto.APIResponse
// @Router       /admin/organizations/{id}/members [post]
func AddOrganizationMember(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req addOrganizationMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	member, err := service.AddOrganizationMember(c.GetInt("id"), organizationId, organizationAccessMode(c), service.AddMemberRequest{UserId: req.UserId, Role: req.Role, Reason: req.Reason}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, member)
}

// UpdateOrganizationMember 更新组织成员角色/状态
//
// @Summary      更新组织成员
// @Description  修改指定成员的角色或启用/停用状态;需要请求头 Idempotency-Key(幂等键,重复提交返回冲突),reason 记入审计日志
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        userId path int true "成员用户 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复变更)"
// @Param        body body updateOrganizationMemberRequest true "角色/状态变更与原因"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/members/{userId} [patch]
// @Router       /admin/organizations/{id}/members/{userId} [patch]
func UpdateOrganizationMember(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	targetUserId, ok := parseOrganizationUserId(c)
	if !ok {
		return
	}
	var req updateOrganizationMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := service.UpdateOrganizationMember(c.GetInt("id"), organizationId, organizationAccessMode(c), targetUserId, service.UpdateMemberRequest{Role: req.Role, Status: req.Status, TransferToUserId: req.TransferToUserId, Reason: req.Reason, IdempotencyKey: c.GetHeader("Idempotency-Key")}, organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// RemoveOrganizationMember 移除组织成员
//
// @Summary      移除组织成员
// @Description  高风险操作:将指定成员移出组织;需要请求头 Idempotency-Key(幂等键,重复提交返回冲突),移除所有者/转移其令牌时可指定 transfer_to_user_id
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        userId path int true "成员用户 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复移除)"
// @Param        body body removeOrganizationMemberRequest false "令牌承接人与原因"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/members/{userId} [delete]
// @Router       /admin/organizations/{id}/members/{userId} [delete]
func RemoveOrganizationMember(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	targetUserId, ok := parseOrganizationUserId(c)
	if !ok {
		return
	}
	var req removeOrganizationMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := service.RemoveOrganizationMember(c.GetInt("id"), organizationId, organizationAccessMode(c), targetUserId, service.RemoveMemberRequest{TransferToUserId: req.TransferToUserId, Reason: req.Reason, IdempotencyKey: c.GetHeader("Idempotency-Key")}, organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ExitOrganization 当前用户退出组织
//
// @Summary      退出组织
// @Description  当前登录用户退出指定组织;需要请求头 Idempotency-Key(幂等键,重复提交返回冲突),退出时可通过 transfer_to_user_id 转移本人负责的令牌
// @Tags         组织
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复退出)"
// @Param        transfer_to_user_id query int false "令牌承接人用户 ID"
// @Param        reason query string false "退出原因"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/members/me [delete]
func ExitOrganization(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	transferToUserId, ok := parseOptionalIntQuery(c, "transfer_to_user_id")
	if !ok {
		return
	}
	if err := service.ExitOrganization(c.GetInt("id"), organizationId, organizationAccessMode(c), service.ExitMemberRequest{TransferToUserId: transferToUserId, Reason: c.Query("reason"), IdempotencyKey: c.GetHeader("Idempotency-Key")}, organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func parseOrganizationUserId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("userId"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid user id"})
		return 0, false
	}
	return id, true
}

func parseOptionalIntQuery(c *gin.Context, name string) (int, bool) {
	value := c.Query(name)
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid " + name})
		return 0, false
	}
	return parsed, true
}
