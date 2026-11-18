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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCubyAppTestDB(t *testing.T) {
	t.Helper()
	setupServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.CubyApp{}, &model.CubyAppGrant{}, &model.Option{}))

	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		// Back to the shipped default before the database goes away.
		require.NoError(t, SetCubyAppManagementEnabled(false))
		common.OptionMap = originalOptionMap
	})
}

func TestListAccountCubyAppsIsNotManagedUntilSwitchedOn(t *testing.T) {
	setupCubyAppTestDB(t)
	admin := createServiceTestUser(t, "cuby-admin", common.RoleAdminUser)
	user := createServiceTestUser(t, "cuby-user", common.RoleCommonUser)
	app, err := CreateCubyApp(admin.Id, CreateCubyAppRequest{AppId: "ai-company", Name: "AI company"})
	require.NoError(t, err)
	_, err = CreateCubyAppGrant(admin.Id, app.Id, model.CubyAppGrantSubjectUser, user.Id)
	require.NoError(t, err)

	// Shipped state: grants may already exist, but the instance must still say
	// "not managed", or every Cuby user's apps would lock on deployment.
	require.False(t, operation_setting.IsCubyAppManagementEnabled())
	_, err = ListAccountCubyApps(user.Id)
	require.ErrorIs(t, err, ErrCubyAppsNotManaged)

	require.NoError(t, SetCubyAppManagementEnabled(true))
	apps, err := ListAccountCubyApps(user.Id)
	require.NoError(t, err)
	assert.Equal(t, []model.AccountCubyApp{
		{Id: "ai-company", Sources: []model.CubyAppSource{{Type: model.CubyAppGrantSubjectUser}}},
	}, apps)

	// The switch is stored, so a restart and the other nodes see it too.
	var stored model.Option
	require.NoError(t, model.DB.Where(&model.Option{Key: operation_setting.CubyAppSettingEnabledOptionKey}).First(&stored).Error)
	assert.Equal(t, "true", stored.Value)

	require.NoError(t, SetCubyAppManagementEnabled(false))
	_, err = ListAccountCubyApps(user.Id)
	require.ErrorIs(t, err, ErrCubyAppsNotManaged)
}

func TestCreateCubyAppAcceptsOnlyIdsCubyAccepts(t *testing.T) {
	setupCubyAppTestDB(t)
	admin := createServiceTestUser(t, "cuby-admin", common.RoleAdminUser)

	for _, appId := range []string{"", "a", "-ab", "AI-Company", "ai_company", "ai company", strings.Repeat("a", 41)} {
		_, err := CreateCubyApp(admin.Id, CreateCubyAppRequest{AppId: appId})
		assert.Error(t, err, "app id %q must be refused", appId)
	}

	app, err := CreateCubyApp(admin.Id, CreateCubyAppRequest{AppId: "  financebuddy  "})
	require.NoError(t, err)
	assert.Equal(t, "financebuddy", app.AppId)
	assert.Equal(t, "financebuddy", app.Name, "an app registered without a name is shown by its id")
	assert.Equal(t, model.CubyAppStatusEnabled, app.Status)
	assert.Equal(t, admin.Id, app.CreatedBy)

	_, err = CreateCubyApp(admin.Id, CreateCubyAppRequest{AppId: strings.Repeat("a", 40)})
	require.NoError(t, err)

	_, err = CreateCubyApp(admin.Id, CreateCubyAppRequest{AppId: "financebuddy", Name: "Again"})
	require.ErrorIs(t, err, ErrCubyAppIdTaken)
}

func TestCreateCubyAppGrantNeedsARealHolderAndRefusesRepeats(t *testing.T) {
	setupCubyAppTestDB(t)
	admin := createServiceTestUser(t, "cuby-admin", common.RoleAdminUser)
	user := createServiceTestUser(t, "cuby-user", common.RoleCommonUser)
	organization := model.Organization{Name: "Harbour Ltd", Slug: "harbour", CreatedBy: admin.Id}
	dissolved := model.Organization{Name: "Gone Ltd", Slug: "gone", CreatedBy: admin.Id, Status: model.OrganizationStatusDissolved}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&dissolved).Error)
	app, err := CreateCubyApp(admin.Id, CreateCubyAppRequest{AppId: "ai-company"})
	require.NoError(t, err)

	_, err = CreateCubyAppGrant(admin.Id, app.Id+100, model.CubyAppGrantSubjectUser, user.Id)
	require.ErrorIs(t, err, ErrCubyAppNotFound)
	_, err = CreateCubyAppGrant(admin.Id, app.Id, model.CubyAppGrantSubjectUser, user.Id+100)
	require.ErrorIs(t, err, ErrCubyAppHolderNotFound)
	_, err = CreateCubyAppGrant(admin.Id, app.Id, model.CubyAppGrantSubjectOrganization, dissolved.Id)
	require.ErrorIs(t, err, ErrCubyAppHolderNotFound)
	_, err = CreateCubyAppGrant(admin.Id, app.Id, "plan", 1)
	require.Error(t, err)

	_, err = CreateCubyAppGrant(admin.Id, app.Id, model.CubyAppGrantSubjectUser, user.Id)
	require.NoError(t, err)
	organizationGrant, err := CreateCubyAppGrant(admin.Id, app.Id, model.CubyAppGrantSubjectOrganization, organization.Id)
	require.NoError(t, err)
	_, err = CreateCubyAppGrant(admin.Id, app.Id, model.CubyAppGrantSubjectOrganization, organization.Id)
	require.ErrorIs(t, err, ErrCubyAppGrantExists)

	views, err := ListCubyAppsForManagement()
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.EqualValues(t, 1, views[0].UserGrantCount)
	assert.EqualValues(t, 1, views[0].OrganizationGrantCount)

	other, err := CreateCubyApp(admin.Id, CreateCubyAppRequest{AppId: "financebuddy"})
	require.NoError(t, err)
	require.ErrorIs(t, DeleteCubyAppGrant(other.Id, organizationGrant.Id), ErrCubyAppGrantNotFound,
		"a grant can only be taken away through the app it belongs to")
	require.NoError(t, DeleteCubyAppGrant(app.Id, organizationGrant.Id))
	require.ErrorIs(t, DeleteCubyAppGrant(app.Id, organizationGrant.Id), ErrCubyAppGrantNotFound)
}

func TestDisablingACubyAppTakesItFromEveryoneAndKeepsItsGrants(t *testing.T) {
	setupCubyAppTestDB(t)
	admin := createServiceTestUser(t, "cuby-admin", common.RoleAdminUser)
	user := createServiceTestUser(t, "cuby-user", common.RoleCommonUser)
	app, err := CreateCubyApp(admin.Id, CreateCubyAppRequest{AppId: "ai-company"})
	require.NoError(t, err)
	_, err = CreateCubyAppGrant(admin.Id, app.Id, model.CubyAppGrantSubjectUser, user.Id)
	require.NoError(t, err)
	require.NoError(t, SetCubyAppManagementEnabled(true))

	disabled := model.CubyAppStatusDisabled
	updated, err := UpdateCubyApp(app.Id, UpdateCubyAppRequest{Status: &disabled})
	require.NoError(t, err)
	assert.Equal(t, model.CubyAppStatusDisabled, updated.Status)
	apps, err := ListAccountCubyApps(user.Id)
	require.NoError(t, err)
	assert.Empty(t, apps)

	enabled := model.CubyAppStatusEnabled
	_, err = UpdateCubyApp(app.Id, UpdateCubyAppRequest{Status: &enabled})
	require.NoError(t, err)
	apps, err = ListAccountCubyApps(user.Id)
	require.NoError(t, err)
	require.Len(t, apps, 1, "switching the app back on must not need the grants to be made again")

	unknown := "paused"
	_, err = UpdateCubyApp(app.Id, UpdateCubyAppRequest{Status: &unknown})
	require.Error(t, err)

	require.NoError(t, DeleteCubyApp(app.Id))
	apps, err = ListAccountCubyApps(user.Id)
	require.NoError(t, err)
	assert.Empty(t, apps)
	require.ErrorIs(t, DeleteCubyApp(app.Id), ErrCubyAppNotFound)
}
