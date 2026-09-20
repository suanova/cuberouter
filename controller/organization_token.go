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
