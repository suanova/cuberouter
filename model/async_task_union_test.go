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

	"github.com/stretchr/testify/require"
)

// TestUserTaskUnionCoversResponsibleOrganizationRows 锁定个人任务页的新并集：
// 个人任务 + 责任人是自己的组织任务。调用者与责任人可能不同，按责任快照判断。
func TestUserTaskUnionCoversResponsibleOrganizationRows(t *testing.T) {
	fixture := setupAsyncTaskScopeTestDB(t)
	require.NoError(t, DB.Model(&Task{}).Where("task_id = ?", fixture.organizationTask.TaskID).Update("responsible_user_id", fixture.user.Id).Error)

	other := User{Username: "async-other-user", Password: "password", AffCode: "async-other-user"}
	require.NoError(t, DB.Create(&other).Error)
	otherOrgTask := Task{
		TaskID:             "other-organization-task",
		UserId:             other.Id,
		ScopeType:          AccountContextTypeOrganization,
		ScopeId:            fixture.organization.Id,
		BillingAccountType: AccountContextTypeOrganization,
		BillingAccountId:   fixture.organization.Id,
		OrganizationId:     fixture.organization.Id,
		ResponsibleUserId:  other.Id,
	}
	myResponsibleTask := Task{
		TaskID:             "my-responsible-task",
		UserId:             other.Id,
		ScopeType:          AccountContextTypeOrganization,
		ScopeId:            fixture.organization.Id,
		BillingAccountType: AccountContextTypeOrganization,
		BillingAccountId:   fixture.organization.Id,
		OrganizationId:     fixture.organization.Id,
		ResponsibleUserId:  fixture.user.Id,
	}
	require.NoError(t, DB.Create(&otherOrgTask).Error)
	require.NoError(t, DB.Create(&myResponsibleTask).Error)

	tasks := TaskGetAllUserTask(fixture.user.Id, 0, 50, SyncTaskQueryParams{IncludeOrganization: true})
	ids := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		ids[task.TaskID] = true
	}
	require.True(t, ids[fixture.personalTask.TaskID])
	require.True(t, ids[fixture.organizationTask.TaskID])
	require.True(t, ids[myResponsibleTask.TaskID])
	require.False(t, ids[otherOrgTask.TaskID])
	require.Equal(t, int64(3), TaskCountAllUserTask(fixture.user.Id, SyncTaskQueryParams{IncludeOrganization: true}))

	organizationTasks := TaskGetAllUserTask(fixture.user.Id, 0, 50, SyncTaskQueryParams{IncludeOrganization: true, BillingAccountType: AccountContextTypeOrganization})
	require.Len(t, organizationTasks, 2)
	require.Equal(t, int64(2), TaskCountAllUserTask(fixture.user.Id, SyncTaskQueryParams{IncludeOrganization: true, BillingAccountType: AccountContextTypeOrganization}))

	personalTasks := TaskGetAllUserTask(fixture.user.Id, 0, 50, SyncTaskQueryParams{IncludeOrganization: true, BillingAccountType: AccountContextTypePersonal})
	require.Len(t, personalTasks, 1)
	require.Equal(t, fixture.personalTask.TaskID, personalTasks[0].TaskID)

	organizationOnly := TaskGetAllUserTask(fixture.user.Id, 0, 50, SyncTaskQueryParams{IncludeOrganization: true, OrganizationID: fixture.organization.Id})
	require.Len(t, organizationOnly, 2)

	tasks = TaskGetAllUserTask(fixture.user.Id, 0, 50, SyncTaskQueryParams{})
	require.Len(t, tasks, 1)
	require.Equal(t, fixture.personalTask.TaskID, tasks[0].TaskID)
}
