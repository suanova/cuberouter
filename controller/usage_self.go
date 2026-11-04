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
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetSelfUsage returns the caller's own usage. The actor is always the
// authenticated user; no user_id parameter is accepted.
func GetSelfUsage(c *gin.Context) {
	actorUserId := c.GetInt("id")
	if actorUserId <= 0 {
		common.ApiErrorMsg(c, "无效的身份信息")
		return
	}
	startStr, endStr := c.Query("start"), c.Query("end")
	startTs, endTs, err := parseBillingDateRange(startStr, endStr)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	pageInfo := common.GetPageQuery(c)
	organizationId, _ := strconv.Atoi(c.Query("organization_id"))
	overviewOrganizationId, _ := strconv.Atoi(c.Query("overview_organization_id"))
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	data, err := service.GetSelfUsage(service.SelfUsageQuery{
		ActorUserId:            actorUserId,
		StartTs:                startTs,
		EndTs:                  endTs,
		ModelName:              c.Query("model_name"),
		TokenId:                tokenId,
		TokenName:              c.Query("token_name"),
		BillingAccountType:     c.Query("billing_account_type"),
		OrganizationID:         organizationId,
		OverviewOrganizationID: overviewOrganizationId,
		Page:                   pageInfo.Page,
		PageSize:               pageInfo.PageSize,
	})
	if err != nil {
		if errors.Is(err, service.ErrSelfUsageOrganizationForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
