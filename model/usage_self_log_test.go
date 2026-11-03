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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSelfUsageLogRowsNetsRefundsAndExcludesTaskAndMJ(t *testing.T) {
	db := newBillingTestDB(t)
	orig := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = orig })

	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	rows := []Log{
		{UserId: 7, ActorUserId: 7, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "k1", BillingAccountType: "personal", PromptTokens: 100, CompletionTokens: 50, CacheTokens: 30, Quota: 1000},
		{UserId: 7, ActorUserId: 7, Type: LogTypeConsume, UsageSource: LogUsageSourceFee, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "k1", BillingAccountType: "personal", Quota: 50},
		{UserId: 7, ActorUserId: 7, Type: LogTypeRefund, UsageSource: LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "k1", BillingAccountType: "personal", Quota: 200},
		{UserId: 7, ActorUserId: 7, Type: LogTypeConsume, UsageSource: LogUsageSourceTask, CreatedAt: ts, ModelName: "wan3.0-video", BillingAccountType: "personal", Quota: 9999},
		{UserId: 7, ActorUserId: 7, Type: LogTypeConsume, UsageSource: LogUsageSourceMJ, CreatedAt: ts, ModelName: "mj_imagine", BillingAccountType: "personal", Quota: 9999},
		{UserId: 7, ActorUserId: 7, Type: LogTypeError, UsageSource: LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", BillingAccountType: "personal", Quota: 0},
		{UserId: 8, ActorUserId: 8, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", BillingAccountType: "personal", Quota: 8888},
		{UserId: 7, ActorUserId: 7, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 2, TokenName: "k2", BillingAccountType: "organization", OrganizationId: 9, OrganizationName: "Acme", Quota: 300},
	}
	require.NoError(t, db.Create(&rows).Error)

	got, err := GetSelfUsageLogRows(7, ts-60, ts+60)
	require.NoError(t, err)
	require.Len(t, got, 2)

	personal := got[0]
	if personal.BillingAccountType != "personal" {
		personal = got[1]
	}
	assert.Equal(t, 1, personal.RequestCount) // fee consume is not a request
	assert.Equal(t, 100, personal.PromptTokens)
	assert.Equal(t, 50, personal.CompletionTokens)
	assert.Equal(t, 30, personal.CacheTokens)
	assert.Equal(t, 1000+50-200, personal.Quota) // consume + fee - refund
}

func TestSelfUsageLogRowsSeparatesLocalDayBoundary(t *testing.T) {
	db := newBillingTestDB(t)
	orig := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = orig })

	late := time.Date(2026, 11, 2, 23, 59, 59, 0, time.Local).Unix()
	early := time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&[]Log{
		{UserId: 7, ActorUserId: 7, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: late, ModelName: "gpt-4o", BillingAccountType: "personal"},
		{UserId: 7, ActorUserId: 7, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: early, ModelName: "gpt-4o", BillingAccountType: "personal"},
	}).Error)

	got, err := GetSelfUsageLogRows(7, late, early)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.NotEqual(t, got[0].DayKey, got[1].DayKey)
}

func TestSelfUsageLogRowsConfiguredDatabases(t *testing.T) {
	cases := []struct {
		name string
		env  string
		open func(string) (*gorm.DB, error)
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
				return gorm.Open(postgres.New(postgres.Config{
					DSN:                  dsn,
					PreferSimpleProtocol: true,
				}), &gorm.Config{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(tc.env))
			if dsn == "" {
				t.Skip(tc.env + " is not configured")
			}
			db, err := tc.open(dsn)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&Log{}))

			const actor = 987654321
			require.NoError(t, db.Where("actor_user_id = ?", actor).Delete(&Log{}).Error)
			t.Cleanup(func() {
				_ = db.Where("actor_user_id = ?", actor).Delete(&Log{}).Error
				_ = sqlDB.Close()
			})

			ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
			rows := []Log{
				{UserId: actor, ActorUserId: actor, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "k1", BillingAccountType: "personal", PromptTokens: 100, CompletionTokens: 50, CacheTokens: 30, Quota: 1000},
				{UserId: actor, ActorUserId: actor, Type: LogTypeConsume, UsageSource: LogUsageSourceFee, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "k1", BillingAccountType: "personal", Quota: 50},
				{UserId: actor, ActorUserId: actor, Type: LogTypeRefund, UsageSource: LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "k1", BillingAccountType: "personal", Quota: 200},
				{UserId: actor, ActorUserId: actor, Type: LogTypeConsume, UsageSource: LogUsageSourceTask, CreatedAt: ts, ModelName: "wan3.0-video", BillingAccountType: "personal", Quota: 9999},
			}
			require.NoError(t, db.Create(&rows).Error)

			orig := LOG_DB
			LOG_DB = db
			t.Cleanup(func() { LOG_DB = orig })

			got, err := GetSelfUsageLogRows(actor, ts-60, ts+60)
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, 1, got[0].RequestCount)
			assert.Equal(t, 100, got[0].PromptTokens)
			assert.Equal(t, 50, got[0].CompletionTokens)
			assert.Equal(t, 30, got[0].CacheTokens)
			assert.Equal(t, 1000+50-200, got[0].Quota)
		})
	}
}
