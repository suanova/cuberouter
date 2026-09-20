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
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ListOrganizationAuditLogs 分页查询组织内审计日志
//
// @Summary      组织审计日志
// @Description  按操作者、目标、动作类型与时间范围过滤组织内审计日志
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Param        operator_user_id query int false "操作者用户 ID"
// @Param        target_type query string false "目标类型"
// @Param        target_id query int false "目标 ID"
// @Param        action_type query string false "动作类型"
// @Param        start_timestamp query int false "起始时间戳(秒)"
// @Param        end_timestamp query int false "截止时间戳(秒)"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/audit-logs [get]
// @Router       /admin/organizations/{id}/audit-logs [get]
func ListOrganizationAuditLogs(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	logs, total, err := service.ListOrganizationAuditLogs(c.GetInt("id"), organizationId, organizationAccessMode(c), organizationAuditQueryFromQuery(c, pageInfo.GetStartIdx(), pageInfo.GetPageSize()))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// ListAllOrganizationAuditLogs 平台管理员跨组织检索审计日志
//
// @Summary      全平台组织审计日志
// @Description  平台视角检索全部组织的审计日志,可按组织 ID/slug、操作者、目标与动作类型过滤
// @Tags         组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Param        organization_id query int false "组织 ID"
// @Param        organization_slug query string false "组织 slug"
// @Param        operator_user_id query int false "操作者用户 ID"
// @Param        target_type query string false "目标类型"
// @Param        target_id query int false "目标 ID"
// @Param        action_type query string false "动作类型"
// @Param        start_timestamp query int false "起始时间戳(秒)"
// @Param        end_timestamp query int false "截止时间戳(秒)"
// @Success      200 {object} dto.APIResponse
// @Router       /admin/organization-audit-logs [get]
func ListAllOrganizationAuditLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logs, total, err := service.ListAllOrganizationAuditLogs(c.GetInt("id"), organizationAuditQueryFromQuery(c, pageInfo.GetStartIdx(), pageInfo.GetPageSize()))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func organizationAuditQueryFromQuery(c *gin.Context, offset int, limit int) service.OrganizationAuditQueryRequest {
	organizationId, _ := strconv.Atoi(c.Query("organization_id"))
	operatorUserId, _ := strconv.Atoi(c.Query("operator_user_id"))
	targetId, _ := strconv.Atoi(c.Query("target_id"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	return service.OrganizationAuditQueryRequest{Offset: offset, Limit: limit, OrganizationId: organizationId, OrganizationSlug: c.Query("organization_slug"), OperatorUserId: operatorUserId, TargetType: c.Query("target_type"), TargetId: targetId, ActionType: c.Query("action_type"), StartTimestamp: startTimestamp, EndTimestamp: endTimestamp}
}
