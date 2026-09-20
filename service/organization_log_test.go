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
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createOrganizationLogFixture(t *testing.T) (model.User, model.User, *model.Organization, *model.Token, *model.Token) {
	t.Helper()
	admin, member, organization := createOrganizationTokenTestOrg(t)
	memberToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-log-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	adminToken, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "admin-log-key", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	insertOrganizationConsumeLog(t, organization.Id, memberToken, 10, "member-secret")
	insertOrganizationConsumeLog(t, organization.Id, adminToken, 20, "admin-secret")
	return admin, member, organization, memberToken, adminToken
}

func insertOrganizationConsumeLog(t *testing.T, organizationId int, token *model.Token, quota int, content string) {
	t.Helper()
	insertOrganizationLog(t, organizationId, token, model.LogTypeConsume, quota, content, model.Log{})
}

func insertOrganizationLog(t *testing.T, organizationId int, token *model.Token, logType int, quota int, content string, overrides model.Log) {
	t.Helper()
	log := model.Log{
		UserId:             token.ResponsibleUserId,
		CreatedAt:          time.Now().Unix(),
		Type:               logType,
		Content:            content,
		TokenId:            token.Id,
		TokenName:          token.Name,
		Quota:              quota,
		PromptTokens:       quota,
		CompletionTokens:   quota,
		ModelName:          "gpt-4o",
		Group:              "default",
		RequestId:          "req-default",
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            organizationId,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organizationId,
		OrganizationId:     organizationId,
		CreatorUserId:      token.CreatorUserId,
		ResponsibleUserId:  token.ResponsibleUserId,
	}
	if overrides.CreatedAt != 0 {
		log.CreatedAt = overrides.CreatedAt
	}
	if overrides.Type != 0 {
		log.Type = overrides.Type
	}
	if overrides.TokenName != "" {
		log.TokenName = overrides.TokenName
	}
	if overrides.ModelName != "" {
		log.ModelName = overrides.ModelName
	}
	if overrides.Group != "" {
		log.Group = overrides.Group
	}
	if overrides.RequestId != "" {
		log.RequestId = overrides.RequestId
	}
	if overrides.PromptTokens != 0 {
		log.PromptTokens = overrides.PromptTokens
	}
	if overrides.CompletionTokens != 0 {
		log.CompletionTokens = overrides.CompletionTokens
	}
	require.NoError(t, model.LOG_DB.Create(&log).Error)
	if log.Type == model.LogTypeConsume && log.BillingAccountType == model.AccountContextTypeOrganization {
		require.NoError(t, model.DB.Create(&model.OrganizationBillingRecord{
			OrganizationId:    organizationId,
			RecordKey:         "test-log:" + strconv.Itoa(organizationId) + ":" + log.RequestId + ":" + strconv.Itoa(log.Id) + ":" + strconv.FormatInt(time.Now().UnixNano(), 10),
			RequestId:         log.RequestId,
			RecordType:        model.OrganizationBillingRecordTypeSettle,
			UsedQuotaDelta:    quota,
			UsageQuota:        quota,
			TokenId:           token.Id,
			TokenName:         log.TokenName,
			ResponsibleUserId: token.ResponsibleUserId,
			CreatorUserId:     token.CreatorUserId,
			ModelName:         log.ModelName,
			Group:             log.Group,
			PromptTokens:      log.PromptTokens,
			CompletionTokens:  log.CompletionTokens,
			TokenCount:        log.PromptTokens + log.CompletionTokens,
			CreatedAt:         log.CreatedAt,
		}).Error)
	}
}

func useStandaloneLogDBWithOrganizationLogs(t *testing.T, organizationId int) {
	t.Helper()
	oldLogDB := model.LOG_DB
	var logs []model.Log
	require.NoError(t, oldLogDB.Where("organization_id = ?", organizationId).Find(&logs).Error)
	logDB, err := gorm.Open(sqlite.Open(t.TempDir()+"/logs.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	for i := range logs {
		logs[i].Id = 0
		require.NoError(t, logDB.Create(&logs[i]).Error)
	}
	model.LOG_DB = logDB
	t.Cleanup(func() {
		model.LOG_DB = oldLogDB
	})
}

func TestOrganizationLogsFiltersByTypeModelTokenGroupRequestId(t *testing.T) {
	admin, _, organization, memberToken, adminToken := createOrganizationLogFixture(t)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeError, 0, "error", model.Log{ModelName: "claude-3-5", Group: "vip", RequestId: "req-error"})
	insertOrganizationLog(t, organization.Id, adminToken, model.LogTypeConsume, 30, "target", model.Log{TokenName: "admin-log-key", ModelName: "gpt-4o-mini", Group: "vip", RequestId: "req-target"})

	logs, total, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20, Type: model.LogTypeConsume, TokenName: "admin-log-key", ModelName: "gpt-4o%", Group: "vip", RequestId: "req-target"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, adminToken.Id, logs[0].TokenId)
	require.Equal(t, "req-target", logs[0].RequestId)
}

func TestOrganizationLogsMemberCannotBypassResponsibleUserFilter(t *testing.T) {
	admin, member, organization, memberToken, _ := createOrganizationLogFixture(t)

	logs, total, err := ListOrganizationLogs(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20, ResponsibleUserId: admin.Id})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, memberToken.Id, logs[0].TokenId)
	require.Equal(t, member.Id, logs[0].ResponsibleUserId)
}

func TestOrganizationLogsModelNameUsesSafeLikeSearch(t *testing.T) {
	admin, _, organization, _, _ := createOrganizationLogFixture(t)

	_, _, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20, ModelName: "%%%"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "%")
}

func TestOrganizationLogsTokenNameUsesExactMatchNotPrefix(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 30, "prefix-a", model.Log{TokenName: "test-org-apikey-0003", RequestId: "prefix-a"})
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 40, "prefix-b", model.Log{TokenName: "test-org-apikey-0004", RequestId: "prefix-b"})
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 50, "exact", model.Log{TokenName: "test-org-apikey-0005", RequestId: "exact"})

	logs, total, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20, TokenName: "test-org-apikey-0005"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "test-org-apikey-0005", logs[0].TokenName)
	require.Equal(t, "exact", logs[0].RequestId)
}

func TestOrganizationLogsAdminCanFilterByResponsibleName(t *testing.T) {
	admin, member, organization, memberToken, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Updates(map[string]interface{}{"username": "responsible-log-owner", "display_name": "Responsible Log Owner"}).Error)

	logs, total, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20, ResponsibleName: "Responsible Log Owner"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, memberToken.Id, logs[0].TokenId)
	require.Equal(t, member.Id, logs[0].ResponsibleUserId)
	require.Equal(t, "responsible-log-owner", logs[0].ResponsibleUsername)
	require.Equal(t, "Responsible Log Owner", logs[0].ResponsibleDisplayName)
}

func TestOrganizationLogsPreferStoredResponsibleSnapshot(t *testing.T) {
	admin, member, organization, memberToken, _ := createOrganizationLogFixture(t)
	requestId := "snapshot-log"
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("token_id = ? AND responsible_user_id = ?", memberToken.Id, member.Id).Updates(map[string]interface{}{
		"request_id":       requestId,
		"username":         "snapshot-username",
		"responsible_name": "Snapshot Display",
	}).Error)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Updates(map[string]interface{}{"username": "changed-username", "display_name": "Changed Display"}).Error)

	logs, total, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20, RequestId: requestId})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "snapshot-username", logs[0].ResponsibleUsername)
	require.Equal(t, "Snapshot Display", logs[0].ResponsibleDisplayName)
}

func TestOrganizationLogsResponsibleNameUsesSnapshotsWithoutLogUsersTable(t *testing.T) {
	admin, member, organization, memberToken, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("token_id = ? AND responsible_user_id = ?", memberToken.Id, member.Id).Updates(map[string]interface{}{
		"username":         "snapshot-standalone",
		"responsible_name": "Standalone Snapshot",
	}).Error)
	useStandaloneLogDBWithOrganizationLogs(t, organization.Id)

	logs, total, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20, ResponsibleName: "Standalone Snapshot"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "snapshot-standalone", logs[0].ResponsibleUsername)
	require.Equal(t, "Standalone Snapshot", logs[0].ResponsibleDisplayName)
}

func TestOrganizationLogsMemberCannotBypassResponsibleNameFilter(t *testing.T) {
	_, member, organization, memberToken, _ := createOrganizationLogFixture(t)

	logs, total, err := ListOrganizationLogs(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20, ResponsibleName: "other-user"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, memberToken.Id, logs[0].TokenId)
	require.Equal(t, member.Id, logs[0].ResponsibleUserId)
}

func TestOrganizationLogsAdminCanSeeAll(t *testing.T) {
	admin, _, organization, _, _ := createOrganizationLogFixture(t)

	logs, total, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, logs, 2)
}

func TestOrganizationLogsMemberOnlySeesOwnResponsibleLogs(t *testing.T) {
	_, member, organization, memberToken, _ := createOrganizationLogFixture(t)

	logs, total, err := ListOrganizationLogs(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, memberToken.Id, logs[0].TokenId)
	require.Equal(t, member.Id, logs[0].ResponsibleUserId)
}

func TestOrganizationLogsContentIsRedacted(t *testing.T) {
	admin, _, organization, _, _ := createOrganizationLogFixture(t)

	logs, _, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20})

	require.NoError(t, err)
	require.NotEmpty(t, logs)
	for _, log := range logs {
		require.Empty(t, log.Content)
	}
}

func TestOrganizationLogsExcludePersonalLogs(t *testing.T) {
	admin, _, organization, _, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: admin.Id, Type: model.LogTypeConsume, Content: "personal", ScopeType: model.AccountContextTypePersonal, ScopeId: admin.Id, BillingAccountType: model.AccountContextTypePersonal, BillingAccountId: admin.Id, Quota: 99}).Error)

	logs, total, err := ListOrganizationLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	for _, log := range logs {
		require.Equal(t, model.AccountContextTypeOrganization, log.ScopeType)
	}
}

func TestOrganizationLogStatsFiltersTokenAndResponsibleUser(t *testing.T) {
	admin, member, organization, memberToken, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("token_id = ?", memberToken.Id).Updates(map[string]interface{}{"created_at": time.Now().Unix()}).Error)

	stats, err := GetOrganizationLogStats(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{ResponsibleUserId: member.Id, TokenId: memberToken.Id})

	require.NoError(t, err)
	require.Equal(t, 10, stats.Quota)
	require.Equal(t, 1, stats.Rpm)
	require.Equal(t, 20, stats.Tpm)
}

func TestOrganizationLogStatsQuotaUsesSelectedDateRange(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 30, "inside", model.Log{CreatedAt: 2000, RequestId: "inside"})
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 70, "outside", model.Log{CreatedAt: 3000, RequestId: "outside"})

	stats, err := GetOrganizationLogStats(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{StartTimestamp: 1500, EndTimestamp: 2500})

	require.NoError(t, err)
	require.Equal(t, 30, stats.Quota)
}

func TestOrganizationLogStatsRpmTpmUseLastSixtySeconds(t *testing.T) {
	admin, _, organization, memberToken, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("organization_id = ?", organization.Id).Updates(map[string]interface{}{"created_at": time.Now().Add(-120 * time.Second).Unix()}).Error)
	recent := time.Now().Unix()
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 30, "recent", model.Log{CreatedAt: recent, PromptTokens: 11, CompletionTokens: 13, RequestId: "recent"})
	insertOrganizationLog(t, organization.Id, memberToken, model.LogTypeConsume, 70, "old", model.Log{CreatedAt: recent - 120, PromptTokens: 17, CompletionTokens: 19, RequestId: "old"})

	stats, err := GetOrganizationLogStats(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{})

	require.NoError(t, err)
	require.Equal(t, 130, stats.Quota)
	require.Equal(t, 1, stats.Rpm)
	require.Equal(t, 24, stats.Tpm)
}

func TestOrganizationLogStatsMemberOnlySeesOwnResponsibleLogs(t *testing.T) {
	admin, member, organization, _, _ := createOrganizationLogFixture(t)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("responsible_user_id IN ?", []int{admin.Id, member.Id}).Updates(map[string]interface{}{"created_at": time.Now().Unix()}).Error)

	stats, err := GetOrganizationLogStats(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationLogListRequest{ResponsibleUserId: admin.Id})

	require.NoError(t, err)
	require.Equal(t, 10, stats.Quota)
	require.Equal(t, 1, stats.Rpm)
	require.Equal(t, 20, stats.Tpm)
}
