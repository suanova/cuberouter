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
