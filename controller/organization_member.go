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
