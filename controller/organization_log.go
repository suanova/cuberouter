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

// ListOrganizationLogs 分页查询组织调用日志
//
// @Summary      组织调用日志
// @Description  分页返回组织内模型调用日志,支持按令牌、负责人、类型、模型、分组、请求 ID 与时间范围过滤
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Param        responsible_user_id query int false "负责人用户 ID"
// @Param        token_id query int false "令牌 ID"
// @Param        type query int false "日志类型"
// @Param        token_name query string false "令牌名称"
// @Param        responsible_name query string false "负责人名称"
// @Param        model_name query string false "模型名称"
// @Param        group query string false "分组"
// @Param        request_id query string false "请求 ID"
// @Param        start_timestamp query int false "起始时间戳(秒)"
// @Param        end_timestamp query int false "截止时间戳(秒)"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/logs [get]
// @Router       /admin/organizations/{id}/logs [get]
func ListOrganizationLogs(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	logs, total, err := service.ListOrganizationLogs(c.GetInt("id"), organizationId, organizationAccessMode(c), organizationLogListRequestFromQuery(c, pageInfo.GetStartIdx(), pageInfo.GetPageSize()))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// GetOrganizationLogStats 获取组织调用日志统计
//
// @Summary      组织调用日志统计
// @Description  按与日志列表相同的过滤条件返回聚合统计(次数、额度等)
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        responsible_user_id query int false "负责人用户 ID"
// @Param        token_id query int false "令牌 ID"
// @Param        type query int false "日志类型"
// @Param        token_name query string false "令牌名称"
// @Param        responsible_name query string false "负责人名称"
// @Param        model_name query string false "模型名称"
// @Param        group query string false "分组"
// @Param        request_id query string false "请求 ID"
// @Param        start_timestamp query int false "起始时间戳(秒)"
// @Param        end_timestamp query int false "截止时间戳(秒)"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/logs/stats [get]
// @Router       /admin/organizations/{id}/logs/stats [get]
func GetOrganizationLogStats(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	stats, err := service.GetOrganizationLogStats(c.GetInt("id"), organizationId, organizationAccessMode(c), organizationLogListRequestFromQuery(c, 0, 0))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func organizationLogListRequestFromQuery(c *gin.Context, offset int, limit int) service.OrganizationLogListRequest {
	responsibleUserId, _ := strconv.Atoi(c.Query("responsible_user_id"))
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	return service.OrganizationLogListRequest{
		Offset:            offset,
		Limit:             limit,
		ResponsibleUserId: responsibleUserId,
		TokenId:           tokenId,
		Type:              logType,
		TokenName:         c.Query("token_name"),
		ResponsibleName:   c.Query("responsible_name"),
		ModelName:         c.Query("model_name"),
		Group:             c.Query("group"),
		RequestId:         c.Query("request_id"),
		StartTimestamp:    startTimestamp,
		EndTimestamp:      endTimestamp,
	}
}
