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

package accountassistant

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/agentkit"

	"github.com/glebarez/sqlite"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TestAccountAssistantReadPathsAcrossDatabases 在真实 SQLite/MySQL/PostgreSQL 上
// 跑一遍助手的全部只读查询。用量汇总用了 FLOOR 分组、LIKE/ESCAPE 过滤和 SUM(CASE)，
// 这些写法在不同方言上的行为必须一致，因此不能只靠 SQLite 单测。
func TestAccountAssistantReadPathsAcrossDatabases(t *testing.T) {
	tests := []struct {
		name string
		env  string
		open func(*testing.T, string) *gorm.DB
	}{
		{name: "sqlite", open: func(t *testing.T, _ string) *gorm.DB {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			return db
		}},
		{name: "mysql", env: "TEST_MYSQL_DSN", open: openAccountAssistantMySQL},
		{name: "postgres", env: "TEST_POSTGRES_DSN", open: openAccountAssistantPostgres},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := ""
			if test.env != "" {
				dsn = strings.TrimSpace(os.Getenv(test.env))
				if dsn == "" {
					t.Skip(test.env + " is not configured")
				}
			}
			db := test.open(t, dsn)
			useAccountAssistantTestDB(t, db)
			assertAccountAssistantReadPaths(t, db)
		})
	}
}

func assertAccountAssistantReadPaths(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedAccountAssistantUsers(t, db)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&[]model.Token{
		{Id: 1, UserId: 1, Key: "sk-alice-secret-key-123456", Name: "alice-key", Status: common.TokenStatusEnabled, ScopeType: "personal", RemainQuota: 100},
		{Id: 2, UserId: 2, Key: "sk-bob-secret-key-654321", Name: "bob-key", Status: common.TokenStatusEnabled, ScopeType: "personal", RemainQuota: 200},
	}).Error)
	require.NoError(t, db.Create(&[]model.Log{
		{Id: 10, UserId: 2, ActorUserId: 2, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, ModelName: "model-b", Quota: 999, CreatedAt: now},
		{Id: 11, UserId: 1, ActorUserId: 1, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, ModelName: "model-a", Quota: 70, PromptTokens: 10, CreatedAt: now},
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID: "task-bob", UserId: 2, Platform: "openai", Status: model.TaskStatusSuccess,
		Action: "generate", SubmitTime: now,
	}).Error)
	require.NoError(t, db.Create(&model.Organization{
		Id: 9, Name: "Acme", NameNormalized: "acme", Slug: "acme",
		Status: model.OrganizationStatusActive, OwnerUserId: 1, CreatedBy: 1,
	}).Error)
	require.NoError(t, db.Create(&[]model.OrganizationMember{
		{OrganizationId: 9, UserId: 1, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: 9, UserId: 2, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
	}).Error)

	alice := agentkit.Principal{UserID: 1}
	bob := agentkit.Principal{UserID: 2}

	account, err := newGetMyAccountTool().Run(context.Background(), alice, `{}`)
	require.NoError(t, err)
	assert.Contains(t, account, `"username":"alice"`)
	assert.NotContains(t, account, "bob")

	keys, err := newListMyKeysTool().Run(context.Background(), alice, `{}`)
	require.NoError(t, err)
	assert.Contains(t, keys, "alice-key")
	assert.NotContains(t, keys, "bob-key")
	assert.NotContains(t, keys, "alice-secret-key")

	usage, err := newQueryMyUsageTool().Run(context.Background(), alice, `{"days":7}`)
	require.NoError(t, err)
	assert.Contains(t, usage, "model-a")
	assert.NotContains(t, usage, "model-b")

	charge, err := newExplainMyChargeTool().Run(context.Background(), alice, `{"log_id":10}`)
	require.NoError(t, err)
	assert.Contains(t, charge, `"found":false`)

	tasks, err := newGetMyTasksTool().Run(context.Background(), alice, `{"task_id":"task-bob"}`)
	require.NoError(t, err)
	assert.Contains(t, tasks, `"found":false`)

	_, memberErr := newListOrgMembersTool().Run(context.Background(), bob, `{"organization_id":9}`)
	require.Error(t, memberErr)
	orgMembers, err := newListOrgMembersTool().Run(context.Background(), alice, `{"organization_id":9}`)
	require.NoError(t, err)
	assert.Contains(t, orgMembers, "al***@example.com")
	assert.NotContains(t, orgMembers, "alice@example.com")
}

// openAccountAssistantMySQL 在一次性数据库里返回连接，避免污染 TEST_MYSQL_DSN 指向的库。
func openAccountAssistantMySQL(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	admin, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	config, err := mysqlDriver.ParseDSN(dsn)
	require.NoError(t, err)
	scratch := fmt.Sprintf("aa_test_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE DATABASE ?", clause.Table{Name: scratch}).Error)
	config.DBName = scratch
	db, err := gorm.Open(mysql.Open(config.FormatDSN()), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
		_ = admin.Exec("DROP DATABASE ?", clause.Table{Name: scratch}).Error
		adminSQL, adminErr := admin.DB()
		if adminErr == nil {
			_ = adminSQL.Close()
		}
	})
	return db
}

// openAccountAssistantPostgres 在一次性 schema 里返回连接，并把 search_path 写进 DSN，
// 保证连接池里的每条连接都落在隔离 schema 内。
func openAccountAssistantPostgres(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	admin, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	require.NoError(t, err)
	scratch := fmt.Sprintf("aa_test_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA ?", clause.Table{Name: scratch}).Error)
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn + " search_path=" + scratch,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
		_ = admin.Exec("DROP SCHEMA ? CASCADE", clause.Table{Name: scratch}).Error
		adminSQL, adminErr := admin.DB()
		if adminErr == nil {
			_ = adminSQL.Close()
		}
	})
	return db
}
