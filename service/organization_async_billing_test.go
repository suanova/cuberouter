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
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaykittypes "github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 源分支把这组断言挂在 EnsureRelayQuotaAvailable 上；cuberouter 的调用点统一走
// PreConsumeBilling -> NewBillingSession -> FundingSource.PreConsume，所以这里
// 直接对着真实入口断言同一组契约：额度检查必须看对账本。
func TestOrganizationTaskQuotaCheckIgnoresResponsiblePersonalQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("quota", 0).Error)
	relayInfo := &relaycommon.RelayInfo{
		UserId:             member.Id,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organization.Id,
		OrganizationId:     organization.Id,
		RequestId:          "org-task-quota-personal",
	}

	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
}

func TestPersonalTaskQuotaCheckStillUsesUserQuota(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "personal-task-quota", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 0).Error)
	relayInfo := &relaycommon.RelayInfo{UserId: user.Id, BillingAccountType: model.AccountContextTypePersonal, BillingAccountId: user.Id}

	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)

	require.NotNil(t, apiErr)
	require.Equal(t, relaykittypes.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
}

func TestOrganizationTaskQuotaCheckRejectsInsufficientTokenQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "limited-task-key", ExpiredTime: -1, RemainQuota: 50})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "limited-task-key"

	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)

	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "token quota")
}

func TestOrganizationTaskQuotaCheckAllowsUnlimitedTokenQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "unlimited-task-key", ExpiredTime: -1, UnlimitedQuota: true})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "unlimited-task-key"

	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
}

func TestOrganizationTaskRefundReturnsQuotaToOrganizationAndToken(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "org-task-refund-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	task := &model.Task{
		UserId:             member.Id,
		Quota:              100,
		TaskID:             "org-task-refund",
		TokenId:            token.Id,
		TokenKey:           token.Key,
		TokenUnlimited:     token.UnlimitedQuota,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organization.Id,
		OrganizationId:     organization.Id,
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            organization.Id,
		ResponsibleUserId:  member.Id,
	}
	require.NoError(t, model.DB.Create(task).Error)
	relayInfo := TaskBillingRelayInfo(task)
	require.NoError(t, PostConsumeQuota(relayInfo, 100, 0, false))

	require.True(t, RefundTaskQuota(context.Background(), task, "test"))

	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrg.UsedQuota)
	var storedUser model.User
	require.NoError(t, model.DB.First(&storedUser, member.Id).Error)
	require.Zero(t, storedUser.Quota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 1000, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
}

func TestOrganizationTaskSessionFailureWritesPreConsumeAndRefundLedger(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "org-task-session-refund-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-task-session-refund"
	_, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	task := &model.Task{
		UserId:                        member.Id,
		Quota:                         100,
		TaskID:                        "org-task-session-refund",
		TokenId:                       token.Id,
		TokenKey:                      token.Key,
		BillingAccountType:            model.AccountContextTypeOrganization,
		BillingAccountId:              organization.Id,
		OrganizationId:                organization.Id,
		ScopeType:                     model.AccountContextTypeOrganization,
		ScopeId:                       organization.Id,
		ResponsibleUserId:             member.Id,
		RequestId:                     relayInfo.RequestId,
		OrganizationBillingSessionId:  relayInfo.OrganizationBillingSessionId,
		OrganizationBillingSessionKey: relayInfo.OrganizationBillingSessionKey,
	}
	require.NoError(t, model.DB.Create(task).Error)

	require.True(t, RefundTaskQuota(context.Background(), task, "test"))

	var records []model.OrganizationBillingRecord
	require.NoError(t, model.DB.Where("session_id = ?", relayInfo.OrganizationBillingSessionId).Order("id asc").Find(&records).Error)
	require.Len(t, records, 2)
	require.Equal(t, model.OrganizationBillingRecordTypePreConsume, records[0].RecordType)
	require.Equal(t, 100, records[0].UsedQuotaDelta)
	require.Zero(t, records[0].UsageQuota)
	require.Equal(t, model.OrganizationBillingRecordTypeRefund, records[1].RecordType)
	require.Equal(t, -100, records[1].UsedQuotaDelta)
	require.Zero(t, records[1].UsageQuota)
}

func TestOrganizationTaskAdjustQuotaChargesOrganizationAndTokenOnly(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "org-task-adjust-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	task := &model.Task{
		UserId:             member.Id,
		Quota:              100,
		TaskID:             "org-task-adjust",
		ChannelId:          0,
		TokenId:            token.Id,
		TokenKey:           token.Key,
		TokenUnlimited:     token.UnlimitedQuota,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organization.Id,
		OrganizationId:     organization.Id,
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            organization.Id,
		ResponsibleUserId:  member.Id,
	}
	require.NoError(t, model.DB.Create(task).Error)
	relayInfo := TaskBillingRelayInfo(task)
	require.NoError(t, PostConsumeQuota(relayInfo, 100, 0, false))

	RecalculateTaskQuota(context.Background(), task, 150, "test")

	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+150, storedOrg.UsedQuota)
	require.Equal(t, organization.RequestCount+1, storedOrg.RequestCount)
	var storedUser model.User
	require.NoError(t, model.DB.First(&storedUser, member.Id).Error)
	require.Zero(t, storedUser.UsedQuota)
	require.Zero(t, storedUser.RequestCount)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 850, storedToken.RemainQuota)
	require.Equal(t, 150, storedToken.UsedQuota)
}

func TestOrganizationMidjourneyRefundReturnsQuotaToOrganizationAndToken(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "org-mj-refund-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	task := &model.Midjourney{
		UserId:             member.Id,
		Quota:              100,
		MjId:               "org-mj-refund",
		TokenId:            token.Id,
		TokenKey:           token.Key,
		TokenUnlimited:     token.UnlimitedQuota,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organization.Id,
		OrganizationId:     organization.Id,
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            organization.Id,
		ResponsibleUserId:  member.Id,
	}
	require.NoError(t, model.DB.Create(task).Error)
	relayInfo := MidjourneyBillingRelayInfo(task)
	require.NoError(t, PostConsumeQuota(relayInfo, 100, 0, false))

	require.True(t, RefundMidjourneyQuota(context.Background(), task, "test"))

	var storedOrg model.Organization
	require.NoError(t, model.DB.First(&storedOrg, organization.Id).Error)
	require.Equal(t, organization.UsedQuota, storedOrg.UsedQuota)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 1000, storedToken.RemainQuota)
	require.Zero(t, storedToken.UsedQuota)
}

func TestOrganizationTaskSettleDifferentActualReplayConflictsWithoutMutation(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "task-settle-conflict-replay", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "task-settle-conflict-replay"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	task := &model.Task{
		UserId:                        member.Id,
		Quota:                         100,
		TaskID:                        "task-settle-conflict-replay",
		TokenId:                       token.Id,
		TokenKey:                      token.Key,
		BillingAccountType:            model.AccountContextTypeOrganization,
		BillingAccountId:              organization.Id,
		OrganizationId:                organization.Id,
		ScopeType:                     model.AccountContextTypeOrganization,
		ScopeId:                       organization.Id,
		CreatorUserId:                 token.CreatorUserId,
		ResponsibleUserId:             member.Id,
		RequestId:                     relayInfo.RequestId,
		OrganizationBillingSessionId:  relayInfo.OrganizationBillingSessionId,
		OrganizationBillingSessionKey: relayInfo.OrganizationBillingSessionKey,
	}
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, session.Settle(140))

	_, err = SettleAsyncTaskOrganizationQuota(task, 160)

	require.EqualError(t, err, "organization idempotency conflict")
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrganization.UsedQuota)
	require.Equal(t, organization.RequestCount+1, storedOrganization.RequestCount)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 860, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)
	var storedSession model.OrganizationBillingSession
	require.NoError(t, model.DB.First(&storedSession, relayInfo.OrganizationBillingSessionId).Error)
	require.Equal(t, model.OrganizationBillingSessionStatusSettled, storedSession.Status)
	require.Equal(t, 140, storedSession.SettledQuota)
	var storedTask model.Task
	require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
	require.Equal(t, 100, storedTask.Quota)
}

func TestOrganizationTaskSettleCrashReplayFillsTaskQuotaOnly(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "task-settle-crash-replay", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "task-settle-crash-replay"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	task := &model.Task{
		UserId:                        member.Id,
		Quota:                         100,
		TaskID:                        "task-settle-crash-replay",
		TokenId:                       token.Id,
		TokenKey:                      token.Key,
		BillingAccountType:            model.AccountContextTypeOrganization,
		BillingAccountId:              organization.Id,
		OrganizationId:                organization.Id,
		ScopeType:                     model.AccountContextTypeOrganization,
		ScopeId:                       organization.Id,
		CreatorUserId:                 token.CreatorUserId,
		ResponsibleUserId:             member.Id,
		RequestId:                     relayInfo.RequestId,
		OrganizationBillingSessionId:  relayInfo.OrganizationBillingSessionId,
		OrganizationBillingSessionKey: relayInfo.OrganizationBillingSessionKey,
	}
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, session.Settle(140))

	handled, err := SettleAsyncTaskOrganizationQuota(task, 140)
	require.NoError(t, err)
	require.True(t, handled)
	require.NoError(t, task.UpdateQuota())
	handled, err = SettleAsyncTaskOrganizationQuota(task, 140)
	require.NoError(t, err)
	require.True(t, handled)
	require.NoError(t, task.UpdateQuota())

	var storedTask model.Task
	require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
	require.Equal(t, 140, storedTask.Quota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrganization.UsedQuota)
	require.Equal(t, organization.RequestCount+1, storedOrganization.RequestCount)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 860, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)
}

func TestOrganizationMidjourneySettleCrashReplayFillsTaskQuotaOnly(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "midjourney-settle-crash-replay", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "midjourney-settle-crash-replay"
	session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
	require.Nil(t, apiErr)
	task := &model.Midjourney{
		UserId:                        member.Id,
		Quota:                         100,
		MjId:                          "midjourney-settle-crash-replay",
		TokenId:                       token.Id,
		TokenKey:                      token.Key,
		BillingAccountType:            model.AccountContextTypeOrganization,
		BillingAccountId:              organization.Id,
		OrganizationId:                organization.Id,
		ScopeType:                     model.AccountContextTypeOrganization,
		ScopeId:                       organization.Id,
		CreatorUserId:                 token.CreatorUserId,
		ResponsibleUserId:             member.Id,
		RequestId:                     relayInfo.RequestId,
		OrganizationBillingSessionId:  relayInfo.OrganizationBillingSessionId,
		OrganizationBillingSessionKey: relayInfo.OrganizationBillingSessionKey,
	}
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, session.Settle(140))

	handled, err := SettleMidjourneyOrganizationQuota(task, 140)
	require.NoError(t, err)
	require.True(t, handled)
	require.NoError(t, task.UpdateBillingState())
	handled, err = SettleMidjourneyOrganizationQuota(task, 140)
	require.NoError(t, err)
	require.True(t, handled)
	require.NoError(t, task.UpdateBillingState())

	var storedTask model.Midjourney
	require.NoError(t, model.DB.First(&storedTask, task.Id).Error)
	require.Equal(t, 140, storedTask.Quota)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrganization.UsedQuota)
	require.Equal(t, organization.RequestCount+1, storedOrganization.RequestCount)
	storedToken, err := model.GetTokenById(token.Id)
	require.NoError(t, err)
	require.Equal(t, 860, storedToken.RemainQuota)
	require.Equal(t, 140, storedToken.UsedQuota)
}

func TestInitTaskPersistsOrganizationBillingAndTokenScope(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "org-init-task-key", ExpiredTime: -1, UnlimitedQuota: true})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.UsingGroup = "default"
	relayInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 3}

	task := model.InitTask(constant.TaskPlatformSuno, relayInfo)

	require.Equal(t, model.AccountContextTypeOrganization, task.ScopeType)
	require.Equal(t, organization.Id, task.ScopeId)
	require.Equal(t, model.AccountContextTypeOrganization, task.BillingAccountType)
	require.Equal(t, organization.Id, task.BillingAccountId)
	require.Equal(t, organization.Id, task.OrganizationId)
	require.Equal(t, member.Id, task.ResponsibleUserId)
	require.Equal(t, token.Id, task.TokenId)
	require.Equal(t, token.Key, task.TokenKey)
	require.True(t, task.TokenUnlimited)
}

func TestPersonalLegacyTaskRefundStillReturnsQuotaToUser(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "legacy-personal-task", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 1000).Error)
	task := &model.Task{UserId: user.Id, Quota: 100, TaskID: "legacy-personal-task"}
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, PostConsumeQuota(TaskBillingRelayInfo(task), 100, 0, false))

	require.True(t, RefundTaskQuota(context.Background(), task, "test"))

	var stored model.User
	require.NoError(t, model.DB.First(&stored, user.Id).Error)
	require.Equal(t, 1000, stored.Quota)
}

// TestPersonalBillingUntouchedByPrecedingOrganizationRequest 是组织计费上线前
// 必须锁死的那条底线：一次组织密钥的完整计费（预扣 -> 结算）之后，同一个人
// 再用个人密钥计费，个人的钱包余额、订阅用量和个人看板必须与组织那次请求
// 之前逐字段一致。组织那笔钱只能从组织账户出。
func TestPersonalBillingUntouchedByPrecedingOrganizationRequest(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	const personalQuota = 5000
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("quota", personalQuota).Error)
	subscriptionId := member.Id + 500000
	seedSubscription(t, subscriptionId, member.Id, 1000, 0)

	orgToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "personal-isolation-org-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	orgRelayInfo := organizationBillingRelayInfo(orgToken, organization.Id)
	orgRelayInfo.RequestId = "personal-isolation-org-request"
	orgSession, apiErr := NewBillingSession(&gin.Context{}, orgRelayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, orgSession.Settle(140))

	// 组织那笔账走完了；此刻个人侧必须一动不动。
	var afterOrg model.User
	require.NoError(t, model.DB.First(&afterOrg, member.Id).Error)
	require.Equal(t, personalQuota, afterOrg.Quota)
	require.Zero(t, afterOrg.UsedQuota)
	require.Zero(t, afterOrg.RequestCount)
	var afterOrgSubscription model.UserSubscription
	require.NoError(t, model.DB.First(&afterOrgSubscription, subscriptionId).Error)
	require.Zero(t, afterOrgSubscription.AmountUsed)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, organization.UsedQuota+140, storedOrganization.UsedQuota)

	// 同一个人的个人密钥照常计费，且只动它自己那份。
	personalToken := model.Token{UserId: member.Id, Key: common.GetRandomString(48), Name: "personal-after-org-key", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 1000, Visibility: model.TokenVisibilityPrivate}
	require.NoError(t, model.DB.Create(&personalToken).Error)
	personalRelayInfo := &relaycommon.RelayInfo{
		UserId:             member.Id,
		TokenId:            personalToken.Id,
		TokenKey:           personalToken.Key,
		ScopeType:          model.AccountContextTypePersonal,
		ScopeId:            member.Id,
		BillingAccountType: model.AccountContextTypePersonal,
		BillingAccountId:   member.Id,
		RequestId:          "personal-isolation-personal-request",
	}
	personalRelayInfo.UserSetting.BillingPreference = "wallet_only"
	personalSession, apiErr := NewBillingSession(&gin.Context{}, personalRelayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, personalSession.Settle(80))

	var afterPersonal model.User
	require.NoError(t, model.DB.First(&afterPersonal, member.Id).Error)
	require.Equal(t, personalQuota-80, afterPersonal.Quota)
	var afterPersonalSubscription model.UserSubscription
	require.NoError(t, model.DB.First(&afterPersonalSubscription, subscriptionId).Error)
	require.Zero(t, afterPersonalSubscription.AmountUsed)
	// 个人密钥自己的额度照扣，说明上面少的 80 确实是这一笔。
	storedPersonalToken, err := model.GetTokenById(personalToken.Id)
	require.NoError(t, err)
	require.Equal(t, 920, storedPersonalToken.RemainQuota)
	require.Equal(t, 80, storedPersonalToken.UsedQuota)
}
