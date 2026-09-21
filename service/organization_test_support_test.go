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
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

// 组织测试共用 model.DB 这个包级全局，且 SQLite 是单写者，
// 所以串行化建库；将来有人加 t.Parallel() 也不会互相踩。
var serviceTestDBMu sync.Mutex

const organizationNameConcurrencyTestTimeout = 10 * time.Second

func organizationNameConcurrencyTestDeadline(t *testing.T) <-chan time.Time {
	t.Helper()
	timer := time.NewTimer(organizationNameConcurrencyTestTimeout)
	t.Cleanup(func() { timer.Stop() })
	return timer.C
}

func waitForOrganizationNameConcurrencyBarrier(
	t *testing.T,
	ready <-chan struct{},
	results <-chan error,
	count int,
	release chan<- struct{},
	deadline <-chan time.Time,
) {
	t.Helper()
	readyCount := 0
	for readyCount < count {
		select {
		case <-ready:
			readyCount++
		case err := <-results:
			close(release)
			t.Fatalf("organization name concurrency operation completed before reaching the barrier (%d/%d ready): %v", readyCount, count, err)
		case <-deadline:
			close(release)
			t.Fatalf("timed out after %s waiting for the organization name concurrency barrier (%d/%d ready)", organizationNameConcurrencyTestTimeout, readyCount, count)
		}
	}
	close(release)
}

func waitForOrganizationNameConcurrencyResults(t *testing.T, results <-chan error, count int, deadline <-chan time.Time) []error {
	t.Helper()
	resultErrors := make([]error, 0, count)
	for len(resultErrors) < count {
		select {
		case err := <-results:
			resultErrors = append(resultErrors, err)
		case <-deadline:
			t.Fatalf("timed out after %s waiting for the organization name concurrency results (%d/%d received)", organizationNameConcurrencyTestTimeout, len(resultErrors), count)
		}
	}
	return resultErrors
}

// setupServiceTestDB 建一个测试专属的进程内 SQLite 并把组织相关表建出来。
//
// 走 model.InitDB() 而不是自己 gorm.Open：InitDB 会执行 initCol()，
// 后者填的是 `key` / `group` 这类保留字的引用格式。跳过它的话，
// 令牌查询会生成列名为空的坏 SQL，组织令牌相关的用例会直接 500。
func setupServiceTestDB(t *testing.T) {
	t.Helper()
	serviceTestDBMu.Lock()
	t.Cleanup(serviceTestDBMu.Unlock)

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMasterNode := common.IsMasterNode
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(30000)", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.UserSubscription{},
		&model.Task{},
		&model.Midjourney{},
		&model.Log{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationInvite{},
		&model.UserAccountContext{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationTokenSystemBlocker{},
		&model.OrganizationIdempotencyRecord{},
		&model.OrganizationBillingSession{},
		&model.OrganizationBillingRecord{},
		&model.OrganizationAuditLog{},
		&model.OrganizationQuotaAdjustment{},
		&model.QuotaData{},
	))

	t.Cleanup(func() {
		// 请求路径上派发的一次性后台任务（额度缓存刷新、退款落库、额度提醒）
		// 到真正干活时才去读进程级全局（model.DB / LOG_DB / common.RedisEnabled），
		// 而用例结束时它们可能还在读。还原全局之前必须先等它们结束，
		// 否则就是和后台 goroutine 抢同一个全局句柄。
		model.WaitForQuotaCacheWorkers()
		WaitForBackgroundWork()
		if model.DB != nil {
			if sqlDB, err := model.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.IsMasterNode = originalMasterNode
		if hadSQLDSN {
			_ = os.Setenv("SQL_DSN", originalSQLDSN)
		} else {
			_ = os.Unsetenv("SQL_DSN")
		}
	})
}

func createServiceTestUser(t *testing.T, username string, role int) model.User {
	t.Helper()
	user := model.User{Username: username, Password: "password", DisplayName: username, Role: role, Status: common.UserStatusEnabled, AffCode: username}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func testOrganizationIdempotencyKey(t *testing.T, operation string) string {
	t.Helper()
	return t.Name() + ":" + operation + ":" + common.GetUUID()
}

func dissolveOrganizationForTest(t *testing.T, operatorUserId, organizationId int, reason string) error {
	t.Helper()
	var organization model.Organization
	if err := model.DB.First(&organization, organizationId).Error; err != nil {
		return err
	}
	return DissolveOrganization(operatorUserId, organizationId, OrganizationAccessModeManagement, DissolveOrganizationRequest{
		ConfirmName:    organization.Name,
		Reason:         reason,
		IdempotencyKey: testOrganizationIdempotencyKey(t, "dissolve"),
	})
}

func stringPtr(value string) *string {
	return &value
}

// organizationRowLocksObservable 报告当前数据库上「写路径是否取到了行锁」能否被观测到。
//
// model.LockForUpdate 在 SQLite 上刻意不下发 FOR UPDATE：SQLite 没有这个语法，
// 靠单写者模型（冲突事务直接失败）达成等价语义。于是所有依赖 FOR 子句的断言
// 在 SQLite 上恒为假，观测到的不是「没加锁」而是「锁不可见」。
//
// 因此这些断言的适用范围按方言收窄：MySQL/PostgreSQL 上照常强断言，
// SQLite 上跳过锁断言、只保留调用本身必须成功这一层验证。
// 真正的并发语义由 organization_concurrency_external_test.go 在真实库上覆盖。
func organizationRowLocksObservable(t *testing.T) bool {
	t.Helper()
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		t.Log("SQLite skips FOR UPDATE by design; row-lock assertions are covered by the MySQL/PostgreSQL organization tests")
		return false
	}
	return true
}

// requireOrganizationRowLock 断言观测到了行锁；SQLite 上该断言不适用。
func requireOrganizationRowLock(t *testing.T, observed bool, message string) {
	t.Helper()
	if !organizationRowLocksObservable(t) {
		return
	}
	require.True(t, observed, message)
}

// requireOrganizationRowLockOrder 断言加锁顺序；SQLite 上该断言不适用。
func requireOrganizationRowLockOrder(t *testing.T, observed []string, expectedPrefix []string) {
	t.Helper()
	if !organizationRowLocksObservable(t) {
		return
	}
	require.GreaterOrEqual(t, len(observed), len(expectedPrefix))
	require.Equal(t, expectedPrefix, observed[:len(expectedPrefix)])
}

// requireOrganizationLockOrderingHolds 断言「A 没有早于 B 发生」这类顺序约束。
// 顺序是相对行锁观测出来的，SQLite 上观测不到锁，于是这类断言在 SQLite 上
// 恒为真、什么也证明不了，必须按方言跳过（见 organizationRowLocksObservable）。
func requireOrganizationLockOrderingHolds(t *testing.T, violated bool, message string) {
	t.Helper()
	if !organizationRowLocksObservable(t) {
		return
	}
	require.False(t, violated, message)
}

// requireOrganizationRowLockCount 断言观测到的行锁次数下限；SQLite 上该断言不适用。
func requireOrganizationRowLockCount(t *testing.T, observed int, minimum int) {
	t.Helper()
	if !organizationRowLocksObservable(t) {
		return
	}
	require.GreaterOrEqual(t, observed, minimum)
}
