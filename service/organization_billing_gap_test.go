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
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaykittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOrganizationBillingSettledRefundRestoresSettledQuotaAndTokenOnce(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "settled-refund-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "settled-refund"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(140))

	funding := session.funding.(*OrganizationFunding)
	require.NoError(t, funding.RefundWithToken())
	require.NoError(t, funding.RefundWithToken())

	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, storedSession.Status)
	require.Equal(t, 140, storedSession.RefundedQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 1000, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
	var refundRecords int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("record_key = ?", "refund:"+strconv.Itoa(storedSession.Id)).Count(&refundRecords).Error)
	require.EqualValues(t, 1, refundRecords)
	var records []model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("session_id = ?", storedSession.Id).Order("id asc").Find(&records).Error)
	require.Len(t, records, 3)
	require.Equal(t, model.OrganizationBillingRecordTypePreConsume, records[0].RecordType)
	require.Equal(t, 100, records[0].UsedQuotaDelta)
	require.Zero(t, records[0].UsageQuota)
	require.Equal(t, model.OrganizationBillingRecordTypeSettle, records[1].RecordType)
	require.Equal(t, 40, records[1].UsedQuotaDelta)
	require.Equal(t, 140, records[1].UsageQuota)
	require.Equal(t, model.OrganizationBillingRecordTypeRefund, records[2].RecordType)
	require.Equal(t, -140, records[2].UsedQuotaDelta)
	require.Equal(t, -140, records[2].UsageQuota)
	require.Zero(t, records[0].UsedQuotaDelta+records[1].UsedQuotaDelta+records[2].UsedQuotaDelta)
	require.Zero(t, records[0].UsageQuota+records[1].UsageQuota+records[2].UsageQuota)
}

func TestOrganizationSettleSameActualReplaySucceeds(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "settle-same-actual-replay", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "settle-same-actual-replay"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(140))

	replayedRelayInfo := *relayInfo
	replayedRelayInfo.Billing = nil
	require.NoError(t, (&OrganizationFunding{relayInfo: &replayedRelayInfo}).SettleWithToken(40))

	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, storedSession.Status)
	require.Equal(t, 140, storedSession.SettledQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 860, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)
}

func TestOrganizationSettleDifferentActualReplayConflictsInMemory(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "settle-different-actual-in-memory", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "settle-different-actual-in-memory"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(140))

	err = session.Settle(160)

	require.EqualError(t, err, "organization idempotency conflict")
	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, storedSession.Status)
	require.Equal(t, 140, storedSession.SettledQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 860, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)
}

func TestOrganizationViolationFeeUsesDedicatedIdempotentBillingSession(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	feeQuota := configureViolationFeeQuota(t, 25)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "violation-fee-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	require.NotNil(t, token)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	require.NotNil(t, relayInfo)
	relayInfo.RequestId = "violation-fee-request"
	relayInfo.StartTime = time.Now()
	relayInfo.PriceData.GroupRatioInfo.GroupRatio = 1
	apiErr := organizationViolationFeeAPIError()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	require.Nil(t, PreConsumeBilling(ctx, 10, relayInfo))
	originalBilling := relayInfo.Billing
	originalSessionId := relayInfo.OrganizationBillingSessionId
	originalSessionKey := relayInfo.OrganizationBillingSessionKey
	require.NoError(t, originalBilling.(*BillingSession).funding.(*OrganizationFunding).RefundWithToken())
	require.True(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, apiErr))
	require.True(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, apiErr))

	require.Equal(t, originalSessionId, relayInfo.OrganizationBillingSessionId, "fee billing must not overwrite the original session reference")
	require.Equal(t, originalSessionKey, relayInfo.OrganizationBillingSessionKey)
	require.Same(t, originalBilling, relayInfo.Billing)
	var originalSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&originalSession, originalSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, originalSession.Status)
	var sessions []model.OrganizationBillingSession
	require.NoError(t, model.DB.Where("idempotency_key = ?", "request:violation-fee-request:violation_fee").Find(&sessions).Error)
	require.Len(t, sessions, 1)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, sessions[0].Status)
	require.Equal(t, "violation-fee-request", sessions[0].RequestId)
	require.Equal(t, feeQuota, sessions[0].SettledQuota)
	var settleRecords []model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("session_id = ? AND record_type = ?", sessions[0].Id, model.OrganizationBillingRecordTypeSettle).Find(&settleRecords).Error)
	require.Len(t, settleRecords, 1)
	require.Equal(t, feeQuota, settleRecords[0].UsageQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+feeQuota, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 1000-feeQuota, storedToken.RemainQuota)
	require.Equal(t, feeQuota, storedToken.UsedQuota)
}

func TestOrganizationViolationFeeReplayFillsMissingLogWithoutDoubleCharge(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
	channel := model.Channel{Name: "violation-fee-replay-channel", Key: "test"}
	require.NoError(t, model.DB.Create(&channel).Error)
	feeQuota := configureViolationFeeQuota(t, 25)
	previousDataExportEnabled := common.DataExportEnabled
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false
	t.Cleanup(func() { common.DataExportEnabled = previousDataExportEnabled })
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "violation-fee-replay-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "violation-fee-replay"
	relayInfo.StartTime = time.Now()
	relayInfo.PriceData.GroupRatioInfo.GroupRatio = 1
	relayInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: channel.Id}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, relayInfo.RequestId)
	ctx.Set("token_name", token.Name)

	require.True(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, organizationViolationFeeAPIError()))
	var session model.OrganizationBillingSession
	require.NoError(t, model.DB.Where("idempotency_key = ?", "request:violation-fee-replay:violation_fee").First(&session).Error)
	require.Equal(t, channel.Id, session.ChannelId)
	require.NoError(t, model.LOG_DB.Where("organization_billing_session_id = ?", session.Id).Delete(&model.Log{}).Error)
	replayedChannel := model.Channel{Name: "violation-fee-replayed-channel", Key: "test"}
	require.NoError(t, model.DB.Create(&replayedChannel).Error)
	relayInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: replayedChannel.Id}
	relayInfo.ResponsibleUserId = member.Id + 1000
	relayInfo.CreatorUserId = member.Id + 1001
	relayInfo.OriginModelName = "replayed-model"
	relayInfo.UsingGroup = "replayed-group"
	replayCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	replayCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	replayCtx.Set(common.RequestIdKey, "different-replay-request-id")
	replayCtx.Set("token_name", "replayed-token-name")
	require.True(t, ChargeViolationFeeIfNeeded(replayCtx, relayInfo, organizationViolationFeeAPIError()))

	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("organization_billing_session_id = ?", session.Id).Find(&logs).Error)
	require.Len(t, logs, 1)
	log := logs[0]
	require.NotNil(t, log.BillingEventKey)
	require.Equal(t, "violation_fee:"+strconv.Itoa(session.Id), *log.BillingEventKey)
	require.Equal(t, relayInfo.RequestId, log.RequestId)
	require.Equal(t, session.Id, log.OrganizationBillingSessionId)
	require.Equal(t, session.IdempotencyKey, log.OrganizationBillingSessionKey)
	require.Equal(t, channel.Id, log.ChannelId)
	require.Equal(t, token.Id, log.TokenId)
	require.Equal(t, token.Name, log.TokenName)
	require.Equal(t, session.ModelName, log.ModelName)
	require.Equal(t, session.Group, log.Group)
	require.Equal(t, model.AccountContextTypeOrganization, log.ScopeType)
	require.Equal(t, organization.Id, log.ScopeId)
	require.Equal(t, model.AccountContextTypeOrganization, log.BillingAccountType)
	require.Equal(t, organization.Id, log.BillingAccountId)
	require.Equal(t, organization.Id, log.OrganizationId)
	require.Equal(t, relayInfo.ActorUserId, log.ActorUserId)
	require.Equal(t, session.CreatorUserId, log.CreatorUserId)
	require.Equal(t, session.ResponsibleUserId, log.ResponsibleUserId)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+feeQuota, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 1000-feeQuota, storedToken.RemainQuota)
	require.Equal(t, feeQuota, storedToken.UsedQuota)
	require.NoError(t, model.DB.First(&channel, channel.Id).Error)
	require.EqualValues(t, feeQuota, channel.UsedQuota)
	require.NoError(t, model.DB.First(&replayedChannel, replayedChannel.Id).Error)
	require.Zero(t, replayedChannel.UsedQuota)
}

func TestOrganizationViolationFeeTokenFailureRollsBackOrganizationQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	feeQuota := configureViolationFeeQuota(t, 25)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "violation-fee-low-token", ExpiredTime: -1, RemainQuota: feeQuota - 1})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "violation-fee-token-failure"
	relayInfo.StartTime = time.Now()
	relayInfo.PriceData.GroupRatioInfo.GroupRatio = 1
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	require.False(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, organizationViolationFeeAPIError()))

	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, feeQuota-1, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
	var sessionCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("idempotency_key = ?", "request:violation-fee-token-failure:violation_fee").Count(&sessionCount).Error)
	require.Zero(t, sessionCount)
}

func TestOrganizationViolationFeeUnlimitedTokenOnlyChargesOrganization(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	feeQuota := configureViolationFeeQuota(t, 25)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "violation-fee-unlimited", ExpiredTime: -1, UnlimitedQuota: true})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "violation-fee-unlimited"
	relayInfo.StartTime = time.Now()
	relayInfo.PriceData.GroupRatioInfo.GroupRatio = 1
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	require.True(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, organizationViolationFeeAPIError()))

	var session model.OrganizationBillingSession
	require.NoError(t, model.DB.Where("idempotency_key = ?", "request:violation-fee-unlimited:violation_fee").First(&session).Error)
	require.True(t, session.TokenUnlimitedQuota)
	require.Zero(t, session.TokenPreConsumedQuota)
	require.Zero(t, session.TokenRemainDeductedQuota)
	require.Equal(t, feeQuota, session.SettledQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+feeQuota, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Zero(t, storedToken.RemainQuota)
	require.Equal(t, feeQuota, storedToken.UsedQuota)
}

func configureViolationFeeQuota(t *testing.T, quota int) int {
	t.Helper()
	previousLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = previousLogConsumeEnabled })
	settings := model_setting.GetGrokSettings()
	previous := *settings
	t.Cleanup(func() { *settings = previous })
	settings.ViolationDeductionEnabled = true
	settings.ViolationDeductionAmount = float64(quota) / float64(common.QuotaPerUnit)
	actual := calcViolationFeeQuota(settings.ViolationDeductionAmount, 1)
	require.Equal(t, quota, actual)
	return actual
}

// ChargeViolationFeeIfNeeded 在 cuberouter 上走 relaykit 的 errors.NewAPIError，
// 罚金码在两个 types 包里都有声明，这里用 relay 侧那一份。
func organizationViolationFeeAPIError() *relaykittypes.NewAPIError {
	return relaykittypes.NewErrorWithStatusCode(errors.New(CSAMViolationMarker), relaykittypes.ErrorCodeViolationFeeGrokCSAM, http.StatusBadRequest)
}
