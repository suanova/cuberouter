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
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type organizationTokenRequest struct {
	Name               string  `json:"name"`
	Status             int     `json:"status"`
	ExpiredTime        int64   `json:"expired_time"`
	RemainQuota        int     `json:"remain_quota"`
	UnlimitedQuota     bool    `json:"unlimited_quota"`
	ModelLimitsEnabled bool    `json:"model_limits_enabled"`
	ModelLimits        string  `json:"model_limits"`
	AllowIps           *string `json:"allow_ips"`
	Group              string  `json:"group"`
	CrossGroupRetry    bool    `json:"cross_group_retry"`
	Visibility         string  `json:"visibility"`
	ResponsibleUserId  int     `json:"responsible_user_id"`
}

type organizationTokenBatchCreateRequest struct {
	TokenCount int `json:"token_count"`
	organizationTokenRequest
}

type organizationTokenBatchDeleteRequest struct {
	Ids []int `json:"ids"`
}

type updateOrganizationTokenResponsibilityRequest struct {
	ResponsibleUserId int    `json:"responsible_user_id"`
	Reason            string `json:"reason"`
}

// ListOrganizationTokens 分页查询组织令牌列表
//
// @Summary      组织令牌列表
// @Description  分页返回组织令牌,支持关键字、状态、负责人、可见性与分组过滤
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Param        keyword query string false "关键字(名称模糊匹配)"
// @Param        status query int false "状态过滤"
// @Param        responsible_user_id query int false "负责人用户 ID"
// @Param        visibility query string false "可见性过滤"
// @Param        group query string false "分组过滤"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/tokens [get]
// @Router       /admin/organizations/{id}/tokens [get]
func ListOrganizationTokens(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	status, _ := strconv.Atoi(c.Query("status"))
	responsibleUserId, _ := strconv.Atoi(c.Query("responsible_user_id"))
	tokens, total, err := service.ListOrganizationTokens(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationTokenListRequest{
		Offset:            pageInfo.GetStartIdx(),
		Limit:             pageInfo.GetPageSize(),
		Keyword:           c.Query("keyword"),
		Status:            status,
		ResponsibleUserId: responsibleUserId,
		Visibility:        c.Query("visibility"),
		Group:             c.Query("group"),
	})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tokens)
	common.ApiSuccess(c, pageInfo)
}

// GetOrganizationToken 获取组织令牌详情
//
// @Summary      获取组织令牌详情
// @Description  返回指定组织令牌的完整信息
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        tokenId path int true "令牌 ID"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/tokens/{tokenId} [get]
// @Router       /admin/organizations/{id}/tokens/{tokenId} [get]
func GetOrganizationToken(c *gin.Context) {
	organizationId, tokenId, ok := parseOrganizationAndTokenId(c)
	if !ok {
		return
	}
	token, err := service.GetOrganizationToken(c.GetInt("id"), organizationId, organizationAccessMode(c), tokenId)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, token)
}

// CreateOrganizationToken 在组织下创建令牌
//
// @Summary      创建组织令牌
// @Description  在组织下创建 API 令牌(名称、额度、模型限制、可见性、负责人等)
// @Tags         组织
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        body body organizationTokenRequest true "令牌属性"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/tokens [post]
func CreateOrganizationToken(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req organizationTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	token, err := service.CreateOrganizationToken(c.GetInt("id"), organizationId, organizationAccessMode(c), req.toServiceRequest(), organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, token)
}

// BatchCreateOrganizationTokens 批量创建组织令牌
//
// @Summary      批量创建组织令牌
// @Description  批量操作:按同一属性模板创建 token_count 个令牌;需要请求头 Idempotency-Key(幂等键,重复提交返回冲突)
// @Tags         组织
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复批量创建)"
// @Param        body body organizationTokenBatchCreateRequest true "批量数量与令牌属性模板"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/token-batches [post]
func BatchCreateOrganizationTokens(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req organizationTokenBatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	result, err := service.BatchCreateOrganizationTokens(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationTokenBatchCreateRequest{
		TokenCount:     req.TokenCount,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		Token:          req.toServiceRequest(),
	}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// UpdateOrganizationToken 更新组织令牌
//
// @Summary      更新组织令牌
// @Description  更新指定组织令牌的属性(名称、额度、状态、模型限制等)
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        tokenId path int true "令牌 ID"
// @Param        body body organizationTokenRequest true "令牌属性"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/tokens/{tokenId} [patch]
// @Router       /admin/organizations/{id}/tokens/{tokenId} [patch]
func UpdateOrganizationToken(c *gin.Context) {
	organizationId, tokenId, ok := parseOrganizationAndTokenId(c)
	if !ok {
		return
	}
	var req organizationTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	token, err := service.UpdateOrganizationToken(c.GetInt("id"), organizationId, organizationAccessMode(c), tokenId, req.toServiceRequest(), organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, token)
}

// DeleteOrganizationToken 删除组织令牌
//
// @Summary      删除组织令牌
// @Description  删除指定的组织令牌(不可恢复)
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        tokenId path int true "令牌 ID"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/tokens/{tokenId} [delete]
// @Router       /admin/organizations/{id}/tokens/{tokenId} [delete]
func DeleteOrganizationToken(c *gin.Context) {
	organizationId, tokenId, ok := parseOrganizationAndTokenId(c)
	if !ok {
		return
	}
	if err := service.DeleteOrganizationToken(c.GetInt("id"), organizationId, organizationAccessMode(c), tokenId, organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// BatchDeleteOrganizationTokens 批量删除组织令牌
//
// @Summary      批量删除组织令牌
// @Description  批量操作:按令牌 ID 列表批量删除(ids 不能为空);需要请求头 Idempotency-Key(幂等键,重复提交返回冲突)
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        Idempotency-Key header string true "幂等键(避免重复批量删除)"
// @Param        body body organizationTokenBatchDeleteRequest true "令牌 ID 列表"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/token-deletions [post]
// @Router       /admin/organizations/{id}/token-deletions [post]
func BatchDeleteOrganizationTokens(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req organizationTokenBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid token ids"})
		return
	}
	count, err := service.BatchDeleteOrganizationTokens(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationTokenBatchDeleteRequest{Ids: req.Ids, IdempotencyKey: c.GetHeader("Idempotency-Key")}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, count)
}

// UpdateOrganizationTokenResponsibility 变更组织令牌负责人
//
// @Summary      变更令牌负责人
// @Description  将指定令牌的负责人变更为其他成员;reason 记入审计日志
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        tokenId path int true "令牌 ID"
// @Param        body body updateOrganizationTokenResponsibilityRequest true "新负责人用户 ID 与原因"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/tokens/{tokenId}/responsible-user [patch]
// @Router       /admin/organizations/{id}/tokens/{tokenId}/responsible-user [patch]
func UpdateOrganizationTokenResponsibility(c *gin.Context) {
	organizationId, tokenId, ok := parseOrganizationAndTokenId(c)
	if !ok {
		return
	}
	var req updateOrganizationTokenResponsibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	token, err := service.UpdateOrganizationTokenResponsibility(c.GetInt("id"), organizationId, organizationAccessMode(c), tokenId, service.UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: req.ResponsibleUserId, Reason: req.Reason}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, token)
}

func (req organizationTokenRequest) toServiceRequest() service.OrganizationTokenRequest {
	return service.OrganizationTokenRequest{
		Name:               req.Name,
		Status:             req.Status,
		ExpiredTime:        req.ExpiredTime,
		RemainQuota:        req.RemainQuota,
		UnlimitedQuota:     req.UnlimitedQuota,
		ModelLimitsEnabled: req.ModelLimitsEnabled,
		ModelLimits:        req.ModelLimits,
		AllowIps:           req.AllowIps,
		Group:              req.Group,
		CrossGroupRetry:    req.CrossGroupRetry,
		Visibility:         req.Visibility,
		ResponsibleUserId:  req.ResponsibleUserId,
	}
}

func parseOrganizationAndTokenId(c *gin.Context) (int, int, bool) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return 0, 0, false
	}
	tokenId, err := strconv.Atoi(c.Param("tokenId"))
	if err != nil || tokenId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid token id"})
		return 0, 0, false
	}
	return organizationId, tokenId, true
}
