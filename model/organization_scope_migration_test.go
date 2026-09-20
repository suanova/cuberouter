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
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// legacyScope* 是组织作用域迁移之前的行结构:只有个人字段,没有任何 scope/billing 列。
type legacyScopeToken struct {
	Id     int
	UserId int
	Key    string
	Name   string
}

func (legacyScopeToken) TableName() string { return "tokens" }

type legacyScopeTask struct {
	Id     int
	UserId int
	TaskID string
}

func (legacyScopeTask) TableName() string { return "tasks" }

type legacyScopeMidjourney struct {
	Id     int
	UserId int
	MjId   string
}

func (legacyScopeMidjourney) TableName() string { return "midjourneys" }

type legacyScopeQuotaData struct {
	Id        int
	UserID    int
	Username  string
	ModelName string
	CreatedAt int64
	Count     int
	Quota     int
}

func (legacyScopeQuotaData) TableName() string { return "quota_data" }

type legacyScopeLog struct {
	Id     int
	UserId int
}

func (legacyScopeLog) TableName() string { return "logs" }

func organizationScopeMigrationLegacyModels() []any {
	return []any{
		&legacyScopeToken{},
		&legacyScopeTask{},
		&legacyScopeMidjourney{},
		&legacyScopeQuotaData{},
		&legacyScopeLog{},
	}
}

func organizationScopeMigrationTargetModels() []any {
	return []any{&Token{}, &Task{}, &Midjourney{}, &QuotaData{}, &Log{}}
}

func seedOrganizationScopeLegacyRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&legacyScopeToken{UserId: 11, Key: "legacy-scope-token", Name: "legacy"}).Error)
	require.NoError(t, db.Create(&legacyScopeTask{UserId: 12, TaskID: "legacy-task"}).Error)
	require.NoError(t, db.Create(&legacyScopeMidjourney{UserId: 13, MjId: "legacy-mj"}).Error)
	require.NoError(t, db.Create(&legacyScopeQuotaData{
		UserID: 14, Username: "legacy-user", ModelName: "legacy-model",
		CreatedAt: 1000, Count: 1, Quota: 20,
	}).Error)
	require.NoError(t, db.Create(&legacyScopeLog{UserId: 15}).Error)
}

// requireOrganizationScopeBackfill 断言历史行整体归一到个人作用域。
func requireOrganizationScopeBackfill(t *testing.T, db *gorm.DB) {
	t.Helper()

	var token Token
	require.NoError(t, db.First(&token).Error)
	assert.Equal(t, TokenScopePersonal, token.ScopeType)
	assert.Equal(t, token.UserId, token.ScopeId)
	assert.Equal(t, token.UserId, token.CreatorUserId)
	assert.Equal(t, token.UserId, token.ResponsibleUserId)
	assert.Equal(t, TokenVisibilityPrivate, token.Visibility)
	assert.Zero(t, token.OrganizationId)

	var task Task
	require.NoError(t, db.First(&task).Error)
	assert.Equal(t, AccountContextTypePersonal, task.ScopeType)
	assert.Equal(t, task.UserId, task.ScopeId)
	assert.Equal(t, AccountContextTypePersonal, task.BillingAccountType)
	assert.Equal(t, task.UserId, task.BillingAccountId)
	assert.Equal(t, task.UserId, task.ActorUserId)
	assert.Equal(t, task.UserId, task.CreatorUserId)
	assert.Equal(t, task.UserId, task.ResponsibleUserId)

	var midjourney Midjourney
	require.NoError(t, db.First(&midjourney).Error)
	assert.Equal(t, AccountContextTypePersonal, midjourney.ScopeType)
	assert.Equal(t, midjourney.UserId, midjourney.ScopeId)
	assert.Equal(t, AccountContextTypePersonal, midjourney.BillingAccountType)
	assert.Equal(t, midjourney.UserId, midjourney.BillingAccountId)
	assert.Equal(t, midjourney.UserId, midjourney.ActorUserId)
	assert.Equal(t, midjourney.UserId, midjourney.CreatorUserId)
	assert.Equal(t, midjourney.UserId, midjourney.ResponsibleUserId)

	var quotaData QuotaData
	require.NoError(t, db.First(&quotaData).Error)
	assert.Equal(t, AccountContextTypePersonal, quotaData.ScopeType)
	assert.Equal(t, quotaData.UserID, quotaData.ScopeId)
	assert.Equal(t, AccountContextTypePersonal, quotaData.BillingAccountType)
	assert.Equal(t, quotaData.UserID, quotaData.BillingAccountId)
	assert.Equal(t, quotaData.UserID, quotaData.ResponsibleUserId)

	var log Log
	require.NoError(t, db.First(&log).Error)
	assert.Equal(t, AccountContextTypePersonal, log.ScopeType)
	assert.Equal(t, log.UserId, log.ScopeId)
	assert.Equal(t, AccountContextTypePersonal, log.BillingAccountType)
	assert.Equal(t, log.UserId, log.BillingAccountId)
	assert.Equal(t, log.UserId, log.ActorUserId)
	assert.Equal(t, log.UserId, log.CreatorUserId)
	assert.Equal(t, log.UserId, log.ResponsibleUserId)
}

// testOrganizationScopeMigrationLegacyUpgrade 覆盖升级路径:旧库只有个人字段的表在迁移后
// 必须整表归一到个人作用域;重复执行不得重放 schema DDL,且要能修复被写坏的作用域列。
func testOrganizationScopeMigrationLegacyUpgrade(t *testing.T, db *gorm.DB, recorder *migrationSQLRecorder) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(organizationScopeMigrationLegacyModels()...))
	seedOrganizationScopeLegacyRows(t, db)

	require.NoError(t, prepareOrganizationScopeMigration(db))
	require.NoError(t, prepareLogBillingEventKeyMigration(db))
	require.NoError(t, db.AutoMigrate(organizationScopeMigrationTargetModels()...))
	requireOrganizationScopeBackfill(t, db)

	// 模拟历史脏数据:作用域列被写成组织值,恢复执行必须把它修回个人作用域。
	//
	// 这里用结构体条件而不是 Where("key = ?"):key 是 MySQL 保留字,而这个用例要在
	// 多方言的临时连接上跑,不能借 commonKeyCol —— 它描述的是进程的主库类型,和
	// 当前连接不一定是同一个方言。结构体条件由 GORM 按当前连接引用,才是安全的。
	require.NoError(t, db.Model(&Token{}).Where(&Token{Key: "legacy-scope-token"}).Updates(map[string]any{
		"creator_user_id": 0,
		"organization_id": 99,
	}).Error)

	recorder.reset()
	require.NoError(t, prepareOrganizationScopeMigration(db), "migration must be safe to resume")
	require.NoError(t, prepareLogBillingEventKeyMigration(db), "log migration must be safe to resume")
	assert.Empty(t, recorder.schemaMutations(), "a resumed migration must not repeat schema DDL")

	var token Token
	require.NoError(t, db.First(&token).Error)
	assert.Zero(t, token.OrganizationId)
	assert.Equal(t, token.UserId, token.CreatorUserId)
}

func TestPrepareOrganizationScopeMigrationLegacyUpgradeSQLite(t *testing.T) {
	recorder := &migrationSQLRecorder{}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: recorder})
	require.NoError(t, err)
	testOrganizationScopeMigrationLegacyUpgrade(t, db, recorder)
}

// statementTouchesTable 判断一条 DDL 是否作用在给定表上：重建表会先建 <table>__temp
// 再 DROP/RENAME 回原表，因此只要命中其中任意一条，就说明这张表被重建了。
func statementTouchesTable(statement string, table string) bool {
	normalized := strings.ToLower(statement)
	return strings.Contains(normalized, "`"+table+"`") ||
		strings.Contains(normalized, "\""+table+"\"") ||
		strings.Contains(normalized, "table "+table)
}

// TestPrepareOrganizationScopeMigrationFreshDatabase 覆盖全新库路径：迁移先于建模执行时
// 必须跳过尚不存在的表；建模后再次执行时，迁移函数自身与其余四张表都不得产生任何 schema
// 变更。tokens 表在 SQLite 上每次 AutoMigrate 都会被重建，那是 Token.Key 的 uniqueIndex
// 标签导致的既有行为（main 分支同样如此），与本次移植无关，因此单独排除。
func TestPrepareOrganizationScopeMigrationFreshDatabase(t *testing.T) {
	recorder := &migrationSQLRecorder{}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: recorder})
	require.NoError(t, err)

	require.NoError(t, prepareOrganizationScopeMigration(db))
	require.NoError(t, prepareLogBillingEventKeyMigration(db))
	require.NoError(t, db.AutoMigrate(organizationScopeMigrationTargetModels()...))

	recorder.reset()
	require.NoError(t, prepareOrganizationScopeMigration(db))
	require.NoError(t, prepareLogBillingEventKeyMigration(db))
	assert.Empty(t, recorder.schemaMutations(), "a resumed migration must not repeat schema DDL")

	require.NoError(t, db.AutoMigrate(organizationScopeMigrationTargetModels()...))
	for _, statement := range recorder.schemaMutations() {
		for _, table := range []string{"logs", "tasks", "midjourneys", "quota_data"} {
			assert.False(t, statementTouchesTable(statement, table),
				"second AutoMigrate must not rebuild %s: %s", table, statement)
		}
	}
	require.NoError(t, ensureOrganizationBillingLogIndexes(db))
}

// TestEnsureLogBillingEventKeyIndex 保护 logs.billing_event_key 的"同一笔账只落一条日志"
// 契约：唯一索引必须被幂等地建出来，NULL 可以重复（历史行没有该键），非 NULL 重复必须被拒。
func TestEnsureLogBillingEventKeyIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	require.NoError(t, prepareLogBillingEventKeyMigration(db))

	for range 2 {
		require.NoError(t, ensureOrganizationBillingLogIndexes(db))
		assert.True(t, db.Migrator().HasIndex("logs", logBillingEventKeyIndex))
	}

	keys := []*string{nil, nil, stringPtr("billing-event-1")}
	for _, key := range keys {
		require.NoError(t, db.Create(&Log{UserId: 1, Type: 1, BillingEventKey: key}).Error,
			"NULL billing event keys must not collide")
	}

	duplicate := db.Create(&Log{UserId: 1, Type: 1, BillingEventKey: stringPtr("billing-event-1")})
	require.Error(t, duplicate.Error, "a duplicate billing event key must be rejected")

	var total int64
	require.NoError(t, db.Model(&Log{}).Count(&total).Error)
	assert.EqualValues(t, 3, total)
}

func stringPtr(value string) *string { return &value }

// randomSuffix 给一次性 schema / 数据库命名,避免并发或重复运行时互相踩踏。
func randomSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// mysqlScratchDatabase 在一个一次性数据库里返回连接。组织作用域迁移写死了 tokens/tasks/
// midjourneys/quota_data/logs 这些表名,无法像其它迁移测试那样改用随机表名,因此必须换库,
// 不能破坏 TEST_MYSQL_DSN 指向的库。
func mysqlScratchDatabase(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	admin, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, dbErr := admin.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	config, err := mysqlDriver.ParseDSN(dsn)
	require.NoError(t, err)
	scratch := "org_scope_" + randomSuffix()
	require.NoError(t, admin.Exec("CREATE DATABASE ?", clause.Table{Name: scratch}).Error)
	t.Cleanup(func() { _ = admin.Exec("DROP DATABASE ?", clause.Table{Name: scratch}).Error })

	config.DBName = scratch
	db, err := gorm.Open(mysql.Open(config.FormatDSN()), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// postgresScratchSchema 在一次性 schema 内返回连接。SET LOCAL 要求处在事务中,因此连同
// 回滚一起交给 t.Cleanup,测试结束后 schema 与其中的表一起消失。
func postgresScratchSchema(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	scratch := "org_scope_" + randomSuffix()
	require.NoError(t, db.Exec("CREATE SCHEMA ?", clause.Table{Name: scratch}).Error)
	t.Cleanup(func() { _ = db.Exec("DROP SCHEMA ? CASCADE", clause.Table{Name: scratch}).Error })
	require.NoError(t, db.Exec("SET search_path TO ?", clause.Table{Name: scratch}).Error)
	return db
}

func TestPrepareOrganizationScopeMigrationConfiguredDatabases(t *testing.T) {
	tests := []struct {
		name string
		env  string
		open func(*testing.T, string) *gorm.DB
	}{
		{name: "mysql", env: "TEST_MYSQL_DSN", open: mysqlScratchDatabase},
		{name: "postgres", env: "TEST_POSTGRES_DSN", open: postgresScratchSchema},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(test.env))
			if dsn == "" {
				t.Skip(test.env + " is not configured")
			}
			recorder := &migrationSQLRecorder{}
			db := test.open(t, dsn)
			db.Logger = recorder
			testOrganizationScopeMigrationLegacyUpgrade(t, db, recorder)
		})
	}
}
