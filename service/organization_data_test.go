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
	"github.com/stretchr/testify/require"
)

func insertOrganizationQuotaData(t *testing.T, organizationId int, responsibleUserId int, modelName string, createdAt int64, quota int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID:             responsibleUserId,
		Username:           modelName + "-user",
		ModelName:          modelName,
		CreatedAt:          createdAt,
		TokenUsed:          quota * 2,
		Count:              1,
		Quota:              quota,
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            organizationId,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organizationId,
		OrganizationId:     organizationId,
		ResponsibleUserId:  responsibleUserId,
	}).Error)
}

func TestOrganizationQuotaDataAdminCanReadOrganizationBillingData(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	insertOrganizationQuotaData(t, organization.Id, admin.Id, "admin-model", 1000, 10)
	insertOrganizationQuotaData(t, organization.Id, member.Id, "member-model", 1001, 20)
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: admin.Id, Username: admin.Username, ModelName: "personal-model", CreatedAt: 1000, Count: 1, Quota: 99, ScopeType: model.AccountContextTypePersonal, ScopeId: admin.Id, BillingAccountType: model.AccountContextTypePersonal, BillingAccountId: admin.Id}).Error)

	data, err := GetOrganizationQuotaData(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationQuotaDataRequest{StartTimestamp: 900, EndTimestamp: 1100})

	require.NoError(t, err)
	require.Len(t, data, 2)
	modelNames := []string{data[0].ModelName, data[1].ModelName}
	require.Contains(t, modelNames, "admin-model")
	require.Contains(t, modelNames, "member-model")
	require.NotContains(t, modelNames, "personal-model")
}

func TestOrganizationQuotaDataMemberOnlyReadsOwnResponsibleData(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	insertOrganizationQuotaData(t, organization.Id, admin.Id, "admin-model", 1000, 10)
	insertOrganizationQuotaData(t, organization.Id, member.Id, "member-model", 1001, 20)

	data, err := GetOrganizationQuotaData(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationQuotaDataRequest{StartTimestamp: 900, EndTimestamp: 1100})

	require.NoError(t, err)
	require.Len(t, data, 1)
	require.Equal(t, "member-model", data[0].ModelName)
	require.Equal(t, member.Id, data[0].ResponsibleUserId)
}

func TestOrganizationQuotaDataRejectsNonMember(t *testing.T) {
	_, _, organization := createOrganizationTokenTestOrg(t)
	nonMember := createServiceTestUser(t, "org-data-non-member", common.RoleCommonUser)

	_, err := GetOrganizationQuotaData(nonMember.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationQuotaDataRequest{StartTimestamp: 900, EndTimestamp: 1100})

	require.Error(t, err)
}

func TestOrganizationQuotaDataExcludesOtherOrganizationData(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	otherOrganization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Other Data Org"})
	require.NoError(t, err)
	insertOrganizationQuotaData(t, organization.Id, member.Id, "target-org-model", 1000, 10)
	insertOrganizationQuotaData(t, otherOrganization.Id, member.Id, "other-org-model", 1001, 20)

	data, err := GetOrganizationQuotaData(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationQuotaDataRequest{StartTimestamp: 900, EndTimestamp: 1100})

	require.NoError(t, err)
	require.Len(t, data, 1)
	require.Equal(t, "target-org-model", data[0].ModelName)
}

func TestOrganizationQuotaDataTimeRangeFilters(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	insertOrganizationQuotaData(t, organization.Id, member.Id, "before-range", 900, 10)
	insertOrganizationQuotaData(t, organization.Id, member.Id, "inside-range", 1000, 20)
	insertOrganizationQuotaData(t, organization.Id, member.Id, "after-range", 1100, 30)

	data, err := GetOrganizationQuotaData(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationQuotaDataRequest{StartTimestamp: 950, EndTimestamp: 1050})

	require.NoError(t, err)
	require.Len(t, data, 1)
	require.Equal(t, "inside-range", data[0].ModelName)
}
