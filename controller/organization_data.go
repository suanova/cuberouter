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
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetOrganizationQuotaData 获取组织额度看板数据
func GetOrganizationQuotaData(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if endTimestamp > 0 && startTimestamp > 0 && endTimestamp-startTimestamp > 2592000 {
		common.ApiErrorI18n(c, i18n.MsgOrganizationQuotaDataTimeSpanTooLong)
		return
	}
	data, err := service.GetOrganizationQuotaData(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationQuotaDataRequest{StartTimestamp: startTimestamp, EndTimestamp: endTimestamp})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
