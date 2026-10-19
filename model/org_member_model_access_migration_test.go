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

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// memberModelAccessColumnDDL returns the recorded statements that add, alter or
// drop the model_access column. SQLite rebuilds a table to change an unrelated
// column by re-creating it from its current schema, which lists model_access
// among the copied columns; such a rebuild does not re-run the migration, so
// only ALTER TABLE statements count here.
func memberModelAccessColumnDDL(statements []string) []string {
	var matches []string
	for _, statement := range statements {
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "ALTER TABLE") {
			continue
		}
		if strings.Contains(strings.ToLower(statement), "model_access") {
			matches = append(matches, statement)
		}
	}
	return matches
}

// memberModelAccessNamespace opens a disposable database namespace for one
// scenario: an in-memory SQLite database, a scratch MySQL database or a scratch
// PostgreSQL schema. Both the migration and the member model allow list getter
// address the real organization_members table name, so the shared database a
// configured DSN names must never be touched.
type memberModelAccessNamespace func(t *testing.T) *gorm.DB

// testMemberModelAccessMigration runs the migration contract on one dialect: a
// fresh database creates organization_members with the model_access column, the
// upgrade adds the column to the previous release's table without losing data
// or the uniqueness guarantee, a restart leaves the column alone, and the
// post-migration getter treats rows that predate the column as unrestricted.
func testMemberModelAccessMigration(t *testing.T, databaseType common.DatabaseType, openNamespace memberModelAccessNamespace, recorder *migrationSQLRecorder) {
	t.Helper()

	t.Run("fresh", func(t *testing.T) {
		db := openNamespace(t)
		require.False(t, db.Migrator().HasColumn("organization_members", "model_access"))

		recorder.reset()
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		require.True(t, db.Migrator().HasColumn("organization_members", "model_access"))

		recorder.reset()
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		assert.Empty(t, memberModelAccessColumnDDL(recorder.snapshot()),
			"a second startup must not repeat the member model access column DDL")
		require.True(t, db.Migrator().HasColumn("organization_members", "model_access"))
	})

	t.Run("upgrade_from_previous_release", func(t *testing.T) {
		db := openNamespace(t)
		// Build the table the release that predates model_access had: the same
		// columns, including the ones the unique index on (organization_id, user_id)
		// needs, minus model_access.
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		require.NoError(t, db.Migrator().DropColumn(&OrganizationMember{}, "ModelAccess"))
		require.False(t, db.Migrator().HasColumn("organization_members", "model_access"))

		// A row of the previous release: no model_access column in the INSERT.
		legacy := OrganizationMember{
			OrganizationId: 1,
			UserId:         2,
			Role:           OrganizationRoleMember,
			Status:         OrganizationMemberStatusActive,
		}
		require.NoError(t, db.Omit("ModelAccess").Create(&legacy).Error)
		require.NotZero(t, legacy.Id)

		recorder.reset()
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		assert.NotEmpty(t, memberModelAccessColumnDDL(recorder.snapshot()),
			"the upgrade must add the member model access column")
		require.True(t, db.Migrator().HasColumn("organization_members", "model_access"))

		// The row that predates the column is preserved, and its NULL model_access
		// reads as "no restriction" rather than failing the scan.
		var upgraded OrganizationMember
		require.NoError(t, db.Where("id = ?", legacy.Id).First(&upgraded).Error)
		assert.Equal(t, legacy.OrganizationId, upgraded.OrganizationId)
		assert.Equal(t, legacy.UserId, upgraded.UserId)
		assert.Equal(t, legacy.Role, upgraded.Role)
		assert.Equal(t, legacy.Status, upgraded.Status)
		assert.Empty(t, upgraded.ModelAccess)
		assert.Nil(t, ParseMemberModelAccess(upgraded.ModelAccess))

		// The upgraded table accepts the column through the model, and the stored
		// payload reads back unchanged.
		raw := `["b","a"]`
		require.NoError(t, db.Model(&OrganizationMember{}).Where("id = ?", legacy.Id).
			Update("model_access", raw).Error)
		var stored OrganizationMember
		require.NoError(t, db.Select("model_access").Where("id = ?", legacy.Id).First(&stored).Error)
		assert.Equal(t, raw, stored.ModelAccess)
		assert.Equal(t, []string{"b", "a"}, ParseMemberModelAccess(stored.ModelAccess))

		recorder.reset()
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		assert.Empty(t, memberModelAccessColumnDDL(recorder.snapshot()),
			"a second startup must not repeat the member model access column DDL")
		require.True(t, db.Migrator().HasColumn("organization_members", "model_access"))
		var preserved OrganizationMember
		require.NoError(t, db.Select("model_access").Where("id = ?", legacy.Id).First(&preserved).Error)
		assert.Equal(t, raw, preserved.ModelAccess)

		// The uniqueness guarantee of the previous release survives the migration.
		duplicate := OrganizationMember{
			OrganizationId: legacy.OrganizationId,
			UserId:         legacy.UserId,
			Role:           OrganizationRoleMember,
			Status:         OrganizationMemberStatusActive,
		}
		require.Error(t, db.Omit("ModelAccess").Create(&duplicate).Error)
	})

	t.Run("application_read_write", func(t *testing.T) {
		db := openNamespace(t)
		require.NoError(t, db.AutoMigrate(&OrganizationMember{}, &Organization{}, &User{}))

		// The getter reads the global handles; point them at this namespace so the
		// post-migration semantics are exercised on this dialect's real engine.
		previousDB, previousLogDB := DB, LOG_DB
		previousRedisEnabled := common.RedisEnabled
		previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
		DB, LOG_DB = db, db
		common.RedisEnabled = false
		common.SetDatabaseTypes(databaseType, databaseType)
		t.Cleanup(func() {
			DB, LOG_DB = previousDB, previousLogDB
			common.RedisEnabled = previousRedisEnabled
			common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		})

		// Two further startups over the already migrated table, the way a restart
		// re-runs the same migration.
		for range 2 {
			require.NoError(t, db.AutoMigrate(&OrganizationMember{}))
		}
		require.True(t, db.Migrator().HasColumn("organization_members", "model_access"))

		legacyUser := createMemberModelAccessTestUser(t, db, "member-model-access-legacy")
		organization := Organization{
			Name:      "member-model-access-org",
			Slug:      "member-model-access-org",
			Status:    OrganizationStatusActive,
			CreatedBy: legacyUser.Id,
		}
		require.NoError(t, db.Create(&organization).Error)

		// A membership row written before the column existed: the INSERT leaves
		// model_access NULL. The row itself is present, so the assertions below
		// cannot pass because the membership is missing.
		require.NoError(t, db.Exec(
			"INSERT INTO organization_members (organization_id, user_id, role, status) VALUES (?, ?, ?, ?)",
			organization.Id, legacyUser.Id, OrganizationRoleMember, OrganizationMemberStatusActive,
		).Error)
		var member OrganizationMember
		require.NoError(t, db.Where("user_id = ?", legacyUser.Id).First(&member).Error)
		require.Equal(t, OrganizationMemberStatusActive, member.Status)
		require.Empty(t, member.ModelAccess)

		// Fail-open after the migration: a membership row that predates the column
		// restricts nothing, and neither does a user without any membership row.
		require.Nil(t, GetUserModelAllowList(legacyUser.Id))
		freeUser := createMemberModelAccessTestUser(t, db, "member-model-access-free")
		require.Nil(t, GetUserModelAllowList(freeUser.Id))

		// A restricted membership governed by the active organization returns the
		// restricted set, sorted, whatever order the column holds.
		require.NoError(t, db.Model(&OrganizationMember{}).Where("id = ?", member.Id).
			Update("model_access", `["b","a"]`).Error)
		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(legacyUser.Id))
	})
}

func createMemberModelAccessTestUser(t *testing.T, db *gorm.DB, username string) *User {
	t.Helper()
	user := User{
		Username:    username,
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     "aff-" + username, // uniqueIndex — must be unique per user
	}
	require.NoError(t, db.Create(&user).Error)
	return &user
}

// memberModelAccessSQLiteNamespace opens a fresh in-memory SQLite database per
// scenario; the database itself is the namespace the real table names live in.
func memberModelAccessSQLiteNamespace(recorder *migrationSQLRecorder) memberModelAccessNamespace {
	return func(t *testing.T) *gorm.DB {
		t.Helper()
		dsn := fmt.Sprintf("file:org_member_model_access_migration_%d?mode=memory&cache=shared", time.Now().UnixNano())
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

func TestMemberModelAccessMigrationSQLite(t *testing.T) {
	recorder := &migrationSQLRecorder{}
	testMemberModelAccessMigration(t, common.DatabaseTypeSQLite, memberModelAccessSQLiteNamespace(recorder), recorder)
}

func TestMemberModelAccessMigrationMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}

	recorder := &migrationSQLRecorder{}
	// The migration and the getter address the real organization_members table
	// name, so every scenario runs in its own scratch database instead of the
	// shared one the DSN names.
	testMemberModelAccessMigration(t, common.DatabaseTypeMySQL, func(t *testing.T) *gorm.DB {
		t.Helper()
		db := mysqlScratchDatabase(t, dsn)
		db.Logger = recorder
		return db
	}, recorder)
}

func TestMemberModelAccessMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not configured")
	}

	recorder := &migrationSQLRecorder{}
	// Same reasoning as MySQL: a disposable schema keeps the shared database's
	// organization_members table (and its idx_org_user index name) out of reach.
	testMemberModelAccessMigration(t, common.DatabaseTypePostgreSQL, func(t *testing.T) *gorm.DB {
		t.Helper()
		db := postgresScratchSchema(t, dsn)
		db.Logger = recorder
		return db
	}, recorder)
}
