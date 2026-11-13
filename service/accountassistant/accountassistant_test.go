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

package accountassistant

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/agentkit"
	"github.com/QuantumNous/new-api/setting/account_assistant_setting"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAccountAssistantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	useAccountAssistantTestDB(t, db)
	return db
}

// useAccountAssistantTestDB 把测试库接到包的全局连接上，并在用例结束后还原。
func useAccountAssistantTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
	})
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Task{},
		&model.Midjourney{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.Log{},
	))
}

func seedAccountAssistantUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&[]model.User{
		{
			Id: 1, Username: "alice", Password: "password-1", DisplayName: "Alice",
			Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			Quota: 1000, UsedQuota: 500, RequestCount: 3, Group: "default", AffCode: "alice-aff",
			Email: "alice@example.com",
		},
		{
			Id: 2, Username: "bob", Password: "password-2", DisplayName: "Bob",
			Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			Quota: 999999, UsedQuota: 88888, RequestCount: 99, Group: "vip", AffCode: "bob-aff",
			Email: "bob@example.com",
		},
	}).Error)
}

func TestBuildRegistryExposesOrganizationToolsOnlyForAdmins(t *testing.T) {
	// Arrange
	setting := account_assistant_setting.AccountAssistantSetting{FAQJSON: "[]"}
	personal := buildRegistry(agentkit.Principal{UserID: 1}, setting)
	orgAdmin := buildRegistry(agentkit.Principal{UserID: 1, IsOrganizationAdmin: true}, setting)

	// Act
	personalNames := toolNames(personal)
	adminNames := toolNames(orgAdmin)

	// Assert
	assert.Len(t, personalNames, 6)
	assert.Len(t, adminNames, 9)
	assert.NotContains(t, personalNames, "query_org_usage")
	assert.Contains(t, adminNames, "query_org_usage")
	assert.Contains(t, adminNames, "list_org_members")
	assert.Contains(t, adminNames, "get_org_summary")
}

func toolNames(registry *agentkit.Registry) []string {
	definitions := registry.Definitions()
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Function.Name)
	}
	return names
}

func TestGetMyAccountOnlyReturnsPrincipalData(t *testing.T) {
	// Arrange
	db := setupAccountAssistantTestDB(t)
	seedAccountAssistantUsers(t, db)
	tool := newGetMyAccountTool()

	// Act
	output, err := tool.Run(context.Background(), agentkit.Principal{UserID: 1}, `{}`)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, `"username":"alice"`)
	assert.Contains(t, output, `"balance_quota":1000`)
	assert.NotContains(t, output, "bob")
	assert.NotContains(t, output, "999999")
}

func TestListMyKeysMasksKeysAndStaysInPrincipalScope(t *testing.T) {
	// Arrange
	db := setupAccountAssistantTestDB(t)
	seedAccountAssistantUsers(t, db)
	require.NoError(t, db.Create(&[]model.Token{
		{Id: 1, UserId: 1, Key: "sk-alice-secret-key-123456", Name: "alice-key", Status: common.TokenStatusEnabled, ScopeType: "personal", RemainQuota: 100},
		{Id: 2, UserId: 2, Key: "sk-bob-secret-key-654321", Name: "bob-key", Status: common.TokenStatusEnabled, ScopeType: "personal", RemainQuota: 200},
	}).Error)
	tool := newListMyKeysTool()

	// Act
	output, err := tool.Run(context.Background(), agentkit.Principal{UserID: 1}, `{}`)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "alice-key")
	assert.NotContains(t, output, "bob-key")
	assert.NotContains(t, output, "alice-secret-key")
	assert.Contains(t, output, "sk-a")
}

func TestExplainMyChargeRejectsAnotherUsersLog(t *testing.T) {
	// Arrange
	db := setupAccountAssistantTestDB(t)
	seedAccountAssistantUsers(t, db)
	require.NoError(t, db.Create(&model.Log{
		Id: 10, UserId: 2, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText,
		ModelName: "gpt-4o", Quota: 50, CreatedAt: time.Now().Unix(),
	}).Error)
	tool := newExplainMyChargeTool()

	// Act
	output, err := tool.Run(context.Background(), agentkit.Principal{UserID: 1}, `{"log_id":10}`)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, `"found":false`)
	assert.NotContains(t, output, "gpt-4o")
}

func TestQueryMyUsageOnlyReturnsPrincipalUsage(t *testing.T) {
	// Arrange
	db := setupAccountAssistantTestDB(t)
	seedAccountAssistantUsers(t, db)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 1, ActorUserId: 1, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: now, ModelName: "model-a", Quota: 70, PromptTokens: 10},
		{UserId: 2, ActorUserId: 2, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: now, ModelName: "model-b", Quota: 999, PromptTokens: 100},
	}).Error)
	tool := newQueryMyUsageTool()

	// Act
	output, err := tool.Run(context.Background(), agentkit.Principal{UserID: 1}, `{"days":7}`)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "model-a")
	assert.NotContains(t, output, "model-b")
	assert.NotContains(t, output, `"quota":999`)
}

func TestGetMyTasksRejectsAnotherUsersTask(t *testing.T) {
	// Arrange
	db := setupAccountAssistantTestDB(t)
	seedAccountAssistantUsers(t, db)
	require.NoError(t, db.Create(&model.Task{
		TaskID: "task-bob", UserId: 2, Platform: "openai", Status: model.TaskStatusSuccess,
		Action: "generate", SubmitTime: time.Now().Unix(),
	}).Error)
	tool := newGetMyTasksTool()

	// Act
	output, err := tool.Run(context.Background(), agentkit.Principal{UserID: 1}, `{"task_id":"task-bob"}`)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, `"found":false`)
	assert.NotContains(t, output, "task-bob")
}

func TestOrganizationToolsRequireOwnerOrAdmin(t *testing.T) {
	// Arrange
	db := setupAccountAssistantTestDB(t)
	seedAccountAssistantUsers(t, db)
	require.NoError(t, db.Create(&model.Organization{
		Id: 9, Name: "Acme", NameNormalized: "acme", Slug: "acme",
		Status: model.OrganizationStatusActive, OwnerUserId: 1, CreatedBy: 1,
	}).Error)
	require.NoError(t, db.Create(&[]model.OrganizationMember{
		{OrganizationId: 9, UserId: 1, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: 9, UserId: 2, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
	}).Error)
	tool := newListOrgMembersTool()

	// Act
	_, memberErr := tool.Run(context.Background(), agentkit.Principal{UserID: 2}, `{"organization_id":9}`)
	output, ownerErr := tool.Run(context.Background(), agentkit.Principal{UserID: 1}, `{"organization_id":9}`)

	// Assert
	require.Error(t, memberErr)
	assert.Contains(t, memberErr.Error(), "无权查看")
	require.NoError(t, ownerErr)
	assert.Contains(t, output, "alice")
	assert.Contains(t, output, "bob")
	assert.Contains(t, output, "al***@example.com")
	assert.NotContains(t, output, "alice@example.com")
}

func TestConsumeRateSlotEnforcesLimitWithoutRedis(t *testing.T) {
	// Arrange
	previousRedisEnabled := common.RedisEnabled
	previousRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
	})
	limiter := new(common.InMemoryRateLimiter)
	limiter.Init(time.Minute)
	userID := 987654

	// Act & Assert
	assert.True(t, consumeRateSlot("test-scope", userID, 2, time.Minute, limiter))
	assert.True(t, consumeRateSlot("test-scope", userID, 2, time.Minute, limiter))
	assert.False(t, consumeRateSlot("test-scope", userID, 2, time.Minute, limiter))
}

func TestTokenBudgetFallbackTracksUsage(t *testing.T) {
	// Arrange
	previousRedisEnabled := common.RedisEnabled
	previousRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
		tokenBudgetMu.Lock()
		delete(tokenBudgetStore, tokenBudgetKey(987655))
		tokenBudgetMu.Unlock()
	})

	// Act
	addTokenUsage(987655, 120)
	addTokenUsage(987655, 30)

	// Assert
	assert.Equal(t, int64(150), tokenUsage(987655))
}

func TestSanitizeHistoryDropsNonChatRolesAndTruncates(t *testing.T) {
	// Arrange
	long := strings.Repeat("字", maxHistoryRunes+50)
	history := []dto.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "tool", Content: "tool output"},
		{Role: "user", Content: long},
		{Role: "assistant", Content: "answer"},
	}

	// Act
	sanitized := sanitizeHistory(history)

	// Assert
	require.Len(t, sanitized, 2)
	assert.Equal(t, "user", sanitized[0].Role)
	assert.Len(t, []rune(sanitized[0].StringContent()), maxHistoryRunes)
	assert.Equal(t, "assistant", sanitized[1].Role)
}

func TestMaskEmailKeepsDomainButHidesLocalPart(t *testing.T) {
	assert.Equal(t, "al***@example.com", maskEmail("alice@example.com"))
	assert.Equal(t, "***", maskEmail("a"))
	assert.Equal(t, "", maskEmail(""))
}

func TestQuotaToUSDConvertsInternalQuota(t *testing.T) {
	previous := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previous })

	assert.InDelta(t, 0.002, quotaToUSD(1000), 1e-9)
	assert.Equal(t, 0.0, quotaToUSD(0))
}
