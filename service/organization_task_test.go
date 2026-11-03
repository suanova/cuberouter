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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHydrateOrganizationTaskNames(t *testing.T) {
	setupServiceTestDB(t)

	user := createServiceTestUser(t, "task-name-user", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("display_name", "Task User").Error)
	token := model.Token{
		UserId: user.Id,
		Key:    "task-name-test-key",
		Name:   "task-name-test-key-name",
		Status: common.TokenStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	task := &model.Task{
		TokenId:           token.Id,
		ResponsibleUserId: user.Id,
		CreatorUserId:     user.Id,
		Quota:             80,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{GroupRatio: 0.8},
		},
	}

	require.NoError(t, HydrateOrganizationTaskNames([]*model.Task{task}))

	assert.Equal(t, "task-name-test-key-name", task.TokenName)
	assert.Equal(t, "Task User", task.ResponsibleName)
	assert.Equal(t, "Task User", task.CreatorName)
	assert.Equal(t, 100, organizationTaskOriginalQuota(task))
}

func TestOrganizationTaskOriginalQuotaFallsBackToChargedQuota(t *testing.T) {
	task := &model.Task{Quota: 80}

	assert.Equal(t, 80, organizationTaskOriginalQuota(task))
}

func TestHydrateOrganizationTaskNamesKeepsStoredSnapshots(t *testing.T) {
	task := &model.Task{
		TokenName:         "stored-key",
		ResponsibleName:   "Stored User",
		CreatorName:       "Stored User",
		TokenId:           12,
		ResponsibleUserId: 7,
		CreatorUserId:     7,
	}

	require.NoError(t, HydrateOrganizationTaskNames([]*model.Task{task}))

	assert.Equal(t, "stored-key", task.TokenName)
	assert.Equal(t, "Stored User", task.ResponsibleName)
	assert.Equal(t, "Stored User", task.CreatorName)
}
