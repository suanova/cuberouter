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
package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPlaygroundFallbackTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.UserSubscription{},
	))

	t.Cleanup(func() {
		if sqlDB, err := model.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainType, originalLogType)
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})
}

func createPlaygroundFallbackUser(t *testing.T, username, group string, quota int) model.User {
	t.Helper()
	user := model.User{
		Username:    username,
		DisplayName: username,
		Status:      common.UserStatusEnabled,
		Group:       group,
		Quota:       quota,
		AuthVersion: 1,
		AffCode:     username,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func createPlaygroundFallbackOrg(t *testing.T, name, group string, quota, usedQuota int) *model.Organization {
	t.Helper()
	organization := &model.Organization{
		Name:      name,
		Slug:      strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		Group:     group,
		Status:    model.OrganizationStatusActive,
		Quota:     quota,
		UsedQuota: usedQuota,
	}
	require.NoError(t, model.DB.Create(organization).Error)
	return organization
}

func joinPlaygroundFallbackOrg(t *testing.T, userId, organizationId int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organizationId,
		UserId:         userId,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)
}

func createActivePlaygroundSubscription(t *testing.T, userId int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId:      userId,
		AmountTotal: 1000000,
		Status:      "active",
		StartTime:   common.GetTimestamp() - 60,
		EndTime:     common.GetTimestamp() + 3600,
	}).Error)
}

func setPlaygroundFallbackPreference(t *testing.T, userId int, preference string) {
	t.Helper()
	userSetting, err := model.GetUserSetting(userId, false)
	require.NoError(t, err)
	userSetting.BillingPreference = preference
	require.NoError(t, model.UpdateUserSetting(userId, userSetting))
}

func playbackFallbackContext(method, path string, userId int) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	if userId > 0 {
		c.Set("id", userId)
	}
	return c
}

func TestShouldSwitchToOrganizationThresholdBoundary(t *testing.T) {
	setupPlaygroundFallbackTestDB(t)
	threshold := setting.PlaygroundOrgFallbackThreshold()

	tests := []struct {
		name  string
		quota int
		want  bool
	}{
		{name: "below threshold", quota: threshold - 1, want: true},
		{name: "exactly threshold", quota: threshold, want: false},
		{name: "above threshold", quota: threshold + 1, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := createPlaygroundFallbackUser(t, "pg-fb-"+strings.ReplaceAll(test.name, " ", "-"), "pg-fb-user", test.quota)
			require.Equal(t, test.want, shouldSwitchToOrganization(user.Id))
		})
	}
}

func TestShouldSwitchToOrganizationSubscriptionGuard(t *testing.T) {
	setupPlaygroundFallbackTestDB(t)

	tests := []struct {
		name       string
		preference string
		withSub    bool
		want       bool
	}{
		{name: "subscription_first with subscription keeps subscription", preference: "subscription_first", withSub: true, want: false},
		{name: "subscription_first without subscription falls to wallet", preference: "subscription_first", withSub: false, want: true},
		{name: "subscription_only with subscription keeps subscription", preference: "subscription_only", withSub: true, want: false},
		{name: "subscription_only without subscription falls to wallet", preference: "subscription_only", withSub: false, want: true},
		{name: "wallet_first with subscription keeps subscription fallback", preference: "wallet_first", withSub: true, want: false},
		{name: "wallet_first without subscription falls to wallet", preference: "wallet_first", withSub: false, want: true},
		{name: "wallet_only ignores subscription", preference: "wallet_only", withSub: true, want: true},
		{name: "default preference behaves as subscription_first", preference: "", withSub: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := createPlaygroundFallbackUser(t, "pg-fb-sub-"+strings.ReplaceAll(test.name, " ", "-"), "pg-fb-user", 0)
			if test.preference != "" {
				setPlaygroundFallbackPreference(t, user.Id, test.preference)
			}
			if test.withSub {
				createActivePlaygroundSubscription(t, user.Id)
			}
			require.Equal(t, test.want, shouldSwitchToOrganization(user.Id))
		})
	}
}

func TestPlaygroundOrganizationFallbackSwitchesContext(t *testing.T) {
	setupPlaygroundFallbackTestDB(t)
	user := createPlaygroundFallbackUser(t, "pg-fb-switch", "pg-fb-user", 0)
	organization := createPlaygroundFallbackOrg(t, "switch org", "pg-fb-org", 100000, 0)
	joinPlaygroundFallbackOrg(t, user.Id, organization.Id)

	c := playbackFallbackContext(http.MethodPost, "/pg/chat/completions", user.Id)
	PlaygroundOrganizationFallback()(c)

	require.False(t, c.IsAborted())
	assert.Equal(t, model.AccountContextTypeOrganization, common.GetContextKeyString(c, constant.ContextKeyBillingAccountType))
	assert.Equal(t, organization.Id, common.GetContextKeyInt(c, constant.ContextKeyBillingAccountId))
	assert.Equal(t, organization.Id, common.GetContextKeyInt(c, constant.ContextKeyOrganizationId))
	assert.Equal(t, user.Id, common.GetContextKeyInt(c, constant.ContextKeyActorUserId))
	assert.Equal(t, "pg-fb-org", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	assert.Equal(t, "pg-fb-org", common.GetContextKeyString(c, constant.ContextKeyUserGroup))
	assert.Equal(t, "playground-org-pg-fb-org", c.GetString("token_name"))
	assert.Equal(t, 0, c.GetInt("token_id"))

	userSetting, err := model.GetUserSetting(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, organization.Id, userSetting.PlaygroundLastOrganizationId)
}

func TestPlaygroundOrganizationFallbackSkipConditions(t *testing.T) {
	setupPlaygroundFallbackTestDB(t)
	user := createPlaygroundFallbackUser(t, "pg-fb-skip", "pg-fb-user", 0)
	organization := createPlaygroundFallbackOrg(t, "skip org", "pg-fb-org", 100000, 0)
	joinPlaygroundFallbackOrg(t, user.Id, organization.Id)

	t.Run("GET request", func(t *testing.T) {
		c := playbackFallbackContext(http.MethodGet, "/pg/video/generations/t1", user.Id)
		PlaygroundOrganizationFallback()(c)
		assert.Empty(t, common.GetContextKeyString(c, constant.ContextKeyBillingAccountType))
	})

	t.Run("access token (PAT)", func(t *testing.T) {
		c := playbackFallbackContext(http.MethodPost, "/pg/chat/completions", user.Id)
		c.Set("use_access_token", true)
		PlaygroundOrganizationFallback()(c)
		assert.Empty(t, common.GetContextKeyString(c, constant.ContextKeyBillingAccountType))
	})

	t.Run("organization scope already set", func(t *testing.T) {
		c := playbackFallbackContext(http.MethodPost, "/pg/chat/completions", user.Id)
		common.SetContextKey(c, constant.ContextKeyBillingAccountType, model.AccountContextTypeOrganization)
		PlaygroundOrganizationFallback()(c)
		assert.Empty(t, c.GetString("token_name"))
	})
}

func TestPlaygroundOrganizationFallbackKeepsPersonalWithoutUsableOrganization(t *testing.T) {
	setupPlaygroundFallbackTestDB(t)
	user := createPlaygroundFallbackUser(t, "pg-fb-noorg", "pg-fb-user", 0)

	c := playbackFallbackContext(http.MethodPost, "/pg/chat/completions", user.Id)
	PlaygroundOrganizationFallback()(c)

	require.False(t, c.IsAborted())
	assert.Empty(t, common.GetContextKeyString(c, constant.ContextKeyBillingAccountType))
	assert.Empty(t, common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
}

func TestPlaygroundOrganizationFallbackFailsOpenOnQueryError(t *testing.T) {
	setupPlaygroundFallbackTestDB(t)
	user := createPlaygroundFallbackUser(t, "pg-fb-failopen", "pg-fb-user", 0)
	require.NoError(t, model.DB.Migrator().DropTable(&model.Organization{}))

	c := playbackFallbackContext(http.MethodPost, "/pg/chat/completions", user.Id)
	require.NotPanics(t, func() {
		PlaygroundOrganizationFallback()(c)
	})

	require.False(t, c.IsAborted())
	assert.Empty(t, common.GetContextKeyString(c, constant.ContextKeyBillingAccountType))
}
