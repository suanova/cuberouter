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
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type createOrganizationInviteRequest struct {
	Email       string `json:"email"`
	TargetEmail string `json:"target_email"`
	Role        string `json:"role"`
	ForceRotate bool   `json:"force_rotate"`
}

type acceptOrganizationInviteRequest struct {
	Status string `json:"status"`
}

func (req createOrganizationInviteRequest) inviteEmail() string {
	email := strings.TrimSpace(req.TargetEmail)
	if email != "" {
		return email
	}
	return req.Email
}

// ListOrganizationInvites 分页查询组织邀请列表
func ListOrganizationInvites(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	invites, total, err := service.ListOrganizationInvitesPaged(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationInviteListRequest{Offset: pageInfo.GetStartIdx(), Limit: pageInfo.GetPageSize()})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(invites)
	common.ApiSuccess(c, pageInfo)
}

// CreateOrganizationInvite 向邮箱发送组织加入邀请
func CreateOrganizationInvite(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req createOrganizationInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	invite, err := service.CreateEmailInvite(c.GetInt("id"), organizationId, organizationAccessMode(c), service.CreateInviteRequest{Email: req.inviteEmail(), Role: req.Role, ForceRotate: req.ForceRotate, IdempotencyKey: c.GetHeader("Idempotency-Key")}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, invite)
}

// GetOrganizationInvite 通过邀请 token 查看邀请详情(公开)
func GetOrganizationInvite(c *gin.Context) {
	view, err := service.GetInviteByToken(c.Param("token"), c.GetInt("id"))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

// AcceptOrganizationInvite 通过邀请 token 接受邀请并加入组织
func AcceptOrganizationInvite(c *gin.Context) {
	var req acceptOrganizationInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = model.OrganizationInviteStatusAccepted
	}
	if status != model.OrganizationInviteStatusAccepted {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid invitation status"})
		return
	}
	if err := service.AcceptInvite(c.Param("token"), c.GetInt("id"), organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// RevokeOrganizationInvite 撤销组织邀请
func RevokeOrganizationInvite(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	inviteId, err := strconv.Atoi(c.Param("invitationId"))
	if err != nil || inviteId <= 0 {
		inviteId, err = strconv.Atoi(c.Param("inviteId"))
	}
	if err != nil || inviteId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid invite id"})
		return
	}
	if err := service.RevokeInvite(c.GetInt("id"), organizationId, organizationAccessMode(c), inviteId, c.Query("reason"), organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
