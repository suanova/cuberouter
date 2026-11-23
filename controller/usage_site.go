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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// siteUsageQueryFromRequest parses the admin site usage filters. Dates are
// required and capped by parseBillingDateRange.
func siteUsageQueryFromRequest(c *gin.Context) (service.SiteUsageQuery, error) {
	startTs, endTs, err := parseBillingDateRange(c.Query("start"), c.Query("end"))
	if err != nil {
		return service.SiteUsageQuery{}, err
	}
	pageInfo := common.GetPageQuery(c)
	tokenId, err := parseOptionalPositiveQueryInt(c.Query("token_id"))
	if err != nil {
		return service.SiteUsageQuery{}, errors.New("invalid token_id")
	}
	organizationId, err := parseOptionalPositiveQueryInt(c.Query("organization_id"))
	if err != nil {
		return service.SiteUsageQuery{}, errors.New("invalid organization_id")
	}
	userId, err := parseOptionalPositiveQueryInt(c.Query("user_id"))
	if err != nil {
		return service.SiteUsageQuery{}, errors.New("invalid user_id")
	}
	return service.SiteUsageQuery{
		StartTs:            startTs,
		EndTs:              endTs,
		ModelName:          c.Query("model_name"),
		TokenId:            tokenId,
		TokenName:          c.Query("token_name"),
		BillingAccountType: c.Query("billing_account_type"),
		OrganizationID:     organizationId,
		UserId:             userId,
		ChartGranularity:   c.Query("chart_granularity"),
		Page:               pageInfo.Page,
		PageSize:           pageInfo.PageSize,
	}, nil
}

// GetSiteUsage returns the authenticated administrator's site-wide usage.
func GetSiteUsage(c *gin.Context) {
	query, err := siteUsageQueryFromRequest(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	data, err := service.GetSiteUsage(query)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// ExportSiteUsage returns every matching site row at the requested granularity.
func ExportSiteUsage(c *gin.Context) {
	query, err := siteUsageQueryFromRequest(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	data, err := service.ExportSiteUsage(query, c.Query("granularity"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
