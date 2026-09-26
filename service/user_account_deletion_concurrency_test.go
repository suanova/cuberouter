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
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 账号收口（DeleteUserAccount）与组织侧的成员操作会碰到同一组行：收口按
// users → organizations → organization_members 上锁并删行，成员移除按
// organizations → organization_members 上锁并改状态。两者并发时可能真撞在一起，
// 本文件用真实并发把这件事跑出来，并在两种引擎上覆盖：
//
//   - SQLite：默认夹具（进程内 cache=shared 内存库，即 setupServiceTestDB）验证
//     「事务能收尾、最终状态自洽」；生产形态夹具（文件库 + WAL + _txlock=immediate，
//     与 common.SQLitePath 的默认 DSN 同形）验证「竞争退化为串行、没有锁错误」。
//   - PostgreSQL（TEST_POSTGRES_DSN + ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1）：
//     真行锁下的竞争结果，最低支持版与生产版各跑一遍。
//
// 用例体只写一份（runXxx），每种夹具各是一个薄壳，避免两边断言漂移。

// migrateAccountDeletionTestSchema 建出账号收口与成员移除两条路径都会用到的表。
//
// SQLite 侧 setupServiceTestDB 已经建过（AutoMigrate 重复调用是幂等的）；
// 外部库侧必须显式建：PostgreSQL 上「表不存在」会让整个事务以 25P02 中止，
// 之后的断言全部失真，而不是报一个能看懂的错。
func migrateAccountDeletionTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.UserAccountContext{},
		&model.UserSession{},
		&model.AuthFlow{},
		&model.ExternalIdentityClaim{},
		&model.PasskeyCredential{},
		&model.TwoFA{},
		&model.TwoFABackupCode{},
		&model.UserOAuthBinding{},
		&model.Token{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationIdempotencyRecord{},
		&model.OrganizationTokenSystemBlocker{},
		&model.OrganizationAuditLog{},
	))
}

// cleanupAccountDeletionTestRows 清掉用例造的行：外部库是复用的（同一个 orgtest），
// 文件库也留在 t.TempDir() 里，不清会让后续排查读到的判据不干净。
func cleanupAccountDeletionTestRows(t *testing.T, db *gorm.DB, userIds, organizationIds []int) {
	t.Helper()
	if len(organizationIds) > 0 {
		require.NoError(t, db.Where("organization_id IN ?", organizationIds).Delete(&model.OrganizationAuditLog{}).Error)
		require.NoError(t, db.Where("organization_id IN ?", organizationIds).Delete(&model.OrganizationIdempotencyRecord{}).Error)
		require.NoError(t, db.Where("organization_id IN ?", organizationIds).Delete(&model.OrganizationTokenSystemBlocker{}).Error)
		require.NoError(t, db.Where("organization_id IN ?", organizationIds).Delete(&model.OrganizationMember{}).Error)
		require.NoError(t, db.Where("id IN ?", organizationIds).Delete(&model.Organization{}).Error)
	}
	if len(userIds) > 0 {
		require.NoError(t, db.Unscoped().Where("user_id IN ?", userIds).Delete(&model.Token{}).Error)
		require.NoError(t, db.Unscoped().Where("user_id IN ?", userIds).Delete(&model.UserAccountContext{}).Error)
		require.NoError(t, db.Unscoped().Where("id IN ?", userIds).Delete(&model.User{}).Error)
	}
}

// accountDeletionRaceSuffix 造一个既唯一又短的名字后缀：createServiceTestUser 会把
// username 同时写进 users.aff_code，而那一列是 varchar(32)，直接拼 36 位 UUID 在
// PostgreSQL 上会被长度校验拒掉（SQLite 不校验长度，只有外部库变体会暴露）。
func accountDeletionRaceSuffix() string {
	return strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
}

// runDeleteUserAccountConcurrentWithMemberRemoval 让收口与成员移除真撞一次，
// 返回两方各自的错误，交由调用方按方言断言失败的性质。
//
// 状态断言与方言无关，必须永远成立：
//   - 账号没了，成员行也必须没了（收口删成员行与删账号在同一事务里）；
//   - 账号还在，成员行必须还在（成员移除只改状态，不删账号）。
func runDeleteUserAccountConcurrentWithMemberRemoval(t *testing.T, db *gorm.DB) (error, error) {
	t.Helper()
	migrateAccountDeletionTestSchema(t, db)

	operator := createServiceTestUser(t, "race-op-"+accountDeletionRaceSuffix(), common.RoleRootUser)
	owner := createServiceTestUser(t, "race-owner-"+accountDeletionRaceSuffix(), common.RoleCommonUser)
	target := createServiceTestUser(t, "race-target-"+accountDeletionRaceSuffix(), common.RoleCommonUser)
	transferTo := createServiceTestUser(t, "race-transfer-"+accountDeletionRaceSuffix(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, owner.Id)
	addDeletionTestMember(t, organization.Id, owner.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive)
	addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusActive)
	addDeletionTestMember(t, organization.Id, transferTo.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusActive)
	t.Cleanup(func() {
		cleanupAccountDeletionTestRows(t, db,
			[]int{operator.Id, owner.Id, target.Id, transferTo.Id},
			[]int{organization.Id})
	})

	// accessMode 与 Idempotency-Key 的取值跟 organization_member_test.go 里的
	// 成员操作用例保持一致：组织内 owner 走 management 模式。
	idempotencyKey := testOrganizationIdempotencyKey(t, "race-remove-member")

	deleteErrCh := make(chan error, 1)
	removeErrCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		deleteErrCh <- DeleteUserAccount(operator.Id, target.Id)
	}()
	go func() {
		defer wg.Done()
		removeErrCh <- RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, RemoveMemberRequest{
			TransferToUserId: transferTo.Id,
			Reason:           "race test",
			IdempotencyKey:   idempotencyKey,
		})
	}()
	wg.Wait()
	deleteErr, removeErr := <-deleteErrCh, <-removeErrCh

	var userCount int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("id = ?", target.Id).Count(&userCount).Error)
	var member model.OrganizationMember
	memberErr := db.Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).First(&member).Error
	if userCount == 0 {
		assert.ErrorIs(t, memberErr, gorm.ErrRecordNotFound, "账号已删，成员行不得残留")
	} else {
		require.NoError(t, memberErr, "账号还在，成员行必须还在")
	}
	return deleteErr, removeErr
}

// requireNoAccountDeletionRaceLockError 断言失败方只能是业务错误，不能是数据库锁
// 等待或死锁——这才是并发用例真正要抓的东西。
//
// 只在「引擎会因竞争报锁错误」时有意义，所以由生产形态 SQLite 变体与 PostgreSQL
// 变体调用；内存夹具（cache=shared）的 SQLITE_LOCKED 由夹具形态决定、与锁序无关，
// 那里用 requireAccountDeletionRaceFailureIsWriteContention。
func requireNoAccountDeletionRaceLockError(t *testing.T, errs ...error) {
	t.Helper()
	for _, err := range errs {
		if err == nil {
			continue
		}
		message := strings.ToLower(err.Error())
		assert.NotContains(t, message, "deadlock")
		assert.NotContains(t, message, "database is locked")
		assert.NotContains(t, message, "lock wait timeout")
		assert.NotContains(t, message, "could not serialize")
	}
}

// requireAccountDeletionRaceFailureIsWriteContention 是内存夹具上的等价断言。
//
// setupServiceTestDB 的库是 cache=shared 的内存库：共享缓存模式下「另一个事务正持有
// 某张表的读锁」会让写语句立刻返回 SQLITE_LOCKED（"database table is locked"，
// busy_timeout 不介入），这是夹具形态决定的，与锁序无关（SQLite 不发 FOR UPDATE）。
// 于是这里只要求：失败必须是写锁竞争，不能是表不存在、约束冲突、SQL 语法这类
// 真正说明环境或代码坏掉的错误。
//
// 收口侧不接受 gorm.ErrRecordNotFound：这个用例里只有收口会删账号，它自己读不到
// users 行说明收口本身坏了。成员移除侧接受它——收口先赢时成员行已随账号一起消失，
// 而 RemoveOrganizationMember 直接对成员行 First、不先判存在，于是它读到的就是
// 「没有这一行」。这是合法结果，文件库与 PostgreSQL 变体接受同一个结果。
func requireAccountDeletionRaceFailureIsWriteContention(t *testing.T, deleteErr, removeErr error) {
	t.Helper()
	if deleteErr != nil {
		assert.Contains(t, strings.ToLower(deleteErr.Error()), "locked",
			"收口只接受写锁竞争失败，其余错误必须暴露出来")
	}
	if removeErr != nil && !errors.Is(removeErr, gorm.ErrRecordNotFound) {
		assert.Contains(t, strings.ToLower(removeErr.Error()), "locked",
			"成员移除只接受写锁竞争失败或成员行已随账号消失，其余错误必须暴露出来")
	}
}

// requireAccountDeletionRaceProgress 断言两方至少有一方成功。
//
// 「收口先赢 → 成员移除找不到成员行」与「成员移除先赢 → 收口照样收口成功」都是合法
// 结果，但两边同时失败说明这次竞争没有推进任何状态，用例就退化成了空跑。
func requireAccountDeletionRaceProgress(t *testing.T, deleteErr, removeErr error) {
	t.Helper()
	require.True(t, deleteErr == nil || removeErr == nil,
		"两方都失败（delete=%v remove=%v），本次竞争没有验证到任何状态", deleteErr, removeErr)
}

func TestDeleteUserAccountConcurrentWithMemberRemoval(t *testing.T) {
	setupServiceTestDB(t)
	deleteErr, removeErr := runDeleteUserAccountConcurrentWithMemberRemoval(t, model.DB)
	requireAccountDeletionRaceFailureIsWriteContention(t, deleteErr, removeErr)
	requireAccountDeletionRaceProgress(t, deleteErr, removeErr)
	t.Logf("sqlite in-memory fixture race outcome: deleteErr=%v removeErr=%v", deleteErr, removeErr)
}

// setupAccountDeletionSQLiteFileDB 用一个生产形态的 SQLite 打开测试库：
// 文件库 + WAL + busy_timeout + _txlock=immediate，与 common.SQLitePath 的默认
// DSN 同形。内存夹具没有这些性质，竞争不会退化成等待。
func setupAccountDeletionSQLiteFileDB(t *testing.T) *gorm.DB {
	t.Helper()
	serviceTestDBMu.Lock()
	t.Cleanup(serviceTestDBMu.Unlock)

	originalDB, originalLogDB := model.DB, model.LOG_DB
	originalMainType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMasterNode := common.IsMasterNode

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SQLitePath = "file:" + filepath.Join(t.TempDir(), "account-deletion.db") +
		"?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_txlock=immediate"
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	db, err := gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	model.DB, model.LOG_DB = db, db

	t.Cleanup(func() {
		// 请求路径上派发的一次性后台任务到真正干活时才去读进程级全局，
		// 还原全局之前必须先等它们结束（同 setupServiceTestDB）。
		model.WaitForQuotaCacheWorkers()
		WaitForBackgroundWork()
		require.NoError(t, sqlDB.Close())
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.IsMasterNode = originalMasterNode
	})
	return db
}

// TestDeleteUserAccountConcurrentWithMemberRemovalSQLiteFileDatabase 是同一个用例体
// 在「生产形态 SQLite」上的变体：内存夹具上的 SQLITE_LOCKED 是共享缓存的产物，
// 生产用的文件库靠 _txlock=immediate + busy_timeout 把并发写退化为串行等待。
func TestDeleteUserAccountConcurrentWithMemberRemovalSQLiteFileDatabase(t *testing.T) {
	db := setupAccountDeletionSQLiteFileDB(t)
	deleteErr, removeErr := runDeleteUserAccountConcurrentWithMemberRemoval(t, db)
	requireNoAccountDeletionRaceLockError(t, deleteErr, removeErr)
	requireAccountDeletionRaceProgress(t, deleteErr, removeErr)
	t.Logf("sqlite file database race outcome: deleteErr=%v removeErr=%v", deleteErr, removeErr)
}

// TestDeleteUserAccountConcurrentWithMemberRemovalExternalDatabase 是同一个用例体
// 在真实 PostgreSQL 上的变体：SQLite 的单写者模型给不出真正的行级竞争。
func TestDeleteUserAccountConcurrentWithMemberRemovalExternalDatabase(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	deleteErr, removeErr := runDeleteUserAccountConcurrentWithMemberRemoval(t, db)
	requireNoAccountDeletionRaceLockError(t, deleteErr, removeErr)
	requireAccountDeletionRaceProgress(t, deleteErr, removeErr)
	t.Logf("postgres race outcome: deleteErr=%v removeErr=%v", deleteErr, removeErr)
}

// runDeleteUserAccountLockOrder 断言收口的行锁顺序是
// users → organizations → organization_members。
//
// 注册路径是 user → member（InsertWithTx 落库后插成员行），管理员加人同样先读 users
// 再写成员行；收口若反过来先把成员行/组织行握在手里再碰 users，就会与它们形成反向
// 锁序（spec §9）。SQLite 不下发 FOR UPDATE，断言在该方言上自动跳过，
// 因此真正断言这条不变式的是外部库变体。
func runDeleteUserAccountLockOrder(t *testing.T, db *gorm.DB) {
	t.Helper()
	migrateAccountDeletionTestSchema(t, db)

	operator := createServiceTestUser(t, "del-op-"+accountDeletionRaceSuffix(), common.RoleRootUser)
	owner := createServiceTestUser(t, "del-owner-"+accountDeletionRaceSuffix(), common.RoleCommonUser)
	target := createServiceTestUser(t, "del-target-"+accountDeletionRaceSuffix(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, owner.Id)
	addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusActive)
	t.Cleanup(func() {
		cleanupAccountDeletionTestRows(t, db,
			[]int{operator.Id, owner.Id, target.Id},
			[]int{organization.Id})
	})

	var lockOrder []string
	callbackName := "test:account-deletion-lock-order"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "users", "organizations", "organization_members":
		default:
			return
		}
		locking, ok := tx.Statement.Clauses["FOR"].Expression.(clause.Locking)
		if !ok || locking.Strength != "UPDATE" {
			return
		}
		lockOrder = append(lockOrder, tx.Statement.Table)
	}))
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(callbackName)
	})

	require.NoError(t, DeleteUserAccount(operator.Id, target.Id))

	t.Logf("observed row lock order: %v", lockOrder)
	requireOrganizationRowLockOrder(t, lockOrder, []string{"users", "organizations", "organization_members"})
}

// TestDeleteUserAccountLockOrderExternalDatabase 在真实 PostgreSQL 上执行锁序断言：
// SQLite 变体里这条断言整体跳过，锁序只有在这里才真正被验证。
func TestDeleteUserAccountLockOrderExternalDatabase(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	runDeleteUserAccountLockOrder(t, db)
}
