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
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// paymentAsiaLegacyTopUp 复刻引入手续费快照列之前的最新发布版 top_ups 表结构
// （含币种/环境快照列），仅用于迁移测试构造一张升级前的代表性表。
type paymentAsiaLegacyTopUp struct {
	Id                 int
	UserId             int `gorm:"index"`
	Amount             int64
	Money              float64
	TradeNo            string `gorm:"unique;type:varchar(255);index"`
	PaymentMethod      string `gorm:"type:varchar(50)"`
	PaymentProvider    string `gorm:"type:varchar(50);default:''"`
	CreateTime         int64
	CompleteTime       int64
	Status             string
	Rate               float64 `gorm:"column:rate"`
	Currency           string  `gorm:"type:varchar(10);default:''"`
	PaymentEnvironment string  `gorm:"type:varchar(16);default:''"`
}

func (paymentAsiaLegacyTopUp) TableName() string {
	return "top_ups"
}

// paymentAsiaFeeSnapshotColumns 是手续费功能新增的订单快照列（additive migration）。
var paymentAsiaFeeSnapshotColumns = []string{"Fee", "FeeRate", "FeeFixed", "NetAmount", "NetQuota"}

// runPaymentAsiaTopUpSnapshotMigration 在 db 上执行完整的快照列迁移场景：
// 旧表（无手续费快照列）→ 两次 AutoMigrate → 列存在、旧数据保留、唯一索引仍生效，
// 且带快照的新订单可以写入并读回。SQLite 与真实 MySQL/PostgreSQL 共用。
func runPaymentAsiaTopUpSnapshotMigration(t *testing.T, db *gorm.DB) {
	t.Helper()

	require.NoError(t, db.Migrator().DropTable("top_ups"))
	require.NoError(t, db.Migrator().CreateTable(&paymentAsiaLegacyTopUp{}))
	legacy := &paymentAsiaLegacyTopUp{
		UserId:             901,
		Amount:             2,
		Money:              15.6,
		TradeNo:            "PA-MIGRATION-LEGACY",
		PaymentMethod:      "paymentasia_alipayhk",
		PaymentProvider:    PaymentProviderPaymentAsia,
		CreateTime:         1_760_000_000,
		Status:             common.TopUpStatusPending,
		Currency:           "HKD",
		PaymentEnvironment: "live",
	}
	require.NoError(t, db.Create(legacy).Error)

	require.True(t, db.Migrator().HasColumn(&TopUp{}, "Currency"))
	require.True(t, db.Migrator().HasColumn(&TopUp{}, "PaymentEnvironment"))
	for _, column := range paymentAsiaFeeSnapshotColumns {
		require.False(t, db.Migrator().HasColumn(&TopUp{}, column),
			"column %s must not exist before the migration", column)
	}

	// 启动迁移在每次进程启动时都会执行：重复执行必须幂等。
	require.NoError(t, db.AutoMigrate(&TopUp{}))
	require.NoError(t, db.AutoMigrate(&TopUp{}))

	require.True(t, db.Migrator().HasColumn(&TopUp{}, "Currency"))
	require.True(t, db.Migrator().HasColumn(&TopUp{}, "PaymentEnvironment"))
	for _, column := range paymentAsiaFeeSnapshotColumns {
		require.True(t, db.Migrator().HasColumn(&TopUp{}, column),
			"column %s must exist after the migration", column)
	}
	assert.True(t, db.Migrator().HasIndex(&TopUp{}, "TradeNo"), "trade_no index must survive the migration")

	var migrated TopUp
	require.NoError(t, db.Where("trade_no = ?", legacy.TradeNo).First(&migrated).Error)
	assert.Equal(t, legacy.UserId, migrated.UserId)
	assert.Equal(t, legacy.Amount, migrated.Amount)
	assert.Equal(t, legacy.Money, migrated.Money)
	assert.Equal(t, legacy.PaymentMethod, migrated.PaymentMethod)
	assert.Equal(t, legacy.PaymentProvider, migrated.PaymentProvider)
	assert.Equal(t, legacy.CreateTime, migrated.CreateTime)
	assert.Equal(t, common.TopUpStatusPending, migrated.Status)
	assert.Equal(t, "HKD", migrated.Currency)
	assert.Equal(t, "live", migrated.PaymentEnvironment)
	// 旧订单不做手续费回填：NetQuota=0 表示结算回退全额入账。
	assert.Zero(t, migrated.Fee)
	assert.Zero(t, migrated.FeeRate)
	assert.Zero(t, migrated.FeeFixed)
	assert.Zero(t, migrated.NetAmount)
	assert.Zero(t, migrated.NetQuota)

	// 唯一性保证必须仍在：重复 trade_no 插入失败。
	duplicate := &TopUp{
		UserId:          902,
		Amount:          1,
		Money:           1,
		TradeNo:         legacy.TradeNo,
		PaymentProvider: PaymentProviderPaymentAsia,
		Status:          common.TopUpStatusPending,
	}
	require.Error(t, db.Create(duplicate).Error)

	snapshot := &TopUp{
		UserId:             903,
		Amount:             100,
		Money:              100,
		TradeNo:            "PA-MIGRATION-SNAPSHOT",
		PaymentMethod:      "paymentasia_creditcard",
		PaymentProvider:    PaymentProviderPaymentAsia,
		CreateTime:         common.GetTimestamp(),
		Status:             common.TopUpStatusPending,
		Currency:           "HKD",
		PaymentEnvironment: "live",
		Fee:                5.7,
		FeeRate:            0.037,
		FeeFixed:           2,
		NetAmount:          94.3,
		NetQuota:           47_150_000,
	}
	require.NoError(t, db.Create(snapshot).Error)

	var readBack TopUp
	require.NoError(t, db.Where("trade_no = ?", snapshot.TradeNo).First(&readBack).Error)
	assert.Equal(t, "HKD", readBack.Currency)
	assert.Equal(t, "live", readBack.PaymentEnvironment)
	assert.Equal(t, 5.7, readBack.Fee)
	assert.Equal(t, 0.037, readBack.FeeRate)
	assert.Equal(t, 2.0, readBack.FeeFixed)
	assert.Equal(t, 94.3, readBack.NetAmount)
	assert.Equal(t, 47_150_000, readBack.NetQuota)
}

// runPaymentAsiaTopUpSnapshotFreshMigration 在全新库上验证同一组列的建表路径：
// 两次 AutoMigrate 幂等，列与 trade_no 唯一索引存在，快照订单可写入并读回。
func runPaymentAsiaTopUpSnapshotFreshMigration(t *testing.T, db *gorm.DB) {
	t.Helper()

	require.NoError(t, db.Migrator().DropTable("top_ups"))
	require.NoError(t, db.AutoMigrate(&TopUp{}))
	require.NoError(t, db.AutoMigrate(&TopUp{}))

	require.True(t, db.Migrator().HasColumn(&TopUp{}, "Currency"))
	require.True(t, db.Migrator().HasColumn(&TopUp{}, "PaymentEnvironment"))
	for _, column := range paymentAsiaFeeSnapshotColumns {
		require.True(t, db.Migrator().HasColumn(&TopUp{}, column),
			"column %s must exist on a fresh database", column)
	}
	assert.True(t, db.Migrator().HasIndex(&TopUp{}, "TradeNo"))

	snapshot := &TopUp{
		UserId:             904,
		Amount:             100,
		Money:              100,
		TradeNo:            "PA-MIGRATION-FRESH",
		PaymentMethod:      "paymentasia_creditcard",
		PaymentProvider:    PaymentProviderPaymentAsia,
		CreateTime:         common.GetTimestamp(),
		Status:             common.TopUpStatusPending,
		Currency:           "USD",
		PaymentEnvironment: "sandbox",
		Fee:                3.9564,
		FeeRate:            0.037,
		FeeFixed:           0.2564,
		NetAmount:          96.0436,
		NetQuota:           48_021_800,
	}
	require.NoError(t, db.Create(snapshot).Error)

	var readBack TopUp
	require.NoError(t, db.Where("trade_no = ?", snapshot.TradeNo).First(&readBack).Error)
	assert.Equal(t, "USD", readBack.Currency)
	assert.Equal(t, "sandbox", readBack.PaymentEnvironment)
	assert.Equal(t, snapshot.Fee, readBack.Fee)
	assert.Equal(t, snapshot.FeeRate, readBack.FeeRate)
	assert.Equal(t, snapshot.FeeFixed, readBack.FeeFixed)
	assert.Equal(t, snapshot.NetAmount, readBack.NetAmount)
	assert.Equal(t, snapshot.NetQuota, readBack.NetQuota)

	duplicate := *snapshot
	duplicate.Id = 0
	require.Error(t, db.Create(&duplicate).Error,
		"fresh schema must reject a duplicate trade_no")
}

// TestPaymentAsiaTopUpSnapshotMigrationSQLite 在临时 SQLite 文件库上验证
// top_ups 增加 Currency / PaymentEnvironment 两列的升级路径与全新建表路径（必跑）。
func TestPaymentAsiaTopUpSnapshotMigrationSQLite(t *testing.T) {
	openDB := func(t *testing.T) *gorm.DB {
		t.Helper()
		dsn := filepath.Join(t.TempDir(), "legacy.db")
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		return db
	}

	t.Run("upgrade_from_legacy", func(t *testing.T) {
		runPaymentAsiaTopUpSnapshotMigration(t, openDB(t))
	})
	t.Run("fresh_database", func(t *testing.T) {
		runPaymentAsiaTopUpSnapshotFreshMigration(t, openDB(t))
	})
}

// TestPaymentAsiaTopUpSnapshotMigrationOnExternalDatabases 在真实 MySQL /
// PostgreSQL 上复验旧表升级与全新库两条路径。用例会删除并重建 top_ups 表，
// 因此必须指向一次性库：未设置对应 DSN 时跳过；DSN 已设置时必须同时设置
// PAYMENTASIA_TEST_ALLOW_DESTRUCTIVE=1，否则用例失败以阻止误连生产库。
func TestPaymentAsiaTopUpSnapshotMigrationOnExternalDatabases(t *testing.T) {
	cases := []struct {
		name string
		env  string
		open func(dsn string) (*gorm.DB, error)
	}{
		{
			name: "mysql",
			env:  "TEST_MYSQL_DSN",
			open: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(mysql.Open(dsn), &gorm.Config{})
			},
		},
		{
			name: "postgres",
			env:  "TEST_POSTGRES_DSN",
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
				"用例会删除并重建 top_ups 表，确认指向一次性库后再打开")

			db, err := tc.open(dsn)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })

			runPaymentAsiaTopUpSnapshotMigration(t, db)
			runPaymentAsiaTopUpSnapshotFreshMigration(t, db)
		})
	}
}
