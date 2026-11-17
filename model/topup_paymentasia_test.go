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
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const paymentAsiaTestQuotaPerUnit = 500000

// pinPaymentAsiaTestDisplay 固定额度展示类型与 QuotaPerUnit，保证 Amount=2 的
// 订单恰好入账 1,000,000，并在用例结束后恢复全局状态。
func pinPaymentAsiaTestDisplay(t *testing.T, displayType string) {
	t.Helper()
	origType := operation_setting.GetQuotaDisplayType()
	origQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		operation_setting.SetQuotaDisplayTypeForTest(origType)
		common.QuotaPerUnit = origQuotaPerUnit
	})
	operation_setting.SetQuotaDisplayTypeForTest(displayType)
	common.QuotaPerUnit = paymentAsiaTestQuotaPerUnit
}

func createPaymentAsiaTestTopUp(t *testing.T, userId int, tradeNo string) *TopUp {
	t.Helper()
	topUp := &TopUp{
		UserId:          userId,
		Amount:          2,
		Money:           15.6,
		TradeNo:         tradeNo,
		PaymentMethod:   "paymentasia_alipayhk",
		PaymentProvider: PaymentProviderPaymentAsia,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(topUp).Error)
	return topUp
}

func paymentAsiaTestMeta() PaymentAsiaRechargeMeta {
	return PaymentAsiaRechargeMeta{
		RequestReference: "PA-REQ-20261117-001",
		Currency:         "HKD",
		Channel:          "paymentasia_alipayhk",
	}
}

// TestRechargePaymentAsiaCreditsExactlyOnce 覆盖结算主路径：首次回调入账一次，
// 重复回调幂等返回 alreadyDone，且审计日志只写一条并带上网关侧关联字段。
func TestRechargePaymentAsiaCreditsExactlyOnce(t *testing.T) {
	truncateTables(t)
	pinPaymentAsiaTestDisplay(t, operation_setting.QuotaDisplayTypeUSD)

	user := insertUserForPaymentGuardTest(t, 601, 0)
	order := createPaymentAsiaTestTopUp(t, user.Id, "PAYMENTASIATESTONCE")
	meta := paymentAsiaTestMeta()

	alreadyDone, err := RechargePaymentAsia(order.TradeNo, meta, "127.0.0.1")
	require.NoError(t, err)
	assert.False(t, alreadyDone)
	assert.Equal(t, 1_000_000, getUserQuotaForPaymentGuardTest(t, user.Id))

	reloaded := GetTopUpByTradeNo(order.TradeNo)
	require.NotNil(t, reloaded)
	assert.Equal(t, common.TopUpStatusSuccess, reloaded.Status)
	assert.NotZero(t, reloaded.CompleteTime)

	var logs []Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", user.Id, LogTypeTopup).Find(&logs).Error)
	require.Len(t, logs, 1)
	otherMap, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	adminInfo, ok := otherMap["admin_info"].(map[string]interface{})
	require.True(t, ok, "topup log must carry admin_info")
	assert.Equal(t, order.TradeNo, adminInfo["trade_no"])
	assert.Equal(t, "HKD", adminInfo["currency"])
	assert.Equal(t, meta.RequestReference, adminInfo["request_reference"])
	assert.Equal(t, meta.Channel, adminInfo["channel"])
	assert.Equal(t, "127.0.0.1", adminInfo["caller_ip"])
	assert.Equal(t, PaymentProviderPaymentAsia, adminInfo["callback_payment_method"])

	alreadyDone, err = RechargePaymentAsia(order.TradeNo, meta, "127.0.0.1")
	require.NoError(t, err)
	assert.True(t, alreadyDone)
	assert.Equal(t, 1_000_000, getUserQuotaForPaymentGuardTest(t, user.Id))

	logs = nil
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", user.Id, LogTypeTopup).Find(&logs).Error)
	assert.Len(t, logs, 1, "重复回调不得重复写审计日志")
}

// TestRechargePaymentAsiaOnExternalDatabases 在真实 MySQL / PostgreSQL 上复验
// 同一场景（一次入账 + 一次幂等）。行锁语义与方言引号只有真实数据库能覆盖，
// SQLite 由 TestRechargePaymentAsiaCreditsExactlyOnce 覆盖。
//
// 用例会清空固定 ID 的 user/topup/log 行，因此必须指向一次性库；未设置
// PAYMENTASIA_TEST_ALLOW_DESTRUCTIVE=1 时跳过，避免误伤。
func TestRechargePaymentAsiaOnExternalDatabases(t *testing.T) {
	cases := []struct {
		name  string
		env   string
		dbTyp common.DatabaseType
		open  func(dsn string) (*gorm.DB, error)
	}{
		{
			name:  "mysql",
			env:   "TEST_MYSQL_DSN",
			dbTyp: common.DatabaseTypeMySQL,
			open: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(mysql.Open(dsn), &gorm.Config{})
			},
		},
		{
			name:  "postgres",
			env:   "TEST_POSTGRES_DSN",
			dbTyp: common.DatabaseTypePostgreSQL,
			open: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(tc.env))
			if dsn == "" {
				t.Skipf("%s is not configured", tc.env)
			}
			require.Equal(t, "1", os.Getenv("PAYMENTASIA_TEST_ALLOW_DESTRUCTIVE"),
				"用例会清理固定 ID 的数据行，确认指向一次性库后再打开")

			db, err := tc.open(dsn)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })

			originalDB, originalLogDB := DB, LOG_DB
			originalMainType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
			t.Cleanup(func() {
				DB, LOG_DB = originalDB, originalLogDB
				common.SetDatabaseTypes(originalMainType, originalLogType)
				initCol()
			})
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(tc.dbTyp, tc.dbTyp)
			initCol()

			require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &Log{}))
			pinPaymentAsiaTestDisplay(t, operation_setting.QuotaDisplayTypeUSD)

			const (
				userID  = 990601
				tradeNo = "PAYMENTASIAEXTERNALONCE"
			)
			// 允许在同一一次性库上重复执行：先清掉本用例的固定行。
			require.NoError(t, db.Unscoped().Where("id = ?", userID).Delete(&User{}).Error)
			require.NoError(t, db.Where("trade_no = ?", tradeNo).Delete(&TopUp{}).Error)
			require.NoError(t, db.Where("user_id = ?", userID).Delete(&Log{}).Error)

			user := &User{
				Id:       userID,
				Username: "paymentasia_external_user",
				Status:   common.UserStatusEnabled,
				Quota:    0,
			}
			require.NoError(t, DB.Create(user).Error)
			order := createPaymentAsiaTestTopUp(t, user.Id, tradeNo)
			meta := paymentAsiaTestMeta()

			alreadyDone, err := RechargePaymentAsia(order.TradeNo, meta, "127.0.0.1")
			require.NoError(t, err)
			assert.False(t, alreadyDone)
			assert.Equal(t, 1_000_000, getUserQuotaFromDB(t, userID))

			reloaded := GetTopUpByTradeNo(tradeNo)
			require.NotNil(t, reloaded)
			assert.Equal(t, common.TopUpStatusSuccess, reloaded.Status)

			alreadyDone, err = RechargePaymentAsia(order.TradeNo, meta, "127.0.0.1")
			require.NoError(t, err)
			assert.True(t, alreadyDone)
			assert.Equal(t, 1_000_000, getUserQuotaFromDB(t, userID))

			var logCount int64
			require.NoError(t, LOG_DB.Model(&Log{}).Where("user_id = ? AND type = ?", userID, LogTypeTopup).Count(&logCount).Error)
			assert.Equal(t, int64(1), logCount, "重复回调不得重复写审计日志")
		})
	}
}
