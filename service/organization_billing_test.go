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
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestOrganizationBillingInsufficientQuotaFails(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Updates(map[string]any{"quota": 10, "used_quota": 9}).Error)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)

	err = PreConsumeOrganizationQuota(relayInfo, 2)

	require.ErrorContains(t, err, "organization quota is not enough")
}

func TestOrganizationBillingSuccessIncreasesOrganizationUsedQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)

	require.NoError(t, PostConsumeQuota(relayInfo, 100, 0, false))

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+100, stored.UsedQuota)
	require.Equal(t, 1, stored.RequestCount)
}

func TestOrganizationBillingDoesNotIncreaseResponsibleUserUsage(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)

	require.NoError(t, PostConsumeQuota(relayInfo, 100, 0, false))

	var stored model.User
	require.NoError(t, model.DB.First(&stored, member.Id).Error)
	require.Zero(t, stored.UsedQuota)
	require.Zero(t, stored.RequestCount)
}

func TestOrganizationBillingRefundReducesUsedQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	require.NoError(t, PostConsumeQuota(relayInfo, 100, 0, false))

	require.NoError(t, PostConsumeQuota(relayInfo, -40, 0, false))

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+60, stored.UsedQuota)
}

func TestOrganizationBillingTokenRemainQuotaDeducted(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)

	require.NoError(t, PostConsumeQuota(relayInfo, 100, 0, false))

	stored, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 900, stored.RemainQuota)
	require.Equal(t, 100, stored.UsedQuota)
}

func TestOrganizationBillingTokenQuotaLookupRejectsMismatchedOrganization(t *testing.T) {
	owner, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:        "quota-scope-key",
		ExpiredTime: -1,
		RemainQuota: 1000,
	})
	require.NoError(t, err)
	otherOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Other Quota Scope Org"})
	require.NoError(t, err)

	relayInfo := organizationBillingRelayInfo(token, otherOrganization.Id)
	_, err = loadTokenForQuotaTx(model.DB, relayInfo)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestOrganizationBillingTokenFailureRollsBackOrganizationQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-failing-key", ExpiredTime: -1, RemainQuota: 100})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Updates(map[string]any{"quota": 100, "used_quota": 0, "request_count": 0}).Error)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	callbackName := "test:organization-post-consume-token-failure"
	forcedErr := errors.New("forced token quota update failure")
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "tokens" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	err = PostConsumeQuota(relayInfo, 10, 0, false)

	require.ErrorIs(t, err, forcedErr)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Zero(t, storedOrganization.UsedQuota)
	require.Zero(t, storedOrganization.RequestCount)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 100, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
}

func TestOrganizationBillingSessionTokenFailureRollsBackPreConsumeRecord(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "session-failing-key", ExpiredTime: -1, RemainQuota: 100})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "session-token-failure"
	callbackName := "test:organization-session-pre-consume-token-failure"
	forcedErr := errors.New("forced session token quota update failure")
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "tokens" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 50)

	require.NotNil(t, apiErr)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
	var sessions int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("request_id = ?", relayInfo.RequestId).Count(&sessions).Error)
	require.Zero(t, sessions)
	var records int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("request_id = ?", relayInfo.RequestId).Count(&records).Error)
	require.Zero(t, records)
}

func TestOrganizationBillingSessionRecordFailureRollsBackPreConsume(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "session-record-failing-key", ExpiredTime: -1, RemainQuota: 100})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "session-record-failure"
	callbackName := "test:organization-session-pre-consume-record-failure"
	forcedErr := errors.New("forced pre-consume record failure")
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "organization_billing_records" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove(callbackName) })

	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 50)

	require.NotNil(t, apiErr)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 100, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
	var sessions int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("request_id = ?", relayInfo.RequestId).Count(&sessions).Error)
	require.Zero(t, sessions)
	var records int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("request_id = ?", relayInfo.RequestId).Count(&records).Error)
	require.Zero(t, records)
}

func TestOrganizationBillingRecordConflictRollsBackTransaction(t *testing.T) {
	_, _, organization := createOrganizationTokenTestOrg(t)
	recordKey := "pre_consume:conflicting-record"
	existing := model.OrganizationBillingRecord{
		OrganizationId:  organization.Id,
		SessionId:       9001,
		RecordKey:       recordKey,
		RecordType:      model.OrganizationBillingRecordTypePreConsume,
		UsedQuotaDelta:  40,
		UsedQuotaBefore: organization.UsedQuota,
		UsedQuotaAfter:  organization.UsedQuota + 40,
		CreatedAt:       common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&existing).Error)

	desired := existing
	desired.Id = 0
	desired.UsedQuotaDelta = 50
	desired.UsedQuotaAfter = organization.UsedQuota + 50
	err := runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Organization{}).
			Where("id = ?", organization.Id).
			Update("used_quota", gorm.Expr("used_quota + ?", 50)).Error; err != nil {
			return err
		}
		return createOrganizationBillingRecordTx(tx, &desired)
	})

	require.Error(t, err)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
	var storedRecord model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("record_key = ?", recordKey).First(&storedRecord).Error)
	require.Equal(t, 40, storedRecord.UsedQuotaDelta)
}

func TestOrganizationBillingIdenticalRecordConflictRollsBackTransaction(t *testing.T) {
	_, _, organization := createOrganizationTokenTestOrg(t)
	existing := model.OrganizationBillingRecord{
		OrganizationId:  organization.Id,
		SessionId:       9002,
		RecordKey:       "pre_consume:identical-record",
		RecordType:      model.OrganizationBillingRecordTypePreConsume,
		UsedQuotaDelta:  40,
		UsedQuotaBefore: organization.UsedQuota,
		UsedQuotaAfter:  organization.UsedQuota + 40,
		CreatedAt:       common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&existing).Error)

	desired := existing
	desired.Id = 0
	err := runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Organization{}).
			Where("id = ?", organization.Id).
			Update("used_quota", gorm.Expr("used_quota + ?", 40)).Error; err != nil {
			return err
		}
		return createOrganizationBillingRecordTx(tx, &desired)
	})

	require.Error(t, err)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
}

func TestOrganizationBillingSessionUnlimitedTokenTracksSettledUsageAndRefund(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "unlimited-usage-key", ExpiredTime: -1, UnlimitedQuota: true})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "unlimited-usage-session"
	funding := &OrganizationFunding{relayInfo: relayInfo}

	require.NoError(t, funding.PreConsumeWithToken(80))
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Zero(t, storedToken.RemainQuota)
	require.Equal(t, 80, storedToken.UsedQuota)

	require.NoError(t, funding.SettleWithTokenActual(100))
	storedToken, err = model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Zero(t, storedToken.RemainQuota)
	require.Equal(t, 100, storedToken.UsedQuota)

	require.NoError(t, funding.RefundWithToken())
	require.NoError(t, funding.RefundWithToken())
	storedToken, err = model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Zero(t, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
}

func TestOrganizationBillingSessionUsesUnlimitedSnapshotAfterTokenBecomesLimited(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "unlimited-snapshot-key", ExpiredTime: -1, UnlimitedQuota: true})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "unlimited-snapshot-session"
	funding := &OrganizationFunding{relayInfo: relayInfo}

	require.NoError(t, funding.PreConsumeWithToken(80))
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]any{"unlimited_quota": false, "remain_quota": 10}).Error)
	require.NoError(t, funding.SettleWithTokenActual(100))
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 10, storedToken.RemainQuota)
	require.Equal(t, 100, storedToken.UsedQuota)

	require.NoError(t, funding.RefundWithToken())
	storedToken, err = model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 10, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
}

func TestOrganizationBillingSessionUsesLimitedSnapshotAfterTokenBecomesUnlimited(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "limited-snapshot-key", ExpiredTime: -1, RemainQuota: 200})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "limited-snapshot-session"
	funding := &OrganizationFunding{relayInfo: relayInfo}

	require.NoError(t, funding.PreConsumeWithToken(80))
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Update("unlimited_quota", true).Error)
	require.NoError(t, funding.SettleWithTokenActual(100))
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 100, storedToken.RemainQuota)
	require.Equal(t, 100, storedToken.UsedQuota)

	require.NoError(t, funding.RefundWithToken())
	storedToken, err = model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 200, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
}

func TestOrganizationBillingSessionPreConsumePersistsSessionAndSkipsPersonalBilling(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Updates(map[string]any{"quota": 0, "used_quota": 0, "request_count": 0}).Error)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "session-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-session-preconsume"
	relayInfo.OriginModelName = "gpt-session"
	relayInfo.UsingGroup = "session-group"
	relayInfo.UserSetting.BillingPreference = "subscription_only"

	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceOrganization, relayInfo.BillingSource)
	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.Where("idempotency_key = ?", "request:org-session-preconsume").First(&storedSession).Error)
	require.Equal(t, organization.Id, storedSession.OrganizationId)
	require.Equal(t, model.OrganizationBillingSessionStatusPreConsumed, storedSession.Status)
	require.Equal(t, 100, storedSession.PreConsumedQuota)
	require.Equal(t, 100, storedSession.TokenPreConsumedQuota)
	require.Equal(t, 100, storedSession.TokenRemainDeductedQuota)
	require.Equal(t, token.Id, storedSession.TokenId)
	require.Equal(t, token.Name, storedSession.TokenName)
	require.Equal(t, member.Id, storedSession.ResponsibleUserId)
	require.Equal(t, token.CreatorUserId, storedSession.CreatorUserId)
	require.Equal(t, "gpt-session", storedSession.ModelName)
	require.Equal(t, "session-group", storedSession.Group)

	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+100, storedOrg.UsedQuota)
	require.Equal(t, organization.RequestCount, storedOrg.RequestCount)
	var storedUser model.User
	require.NoError(t, model.DB.First(&storedUser, member.Id).Error)
	require.Zero(t, storedUser.UsedQuota)
	require.Zero(t, storedUser.RequestCount)
	var preConsumeRecord model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("record_key = ?", "pre_consume:"+strconv.Itoa(storedSession.Id)).First(&preConsumeRecord).Error)
	require.Equal(t, "pre_consume", preConsumeRecord.RecordType)
	require.Equal(t, 100, preConsumeRecord.UsedQuotaDelta)
	require.Zero(t, preConsumeRecord.UsageQuota)
	require.Equal(t, organization.UsedQuota, preConsumeRecord.UsedQuotaBefore)
	require.Equal(t, organization.UsedQuota+100, preConsumeRecord.UsedQuotaAfter)
	require.Equal(t, relayInfo.RequestId, preConsumeRecord.RequestId)
	require.Equal(t, token.Id, preConsumeRecord.TokenId)
	require.Equal(t, member.Id, preConsumeRecord.ResponsibleUserId)
	require.Equal(t, "gpt-session", preConsumeRecord.ModelName)
	require.Equal(t, "session-group", preConsumeRecord.Group)
}

func TestOrganizationBillingSessionRequestReplayIsIdempotentAndConflictIsRejected(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "idempotent-session-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	first := organizationBillingRelayInfo(token, organization.Id)
	first.RequestId = "org-session-replay"
	_, apiErr := NewBillingSession(&gin.Context{}, first, 100)
	require.Nil(t, apiErr)

	replay := organizationBillingRelayInfo(token, organization.Id)
	replay.RequestId = "org-session-replay"
	_, apiErr = NewBillingSession(&gin.Context{}, replay, 100)
	require.Nil(t, apiErr)

	var sessionCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("idempotency_key = ?", "request:org-session-replay").Count(&sessionCount).Error)
	require.Equal(t, int64(1), sessionCount)
	var preConsumeRecordCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("record_key = ?", "pre_consume:"+strconv.Itoa(first.OrganizationBillingSessionId)).Count(&preConsumeRecordCount).Error)
	require.EqualValues(t, 1, preConsumeRecordCount)
	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+100, storedOrg.UsedQuota)
	require.Equal(t, organization.RequestCount, storedOrg.RequestCount)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 900, storedToken.RemainQuota)

	conflicting := organizationBillingRelayInfo(token, organization.Id)
	conflicting.RequestId = "org-session-replay"
	_, apiErr = NewBillingSession(&gin.Context{}, conflicting, 200)
	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "organization billing session conflict")
}

func TestOrganizationBillingSessionReplayPrecheckSkipsCreate(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "rows-affected-replay-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	first := organizationBillingRelayInfo(token, organization.Id)
	first.RequestId = "org-session-rows-affected-replay"
	_, apiErr := NewBillingSession(&gin.Context{}, first, 100)
	require.Nil(t, apiErr)

	sessionCreateCalls := 0
	callbackName := "test:organization-session-replay-skips-create"
	require.NoError(t, model.DB.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "organization_billing_sessions" {
			sessionCreateCalls++
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove(callbackName) })

	replay := organizationBillingRelayInfo(token, organization.Id)
	replay.RequestId = first.RequestId
	_, apiErr = NewBillingSession(&gin.Context{}, replay, 100)
	require.Nil(t, apiErr)
	require.Zero(t, sessionCreateCalls)

	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+100, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 900, storedToken.RemainQuota)
	var recordCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("organization_id = ?", organization.Id).Count(&recordCount).Error)
	require.EqualValues(t, 1, recordCount)
}

func TestOrganizationBillingSessionInsertConflictResolvesReplay(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "insert-conflict-replay-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	first := organizationBillingRelayInfo(token, organization.Id)
	first.RequestId = "org-session-insert-conflict-replay"
	_, apiErr := NewBillingSession(&gin.Context{}, first, 100)
	require.Nil(t, apiErr)

	failNextSessionLookup := true
	callbackName := "test:organization-session-replay-lookup-race"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if failNextSessionLookup && tx.Statement != nil && tx.Statement.Table == "organization_billing_sessions" {
			failNextSessionLookup = false
			tx.AddError(gorm.ErrRecordNotFound)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove(callbackName) })

	replay := organizationBillingRelayInfo(token, organization.Id)
	replay.RequestId = first.RequestId
	_, apiErr = NewBillingSession(&gin.Context{}, replay, 100)
	require.Nil(t, apiErr)

	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+100, storedOrganization.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 900, storedToken.RemainQuota)
	var sessionCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("idempotency_key = ?", "request:"+first.RequestId).Count(&sessionCount).Error)
	require.EqualValues(t, 1, sessionCount)
}

func TestOrganizationBillingSessionConcurrentPreConsumeDoesNotLoseUpdates(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "concurrent-session-key", ExpiredTime: -1, RemainQuota: 100000})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Updates(map[string]any{"quota": 100000, "used_quota": 0, "request_count": 0}).Error)

	const workers = 10
	const quota = 5
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			relayInfo := organizationBillingRelayInfo(token, organization.Id)
			relayInfo.RequestId = "org-session-concurrent-" + strconv.Itoa(i)
			_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, quota)
			if apiErr != nil {
				errs <- apiErr
				return
			}
			errs <- nil
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, workers*quota, storedOrg.UsedQuota)
	require.Zero(t, storedOrg.RequestCount)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 100000-workers*quota, storedToken.RemainQuota)
	require.Equal(t, workers*quota, storedToken.UsedQuota)
}

func TestOrganizationBillingSessionUsesTaskIdAndSupportsZeroPreConsume(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "task-session-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.TaskRelayInfo = &relaycommon.TaskRelayInfo{OriginTaskID: "org-task-session"}

	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 0)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.NoError(t, session.Settle(25))

	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.Where("idempotency_key = ?", "task:org-task-session").First(&storedSession).Error)
	require.Equal(t, "", storedSession.RequestId)
	require.Equal(t, "org-task-session", storedSession.TaskId)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, storedSession.Status)
	require.Zero(t, storedSession.PreConsumedQuota)
	require.Equal(t, 25, storedSession.SettledQuota)
	require.Equal(t, 25, storedSession.TokenRemainDeductedQuota)
	var preConsumeRecords int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("record_key = ?", "pre_consume:"+strconv.Itoa(storedSession.Id)).Count(&preConsumeRecords).Error)
	require.Zero(t, preConsumeRecords)
	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+25, storedOrg.UsedQuota)
}

func TestOrganizationBillingSessionUnlimitedTokenFailureRefundsOrganizationQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "unlimited-refund-session-key", ExpiredTime: -1, UnlimitedQuota: true})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-session-unlimited-refund"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 80)
	require.Nil(t, apiErr)
	require.True(t, session.NeedsRefund())

	session.Refund(&gin.Context{})

	require.Eventually(t, func() bool {
		var storedOrg model.Organization
		if err := model.DB.First(&storedOrg, organization.Id).Error; err != nil {
			return false
		}
		if storedOrg.UsedQuota != organization.UsedQuota || storedOrg.RequestCount != organization.RequestCount {
			return false
		}
		var storedSession model.OrganizationBillingSession
		if err := model.DB.Where("idempotency_key = ?", "request:org-session-unlimited-refund").First(&storedSession).Error; err != nil {
			return false
		}
		return storedSession.Status == model.OrganizationBillingSessionStatusRefunded && storedSession.RefundedQuota == 80
	}, time.Second, 10*time.Millisecond)
}

func TestOrganizationBillingSessionRefundWorksAfterOrganizationDisabled(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "disabled-refund-session-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-session-disabled-refund"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 90)
	require.Nil(t, apiErr)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDisabled).Error)

	require.NoError(t, session.funding.(*OrganizationFunding).RefundWithToken())

	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrg.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 1000, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
}

func TestOrganizationBillingSessionNegativeSettleWorksAfterOrganizationDisabled(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "disabled-settle-session-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-session-disabled-settle"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 90)
	require.Nil(t, apiErr)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDisabled).Error)

	require.NoError(t, session.Settle(40))

	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+40, storedOrg.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 960, storedToken.RemainQuota)
	require.Equal(t, 40, storedToken.UsedQuota)
}

func TestOrganizationBillingSessionSettleWorksAfterTokenDeleted(t *testing.T) {
	testCases := []struct {
		name        string
		actualQuota int
	}{
		{name: "returns unused pre-consume", actualQuota: 40},
		{name: "charges above pre-consume", actualQuota: 140},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, member, organization := createOrganizationTokenTestOrg(t)
			token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
				Name:        "deleted-settle-key",
				ExpiredTime: -1,
				RemainQuota: 1000,
			})
			require.NoError(t, err)
			relayInfo := organizationBillingRelayInfo(token, organization.Id)
			relayInfo.RequestId = "deleted-settle-" + strconv.Itoa(testCase.actualQuota)
			session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
			require.Nil(t, apiErr)
			require.NoError(t, DeleteOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id))

			require.NoError(t, session.Settle(testCase.actualQuota))

			var storedSession model.OrganizationBillingSession
			require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
			require.Equal(t, model.OrganizationBillingSessionStatusSettled, storedSession.Status)
			require.Equal(t, testCase.actualQuota, storedSession.SettledQuota)
			require.Equal(t, testCase.actualQuota, storedSession.TokenRemainDeductedQuota)
			var storedOrganization model.Organization
			require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
			require.Equal(t, organization.UsedQuota+testCase.actualQuota, storedOrganization.UsedQuota)
			var deletedToken model.Token
			require.NoError(t, model.DB.Unscoped().First(&deletedToken, token.Id).Error)
			require.True(t, deletedToken.DeletedAt.Valid)
			require.Equal(t, 1000-testCase.actualQuota, deletedToken.RemainQuota)
			require.Equal(t, testCase.actualQuota, deletedToken.UsedQuota)
			var records []model.OrganizationBillingRecord
			require.NoError(t, model.DB.Where("session_id = ?", storedSession.Id).Find(&records).Error)
			require.Len(t, records, 2)
			ledgerTotal := 0
			for _, record := range records {
				ledgerTotal += record.UsedQuotaDelta
			}
			require.Equal(t, testCase.actualQuota, ledgerTotal)
		})
	}
}

func TestOrganizationBillingSessionRefundWorksAfterTokenDeleted(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:        "deleted-refund-key",
		ExpiredTime: -1,
		RemainQuota: 1000,
	})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "deleted-refund"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 90)
	require.Nil(t, apiErr)
	require.NoError(t, DeleteOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id))
	_, authErr := model.ValidateUserToken(token.Key)
	require.Error(t, authErr, "deleted key must remain unusable for new requests")

	require.NoError(t, session.funding.(*OrganizationFunding).RefundWithToken())

	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, storedSession.Status)
	require.Equal(t, 90, storedSession.RefundedQuota)
	require.Zero(t, storedSession.TokenRemainDeductedQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
	var deletedToken model.Token
	require.NoError(t, model.DB.Unscoped().First(&deletedToken, token.Id).Error)
	require.True(t, deletedToken.DeletedAt.Valid)
	require.Equal(t, 1000, deletedToken.RemainQuota)
	require.Zero(t, deletedToken.UsedQuota)
}

func TestOrganizationBillingSessionSettleUsesLockedSessionAfterCallerIdentityMutation(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:        "mutated-caller-settle-key",
		ExpiredTime: -1,
		RemainQuota: 1000,
	})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "mutated-caller-settle"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	sessionId := relayInfo.OrganizationBillingSessionId
	tamperedId := organization.Id + 1_000_000
	relayInfo.TokenId = token.Id + 1_000_000
	relayInfo.TokenKey = "tampered-caller-token"
	relayInfo.ScopeType = model.AccountContextTypePersonal
	relayInfo.ScopeId = tamperedId
	relayInfo.BillingAccountId = tamperedId
	relayInfo.OrganizationId = tamperedId
	relayInfo.OrganizationQuota = -1

	require.NoError(t, session.Settle(140))

	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, sessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, storedSession.Status)
	require.Equal(t, 140, storedSession.SettledQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrganization.UsedQuota)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, 860, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)
	require.Equal(t, organization.Id, relayInfo.OrganizationId)
	require.Equal(t, organization.Quota-(organization.UsedQuota+140), relayInfo.OrganizationQuota)
}

func TestOrganizationBillingSessionRefundUsesLockedSessionAfterCallerIdentityMutation(t *testing.T) {
	owner, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:        "mutated-caller-refund-key",
		ExpiredTime: -1,
		RemainQuota: 1000,
	})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "mutated-caller-refund"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 90)
	require.Nil(t, apiErr)
	sessionId := relayInfo.OrganizationBillingSessionId
	otherOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Mutated Caller Refund Org"})
	require.NoError(t, err)
	relayInfo.TokenId = token.Id + 1_000_000
	relayInfo.TokenKey = "tampered-caller-token"
	relayInfo.ScopeType = model.AccountContextTypePersonal
	relayInfo.ScopeId = otherOrganization.Id
	relayInfo.BillingAccountId = otherOrganization.Id
	relayInfo.OrganizationId = otherOrganization.Id
	relayInfo.OrganizationQuota = -1

	require.NoError(t, session.funding.(*OrganizationFunding).RefundWithToken())

	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, sessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, storedSession.Status)
	require.Equal(t, 90, storedSession.RefundedQuota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrganization.UsedQuota)
	var storedOtherOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOtherOrganization, otherOrganization.Id).Error)
	require.Zero(t, storedOtherOrganization.UsedQuota)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, 1000, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
	require.Equal(t, organization.Id, relayInfo.OrganizationId)
	require.Equal(t, organization.Quota-organization.UsedQuota, relayInfo.OrganizationQuota)
}

func TestOrganizationBillingSessionRefundRejectsPhysicallyDeletedToken(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:        "hard-deleted-refund-key",
		ExpiredTime: -1,
		RemainQuota: 1000,
	})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "hard-deleted-refund"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, model.DB.Unscoped().Delete(&model.Token{}, token.Id).Error)

	err = session.funding.(*OrganizationFunding).RefundWithToken()

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusPreConsumed, storedSession.Status)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, 100, storedOrganization.UsedQuota)
}

func TestOrganizationBillingSessionRejectsTokenFromDifferentOrganization(t *testing.T) {
	owner, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:        "mismatched-session-key",
		ExpiredTime: -1,
		RemainQuota: 1000,
	})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "mismatched-session"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	otherOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Mismatched Session Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).
		Where("id = ?", relayInfo.OrganizationBillingSessionId).
		UpdateColumn("organization_id", otherOrganization.Id).Error)

	err = session.Settle(40)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusPreConsumed, storedSession.Status)
	var storedOriginal model.Organization
	require.NoError(t, model.DB.First(&storedOriginal, organization.Id).Error)
	require.Equal(t, 100, storedOriginal.UsedQuota)
	var storedOther model.Organization
	require.NoError(t, model.DB.First(&storedOther, otherOrganization.Id).Error)
	require.Zero(t, storedOther.UsedQuota)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, 900, storedToken.RemainQuota)
	require.Equal(t, 100, storedToken.UsedQuota)
}

func TestOrganizationBillingSessionSettleAndRefundWriteRecordsIdempotently(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "settle-session-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-session-settle"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)

	require.NoError(t, session.Settle(140))
	require.NoError(t, session.Settle(140))

	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.Where("idempotency_key = ?", "request:org-session-settle").First(&storedSession).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, storedSession.Status)
	require.Equal(t, 140, storedSession.SettledQuota)
	require.Equal(t, 140, storedSession.TokenRemainDeductedQuota)
	var settleRecords []model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("record_key = ?", "settle:"+strconv.Itoa(storedSession.Id)).Find(&settleRecords).Error)
	require.Len(t, settleRecords, 1)
	require.Equal(t, model.OrganizationBillingRecordTypeSettle, settleRecords[0].RecordType)
	require.Equal(t, settleRecords[0].UsedQuotaAfter-settleRecords[0].UsedQuotaBefore, settleRecords[0].UsedQuotaDelta)
	require.Equal(t, 40, settleRecords[0].UsedQuotaDelta)
	require.Equal(t, 140, settleRecords[0].UsageQuota)

	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrg.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 860, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)

	refundRelayInfo := organizationBillingRelayInfo(token, organization.Id)
	refundRelayInfo.RequestId = "org-session-refund"
	refundSession, apiErr := NewBillingSession(&gin.Context{}, refundRelayInfo, 70)
	require.Nil(t, apiErr)
	require.NoError(t, refundSession.funding.(*OrganizationFunding).RefundWithToken())
	require.NoError(t, refundSession.funding.(*OrganizationFunding).RefundWithToken())

	var refundStored model.OrganizationBillingSession
	require.NoError(t, model.DB.Where("idempotency_key = ?", "request:org-session-refund").First(&refundStored).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, refundStored.Status)
	require.Equal(t, 70, refundStored.RefundedQuota)
	var refundRecords []model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("record_key = ?", "refund:"+strconv.Itoa(refundStored.Id)).Find(&refundRecords).Error)
	require.Len(t, refundRecords, 1)
	require.Equal(t, -70, refundRecords[0].UsedQuotaDelta)
	require.Zero(t, refundRecords[0].UsageQuota)
	var refundPreConsumeRecord model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("record_key = ?", "pre_consume:"+strconv.Itoa(refundStored.Id)).First(&refundPreConsumeRecord).Error)
	require.Equal(t, 70, refundPreConsumeRecord.UsedQuotaDelta)
	require.Equal(t, 0, refundPreConsumeRecord.UsedQuotaDelta+refundRecords[0].UsedQuotaDelta)

	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrg.UsedQuota)
	storedToken, err = model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 860, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)
}

func TestOrganizationBillingFailedRequestKeepsZeroLogAndBalancedLedger(t *testing.T) {
	owner, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "failed-request-ledger-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "failed-request-balanced-ledger"
	relayInfo.OriginModelName = "gpt-failed-request"
	relayInfo.UsingGroup = "default"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 80)
	require.Nil(t, apiErr)
	ctx := &gin.Context{}
	ctx.Set("username", member.Username)
	ctx.Set(common.RequestIdKey, relayInfo.RequestId)
	session.Refund(ctx)
	// Refund 把退款的落库派发到 gopool 上，而退款路径会回写 relayInfo 上的会话字段。
	// 先等这批后台任务结束，之后对 relayInfo 的读才有 happens-before。
	WaitForBackgroundWork()
	sessionId := relayInfo.OrganizationBillingSessionId
	model.RecordErrorLog(ctx, member.Id, RelayConsumeLogParams(relayInfo, model.RecordConsumeLogParams{
		ModelName: relayInfo.OriginModelName,
		TokenName: token.Name,
		TokenId:   token.Id,
		Group:     relayInfo.UsingGroup,
		Content:   "upstream failed",
	}))
	require.Eventually(t, func() bool {
		var storedSession model.OrganizationBillingSession
		if err := model.DB.Where("id = ?", sessionId).First(&storedSession).Error; err != nil {
			return false
		}
		var recordCount int64
		if err := model.DB.Model(&model.OrganizationBillingRecord{}).
			Where("session_id = ?", sessionId).
			Count(&recordCount).Error; err != nil {
			return false
		}
		return storedSession.Status == model.OrganizationBillingSessionStatusRefunded && recordCount == 2
	}, time.Second, 10*time.Millisecond)

	var errorLog model.Log
	require.NoError(t, model.LOG_DB.Where("organization_billing_session_id = ? AND type = ?", sessionId, model.LogTypeError).First(&errorLog).Error)
	require.Zero(t, errorLog.Quota)
	records, total, err := ListOrganizationBillingDetails(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationBillingDetailListRequest{Limit: 20, RequestId: relayInfo.RequestId})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, records, 2)
	ledgerTotal := 0
	ledgerDeltas := make(map[string]int, len(records))
	for _, record := range records {
		ledgerTotal += record.LedgerQuotaDelta
		ledgerDeltas[record.RecordType] = record.LedgerQuotaDelta
	}
	require.Zero(t, ledgerTotal)
	require.Equal(t, 80, ledgerDeltas[model.OrganizationBillingRecordTypePreConsume])
	require.Equal(t, -80, ledgerDeltas[model.OrganizationBillingRecordTypeRefund])
}

func TestOrganizationBillingUnlimitedTokenSessionTracksUsedQuotaWithoutMutatingRemainQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "unlimited-session-key", ExpiredTime: -1, UnlimitedQuota: true})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-session-unlimited"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)

	require.NoError(t, session.Settle(140))

	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Zero(t, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)
	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.Where("idempotency_key = ?", "request:org-session-unlimited").First(&storedSession).Error)
	require.True(t, storedSession.TokenUnlimitedQuota)
	require.Zero(t, storedSession.TokenPreConsumedQuota)
	require.Zero(t, storedSession.TokenRemainDeductedQuota)
}

func TestOrganizationBillingQuotaAdjustmentWritesAppendOnlyRecord(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	platformAdmin := createServiceTestUser(t, "quota-adjust-platform", common.RoleRootUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", admin.Id).Update("quota", 0).Error)

	adjustment, err := AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 123, IdempotencyKey: "org-adjust-record", Reason: "record"})

	require.NoError(t, err)
	require.NotNil(t, adjustment)
	var record model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("record_key = ?", "adjust:org-adjust-record").First(&record).Error)
	require.Equal(t, model.OrganizationBillingRecordTypeAdjustment, record.RecordType)
	require.Equal(t, 123, record.QuotaDelta)
	require.Equal(t, organization.Quota, record.QuotaBefore)
	require.Equal(t, organization.Quota+123, record.QuotaAfter)
	require.Zero(t, record.UsedQuotaDelta)
}

func TestOrganizationRelayBillingNonStreamSuccessSettlesSession(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "relay-success-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-relay-success"

	apiErr := PreConsumeBilling(&gin.Context{}, 100, relayInfo)
	require.Nil(t, apiErr)
	require.NotZero(t, relayInfo.OrganizationBillingSessionId)
	require.Equal(t, "request:org-relay-success", relayInfo.OrganizationBillingSessionKey)

	require.NoError(t, SettleBilling(&gin.Context{}, relayInfo, 135))

	var session model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&session, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, session.Status)
	require.Equal(t, 135, session.SettledQuota)
	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+135, storedOrg.UsedQuota)
	require.Equal(t, organization.RequestCount+1, storedOrg.RequestCount)
}

func TestOrganizationRelayBillingUpstreamFailureRefundsSession(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "relay-failure-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-relay-failure"

	apiErr := PreConsumeBilling(&gin.Context{}, 100, relayInfo)
	require.Nil(t, apiErr)
	relayInfo.Billing.Refund(&gin.Context{})
	// 同上：退款异步落库并回写 relayInfo，先冲刷再读会话 id。
	WaitForBackgroundWork()
	sessionId := relayInfo.OrganizationBillingSessionId

	require.Eventually(t, func() bool {
		var session model.OrganizationBillingSession
		if err := model.DB.First(&session, sessionId).Error; err != nil {
			return false
		}
		var storedOrg model.Organization
		if err := model.DB.First(&storedOrg, organization.Id).Error; err != nil {
			return false
		}
		return session.Status == model.OrganizationBillingSessionStatusRefunded &&
			session.RefundedQuota == 100 &&
			storedOrg.UsedQuota == organization.UsedQuota &&
			storedOrg.RequestCount == organization.RequestCount
	}, time.Second, 10*time.Millisecond)
}

func TestOrganizationBillingStreamHeartbeatUpdatesSessionExpiry(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "heartbeat-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-heartbeat"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	relayInfo.Billing = session
	before := common.GetTimestamp()

	require.NoError(t, TouchOrganizationBillingSession(relayInfo, 30))

	var stored model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&stored, relayInfo.OrganizationBillingSessionId).Error)
	require.GreaterOrEqual(t, stored.LastHeartbeatAt, before)
	require.GreaterOrEqual(t, stored.ExpiresAt, stored.LastHeartbeatAt+30)

	require.NoError(t, session.Settle(100))
	settledHeartbeat := stored.LastHeartbeatAt
	require.NoError(t, TouchOrganizationBillingSession(relayInfo, 30))
	require.NoError(t, model.DB.First(&stored, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, stored.Status)
	require.Equal(t, settledHeartbeat, stored.LastHeartbeatAt)
}

func TestOrganizationBillingRepairClaimsExpiredSessionOnceAndRefunds(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "repair-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-repair"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("id = ?", session.funding.(*OrganizationFunding).session.Id).UpdateColumns(map[string]any{"expires_at": now - 1}).Error)

	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	claimedAgain, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Empty(t, claimedAgain)

	repaired, err := RepairOrganizationBillingSessions(claimed, now)
	require.NoError(t, err)
	require.Equal(t, 1, repaired)
	repairedAgain, err := RepairOrganizationBillingSessions(claimed, now)
	require.NoError(t, err)
	require.Zero(t, repairedAgain)

	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, claimed[0].Id).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, storedSession.Status)
	var records []model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("session_id = ?", claimed[0].Id).Order("id asc").Find(&records).Error)
	require.Len(t, records, 2)
	require.Equal(t, model.OrganizationBillingRecordTypePreConsume, records[0].RecordType)
	require.Equal(t, 100, records[0].UsedQuotaDelta)
	require.Zero(t, records[0].UsageQuota)
	require.Equal(t, model.OrganizationBillingRecordTypeRefund, records[1].RecordType)
	require.Equal(t, -100, records[1].UsedQuotaDelta)
	require.Zero(t, records[1].UsageQuota)
	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrg.UsedQuota)
}

func TestOrganizationBillingRepairRefundsDeletedTokenAndUnblocksDissolve(t *testing.T) {
	owner, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:        "deleted-repair-key",
		ExpiredTime: -1,
		RemainQuota: 1000,
	})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "deleted-repair"
	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, DeleteOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id))
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).
		Where("id = ?", relayInfo.OrganizationBillingSessionId).
		UpdateColumns(map[string]any{
			"status":            model.OrganizationBillingSessionStatusFailed,
			"last_repair_error": "record not found",
			"expires_at":        now - 1,
		}).Error)

	err = DissolveOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, DissolveOrganizationRequest{
		ConfirmName:    organization.Name,
		Reason:         "blocked before billing repair",
		IdempotencyKey: "deleted-repair-dissolve-blocked",
	})
	var blocked *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blocked)
	require.Contains(t, blocked.Blockers, "active_billing_sessions")

	repaired, err := RepairExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Equal(t, 1, repaired)
	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, storedSession.Status)
	claimedAgain, err := ClaimExpiredOrganizationBillingSessions(10, now+OrganizationBillingRepairLeaseSeconds+1)
	require.NoError(t, err)
	require.Empty(t, claimedAgain)

	require.NoError(t, DissolveOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, DissolveOrganizationRequest{
		ConfirmName:    organization.Name,
		Reason:         "billing repaired",
		IdempotencyKey: "deleted-repair-dissolve-success",
	}))
}

func TestOrganizationBillingRepairContinuesAfterFirstFailure(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	organization := model.Organization{
		Name:      "Repair Batch Org",
		Slug:      "repair-batch-org",
		Status:    model.OrganizationStatusActive,
		Quota:     1000,
		UsedQuota: 100,
		CreatedBy: 1,
	}
	require.NoError(t, model.DB.Create(&organization).Error)
	failedSession := model.OrganizationBillingSession{
		OrganizationId:      organization.Id + 1,
		IdempotencyKey:      "repair-batch-failed",
		Status:              model.OrganizationBillingSessionStatusPreConsumed,
		PreConsumedQuota:    50,
		LastHeartbeatAt:     now - 100,
		ExpiresAt:           now - 2,
		TokenUnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&failedSession).Error)
	validSession := model.OrganizationBillingSession{
		OrganizationId:      organization.Id,
		IdempotencyKey:      "repair-batch-valid",
		Status:              model.OrganizationBillingSessionStatusPreConsumed,
		PreConsumedQuota:    100,
		TokenUnlimitedQuota: true,
		LastHeartbeatAt:     now - 100,
		ExpiresAt:           now - 1,
	}
	require.NoError(t, model.DB.Create(&validSession).Error)

	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 2)
	require.Equal(t, []int{failedSession.Id, validSession.Id}, []int{claimed[0].Id, claimed[1].Id})

	repaired, err := RepairOrganizationBillingSessions(claimed, now)

	require.Error(t, err)
	require.Equal(t, 1, repaired)
	require.NoError(t, model.DB.First(&failedSession, failedSession.Id).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusFailed, failedSession.Status)
	require.NotEmpty(t, failedSession.LastRepairError)
	require.Equal(t, now+60, failedSession.ExpiresAt)
	require.NoError(t, model.DB.First(&validSession, validSession.Id).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, validSession.Status)
	var refundRecordCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("record_key = ?", "refund:"+strconv.Itoa(validSession.Id)).Count(&refundRecordCount).Error)
	require.EqualValues(t, 1, refundRecordCount)
}

func TestOrganizationBillingRepairReturnsOriginalRefundErrorWhenFailureMarkAlsoFails(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	organization := model.Organization{
		Name:      "Repair Error Priority Org",
		Slug:      "repair-error-priority-org",
		Status:    model.OrganizationStatusActive,
		Quota:     1000,
		UsedQuota: 100,
		CreatedBy: 1,
	}
	require.NoError(t, model.DB.Create(&organization).Error)
	failedSession := model.OrganizationBillingSession{
		OrganizationId:      organization.Id + 1,
		IdempotencyKey:      "repair-error-priority-failed",
		Status:              model.OrganizationBillingSessionStatusPreConsumed,
		PreConsumedQuota:    50,
		LastHeartbeatAt:     now - 100,
		ExpiresAt:           now - 2,
		TokenUnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&failedSession).Error)
	validSession := model.OrganizationBillingSession{
		OrganizationId:      organization.Id,
		IdempotencyKey:      "repair-error-priority-valid",
		Status:              model.OrganizationBillingSessionStatusPreConsumed,
		PreConsumedQuota:    100,
		TokenUnlimitedQuota: true,
		LastHeartbeatAt:     now - 100,
		ExpiresAt:           now - 1,
	}
	require.NoError(t, model.DB.Create(&validSession).Error)

	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 2)

	markErr := errors.New("forced repair failure mark error")
	callbackName := "test:organization-billing-repair-mark-failure"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		updates, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table != "organization_billing_sessions" || !ok || updates["status"] != model.OrganizationBillingSessionStatusFailed {
			return
		}
		tx.AddError(markErr)
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Update().Remove(callbackName)
	})

	repaired, repairErr := RepairOrganizationBillingSessions(claimed, now)

	require.ErrorIs(t, repairErr, gorm.ErrRecordNotFound)
	require.NotErrorIs(t, repairErr, markErr)
	require.Equal(t, 1, repaired)
	require.NoError(t, model.DB.First(&failedSession, failedSession.Id).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRepairing, failedSession.Status)
	require.NoError(t, model.DB.First(&validSession, validSession.Id).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, validSession.Status)
	var refundRecordCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("record_key = ?", "refund:"+strconv.Itoa(validSession.Id)).Count(&refundRecordCount).Error)
	require.EqualValues(t, 1, refundRecordCount)
}

func TestOrganizationBillingRepairDoesNotReclaimFreshRepairingLease(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	session := model.OrganizationBillingSession{
		OrganizationId:  1,
		IdempotencyKey:  "repair-fresh-lease",
		Status:          model.OrganizationBillingSessionStatusRepairing,
		RepairAttempts:  1,
		LastHeartbeatAt: now,
		ExpiresAt:       now - 1,
	}
	require.NoError(t, model.DB.Create(&session).Error)

	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now+59)

	require.NoError(t, err)
	require.Empty(t, claimed)
	require.NoError(t, model.DB.First(&session, session.Id).Error)
	require.Equal(t, 1, session.RepairAttempts)
	require.Equal(t, now, session.LastHeartbeatAt)
	require.Equal(t, now-1, session.ExpiresAt)
}

func TestOrganizationBillingRepairReclaimsStaleRepairingExactlyOnce(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	session := model.OrganizationBillingSession{
		OrganizationId:  1,
		IdempotencyKey:  "repair-stale-lease",
		Status:          model.OrganizationBillingSessionStatusRepairing,
		RepairAttempts:  1,
		LastHeartbeatAt: now - 60,
		ExpiresAt:       now + 999,
	}
	require.NoError(t, model.DB.Create(&session).Error)

	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, 2, claimed[0].RepairAttempts)
	require.Equal(t, now, claimed[0].LastHeartbeatAt)
	require.Equal(t, now+60, claimed[0].ExpiresAt)

	claimedAgain, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Empty(t, claimedAgain)
}

func TestOrganizationBillingRepairCrashLeaseIsSixtySeconds(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	session := model.OrganizationBillingSession{
		OrganizationId:  1,
		IdempotencyKey:  "repair-crash-lease",
		Status:          model.OrganizationBillingSessionStatusPreConsumed,
		LastHeartbeatAt: now - 100,
		ExpiresAt:       now - 1,
	}
	require.NoError(t, model.DB.Create(&session).Error)

	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, model.OrganizationBillingSessionStatusRepairing, claimed[0].Status)
	require.Equal(t, 1, claimed[0].RepairAttempts)
	require.Equal(t, now, claimed[0].LastHeartbeatAt)
	require.Equal(t, now+60, claimed[0].ExpiresAt)

	claimedBeforeLeaseExpiry, err := ClaimExpiredOrganizationBillingSessions(10, now+59)
	require.NoError(t, err)
	require.Empty(t, claimedBeforeLeaseExpiry)
	claimedAtLeaseExpiry, err := ClaimExpiredOrganizationBillingSessions(10, now+60)
	require.NoError(t, err)
	require.Len(t, claimedAtLeaseExpiry, 1)
	require.Equal(t, 2, claimedAtLeaseExpiry[0].RepairAttempts)
}

func TestOrganizationBillingRepairFailedSessionRetainsErrorAndRetriesAtNextExpiry(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	session := model.OrganizationBillingSession{
		OrganizationId:      999999,
		IdempotencyKey:      "repair-failed-retry",
		Status:              model.OrganizationBillingSessionStatusPreConsumed,
		PreConsumedQuota:    100,
		TokenUnlimitedQuota: true,
		LastHeartbeatAt:     now - 100,
		ExpiresAt:           now - 1,
	}
	require.NoError(t, model.DB.Create(&session).Error)

	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	repaired, repairErr := RepairOrganizationBillingSessions(claimed, now)
	require.Error(t, repairErr)
	require.Zero(t, repaired)
	require.NoError(t, model.DB.First(&session, session.Id).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusFailed, session.Status)
	require.NotEmpty(t, session.LastRepairError)
	require.Equal(t, session.ErrorMessage, session.LastRepairError)
	require.Equal(t, now+60, session.ExpiresAt)
	firstRepairError := session.LastRepairError

	claimedBeforeRetry, err := ClaimExpiredOrganizationBillingSessions(10, now+59)
	require.NoError(t, err)
	require.Empty(t, claimedBeforeRetry)
	claimedForRetry, err := ClaimExpiredOrganizationBillingSessions(10, now+60)
	require.NoError(t, err)
	require.Len(t, claimedForRetry, 1)
	require.Equal(t, firstRepairError, claimedForRetry[0].LastRepairError)
	repaired, repairErr = RepairOrganizationBillingSessions(claimedForRetry, now+60)
	require.Error(t, repairErr)
	require.Zero(t, repaired)
	require.NoError(t, model.DB.First(&session, session.Id).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusFailed, session.Status)
	require.NotEmpty(t, session.LastRepairError)
	require.Equal(t, 2, session.RepairAttempts)
	require.Equal(t, now+120, session.ExpiresAt)
}

func TestOrganizationBillingRepairFailedAudit(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	organization := model.Organization{Name: "Repair Failed Audit Org", Slug: "repair-failed-audit-org", Status: model.OrganizationStatusActive, Quota: 1000, UsedQuota: 100, CreatedBy: 1}
	require.NoError(t, model.DB.Create(&organization).Error)
	session := model.OrganizationBillingSession{OrganizationId: organization.Id, IdempotencyKey: "repair-failed-audit", Status: model.OrganizationBillingSessionStatusPreConsumed, PreConsumedQuota: 50, TokenRemainDeductedQuota: 10, TokenId: 999999, LastHeartbeatAt: now - 100, ExpiresAt: now - 1}
	require.NoError(t, model.DB.Create(&session).Error)
	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	rawSecret := "sk-organization-repair-audit-secret"
	repairErr := errors.New("api_key=" + rawSecret)
	callbackName := "test:organization-billing-repair-audit-error"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "tokens" {
			tx.AddError(repairErr)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove(callbackName) })

	repaired, err := RepairOrganizationBillingSessions(claimed, now)

	require.ErrorIs(t, err, repairErr)
	require.Zero(t, repaired)
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionBillingRepairFailed).First(&audit).Error)
	require.Zero(t, audit.OperatorUserId)
	require.Equal(t, organizationAuditOperatorRoleSystem, audit.OperatorRole)
	require.Equal(t, "billing_session", audit.TargetType)
	require.Equal(t, session.Id, audit.TargetId)
	require.NotContains(t, audit.Reason, rawSecret)
	require.NotContains(t, audit.TargetMetadata, rawSecret)
	require.Contains(t, audit.TargetMetadata, "session_id")
}

func TestOrganizationBillingRepairFailedAuditFailurePreservesOriginalError(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	organization := model.Organization{Name: "Repair Audit Failure Org", Slug: "repair-audit-failure-org", Status: model.OrganizationStatusActive, Quota: 1000, UsedQuota: 100, CreatedBy: 1}
	require.NoError(t, model.DB.Create(&organization).Error)
	session := model.OrganizationBillingSession{OrganizationId: organization.Id, IdempotencyKey: "repair-audit-write-failure", Status: model.OrganizationBillingSessionStatusPreConsumed, PreConsumedQuota: 50, TokenRemainDeductedQuota: 10, TokenId: 999999, LastHeartbeatAt: now - 100, ExpiresAt: now - 1}
	require.NoError(t, model.DB.Create(&session).Error)
	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	repairErr := errors.New("forced token refund failure")
	auditErr := errors.New("forced repair audit failure")
	auditAttempted := false
	queryCallback := "test:organization-billing-repair-original-error"
	createCallback := "test:organization-billing-repair-audit-failure"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "tokens" {
			tx.AddError(repairErr)
		}
	}))
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(createCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "organization_audit_logs" {
			auditAttempted = true
			tx.AddError(auditErr)
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(queryCallback)
		_ = model.DB.Callback().Create().Remove(createCallback)
	})

	_, err = RepairOrganizationBillingSessions(claimed, now)

	require.ErrorIs(t, err, repairErr)
	require.NotErrorIs(t, err, auditErr)
	require.True(t, auditAttempted)
}

func TestOrganizationBillingRepairFailedAuditSkipsWhenFailedTransitionIsLost(t *testing.T) {
	setupServiceTestDB(t)
	now := int64(1_000_000)
	organization := model.Organization{Name: "Repair Transition Lost Org", Slug: "repair-transition-lost-org", Status: model.OrganizationStatusActive, Quota: 1000, CreatedBy: 1}
	require.NoError(t, model.DB.Create(&organization).Error)
	session := model.OrganizationBillingSession{OrganizationId: organization.Id, IdempotencyKey: "repair-transition-lost", Status: model.OrganizationBillingSessionStatusRefunded, RefundedAt: now - 1}
	require.NoError(t, model.DB.Create(&session).Error)

	recordOrganizationBillingRepairFailure(session.Id, errors.New("stale repair error"), now)

	var stored model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&stored, session.Id).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, stored.Status)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionBillingRepairFailed).Count(&auditCount).Error)
	require.Zero(t, auditCount)
}

func TestOrganizationBillingSessionStartsWithRepairExpiry(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "repair-expiry-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-repair-expiry"
	before := common.GetTimestamp()

	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)

	var stored model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&stored, relayInfo.OrganizationBillingSessionId).Error)
	require.GreaterOrEqual(t, stored.LastHeartbeatAt, before)
	require.Greater(t, stored.ExpiresAt, stored.LastHeartbeatAt)
}

func TestOrganizationBillingClaimRechecksExpiryBeforeRepairing(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "repair-race-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-repair-race"
	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("id = ?", relayInfo.OrganizationBillingSessionId).UpdateColumns(map[string]any{"expires_at": now - 1}).Error)

	require.NoError(t, TouchOrganizationBillingSession(relayInfo, 30))
	claimed, err := claimExpiredOrganizationBillingSession(relayInfo.OrganizationBillingSessionId, now)
	require.NoError(t, err)
	require.Nil(t, claimed)

	var stored model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&stored, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusPreConsumed, stored.Status)
	require.Zero(t, stored.RepairAttempts)
	require.Greater(t, stored.ExpiresAt, now)
}

func TestOrganizationBillingLoadSessionRequiresMatchingSessionKey(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "session-key-match-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-session-key-match"
	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)

	mismatchedInfo := organizationBillingRelayInfo(token, organization.Id)
	mismatchedInfo.RequestId = "org-session-key-match"
	mismatchedInfo.OrganizationBillingSessionId = relayInfo.OrganizationBillingSessionId
	mismatchedInfo.OrganizationBillingSessionKey = "request:other-session"
	funding := &OrganizationFunding{relayInfo: mismatchedInfo}

	require.Error(t, funding.SettleWithToken(0))
	var stored model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&stored, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusPreConsumed, stored.Status)
}

func TestOrganizationBillingRepairDoesNotClaimTerminalSessions(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "repair-terminal-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	now := common.GetTimestamp()

	settledInfo := organizationBillingRelayInfo(token, organization.Id)
	settledInfo.RequestId = "org-repair-terminal-settled"
	settledSession, apiErr := NewBillingSession(&gin.Context{}, settledInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, settledSession.Settle(100))
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("id = ?", settledInfo.OrganizationBillingSessionId).UpdateColumns(map[string]any{"expires_at": now - 1}).Error)

	refundedInfo := organizationBillingRelayInfo(token, organization.Id)
	refundedInfo.RequestId = "org-repair-terminal-refunded"
	refundedSession, apiErr := NewBillingSession(&gin.Context{}, refundedInfo, 50)
	require.Nil(t, apiErr)
	require.NoError(t, refundedSession.funding.(*OrganizationFunding).RefundWithToken())
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("id = ?", refundedInfo.OrganizationBillingSessionId).UpdateColumns(map[string]any{"expires_at": now - 1}).Error)

	claimed, err := ClaimExpiredOrganizationBillingSessions(10, now)
	require.NoError(t, err)
	require.Empty(t, claimed)

	var settledStored model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&settledStored, settledInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, settledStored.Status)
	require.Zero(t, settledStored.RepairAttempts)

	var refundedStored model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&refundedStored, refundedInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusRefunded, refundedStored.Status)
	require.Zero(t, refundedStored.RepairAttempts)
}

func TestOrganizationBillingSessionReferencePersistsToTask(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "task-session-reference-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-task-session-reference"
	relayInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 7}
	apiErr := PreConsumeBilling(&gin.Context{}, 100, relayInfo)
	require.Nil(t, apiErr)
	require.NotNil(t, relayInfo)

	task := model.InitTask(constant.TaskPlatformSuno, relayInfo)

	require.Equal(t, relayInfo.OrganizationBillingSessionId, task.OrganizationBillingSessionId)
	require.Equal(t, relayInfo.OrganizationBillingSessionKey, task.OrganizationBillingSessionKey)
}

func TestOrganizationBillingRefundDoesNotMakeUsedQuotaNegative(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	require.NoError(t, PostConsumeQuota(relayInfo, 30, 0, false))

	require.NoError(t, PostConsumeQuota(relayInfo, -100, 0, false))

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Zero(t, stored.UsedQuota)
}

func TestOrganizationBillingConcurrentConsumeDoesNotLoseUpdates(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-key", ExpiredTime: -1, RemainQuota: 100000})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Updates(map[string]any{"quota": 100000, "used_quota": 0, "request_count": 0}).Error)

	const workers = 20
	const quota = 5
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- ConsumeOrganizationQuotaWithRequestCount(organizationBillingRelayInfo(token, organization.Id), quota, true)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, workers*quota, stored.UsedQuota)
	require.Equal(t, workers, stored.RequestCount)
}

func TestOrganizationBillingConcurrentConsumeStopsAtQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "billing-key", ExpiredTime: -1, RemainQuota: 100000})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Updates(map[string]any{"quota": 25, "used_quota": 0, "request_count": 0}).Error)

	const workers = 10
	const quota = 5
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- ConsumeOrganizationQuotaWithRequestCount(organizationBillingRelayInfo(token, organization.Id), quota, true)
		}()
	}
	wg.Wait()
	close(errs)
	var successCount int
	for err := range errs {
		if err == nil {
			successCount++
		} else {
			require.ErrorContains(t, err, "organization quota is not enough")
		}
	}

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, 5, successCount)
	require.Equal(t, 25, stored.UsedQuota)
	require.Equal(t, successCount, stored.RequestCount)
}

func TestOrganizationRelayLogFieldsComplete(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "log-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	common.LogConsumeEnabled = true
	relayInfo.OrganizationBillingSessionId = 7
	relayInfo.OrganizationBillingSessionKey = "request:log-key"

	model.RecordConsumeLog(&gin.Context{}, relayInfo.UserId, RelayConsumeLogParams(relayInfo, model.RecordConsumeLogParams{TokenId: token.Id, TokenName: token.Name, Quota: 3}))

	var log model.Log
	require.NoError(t, model.LOG_DB.Where("token_id = ?", token.Id).First(&log).Error)
	require.Equal(t, model.AccountContextTypeOrganization, log.ScopeType)
	require.Equal(t, organization.Id, log.ScopeId)
	require.Equal(t, model.AccountContextTypeOrganization, log.BillingAccountType)
	require.Equal(t, organization.Id, log.BillingAccountId)
	require.Equal(t, organization.Id, log.OrganizationId)
	require.Equal(t, token.CreatorUserId, log.CreatorUserId)
	require.Equal(t, token.ResponsibleUserId, log.ResponsibleUserId)
	require.Equal(t, organization.Name, log.OrganizationName)
	require.Equal(t, "org-token-member", log.CreatorName)
	require.Equal(t, "org-token-member", log.ResponsibleName)
	require.Equal(t, relayInfo.OrganizationBillingSessionId, log.OrganizationBillingSessionId)
	require.Equal(t, relayInfo.OrganizationBillingSessionKey, log.OrganizationBillingSessionKey)
}

func organizationBillingRelayInfo(token *model.Token, organizationId int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		UserId:             token.ResponsibleUserId,
		TokenId:            token.Id,
		TokenKey:           token.Key,
		TokenUnlimited:     token.UnlimitedQuota,
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            organizationId,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organizationId,
		OrganizationId:     organizationId,
		ActorUserId:        token.ResponsibleUserId,
		CreatorUserId:      token.CreatorUserId,
		ResponsibleUserId:  token.ResponsibleUserId,
	}
}
