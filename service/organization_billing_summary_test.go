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
package service

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func insertOrganizationBillingSummaryRecord(t *testing.T, organizationId int, token *model.Token, recordType string, usedQuotaDelta int, recordKey string, createdAt int64, sessionIds ...int) {
	t.Helper()
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	promptTokens := 0
	completionTokens := 0
	tokenCount := 0
	sessionId := 0
	if len(sessionIds) > 0 {
		sessionId = sessionIds[0]
	}
	if recordType == model.OrganizationBillingRecordTypeSettle {
		promptTokens = usedQuotaDelta
		completionTokens = usedQuotaDelta
		tokenCount = promptTokens + completionTokens
	}
	require.NoError(t, model.DB.Create(&model.OrganizationBillingRecord{
		OrganizationId:    organizationId,
		SessionId:         sessionId,
		RecordKey:         recordKey,
		RequestId:         recordKey,
		RecordType:        recordType,
		QuotaDelta:        0,
		UsedQuotaDelta:    usedQuotaDelta,
		UsageQuota:        usedQuotaDelta,
		TokenId:           token.Id,
		TokenName:         token.Name,
		ResponsibleUserId: token.ResponsibleUserId,
		CreatorUserId:     token.CreatorUserId,
		ModelName:         "gpt-4o",
		Group:             "default",
		PromptTokens:      promptTokens,
		CompletionTokens:  completionTokens,
		TokenCount:        tokenCount,
		CreatedAt:         createdAt,
	}).Error)
}

func clearOrganizationBillingSummaryFacts(t *testing.T, organizationId int) {
	t.Helper()
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("organization_id = ?", organizationId).Delete(&model.Log{}).Error)
	require.NoError(t, model.DB.Where("organization_id = ?", organizationId).Delete(&model.OrganizationBillingRecord{}).Error)
}

func TestOrganizationBillingSummaryDoesNotDependOnResponsibleUserUsedQuota(t *testing.T) {
	admin, member, organization, memberToken, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Updates(map[string]any{"used_quota": 9999, "request_count": 9999}).Error)
	insertOrganizationConsumeLog(t, organization.Id, memberToken, 15, "summary")

	summary, err := GetOrganizationBillingSummary(admin.Id, organization.Id, OrganizationAccessModeWorkspace)

	require.NoError(t, err)
	require.Equal(t, 45, summary.CurrentMonthQuota)
}

func TestOrganizationBillingSummaryIgnoresLogsWithoutBillingRecords(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationBillingRecord{}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId:             memberToken.ResponsibleUserId,
		CreatedAt:          time.Now().Unix(),
		Type:               model.LogTypeConsume,
		TokenId:            memberToken.Id,
		TokenName:          memberToken.Name,
		Quota:              999,
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            organization.Id,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organization.Id,
		OrganizationId:     organization.Id,
		ResponsibleUserId:  memberToken.ResponsibleUserId,
	}).Error)

	summary, err := GetOrganizationBillingSummary(admin.Id, organization.Id, OrganizationAccessModeWorkspace)

	require.NoError(t, err)
	require.Zero(t, summary.CurrentMonthQuota)
}

func TestOrganizationBillingSummariesUseSessionSettledRecordUsage(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "summary-session-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "summary-session-settle"
	relayInfo.OriginModelName = "gpt-summary"
	relayInfo.UsingGroup = "default"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)

	require.NoError(t, session.Settle(100))

	summary, err := GetOrganizationBillingSummary(admin.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	require.Equal(t, 100, summary.CurrentMonthQuota)
	memberSummary, err := GetMyOrganizationMemberBilling(member.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	require.Equal(t, 100, memberSummary.CurrentMonthQuota)
	require.Equal(t, 1, memberSummary.RequestCount)
	userSummary, err := ListOrganizationBillingUserSummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingUserSummaryRequest{Month: time.Now().Format("2006-01")})
	require.NoError(t, err)
	var userItem *OrganizationBillingUserSummaryItem
	for i := range userSummary.Items {
		if userSummary.Items[i].ResponsibleUserId == member.Id {
			userItem = &userSummary.Items[i]
		}
	}
	require.NotNil(t, userItem)
	require.Equal(t, 100, userItem.Quota)
	require.Equal(t, 1, userItem.RequestCount)
	monthlySummary, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 1})
	require.NoError(t, err)
	require.Len(t, monthlySummary.Items, 1)
	require.Equal(t, 100, monthlySummary.Items[0].Quota)
	require.Equal(t, 1, monthlySummary.Items[0].RequestCount)
	records, total, err := ListOrganizationBillingDetails(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingDetailListRequest{Limit: 20, RequestId: "summary-session-settle"})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, records, 2)
	recordsByType := make(map[string]*model.OrganizationBillingRecord, len(records))
	for _, record := range records {
		recordsByType[record.RecordType] = record
	}
	require.Equal(t, 100, recordsByType[model.OrganizationBillingRecordTypePreConsume].UsedQuotaDelta)
	require.Zero(t, recordsByType[model.OrganizationBillingRecordTypePreConsume].UsageQuota)
	require.Equal(t, 100, recordsByType[model.OrganizationBillingRecordTypePreConsume].LedgerQuotaDelta)
	require.Zero(t, recordsByType[model.OrganizationBillingRecordTypeSettle].UsedQuotaDelta)
	require.Equal(t, 100, recordsByType[model.OrganizationBillingRecordTypeSettle].UsageQuota)
	require.Zero(t, recordsByType[model.OrganizationBillingRecordTypeSettle].LedgerQuotaDelta)
}

func TestOrganizationBillingDetailsUseHistoricalSettleUsageAsLedgerDelta(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	require.NoError(t, model.DB.Create(&model.OrganizationBillingRecord{
		OrganizationId:    organization.Id,
		SessionId:         904,
		RecordKey:         "legacy-settle-ledger",
		RequestId:         "legacy-settle-ledger",
		RecordType:        model.OrganizationBillingRecordTypeSettle,
		UsedQuotaDelta:    40,
		UsageQuota:        140,
		TokenId:           memberToken.Id,
		TokenName:         memberToken.Name,
		ResponsibleUserId: memberToken.ResponsibleUserId,
		CreatedAt:         time.Now().Unix(),
	}).Error)

	records, total, err := ListOrganizationBillingDetails(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingDetailListRequest{Limit: 20, RequestId: "legacy-settle-ledger"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, records, 1)
	require.Equal(t, 140, records[0].LedgerQuotaDelta)
}

func TestOrganizationBillingSummariesUseRecordTokenUsageWithoutLogs(t *testing.T) {
	previousLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() {
		common.LogConsumeEnabled = previousLogConsumeEnabled
	})
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "summary-token-usage-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "summary-record-token-usage"
	relayInfo.OriginModelName = "gpt-token-usage"
	relayInfo.UsingGroup = "default"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(100))
	model.RecordConsumeLog(&gin.Context{}, relayInfo.UserId, RelayConsumeLogParams(relayInfo, model.RecordConsumeLogParams{
		PromptTokens:     7,
		CompletionTokens: 11,
		ModelName:        "gpt-token-usage",
		TokenName:        token.Name,
		Quota:            100,
		TokenId:          token.Id,
		Group:            "default",
	}))

	userSummary, err := ListOrganizationBillingUserSummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingUserSummaryRequest{Month: time.Now().Format("2006-01")})
	require.NoError(t, err)
	var userItem *OrganizationBillingUserSummaryItem
	for i := range userSummary.Items {
		if userSummary.Items[i].ResponsibleUserId == member.Id {
			userItem = &userSummary.Items[i]
		}
	}
	require.NotNil(t, userItem)
	require.Equal(t, 7, userItem.PromptTokens)
	require.Equal(t, 11, userItem.CompletionTokens)
	require.Equal(t, 1, userItem.TokenCount)
	monthlySummary, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 1})
	require.NoError(t, err)
	require.Len(t, monthlySummary.Items, 1)
	require.Equal(t, 7, monthlySummary.Items[0].PromptTokens)
	require.Equal(t, 11, monthlySummary.Items[0].CompletionTokens)
	require.Equal(t, 1, monthlySummary.Items[0].TokenCount)
}

func TestOrganizationMemberBillingOnlyOwnBilling(t *testing.T) {
	_, member, organization, _, _ := createOrganizationLogFixture(t)

	summary, err := GetMyOrganizationMemberBilling(member.Id, organization.Id, OrganizationAccessModeWorkspace)

	require.NoError(t, err)
	require.Equal(t, member.Id, summary.ResponsibleUserId)
	require.Equal(t, 1, summary.ResponsibleKeyCount)
	require.Equal(t, 10, summary.CurrentMonthQuota)
	require.Equal(t, 1, summary.RequestCount)
}

func TestOrganizationMemberBillingRejectsDisabledMemberDirectServiceAccess(t *testing.T) {
	admin, member, organization, _, _ := createOrganizationLogFixture(t)
	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	summary, err := GetMyOrganizationMemberBilling(member.Id, organization.Id, OrganizationAccessModeWorkspace)

	require.ErrorContains(t, err, "organization disabled")
	require.Nil(t, summary)
}

func TestOrganizationBillingSummaryAdminCanSeeMemberOverview(t *testing.T) {
	admin, member, organization, _, _ := createOrganizationLogFixture(t)

	summary, err := GetOrganizationBillingSummary(admin.Id, organization.Id, OrganizationAccessModeWorkspace)

	require.NoError(t, err)
	require.Equal(t, organization.Quota, summary.Quota)
	require.Equal(t, organization.Quota-organization.UsedQuota, summary.AvailableQuota)
	require.EqualValues(t, 2, summary.OrganizationKeys)
	require.Len(t, summary.Members, 2)
	var found bool
	for _, item := range summary.Members {
		if item.ResponsibleUserId == member.Id {
			found = true
			require.Equal(t, 10, item.Quota)
			require.Equal(t, 1, item.RequestCount)
			require.Equal(t, 1, item.TokenCount)
		}
	}
	require.True(t, found)
}

func TestOrganizationBillingSummaryMemberCannotSeeOrganizationOverview(t *testing.T) {
	_, member, organization, _, _ := createOrganizationLogFixture(t)

	summary, err := GetOrganizationBillingSummary(member.Id, organization.Id, OrganizationAccessModeWorkspace)

	require.ErrorContains(t, err, "permission denied")
	require.Nil(t, summary)
}

func TestOrganizationBillingUserSummariesAdminSeesAllActiveMembers(t *testing.T) {
	admin, member, organization, memberToken, adminToken := createOrganizationLogFixture(t)
	now := time.Now()
	month := now.Format("2006-01")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Updates(map[string]any{"username": "billing-user-owner", "display_name": "Billing User Owner"}).Error)
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 17, "member", model.Log{CreatedAt: now.Unix(), PromptTokens: 3, CompletionTokens: 4, RequestId: "user-summary-member"})
	insertOrganizationLog(t, organization.Id, adminToken, model.LogTypeConsume, 19, "admin", model.Log{CreatedAt: now.Unix(), PromptTokens: 5, CompletionTokens: 6, RequestId: "user-summary-admin"})

	response, err := ListOrganizationBillingUserSummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingUserSummaryRequest{Month: month})

	require.NoError(t, err)
	require.Equal(t, month, response.Month)
	require.Len(t, response.Items, 2)
	var memberItem *OrganizationBillingUserSummaryItem
	for i := range response.Items {
		if response.Items[i].ResponsibleUserId == member.Id {
			memberItem = &response.Items[i]
		}
	}
	require.NotNil(t, memberItem)
	require.Equal(t, "billing-user-owner", memberItem.ResponsibleUsername)
	require.Equal(t, "Billing User Owner", memberItem.ResponsibleDisplayName)
	require.Equal(t, model.OrganizationRoleMember, memberItem.Role)
	require.Equal(t, model.OrganizationMemberStatusActive, memberItem.Status)
	require.Equal(t, 17, memberItem.Quota)
	require.Equal(t, 1, memberItem.RequestCount)
	require.Equal(t, 3, memberItem.PromptTokens)
	require.Equal(t, 4, memberItem.CompletionTokens)
	require.Equal(t, 1, memberItem.TokenCount)
}

func TestOrganizationBillingUserSummariesMemberOnlyOwnOverview(t *testing.T) {
	_, member, organization, memberToken, adminToken := createOrganizationLogFixture(t)
	now := time.Now()
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 23, "member", model.Log{CreatedAt: now.Unix(), RequestId: "user-summary-own"})
	insertOrganizationLog(t, organization.Id, adminToken, model.LogTypeConsume, 29, "admin", model.Log{CreatedAt: now.Unix(), RequestId: "user-summary-admin"})

	response, err := ListOrganizationBillingUserSummaries(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingUserSummaryRequest{Month: now.Format("2006-01")})

	require.NoError(t, err)
	require.Len(t, response.Items, 1)
	require.Equal(t, member.Id, response.Items[0].ResponsibleUserId)
	require.Equal(t, 23, response.Items[0].Quota)
}

func TestOrganizationBillingMonthlySummariesUseRecordMonthSemantics(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	previousMonthStart := currentMonthStart.AddDate(0, -1, 0)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("created_at", previousMonthStart.Add(time.Hour).Unix()).Error)
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationBillingRecord{}).Error)
	insertOrganizationBillingSummaryRecord(t, organization.Id, memberToken, model.OrganizationBillingRecordTypeSettle, 100, "semantic-settle-previous", previousMonthStart.Add(2*time.Hour).Unix(), 901)
	insertOrganizationBillingSummaryRecord(t, organization.Id, memberToken, model.OrganizationBillingRecordTypeRefund, -40, "semantic-refund-current", currentMonthStart.Add(2*time.Hour).Unix(), 901)
	insertOrganizationBillingSummaryRecord(t, organization.Id, memberToken, model.OrganizationBillingRecordTypeRefund, -25, "semantic-unsettled-refund-current", currentMonthStart.Add(2*time.Hour).Unix(), 902)
	require.NoError(t, model.DB.Create(&model.OrganizationBillingRecord{
		OrganizationId: organization.Id,
		RecordKey:      "semantic-adjust-current",
		RecordType:     model.OrganizationBillingRecordTypeAdjustment,
		QuotaDelta:     500,
		CreatedAt:      currentMonthStart.Add(3 * time.Hour).Unix(),
	}).Error)

	response, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 2})

	require.NoError(t, err)
	require.Len(t, response.Items, 2)
	require.Equal(t, currentMonthStart.Format("2006-01"), response.Items[0].Month)
	require.Equal(t, -40, response.Items[0].Quota)
	require.Zero(t, response.Items[0].RequestCount)
	require.Zero(t, response.Items[0].ResponsibleUserCount)
	require.Equal(t, previousMonthStart.Format("2006-01"), response.Items[1].Month)
	require.Equal(t, 100, response.Items[1].Quota)
	require.Equal(t, 1, response.Items[1].RequestCount)
}

func TestOrganizationBillingSummariesIgnoreHistoricalRefundWithoutSettle(t *testing.T) {
	admin, member, organization, memberToken, _ := createOrganizationLogFixture(t)
	now := time.Now()
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationBillingSummaryRecord(t, organization.Id, memberToken, model.OrganizationBillingRecordTypeRefund, -60, "historical-unsettled-refund", now.Unix(), 903)
	require.NoError(t, model.DB.Create(&model.OrganizationBillingRecord{
		OrganizationId: organization.Id + 1000,
		SessionId:      903,
		RecordKey:      "other-organization-same-session-settle",
		RecordType:     model.OrganizationBillingRecordTypeSettle,
		UsageQuota:     60,
		CreatedAt:      now.Unix(),
	}).Error)

	summary, err := GetOrganizationBillingSummary(admin.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	require.Zero(t, summary.CurrentMonthQuota)

	memberSummary, err := GetMyOrganizationMemberBilling(member.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	require.Zero(t, memberSummary.CurrentMonthQuota)
	require.Zero(t, memberSummary.RequestCount)

	userSummary, err := ListOrganizationBillingUserSummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingUserSummaryRequest{Month: now.Format("2006-01")})
	require.NoError(t, err)
	for _, item := range userSummary.Items {
		if item.ResponsibleUserId == member.Id {
			require.Zero(t, item.Quota)
			require.Zero(t, item.RequestCount)
		}
	}

	monthlySummary, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 1})
	require.NoError(t, err)
	require.Len(t, monthlySummary.Items, 1)
	require.Zero(t, monthlySummary.Items[0].Quota)
	require.Zero(t, monthlySummary.Items[0].ResponsibleUserCount)
}

func TestOrganizationBillingMonthlySummariesUseApplicationTimezoneUnixRanges(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	previousLocal := time.Local
	time.Local = location
	t.Cleanup(func() { time.Local = previousLocal })
	t.Setenv("TZ", "UTC")
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	now := time.Now().In(location)
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	previousMonthStart := currentMonthStart.AddDate(0, -1, 0)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("created_at", previousMonthStart.Add(time.Hour).Unix()).Error)
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationBillingSummaryRecord(t, organization.Id, memberToken, model.OrganizationBillingRecordTypeSettle, 31, "application-timezone-current", currentMonthStart.Add(30*time.Minute).Unix())
	insertOrganizationBillingSummaryRecord(t, organization.Id, memberToken, model.OrganizationBillingRecordTypeSettle, 7, "application-timezone-previous", currentMonthStart.Add(-30*time.Minute).Unix())

	response, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 2})

	require.NoError(t, err)
	require.Len(t, response.Items, 2)
	require.Equal(t, currentMonthStart.Format("2006-01"), response.Items[0].Month)
	require.Equal(t, currentMonthStart.Unix(), response.Items[0].MonthStart)
	require.Equal(t, currentMonthStart.AddDate(0, 1, 0).Unix()-1, response.Items[0].MonthEnd)
	require.Equal(t, 31, response.Items[0].Quota)
	require.Equal(t, 1, response.Items[0].ResponsibleUserCount)
	require.Equal(t, previousMonthStart.Format("2006-01"), response.Items[1].Month)
	require.Equal(t, 7, response.Items[1].Quota)
	require.Equal(t, 1, response.Items[1].ResponsibleUserCount)
}

func TestOrganizationBillingUserSummariesSupportsMonthRange(t *testing.T) {
	admin, member, organization, memberToken, _ := createOrganizationLogFixture(t)
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	previousMonthStart := currentMonthStart.AddDate(0, -1, 0)
	beforeRangeMonthStart := currentMonthStart.AddDate(0, -2, 0)
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 31, "current", model.Log{CreatedAt: currentMonthStart.Add(time.Hour).Unix(), RequestId: "user-summary-current"})
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 37, "previous", model.Log{CreatedAt: previousMonthStart.Add(time.Hour).Unix(), RequestId: "user-summary-previous"})
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 99, "before-range", model.Log{CreatedAt: beforeRangeMonthStart.Add(time.Hour).Unix(), RequestId: "user-summary-before-range"})

	response, err := ListOrganizationBillingUserSummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingUserSummaryRequest{StartMonth: previousMonthStart.Format("2006-01"), EndMonth: currentMonthStart.Format("2006-01")})

	require.NoError(t, err)
	require.Equal(t, previousMonthStart.Format("2006-01")+"~"+currentMonthStart.Format("2006-01"), response.Month)
	var memberItem *OrganizationBillingUserSummaryItem
	for i := range response.Items {
		if response.Items[i].ResponsibleUserId == member.Id {
			memberItem = &response.Items[i]
		}
	}
	require.NotNil(t, memberItem)
	require.Equal(t, 68, memberItem.Quota)
	require.Equal(t, 2, memberItem.RequestCount)
}

func TestOrganizationBillingMonthlySummariesAdminReturnsRecentMonthFirst(t *testing.T) {
	admin, _, organization, memberToken, adminToken := createOrganizationLogFixture(t)
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	previousMonthStart := currentMonthStart.AddDate(0, -1, 0)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("created_at", previousMonthStart.Add(12*time.Hour).Unix()).Error)
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 11, "previous", model.Log{CreatedAt: previousMonthStart.Add(2 * time.Hour).Unix(), PromptTokens: 3, CompletionTokens: 4, RequestId: "billing-previous"})
	insertOrganizationLog(t, organization.Id, adminToken, model.LogTypeConsume, 13, "current", model.Log{CreatedAt: currentMonthStart.Add(2 * time.Hour).Unix(), PromptTokens: 5, CompletionTokens: 6, RequestId: "billing-current"})

	response, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 2})

	require.NoError(t, err)
	require.Len(t, response.Items, 2)
	require.Equal(t, currentMonthStart.Format("2006-01"), response.Items[0].Month)
	require.Equal(t, 13, response.Items[0].Quota)
	require.Equal(t, 1, response.Items[0].RequestCount)
	require.Equal(t, 5, response.Items[0].PromptTokens)
	require.Equal(t, 6, response.Items[0].CompletionTokens)
	require.Equal(t, 1, response.Items[0].TokenCount)
	require.Equal(t, 1, response.Items[0].ResponsibleUserCount)
	require.Equal(t, previousMonthStart.Format("2006-01"), response.Items[1].Month)
	require.Equal(t, 11, response.Items[1].Quota)
}

func TestOrganizationBillingMonthlySummariesMemberOnlyOwnBilling(t *testing.T) {
	_, member, organization, memberToken, adminToken := createOrganizationLogFixture(t)
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 17, "member", model.Log{CreatedAt: currentMonthStart.Add(time.Hour).Unix(), RequestId: "member-billing"})
	insertOrganizationLog(t, organization.Id, adminToken, model.LogTypeConsume, 19, "admin", model.Log{CreatedAt: currentMonthStart.Add(2 * time.Hour).Unix(), RequestId: "admin-billing"})

	response, err := ListOrganizationBillingMonthlySummaries(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 1})

	require.NoError(t, err)
	require.Len(t, response.Items, 1)
	require.Equal(t, 17, response.Items[0].Quota)
	require.Equal(t, 1, response.Items[0].RequestCount)
	require.Equal(t, 1, response.Items[0].TokenCount)
	require.Equal(t, 1, response.Items[0].ResponsibleUserCount)
}

func TestOrganizationBillingMonthlySummariesExcludePersonalAndOtherOrganizationBilling(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 23, "target", model.Log{CreatedAt: currentMonthStart.Add(time.Hour).Unix(), RequestId: "target-billing"})
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: admin.Id, CreatedAt: currentMonthStart.Add(time.Hour).Unix(), Type: model.LogTypeConsume, Quota: 99, ScopeType: model.AccountContextTypePersonal, ScopeId: admin.Id, BillingAccountType: model.AccountContextTypePersonal, BillingAccountId: admin.Id, OrganizationId: 0}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: admin.Id, CreatedAt: currentMonthStart.Add(time.Hour).Unix(), Type: model.LogTypeConsume, Quota: 88, ScopeType: model.AccountContextTypeOrganization, ScopeId: organization.Id + 1000, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: organization.Id + 1000, OrganizationId: organization.Id + 1000}).Error)

	response, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 1})

	require.NoError(t, err)
	require.Len(t, response.Items, 1)
	require.Equal(t, 23, response.Items[0].Quota)
}

func TestOrganizationBillingMonthlySummariesStartFromOrganizationCreatedMonth(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	createdMonthStart := currentMonthStart.AddDate(0, -2, 0)
	beforeCreatedMonthStart := currentMonthStart.AddDate(0, -3, 0)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("created_at", createdMonthStart.Add(12*time.Hour).Unix()).Error)
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 101, "created-month", model.Log{CreatedAt: createdMonthStart.Add(time.Hour).Unix(), RequestId: "created-month"})
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 999, "before-created", model.Log{CreatedAt: beforeCreatedMonthStart.Add(time.Hour).Unix(), RequestId: "before-created"})

	response, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 12})

	require.NoError(t, err)
	require.Len(t, response.Items, 3)
	require.Equal(t, currentMonthStart.Format("2006-01"), response.Items[0].Month)
	require.Equal(t, currentMonthStart.AddDate(0, -1, 0).Format("2006-01"), response.Items[1].Month)
	require.Equal(t, createdMonthStart.Format("2006-01"), response.Items[2].Month)
	require.Equal(t, 101, response.Items[2].Quota)
	for _, item := range response.Items {
		require.NotEqual(t, beforeCreatedMonthStart.Format("2006-01"), item.Month)
	}
}

func TestOrganizationBillingMonthlySummariesNormalizeMonths(t *testing.T) {
	admin, _, organization, _, _ := createOrganizationLogFixture(t)
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("created_at", currentMonthStart.AddDate(0, -40, 0).Unix()).Error)

	defaultResponse, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 0})
	require.NoError(t, err)
	require.Len(t, defaultResponse.Items, 12)

	cappedResponse, err := ListOrganizationBillingMonthlySummaries(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingMonthlySummaryRequest{Months: 100})
	require.NoError(t, err)
	require.Len(t, cappedResponse.Items, 36)
}

func TestOrganizationBillingDetailsAdminCanPageAndFilter(t *testing.T) {
	admin, member, organization, memberToken, adminToken := createOrganizationLogFixture(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Updates(map[string]interface{}{"username": "billing-owner", "display_name": "Billing Owner"}).Error)
	now := time.Now()
	month := now.Format("2006-01")
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 31, "target", model.Log{CreatedAt: now.Unix(), TokenName: "billing-key-target", ModelName: "gpt-billing", Group: "vip", RequestId: "billing-detail-target", PromptTokens: 7, CompletionTokens: 8})
	insertOrganizationLog(t, organization.Id, adminToken, model.LogTypeConsume, 41, "other", model.Log{CreatedAt: now.Unix(), TokenName: "billing-key-other", ModelName: "gpt-billing", Group: "vip", RequestId: "billing-detail-other"})

	logs, total, err := ListOrganizationBillingDetails(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingDetailListRequest{Limit: 1, Month: month, TokenName: "billing-key-target", ResponsibleName: "Billing Owner", ModelName: "gpt-billing", Group: "vip", RequestId: "billing-detail-target"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "billing-key-target", logs[0].TokenName)
	require.Equal(t, member.Id, logs[0].ResponsibleUserId)
	require.Equal(t, "billing-owner", logs[0].ResponsibleUsername)
	require.Equal(t, "Billing Owner", logs[0].ResponsibleDisplayName)
	require.Equal(t, model.OrganizationBillingRecordTypeSettle, logs[0].RecordType)
	require.Equal(t, 31, logs[0].UsedQuotaDelta)
	require.Equal(t, 31, logs[0].UsageQuota)
}

func TestOrganizationBillingDetailsResponsibleNameUsesUsersTable(t *testing.T) {
	admin, member, organization, _, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Updates(map[string]interface{}{
		"username":     "billing-record-user",
		"display_name": "Billing Record User",
	}).Error)

	logs, total, err := ListOrganizationBillingDetails(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingDetailListRequest{Limit: 20, ResponsibleName: "Billing Record User"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "billing-record-user", logs[0].ResponsibleUsername)
	require.Equal(t, "Billing Record User", logs[0].ResponsibleDisplayName)
}

func TestOrganizationBillingDetailsMemberOnlyOwnBilling(t *testing.T) {
	admin, member, organization, memberToken, adminToken := createOrganizationLogFixture(t)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 37, "member", model.Log{RequestId: "member-detail"})
	insertOrganizationLog(t, organization.Id, adminToken, model.LogTypeConsume, 47, "admin", model.Log{RequestId: "admin-detail"})

	logs, total, err := ListOrganizationBillingDetails(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingDetailListRequest{Limit: 20, ResponsibleName: admin.Username})

	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	for _, log := range logs {
		require.Equal(t, member.Id, log.ResponsibleUserId)
	}
}

func TestOrganizationBillingDetailsExcludeNonConsumePersonalAndOtherOrganization(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	clearOrganizationBillingSummaryFacts(t, organization.Id)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 53, "target", model.Log{RequestId: "target-detail"})
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeError, 0, "error", model.Log{RequestId: "error-detail"})
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: admin.Id, CreatedAt: time.Now().Unix(), Type: model.LogTypeConsume, Quota: 99, ScopeType: model.AccountContextTypePersonal, ScopeId: admin.Id, BillingAccountType: model.AccountContextTypePersonal, BillingAccountId: admin.Id}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: admin.Id, CreatedAt: time.Now().Unix(), Type: model.LogTypeConsume, Quota: 88, ScopeType: model.AccountContextTypeOrganization, ScopeId: organization.Id + 1000, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: organization.Id + 1000, OrganizationId: organization.Id + 1000}).Error)

	logs, total, err := ListOrganizationBillingDetails(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingDetailListRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "target-detail", logs[0].RequestId)
}

func TestOrganizationBillingDetailsInvalidMonthFails(t *testing.T) {
	admin, _, organization, _, _ := createOrganizationLogFixture(t)

	_, _, err := ListOrganizationBillingDetails(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingDetailListRequest{Month: "2026/05"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid billing month")
}
