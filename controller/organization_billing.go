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

// GetOrganizationBillingSummary 获取组织账单总览
func GetOrganizationBillingSummary(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	summary, err := service.GetOrganizationBillingSummary(c.GetInt("id"), organizationId, organizationAccessMode(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// GetMyOrganizationMemberBilling 获取当前用户在组织内的成员账单
func GetMyOrganizationMemberBilling(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	summary, err := service.GetMyOrganizationMemberBilling(c.GetInt("id"), organizationId, organizationAccessMode(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// ListOrganizationBillingUserSummaries 按成员汇总组织账单
func ListOrganizationBillingUserSummaries(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	response, err := service.ListOrganizationBillingUserSummaries(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationBillingUserSummaryRequest{Month: c.Query("month"), StartMonth: c.Query("start_month"), EndMonth: c.Query("end_month")})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

// ListOrganizationBillingMonthlySummaries 按月份汇总组织账单
func ListOrganizationBillingMonthlySummaries(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	months, _ := strconv.Atoi(c.Query("months"))
	response, err := service.ListOrganizationBillingMonthlySummaries(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationBillingMonthlySummaryRequest{Months: months})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

// ListOrganizationBillingDetails 分页查询组织账单明细
func ListOrganizationBillingDetails(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	records, total, err := service.ListOrganizationBillingDetails(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationBillingDetailListRequest{
		Offset:          pageInfo.GetStartIdx(),
		Limit:           pageInfo.GetPageSize(),
		Month:           c.Query("month"),
		TokenName:       c.Query("token_name"),
		ResponsibleName: c.Query("responsible_name"),
		ModelName:       c.Query("model_name"),
		Group:           c.Query("group"),
		RequestId:       c.Query("request_id"),
		StartTimestamp:  startTimestamp,
		EndTimestamp:    endTimestamp,
	})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}
