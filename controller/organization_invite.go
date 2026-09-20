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
//
// @Summary      组织邀请列表
// @Description  分页返回组织已发出的成员邀请
// @Tags         组织
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/invitations [get]
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
//
// @Summary      创建组织邀请
// @Description  向指定邮箱发出组织加入邀请;需要请求头 Idempotency-Key(幂等键,重复提交返回冲突),force_rotate 可强制轮换同邮箱未决邀请
// @Tags         组织
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复邀请)"
// @Param        body body createOrganizationInviteRequest true "邀请邮箱与角色"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/invitations [post]
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
//
// @Summary      查看组织邀请详情
// @Description  凭邀请 token 查看邀请信息(组织、角色、有效期);登录可选,未登录也可查看
// @Tags         组织
// @Security     ApiKeyAuth[]
// @Produce      json
// @Param        token path string true "邀请 token"
// @Success      200 {object} dto.APIResponse
// @Router       /organization-invitations/{token} [get]
func GetOrganizationInvite(c *gin.Context) {
	view, err := service.GetInviteByToken(c.Param("token"), c.GetInt("id"))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

// AcceptOrganizationInvite 通过邀请 token 接受邀请并加入组织
//
// @Summary      接受组织邀请
// @Description  登录用户凭邀请 token 接受邀请并加入组织;body 可省略,缺省视为 accepted
// @Tags         组织
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        token path string true "邀请 token"
// @Param        body body acceptOrganizationInviteRequest false "邀请状态(缺省 accepted)"
// @Success      200 {object} dto.APIResponse
// @Router       /organization-invitations/{token} [patch]
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
//
// @Summary      撤销组织邀请
// @Description  撤销指定的未决邀请;reason 记入审计日志
// @Tags         组织
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        invitationId path int true "邀请 ID"
// @Param        reason query string false "撤销原因"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/invitations/{invitationId} [delete]
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
