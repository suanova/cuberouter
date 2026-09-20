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
//
// @Summary      组织账单总览
// @Description  返回组织的额度消耗汇总(钱包+订阅口径)
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/billing/summary [get]
// @Router       /admin/organizations/{id}/billing/summary [get]
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
//
// @Summary      我的组织成员账单
// @Description  返回当前登录用户在指定组织内的个人消耗汇总
// @Tags         组织
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/billing/members/me [get]
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
//
// @Summary      组织成员账单汇总
// @Description  按成员维度汇总组织消耗,支持按月份或月份区间过滤
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        month query string false "月份(YYYY-MM)"
// @Param        start_month query string false "起始月份(YYYY-MM)"
// @Param        end_month query string false "截止月份(YYYY-MM)"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/billing/user-summaries [get]
// @Router       /admin/organizations/{id}/billing/user-summaries [get]
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
//
// @Summary      组织月度账单汇总
// @Description  按月份维度汇总组织消耗,months 控制回溯月数
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        months query int false "回溯月数"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/billing/monthly-summaries [get]
// @Router       /admin/organizations/{id}/billing/monthly-summaries [get]
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
//
// @Summary      组织账单明细
// @Description  分页返回组织消耗明细记录,支持月份、令牌、负责人、模型、分组、请求 ID 与时间范围过滤
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Param        month query string false "月份(YYYY-MM)"
// @Param        token_name query string false "令牌名称"
// @Param        responsible_name query string false "负责人名称"
// @Param        model_name query string false "模型名称"
// @Param        group query string false "分组"
// @Param        request_id query string false "请求 ID"
// @Param        start_timestamp query int false "起始时间戳(秒)"
// @Param        end_timestamp query int false "截止时间戳(秒)"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/billing/records [get]
// @Router       /admin/organizations/{id}/billing/records [get]
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
