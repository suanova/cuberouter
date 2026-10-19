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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// memberQuotaColumnDDL returns the recorded statements that add, alter or drop
// the quota_limit and used_quota columns. SQLite rebuilds a table to change an
// unrelated column by re-creating it from its current schema, which lists both
// quota columns among the copied columns; such a rebuild does not re-run the
// migration, so only ALTER TABLE statements count here.
func memberQuotaColumnDDL(statements []string) []string {
	var matches []string
	for _, statement := range statements {
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "ALTER TABLE") {
			continue
		}
		lowered := strings.ToLower(statement)
		if strings.Contains(lowered, "quota_limit") || strings.Contains(lowered, "used_quota") {
			matches = append(matches, statement)
		}
	}
	return matches
}

// requireMemberQuotaColumns asserts the database-level contract of the member
// quota columns: integer storage, NOT NULL, and a default of 0 so a membership
// that predates the columns reads as unlimited with nothing used.
func requireMemberQuotaColumns(t *testing.T, db *gorm.DB) {
	t.Helper()

	columnTypes, err := db.Migrator().ColumnTypes(&OrganizationMember{})
	require.NoError(t, err)
	byName := make(map[string]gorm.ColumnType, len(columnTypes))
	for _, columnType := range columnTypes {
		byName[strings.ToLower(columnType.Name())] = columnType
	}

	for _, name := range []string{"quota_limit", "used_quota"} {
		columnType, ok := byName[name]
		require.True(t, ok, "organization_members must have a %s column", name)
		assert.Contains(t, strings.ToUpper(columnType.DatabaseTypeName()), "INT",
			"%s must be stored as an integer type", name)
		defaultValue, ok := columnType.DefaultValue()
		require.True(t, ok, "%s must declare a database default", name)
		assert.Equal(t, "0", strings.TrimSpace(defaultValue), "%s must default to 0", name)
		nullable, ok := columnType.Nullable()
		require.True(t, ok, "%s must report nullability", name)
		assert.False(t, nullable, "%s must be NOT NULL", name)
	}
}

// memberQuotaNamespace opens a disposable database namespace for one scenario:
// an in-memory SQLite database, a scratch MySQL database or a scratch
// PostgreSQL schema. The migration addresses the real organization_members
// table name, so the shared database a configured DSN names must never be
// touched.
type memberQuotaNamespace func(t *testing.T) *gorm.DB

// testOrganizationMemberQuotaMigration runs the migration contract on one
// dialect: a fresh database creates organization_members with both quota
// columns, the upgrade adds them to the previous release's table with every
// existing row reading 0, and a restart repeats neither the DDL nor a data
// change.
func testOrganizationMemberQuotaMigration(t *testing.T, openNamespace memberQuotaNamespace, recorder *migrationSQLRecorder) {
	t.Helper()

	t.Run("fresh", func(t *testing.T) {
		db := openNamespace(t)
		require.False(t, db.Migrator().HasColumn("organization_members", "quota_limit"))
		require.False(t, db.Migrator().HasColumn("organization_members", "used_quota"))

		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		requireMemberQuotaColumns(t, db)

		// The INSERT omits both columns so the database default has to apply.
		member := OrganizationMember{
			OrganizationId: 1,
			UserId:         2,
			Role:           OrganizationRoleMember,
			Status:         OrganizationMemberStatusActive,
		}
		require.NoError(t, db.Omit("QuotaLimit", "UsedQuota").Create(&member).Error)
		require.NotZero(t, member.Id)

		var stored OrganizationMember
		require.NoError(t, db.Select("quota_limit", "used_quota").Where("id = ?", member.Id).First(&stored).Error)
		assert.Zero(t, stored.QuotaLimit, "a new membership must start with no member quota limit")
		assert.Zero(t, stored.UsedQuota, "a new membership must start with no recorded usage")

		recorder.reset()
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		assert.Empty(t, memberQuotaColumnDDL(recorder.snapshot()),
			"a second startup must not repeat the member quota column DDL")
		requireMemberQuotaColumns(t, db)
	})

	t.Run("upgrade_from_previous_release", func(t *testing.T) {
		db := openNamespace(t)
		// Build the table the release that predates member quotas had: the same
		// columns, including the ones the unique index on (organization_id, user_id)
		// needs, minus quota_limit and used_quota.
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		require.NoError(t, db.Migrator().DropColumn(&OrganizationMember{}, "QuotaLimit"))
		require.NoError(t, db.Migrator().DropColumn(&OrganizationMember{}, "UsedQuota"))
		require.False(t, db.Migrator().HasColumn("organization_members", "quota_limit"))
		require.False(t, db.Migrator().HasColumn("organization_members", "used_quota"))

		// A row of the previous release: the INSERT cannot name either column.
		legacy := OrganizationMember{
			OrganizationId: 1,
			UserId:         2,
			Role:           OrganizationRoleMember,
			Status:         OrganizationMemberStatusActive,
		}
		require.NoError(t, db.Omit("QuotaLimit", "UsedQuota").Create(&legacy).Error)
		require.NotZero(t, legacy.Id)

		recorder.reset()
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		assert.NotEmpty(t, memberQuotaColumnDDL(recorder.snapshot()),
			"the upgrade must add the member quota columns")
		requireMemberQuotaColumns(t, db)

		// The row that predates the columns is preserved, and both quota columns
		// read 0: no limit and no recorded usage.
		var upgraded OrganizationMember
		require.NoError(t, db.Where("id = ?", legacy.Id).First(&upgraded).Error)
		assert.Equal(t, legacy.OrganizationId, upgraded.OrganizationId)
		assert.Equal(t, legacy.UserId, upgraded.UserId)
		assert.Equal(t, legacy.Role, upgraded.Role)
		assert.Equal(t, legacy.Status, upgraded.Status)
		assert.Zero(t, upgraded.QuotaLimit)
		assert.Zero(t, upgraded.UsedQuota)

		// The upgraded table accepts the columns through the model, and the stored
		// counters read back unchanged.
		require.NoError(t, db.Model(&OrganizationMember{}).Where("id = ?", legacy.Id).
			Updates(map[string]any{"quota_limit": 5000, "used_quota": 1200}).Error)
		var counted OrganizationMember
		require.NoError(t, db.Select("quota_limit", "used_quota").Where("id = ?", legacy.Id).First(&counted).Error)
		assert.Equal(t, 5000, counted.QuotaLimit)
		assert.Equal(t, 1200, counted.UsedQuota)

		recorder.reset()
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		assert.Empty(t, memberQuotaColumnDDL(recorder.snapshot()),
			"a second startup must not repeat the member quota column DDL")
		requireMemberQuotaColumns(t, db)
		var preserved OrganizationMember
		require.NoError(t, db.Select("quota_limit", "used_quota").Where("id = ?", legacy.Id).First(&preserved).Error)
		assert.Equal(t, 5000, preserved.QuotaLimit, "a restart must not reset the member quota limit")
		assert.Equal(t, 1200, preserved.UsedQuota, "a restart must not reset the member usage counter")

		// The uniqueness guarantee of the previous release survives the migration.
		duplicate := OrganizationMember{
			OrganizationId: legacy.OrganizationId,
			UserId:         legacy.UserId,
			Role:           OrganizationRoleMember,
			Status:         OrganizationMemberStatusActive,
		}
		require.Error(t, db.Omit("QuotaLimit", "UsedQuota").Create(&duplicate).Error)
	})
}

// organizationMemberQuotaSQLiteNamespace opens a fresh in-memory SQLite database
// per scenario; the database itself is the namespace the real table names live in.
func organizationMemberQuotaSQLiteNamespace(recorder *migrationSQLRecorder) memberQuotaNamespace {
	return func(t *testing.T) *gorm.DB {
		t.Helper()
		dsn := fmt.Sprintf("file:org_member_quota_migration_%d?mode=memory&cache=shared", time.Now().UnixNano())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: recorder})
		require.NoError(t, err)
		t.Cleanup(func() {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		})
		return db
	}
}

func TestOrganizationMemberQuotaMigrationSQLite(t *testing.T) {
	recorder := &migrationSQLRecorder{}
	testOrganizationMemberQuotaMigration(t, organizationMemberQuotaSQLiteNamespace(recorder), recorder)
}

func TestOrganizationMemberQuotaMigrationMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}

	recorder := &migrationSQLRecorder{}
	// The migration addresses the real organization_members table name, so every
	// scenario runs in its own scratch database instead of the shared one the DSN
	// names.
	testOrganizationMemberQuotaMigration(t, func(t *testing.T) *gorm.DB {
		t.Helper()
		db := mysqlScratchDatabase(t, dsn)
		db.Logger = recorder
		return db
	}, recorder)
}

func TestOrganizationMemberQuotaMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not configured")
	}

	recorder := &migrationSQLRecorder{}
	// Same reasoning as MySQL: a disposable schema keeps the shared database's
	// organization_members table (and its idx_org_user index name) out of reach.
	testOrganizationMemberQuotaMigration(t, func(t *testing.T) *gorm.DB {
		t.Helper()
		db := postgresScratchSchema(t, dsn)
		db.Logger = recorder
		return db
	}, recorder)
}
