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
package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type asyncTaskScopeFixture struct {
	user             User
	organization     Organization
	personalTask     Task
	organizationTask Task
	personalMJ       Midjourney
	organizationMJ   Midjourney
}

func setupAsyncTaskScopeTestDB(t *testing.T) asyncTaskScopeFixture {
	t.Helper()

	previousDB := DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/async-task-scope.db?_pragma=busy_timeout(30000)"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, DB.AutoMigrate(&User{}, &Organization{}, &Task{}, &Midjourney{}))

	fixture := asyncTaskScopeFixture{
		user: User{Username: "async-scope-user", Password: "password", AffCode: "async-scope-user"},
	}
	require.NoError(t, DB.Create(&fixture.user).Error)
	fixture.organization = Organization{Name: "Async Scope Org", Slug: "async-scope-org", CreatedBy: fixture.user.Id}
	require.NoError(t, DB.Create(&fixture.organization).Error)

	fixture.personalTask = Task{
		TaskID:             "personal-task",
		UserId:             fixture.user.Id,
		ScopeType:          AccountContextTypePersonal,
		ScopeId:            fixture.user.Id,
		BillingAccountType: AccountContextTypePersonal,
		BillingAccountId:   fixture.user.Id,
	}
	fixture.organizationTask = Task{
		TaskID:             "organization-task",
		UserId:             fixture.user.Id,
		ScopeType:          AccountContextTypeOrganization,
		ScopeId:            fixture.organization.Id,
		BillingAccountType: AccountContextTypeOrganization,
		BillingAccountId:   fixture.organization.Id,
		OrganizationId:     fixture.organization.Id,
	}
	fixture.personalMJ = Midjourney{
		MjId:               "personal-mj",
		UserId:             fixture.user.Id,
		ScopeType:          AccountContextTypePersonal,
		ScopeId:            fixture.user.Id,
		BillingAccountType: AccountContextTypePersonal,
		BillingAccountId:   fixture.user.Id,
	}
	fixture.organizationMJ = Midjourney{
		MjId:               "organization-mj",
		UserId:             fixture.user.Id,
		ScopeType:          AccountContextTypeOrganization,
		ScopeId:            fixture.organization.Id,
		BillingAccountType: AccountContextTypeOrganization,
		BillingAccountId:   fixture.organization.Id,
		OrganizationId:     fixture.organization.Id,
	}
	require.NoError(t, DB.Create(&fixture.personalTask).Error)
	require.NoError(t, DB.Create(&fixture.organizationTask).Error)
	require.NoError(t, DB.Create(&fixture.personalMJ).Error)
	require.NoError(t, DB.Create(&fixture.organizationMJ).Error)

	return fixture
}

func TestAsyncTaskScopeIsolatesTaskLookups(t *testing.T) {
	fixture := setupAsyncTaskScopeTestDB(t)
	personal := AsyncTaskScope{UserId: fixture.user.Id, ScopeType: AccountContextTypePersonal, ScopeId: fixture.user.Id}
	organization := AsyncTaskScope{UserId: fixture.user.Id, ScopeType: AccountContextTypeOrganization, ScopeId: fixture.organization.Id}

	task, exists, err := GetByTaskId(personal, fixture.personalTask.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, fixture.personalTask.TaskID, task.TaskID)

	_, exists, err = GetByTaskId(personal, fixture.organizationTask.TaskID)
	require.NoError(t, err)
	require.False(t, exists)

	_, exists, err = GetByTaskId(organization, fixture.personalTask.TaskID)
	require.NoError(t, err)
	require.False(t, exists)

	task, exists, err = GetByTaskId(organization, fixture.organizationTask.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, fixture.organizationTask.TaskID, task.TaskID)

	personalTasks, err := GetByTaskIds(personal, []any{fixture.personalTask.TaskID, fixture.organizationTask.TaskID})
	require.NoError(t, err)
	require.Len(t, personalTasks, 1)
	require.Equal(t, fixture.personalTask.TaskID, personalTasks[0].TaskID)

	organizationTasks, err := GetByTaskIds(organization, []any{fixture.personalTask.TaskID, fixture.organizationTask.TaskID})
	require.NoError(t, err)
	require.Len(t, organizationTasks, 1)
	require.Equal(t, fixture.organizationTask.TaskID, organizationTasks[0].TaskID)
}

func TestAsyncTaskScopeIsolatesMidjourneyLookups(t *testing.T) {
	fixture := setupAsyncTaskScopeTestDB(t)
	personal := AsyncTaskScope{UserId: fixture.user.Id, ScopeType: AccountContextTypePersonal, ScopeId: fixture.user.Id}
	organization := AsyncTaskScope{UserId: fixture.user.Id, ScopeType: AccountContextTypeOrganization, ScopeId: fixture.organization.Id}

	personalMJ := GetByMJId(personal, fixture.personalMJ.MjId)
	require.NotNil(t, personalMJ)
	require.Equal(t, fixture.personalMJ.MjId, personalMJ.MjId)
	require.Nil(t, GetByMJId(personal, fixture.organizationMJ.MjId))
	require.Nil(t, GetByMJId(organization, fixture.personalMJ.MjId))
	organizationMJ := GetByMJId(organization, fixture.organizationMJ.MjId)
	require.NotNil(t, organizationMJ)
	require.Equal(t, fixture.organizationMJ.MjId, organizationMJ.MjId)

	personalTasks := GetByMJIds(personal, []string{fixture.personalMJ.MjId, fixture.organizationMJ.MjId})
	require.Len(t, personalTasks, 1)
	require.Equal(t, fixture.personalMJ.MjId, personalTasks[0].MjId)

	organizationTasks := GetByMJIds(organization, []string{fixture.personalMJ.MjId, fixture.organizationMJ.MjId})
	require.Len(t, organizationTasks, 1)
	require.Equal(t, fixture.organizationMJ.MjId, organizationTasks[0].MjId)
}

func TestAsyncTaskScopeRejectsInvalidOrIncompleteScope(t *testing.T) {
	fixture := setupAsyncTaskScopeTestDB(t)
	invalidScopes := []AsyncTaskScope{
		{},
		{UserId: fixture.user.Id, ScopeType: "invalid", ScopeId: fixture.user.Id},
		{UserId: fixture.user.Id, ScopeType: AccountContextTypePersonal},
	}

	for _, scope := range invalidScopes {
		_, exists, err := GetByTaskId(scope, fixture.personalTask.TaskID)
		require.NoError(t, err)
		require.False(t, exists)
		require.Empty(t, mustGetTaskBatch(t, scope, fixture.personalTask.TaskID))
		require.Nil(t, GetByMJId(scope, fixture.personalMJ.MjId))
		require.Empty(t, GetByMJIds(scope, []string{fixture.personalMJ.MjId}))
	}

	incompleteOrganizationTask := Task{
		TaskID:             "incomplete-organization-task",
		UserId:             fixture.user.Id,
		ScopeType:          AccountContextTypeOrganization,
		ScopeId:            fixture.organization.Id,
		BillingAccountType: AccountContextTypeOrganization,
		BillingAccountId:   fixture.organization.Id,
		OrganizationId:     fixture.organization.Id + 1,
	}
	incompleteOrganizationMJ := Midjourney{
		MjId:               "incomplete-organization-mj",
		UserId:             fixture.user.Id,
		ScopeType:          AccountContextTypeOrganization,
		ScopeId:            fixture.organization.Id,
		BillingAccountType: AccountContextTypeOrganization,
		BillingAccountId:   fixture.organization.Id,
		OrganizationId:     fixture.organization.Id + 1,
	}
	require.NoError(t, DB.Create(&incompleteOrganizationTask).Error)
	require.NoError(t, DB.Create(&incompleteOrganizationMJ).Error)
	organization := AsyncTaskScope{UserId: fixture.user.Id, ScopeType: AccountContextTypeOrganization, ScopeId: fixture.organization.Id}

	_, exists, err := GetByTaskId(organization, incompleteOrganizationTask.TaskID)
	require.NoError(t, err)
	require.False(t, exists)
	require.Nil(t, GetByMJId(organization, incompleteOrganizationMJ.MjId))
}

func mustGetTaskBatch(t *testing.T, scope AsyncTaskScope, taskID string) []*Task {
	t.Helper()
	tasks, err := GetByTaskIds(scope, []any{taskID})
	require.NoError(t, err)
	return tasks
}
