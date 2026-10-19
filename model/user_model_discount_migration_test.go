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
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newUserModelDiscountMigrationTable reserves a uniquely named scratch table for
// one migration run and drops it afterwards. Configured MySQL and PostgreSQL
// instances are shared, so the tests never touch the real users table there.
func newUserModelDiscountMigrationTable(t *testing.T, db *gorm.DB) string {
	t.Helper()
	tableName := fmt.Sprintf("user_discount_migration_%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = db.Migrator().DropTable(tableName) })
	return tableName
}

// userModelDiscountColumnDDL returns the recorded statements that add, alter or
// drop the model_discounts column. SQLite rebuilds a table to change an unrelated
// column by re-creating it from its current schema, which lists model_discounts
// among the copied columns; such a rebuild does not re-run the migration, so only
// ALTER TABLE statements count here.
func userModelDiscountColumnDDL(statements []string) []string {
	var matches []string
	for _, statement := range statements {
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "ALTER TABLE") {
			continue
		}
		if strings.Contains(strings.ToLower(statement), "model_discounts") {
			matches = append(matches, statement)
		}
	}
	return matches
}

// testUserModelDiscountsMigration runs the migration contract on one dialect: a
// fresh database creates users with the discount column, the upgrade adds the
// column to the previous release's table without losing data, and a restart
// leaves the column alone.
//
// Every operation starts from a fresh db.Table handle: chaining clauses onto a
// reused handle also feeds them into the migration's introspection queries.
func testUserModelDiscountsMigration(t *testing.T, db *gorm.DB, recorder *migrationSQLRecorder) {
	t.Helper()

	t.Run("fresh", func(t *testing.T) {
		tableName := newUserModelDiscountMigrationTable(t, db)
		require.False(t, db.Table(tableName).Migrator().HasColumn(tableName, "model_discounts"))

		recorder.reset()
		require.NoError(t, db.Table(tableName).AutoMigrate(&User{}))
		require.True(t, db.Table(tableName).Migrator().HasColumn(tableName, "model_discounts"))

		recorder.reset()
		require.NoError(t, db.Table(tableName).AutoMigrate(&User{}))
		assert.Empty(t, userModelDiscountColumnDDL(recorder.snapshot()),
			"a second startup must not repeat the discount column DDL")
		require.True(t, db.Table(tableName).Migrator().HasColumn(tableName, "model_discounts"))
	})

	t.Run("upgrade_from_previous_release", func(t *testing.T) {
		tableName := newUserModelDiscountMigrationTable(t, db)
		require.NoError(t, db.Table(tableName).AutoMigrate(&User{}))
		// Turn the scratch table into the users table of the release that predates
		// model_discounts, so the upgrade runs against a representative database of
		// the previous version.
		require.NoError(t, db.Table(tableName).Migrator().DropColumn(&User{}, "ModelDiscounts"))
		require.False(t, db.Table(tableName).Migrator().HasColumn(tableName, "model_discounts"))

		// A row of the previous release: no model_discounts column in the INSERT.
		legacy := User{
			Username:    "legacy-discount",
			Password:    "password",
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Group:       "default",
			AffCode:     "aff-legacy-discount",
			AuthVersion: 1,
		}
		require.NoError(t, db.Table(tableName).Omit("ModelDiscounts").Create(&legacy).Error)
		require.NotZero(t, legacy.Id)

		recorder.reset()
		require.NoError(t, db.Table(tableName).AutoMigrate(&User{}))
		assert.NotEmpty(t, userModelDiscountColumnDDL(recorder.snapshot()),
			"the upgrade must add the discount column")
		require.True(t, db.Table(tableName).Migrator().HasColumn(tableName, "model_discounts"))

		// The row that predates the column is preserved, and its NULL discount
		// column reads as "no discounts" rather than failing the scan.
		var upgraded User
		require.NoError(t, db.Table(tableName).Where("id = ?", legacy.Id).First(&upgraded).Error)
		assert.Equal(t, legacy.Username, upgraded.Username)
		assert.Equal(t, legacy.Password, upgraded.Password)
		assert.Equal(t, legacy.Group, upgraded.Group)
		assert.Equal(t, legacy.AffCode, upgraded.AffCode)
		assert.EqualValues(t, legacy.AuthVersion, upgraded.AuthVersion)
		assert.Empty(t, upgraded.ModelDiscounts)
		assert.Empty(t, upgraded.ToBaseUser().GetModelDiscounts())

		// The upgraded table accepts the discount column through the model.
		raw := `{"m":0.8}`
		require.NoError(t, db.Table(tableName).Model(&User{}).Where("id = ?", legacy.Id).
			Update("model_discounts", raw).Error)
		var stored User
		require.NoError(t, db.Table(tableName).Select("model_discounts").Where("id = ?", legacy.Id).First(&stored).Error)
		assert.Equal(t, raw, stored.ModelDiscounts)
		assert.Equal(t, map[string]float64{"m": 0.8}, ParseUserModelDiscounts(stored.ModelDiscounts))

		recorder.reset()
		require.NoError(t, db.Table(tableName).AutoMigrate(&User{}))
		assert.Empty(t, userModelDiscountColumnDDL(recorder.snapshot()),
			"a second startup must not repeat the discount column DDL")
		require.True(t, db.Table(tableName).Migrator().HasColumn(tableName, "model_discounts"))
		var preserved User
		require.NoError(t, db.Table(tableName).Select("model_discounts").Where("id = ?", legacy.Id).First(&preserved).Error)
		assert.Equal(t, raw, preserved.ModelDiscounts)

		// The uniqueness guarantee of the previous release survives the migration.
		duplicate := User{
			Username:    legacy.Username,
			Password:    "password",
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Group:       "default",
			AffCode:     "aff-legacy-discount-duplicate",
			AuthVersion: 1,
		}
		require.Error(t, db.Table(tableName).Omit("ModelDiscounts").Create(&duplicate).Error)
	})
}

func TestUserModelDiscountMigrationSQLite(t *testing.T) {
	recorder := &migrationSQLRecorder{}
	dsn := fmt.Sprintf("file:user_discount_migration_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: recorder})
	require.NoError(t, err)
	testUserModelDiscountsMigration(t, db, recorder)

	// The same column through the application's own handles: the global DB, the
	// real users table, and the discount read path that serves relay requests.
	t.Run("application_read_write", func(t *testing.T) {
		db := setupUserModelDiscountTestDB(t)

		// Two further startups over the already migrated global users table, the
		// way a restart re-runs the same migration.
		for range 2 {
			require.NoError(t, db.AutoMigrate(&User{}))
		}
		require.True(t, db.Migrator().HasColumn("users", "model_discounts"))

		// A row written before the column existed: INSERT without model_discounts.
		require.NoError(t, db.Exec(
			"INSERT INTO users (id, username, password, role, status, aff_code, auth_version) VALUES (?, ?, ?, ?, ?, ?, ?)",
			9801, "discount-legacy-row", "password", common.RoleCommonUser, common.UserStatusEnabled, "aff-discount-legacy-row", 1,
		).Error)

		discounts, err := GetUserModelDiscounts(9801, true)
		require.NoError(t, err)
		assert.Empty(t, discounts, "a row from before the column existed has no discounts")

		require.NoError(t, DB.Model(&User{}).Where("id = ?", 9801).Update("model_discounts", `{"m":0.8}`).Error)
		discounts, err = GetUserModelDiscounts(9801, true)
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"m": 0.8}, discounts)
	})
}

func TestUserModelDiscountMigrationMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}

	recorder := &migrationSQLRecorder{}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: recorder})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	testUserModelDiscountsMigration(t, db, recorder)
}

func TestUserModelDiscountMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not configured")
	}

	recorder := &migrationSQLRecorder{}
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: recorder})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	testUserModelDiscountsMigration(t, db, recorder)
}
