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
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// legacyOrganizationWithoutNormalizedName 是组织名唯一性迁移之前的表结构。
type legacyOrganizationWithoutNormalizedName struct {
	Id        int    `gorm:"primaryKey"`
	Name      string `gorm:"type:varchar(128);not null;index"`
	Slug      string `gorm:"type:varchar(64);not null;uniqueIndex"`
	CreatedBy int    `gorm:"index;not null"`
}

func (legacyOrganizationWithoutNormalizedName) TableName() string {
	return "organizations"
}

func openOrganizationNameMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/legacy.db"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestNormalizeOrganizationNameTrimsAndFoldsCase(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "metastone 设计组", NormalizeOrganizationName("  MetaStone 设计组  "))
}

func TestPrepareOrganizationNameUniquenessMigrationBackfillsAndIsResumable(t *testing.T) {
	db := openOrganizationNameMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&legacyOrganizationWithoutNormalizedName{}))
	require.NoError(t, db.Create(&legacyOrganizationWithoutNormalizedName{
		Name: " Design Group ", Slug: "design-group",
	}).Error)

	require.NoError(t, prepareOrganizationNameUniquenessMigration(db))
	require.NoError(t, prepareOrganizationNameUniquenessMigration(db), "migration must be safe to resume")
	require.NoError(t, db.AutoMigrate(&Organization{}))

	var organization Organization
	require.NoError(t, db.First(&organization).Error)
	assert.Equal(t, "design group", organization.NameNormalized)
	assert.True(t, db.Migrator().HasIndex(&Organization{}, organizationNameNormalizedIndex))
	assert.Error(t, db.Create(&Organization{
		Name: "design group", Slug: "design-group-2", CreatedBy: 2,
	}).Error, "the unique index must reject a later duplicate")
}

func TestPrepareOrganizationNameUniquenessMigrationRejectsLegacyConflicts(t *testing.T) {
	db := openOrganizationNameMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&legacyOrganizationWithoutNormalizedName{}))
	require.NoError(t, db.Create(&legacyOrganizationWithoutNormalizedName{Name: "Design Group", Slug: "design-group"}).Error)
	require.NoError(t, db.Create(&legacyOrganizationWithoutNormalizedName{Name: " design group ", Slug: "design-group-2"}).Error)

	err := prepareOrganizationNameUniquenessMigration(db)
	require.ErrorContains(t, err, `duplicate normalized organization name "design group"`)
}

// 冲突未解决时迁移不得算作完成，否则重名问题会被永久跳过。
func TestPrepareOrganizationNameUniquenessMigrationConflictDoesNotComplete(t *testing.T) {
	db := openOrganizationNameMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&legacyOrganizationWithoutNormalizedName{}))
	require.NoError(t, db.Create(&legacyOrganizationWithoutNormalizedName{Name: "Design Group", Slug: "design-group"}).Error)
	require.NoError(t, db.Create(&legacyOrganizationWithoutNormalizedName{Name: " design group ", Slug: "design-group-2"}).Error)

	require.Error(t, prepareOrganizationNameUniquenessMigration(db))
	assert.False(t, db.Migrator().HasIndex(&Organization{}, organizationNameNormalizedIndex),
		"a conflicting legacy database must not get the unique index")
	complete, err := organizationNameUniquenessMigrationComplete(db)
	require.NoError(t, err)
	assert.False(t, complete)
}

// 索引已存在即视为迁移完成：不得再回填，否则会把运维手工修正过的名字覆盖掉。
func TestPrepareOrganizationNameUniquenessMigrationSkipsBackfillAfterIndexExists(t *testing.T) {
	db := openOrganizationNameMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&Organization{}))
	organization := Organization{Name: "Completed Migration", Slug: "completed-migration", CreatedBy: 1}
	require.NoError(t, db.Create(&organization).Error)
	require.NoError(t, db.Table("organizations").Where("id = ?", organization.Id).
		UpdateColumn("name_normalized", "must-not-be-rewritten").Error)

	require.NoError(t, prepareOrganizationNameUniquenessMigration(db))

	var nameNormalized string
	require.NoError(t, db.Table("organizations").Where("id = ?", organization.Id).
		Pluck("name_normalized", &nameNormalized).Error)
	assert.Equal(t, "must-not-be-rewritten", nameNormalized)
}

// MySQL 默认排序规则大小写不敏感，组织名唯一性会被 "Design" / "design" 绕过，
// 因此必须显式改成 utf8mb4_bin。
func TestEnsureOrganizationNameNormalizedComparisonUsesMySQLBinaryCollation(t *testing.T) {
	recorder := &migrationSQLRecorder{}
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
		Logger:               recorder,
	})
	require.NoError(t, err)

	require.NoError(t, ensureOrganizationNameNormalizedComparison(db))
	statements := recorder.snapshot()
	require.Len(t, statements, 1)
	assert.Equal(t,
		"ALTER TABLE `organizations` MODIFY COLUMN `name_normalized` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL",
		statements[0],
	)
}

// 全新库上 prepareOrganizationNameUniquenessMigration 先于 AutoMigrate 执行：此时唯一索引
// 尚不存在，"是否已完成"必须再审一次，否则建索引后的重名校验会被跳过。
func TestFreshDatabaseMigrationsRecheckNameContractAfterCreatingIndex(t *testing.T) {
	testCases := []struct {
		name    string
		migrate func() error
	}{
		{name: "ordinary", migrate: migrateDB},
		{name: "fast", migrate: migrateDBFast},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := &migrationSQLRecorder{}
			db, err := gorm.Open(sqlite.Open(t.TempDir()+"/fresh.db?_pragma=busy_timeout(30000)"), &gorm.Config{Logger: recorder})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)

			oldDB, oldLogDB := DB, LOG_DB
			oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
			initCol()
			t.Cleanup(func() {
				DB, LOG_DB = oldDB, oldLogDB
				common.SetDatabaseTypes(oldMainType, oldLogType)
				initCol()
			})

			require.NoError(t, testCase.migrate())
			statements := recorder.snapshot()
			createdAt := -1
			for i, statement := range statements {
				if strings.Contains(statement, "CREATE UNIQUE INDEX") && strings.Contains(statement, organizationNameNormalizedIndex) {
					createdAt = i
					break
				}
			}
			require.GreaterOrEqual(t, createdAt, 0, "organization name unique index was not created")
			for _, statement := range statements[createdAt+1:] {
				if strings.Contains(statement, organizationNameNormalizedIndex) && strings.Contains(statement, "sqlite_master") {
					return
				}
			}
			t.Fatal("organization name comparison contract was not rechecked after creating the unique index")
		})
	}
}

// openOrganizationNameMigrationConfiguredDB 在一次性命名空间里返回连接，见
// organization_scope_migration_test.go 中同类 helper 的说明。
func openOrganizationNameMigrationConfiguredDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	switch dialect {
	case "mysql":
		return mysqlScratchDatabase(t, requireEnvOrSkip(t, "TEST_MYSQL_DSN"))
	default:
		dsn := requireEnvOrSkip(t, "TEST_POSTGRES_DSN")
		db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })

		scratch := "org_name_" + randomSuffix()
		require.NoError(t, db.Exec("CREATE SCHEMA ?", clause.Table{Name: scratch}).Error)
		t.Cleanup(func() { _ = db.Exec("DROP SCHEMA ? CASCADE", clause.Table{Name: scratch}).Error })
		require.NoError(t, db.Exec("SET search_path TO ?", clause.Table{Name: scratch}).Error)
		return db
	}
}

func requireEnvOrSkip(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Skip(name + " is not configured")
	}
	return value
}

func TestOrganizationNameMigrationConfiguredDatabases(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openOrganizationNameMigrationConfiguredDB(t, dialect)

			t.Run("backfills_trimmed_case_folded_names", func(t *testing.T) {
				require.NoError(t, db.Migrator().DropTable(&legacyOrganizationWithoutNormalizedName{}))
				require.NoError(t, db.AutoMigrate(&legacyOrganizationWithoutNormalizedName{}))
				require.NoError(t, db.Create(&legacyOrganizationWithoutNormalizedName{
					Name: "  Design Group  ", Slug: "legacy-backfill", CreatedBy: 1,
				}).Error)

				require.NoError(t, prepareOrganizationNameUniquenessMigration(db))

				var normalized string
				require.NoError(t, db.Table("organizations").Select("name_normalized").Scan(&normalized).Error)
				assert.Equal(t, "design group", normalized)
			})

			t.Run("unique_index_rejects_later_duplicate", func(t *testing.T) {
				require.NoError(t, db.Migrator().DropTable(&legacyOrganizationWithoutNormalizedName{}))
				require.NoError(t, db.AutoMigrate(&legacyOrganizationWithoutNormalizedName{}))
				require.NoError(t, db.Create(&legacyOrganizationWithoutNormalizedName{
					Name: "Design Group", Slug: "legacy-unique", CreatedBy: 1,
				}).Error)

				require.NoError(t, prepareOrganizationNameUniquenessMigration(db))
				require.NoError(t, db.AutoMigrate(&Organization{}))

				require.Error(t, db.Create(&Organization{
					Name: "  design group  ", Slug: "later-duplicate", CreatedBy: 2,
				}).Error)
			})

			t.Run("successful_migration_is_idempotent", func(t *testing.T) {
				require.NoError(t, db.Migrator().DropTable(&legacyOrganizationWithoutNormalizedName{}))
				require.NoError(t, db.AutoMigrate(&legacyOrganizationWithoutNormalizedName{}))
				require.NoError(t, db.Create(&legacyOrganizationWithoutNormalizedName{
					Name: "  Idempotent Group  ", Slug: "legacy-idempotent", CreatedBy: 1,
				}).Error)

				// 与 migrateDB 同序：迁移先补齐列并回填，AutoMigrate 再按模型标签建唯一索引，
				// 最后重跑一次确认已建索引时走快速路径。
				require.NoError(t, prepareOrganizationNameUniquenessMigration(db))
				require.NoError(t, db.AutoMigrate(&Organization{}))
				require.NoError(t, prepareOrganizationNameUniquenessMigration(db))

				var organization Organization
				require.NoError(t, db.First(&organization).Error)
				assert.Equal(t, "idempotent group", organization.NameNormalized)
				assert.True(t, db.Migrator().HasIndex(&Organization{}, organizationNameNormalizedIndex))
			})
		})
	}
}
