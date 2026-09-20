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
