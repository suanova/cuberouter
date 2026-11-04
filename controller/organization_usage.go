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
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetOrganizationUsage returns the organization's daily usage table. The actor
// and organization come from the session and route; filters come from the query.
func GetOrganizationUsage(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	query, err := organizationUsageQueryFromRequest(c, organizationId)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	data, err := service.GetOrganizationUsage(query)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// ExportOrganizationUsage returns every row matching the table filters at the
// requested granularity (day by default, hour on request).
func ExportOrganizationUsage(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	query, err := organizationUsageQueryFromRequest(c, organizationId)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	data, err := service.ExportOrganizationUsage(query, c.Query("granularity"))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func organizationUsageQueryFromRequest(c *gin.Context, organizationId int) (service.OrganizationUsageQuery, error) {
	operatorUserId := c.GetInt("id")
	if operatorUserId <= 0 {
		return service.OrganizationUsageQuery{}, errors.New("无效的身份信息")
	}
	startTs, endTs, err := parseBillingDateRange(c.Query("start"), c.Query("end"))
	if err != nil {
		return service.OrganizationUsageQuery{}, err
	}
	pageInfo := common.GetPageQuery(c)
	tokenId, err := parseOptionalPositiveQueryInt(c.Query("token_id"))
	if err != nil {
		return service.OrganizationUsageQuery{}, errors.New("invalid token_id")
	}
	responsibleUserId, err := parseOptionalPositiveQueryInt(c.Query("responsible_user_id"))
	if err != nil {
		return service.OrganizationUsageQuery{}, errors.New("invalid responsible_user_id")
	}
	return service.OrganizationUsageQuery{
		OperatorUserId:    operatorUserId,
		OrganizationId:    organizationId,
		AccessMode:        organizationAccessMode(c),
		StartTs:           startTs,
		EndTs:             endTs,
		ModelName:         c.Query("model_name"),
		TokenId:           tokenId,
		TokenName:         c.Query("token_name"),
		ResponsibleUserId: responsibleUserId,
		Page:              pageInfo.Page,
		PageSize:          pageInfo.PageSize,
	}, nil
}

// parseOptionalPositiveQueryInt treats an absent optional filter as "no
// filter" but refuses a malformed or non-positive value instead of silently
// turning it into an unfiltered query.
func parseOptionalPositiveQueryInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid query integer")
	}
	return parsed, nil
}
