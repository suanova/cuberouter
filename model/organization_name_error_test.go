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
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestIsOrganizationNameDuplicateErrorUsesDriverConstraintMetadata(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "mysql name index",
			err:  &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'acme' for key 'idx_organizations_name_normalized'"},
			want: true,
		},
		{
			name: "mysql qualified name index",
			err:  &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'acme' for key 'organizations.idx_organizations_name_normalized'"},
			want: true,
		},
		{
			name: "mysql slug index",
			err:  &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'acme' for key 'organizations.uni_organizations_slug'"},
			want: false,
		},
		{
			name: "postgres name constraint",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "idx_organizations_name_normalized"},
			want: true,
		},
		{
			name: "postgres slug constraint",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "uni_organizations_slug"},
			want: false,
		},
		{
			name: "wrapped name constraint",
			err: fmt.Errorf("insert organization: %w", &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "idx_organizations_name_normalized",
			}),
			want: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, IsOrganizationNameDuplicateError(testCase.err))
		})
	}
}

func TestIsOrganizationNameDuplicateErrorClassifiesRealSQLiteConstraint(t *testing.T) {
	setupModelTestDB(t)
	require.NoError(t, DB.Create(&Organization{Name: "Acme", Slug: "acme", CreatedBy: 1}).Error)

	nameErr := DB.Create(&Organization{Name: " acme ", Slug: "acme-two", CreatedBy: 2}).Error
	require.Error(t, nameErr)
	require.True(t, IsOrganizationNameDuplicateError(fmt.Errorf("wrapped: %w", nameErr)))

	slugErr := DB.Create(&Organization{Name: "Distinct", Slug: "acme", CreatedBy: 3}).Error
	require.Error(t, slugErr)
	require.False(t, IsOrganizationNameDuplicateError(slugErr))
}
