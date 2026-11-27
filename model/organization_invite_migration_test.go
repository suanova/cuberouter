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

// legacyOrganizationInviteWithoutMemberRestrictions is the invitation table as
// it existed before member restrictions: same table name, none of the two new
// columns. AutoMigrating the current model over it is the upgrade path a real
// deployment takes.
type legacyOrganizationInviteWithoutMemberRestrictions struct {
	Id               int
	OrganizationId   int
	Type             string
	TargetEmail      string
	Role             string
	TokenHash        string
	Status           string
	InviterUserId    int
	CreatedAt        int64
	UpdatedAt        int64
	ExpiredAt        int64
	DeliveryStatus   string
	DeliveryAttempts int
}

func (legacyOrganizationInviteWithoutMemberRestrictions) TableName() string {
	return "organization_invitations"
}

func TestOrganizationInviteMemberRestrictionsMigrationConfiguredDatabases(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openOrganizationNameMigrationConfiguredDB(t, dialect)

			require.NoError(t, db.Migrator().DropTable(&legacyOrganizationInviteWithoutMemberRestrictions{}))
			require.NoError(t, db.AutoMigrate(&legacyOrganizationInviteWithoutMemberRestrictions{}))
			legacy := legacyOrganizationInviteWithoutMemberRestrictions{
				OrganizationId:   1,
				Type:             OrganizationInviteTypeEmail,
				TargetEmail:      "legacy-restrictions@example.com",
				Role:             OrganizationRoleMember,
				TokenHash:        "legacy-invite-restrictions-token-hash",
				Status:           OrganizationInviteStatusPending,
				InviterUserId:    2,
				CreatedAt:        1000,
				UpdatedAt:        1000,
				ExpiredAt:        2000,
				DeliveryStatus:   OrganizationInviteDeliverySent,
				DeliveryAttempts: 1,
			}
			require.NoError(t, db.Create(&legacy).Error)

			require.NoError(t, db.AutoMigrate(&OrganizationInvite{}))
			require.NoError(t, db.AutoMigrate(&OrganizationInvite{}))
			require.True(t, db.Migrator().HasColumn(&OrganizationInvite{}, "model_access"))
			require.True(t, db.Migrator().HasColumn(&OrganizationInvite{}, "quota_limit"))

			var stored OrganizationInvite
			require.NoError(t, db.First(&stored, legacy.Id).Error)
			require.Nil(t, stored.ModelAccess)
			require.Nil(t, stored.QuotaLimit)
			require.Equal(t, legacy.TargetEmail, stored.TargetEmail)
		})
	}
}
