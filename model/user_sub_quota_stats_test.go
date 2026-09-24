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

// Port from develop fb29c98 (51b3f79/#86, 2c55d2e, f951584):
// FillUsersSubscriptionQuotaStats aggregation tests (SQLite) plus an
// opt-in PostgreSQL dialect check gated on TEST_PG_DSN.

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupSubQuotaTestDB builds an isolated SQLite database with the users,
// user_subscriptions and subscription_plans tables and swaps the package
// DB for the duration of the test.
func setupSubQuotaTestDB(t *testing.T) {
	t.Helper()
	// Tests have no Redis; keep the plan cache on its memory tier only.
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	previousDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "open sqlite")
	DB = db
	t.Cleanup(func() { DB = previousDB })

	require.NoError(t, DB.AutoMigrate(&User{}, &UserSubscription{}, &SubscriptionPlan{}), "migrate tables")
}

// seedPlanRaw inserts a plan via raw SQL (bypassing hooks); returns plan id.
func seedPlanRaw(t *testing.T, title string, priceAmount float64) int {
	t.Helper()
	require.NoError(t, DB.Exec(
		"INSERT INTO subscription_plans (title, price_amount) VALUES (?, ?)",
		title, priceAmount,
	).Error, "seed plan %s", title)
	var id int
	require.NoError(t, DB.Raw("SELECT id FROM subscription_plans WHERE title = ?", title).Scan(&id).Error,
		"query seed plan %s", title)
	require.NotZero(t, id, "seed plan id for %s", title)
	// The plan cache is process-wide; invalidate after seeding so other
	// cases cannot observe a stale entry under the same id.
	InvalidateSubscriptionPlanCache(id)
	return id
}

// seedUserRaw inserts a user via raw SQL (bypassing hooks/unique index
// validation).
func seedUserRaw(t *testing.T, username string) int {
	t.Helper()
	require.NoError(t, DB.Exec(
		"INSERT INTO users (username, password, aff_code, role, status, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		username, "pwd12345", username+"-aff", 1, 1, time.Now().Unix(),
	).Error, "seed user %s", username)
	var id int
	require.NoError(t, DB.Raw("SELECT id FROM users WHERE username = ?", username).Scan(&id).Error,
		"query seed user %s", username)
	require.NotZero(t, id, "seed user id for %s", username)
	return id
}

// seedSubscriptionRaw inserts a subscription with explicit status/end_time
// (bypassing the BeforeCreate hook).
func seedSubscriptionRaw(t *testing.T, userId, planId int, amountTotal, amountUsed int64, status string, endTime int64) {
	t.Helper()
	require.NoError(t, DB.Exec(
		"INSERT INTO user_subscriptions (user_id, plan_id, amount_total, amount_used, start_time, end_time, status, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'order', 0, 0)",
		userId, planId, amountTotal, amountUsed, endTime-1000, endTime, status,
	).Error, "seed subscription user=%d", userId)
}

// No active subscription -> all zero values.
func TestFillUsersSubscriptionQuotaStats_NoActive(t *testing.T) {
	setupSubQuotaTestDB(t)

	uid := seedUserRaw(t, "noactive")
	plan := seedPlanRaw(t, "noactive-plan", 5.0)
	// Expired + cancelled subscriptions must not be counted.
	seedSubscriptionRaw(t, uid, plan, 1000, 200, "expired", time.Now().Unix()-10)
	seedSubscriptionRaw(t, uid, plan, 500, 100, "cancelled", time.Now().Unix()+10000)

	users := []*User{{Id: uid}}
	FillUsersSubscriptionQuotaStats(users)

	u := users[0]
	assert.EqualValues(t, 0, u.SubscriptionTotalQuota)
	assert.EqualValues(t, 0, u.SubscriptionRemainQuota)
	assert.EqualValues(t, 0, u.SubscriptionUsedQuota)
	assert.False(t, u.SubscriptionUnlimited)
	assert.InDelta(t, 0, u.SubscriptionRemainValue, 1e-9)
}

// Single active limited subscription: total=amount_total,
// remain=max(0,total-used), used=amount_used.
func TestFillUsersSubscriptionQuotaStats_SingleActive(t *testing.T) {
	setupSubQuotaTestDB(t)

	uid := seedUserRaw(t, "single")
	plan := seedPlanRaw(t, "single-plan", 9.9)
	seedSubscriptionRaw(t, uid, plan, 1000, 200, "active", time.Now().Unix()+10000)

	users := []*User{{Id: uid}}
	FillUsersSubscriptionQuotaStats(users)

	u := users[0]
	assert.EqualValues(t, 1000, u.SubscriptionTotalQuota)
	assert.EqualValues(t, 800, u.SubscriptionRemainQuota)
	assert.EqualValues(t, 200, u.SubscriptionUsedQuota)
	assert.False(t, u.SubscriptionUnlimited)
	// Plan price 9.9: remain value = 9.9 x 800/1000 = 7.92
	assert.InDelta(t, 7.92, u.SubscriptionRemainValue, 0.001)
}

// Summing across subscriptions + over-used subscription clamped to 0 +
// unlimited subscription flagged.
func TestFillUsersSubscriptionQuotaStats_Mixed(t *testing.T) {
	setupSubQuotaTestDB(t)

	uidA := seedUserRaw(t, "mixa")
	uidB := seedUserRaw(t, "mixb")
	now := time.Now().Unix()

	planPaid := seedPlanRaw(t, "paid", 10.0)
	planFree := seedPlanRaw(t, "free", 0.0)

	// A: limited 1000 used 200 (paid); limited 500 used 600 over-used
	// (free, contributes 0 to remain); unlimited (<=0) used 50 (paid)
	seedSubscriptionRaw(t, uidA, planPaid, 1000, 200, "active", now+10000)
	seedSubscriptionRaw(t, uidA, planFree, 500, 600, "active", now+20000)
	seedSubscriptionRaw(t, uidA, planPaid, 0, 50, "active", now+30000)

	// B: a single limited subscription (paid)
	seedSubscriptionRaw(t, uidB, planPaid, 300, 100, "active", now+10000)

	// Noise: C has no subscriptions; D only has an expired one.
	_ = seedUserRaw(t, "mixc")
	uidD := seedUserRaw(t, "mixd")
	seedSubscriptionRaw(t, uidD, planPaid, 999, 1, "active", now-1)

	users := []*User{{Id: uidA}, {Id: uidB}, {Id: uidD}}
	FillUsersSubscriptionQuotaStats(users)

	a := users[0]
	// total counts limited subscriptions only: 1000+500=1500
	assert.EqualValues(t, 1500, a.SubscriptionTotalQuota)
	// remain = max(0,1000-200) + max(0,500-600) = 800 + 0 = 800
	assert.EqualValues(t, 800, a.SubscriptionRemainQuota)
	// used includes the unlimited subscription usage: 200+600+50=850
	assert.EqualValues(t, 850, a.SubscriptionUsedQuota)
	assert.True(t, a.SubscriptionUnlimited, "want unlimited (has an amount_total<=0 active sub)")
	// remain value: limited with remaining = 10 x 800/1000 = 8
	assert.InDelta(t, 8, a.SubscriptionRemainValue, 0.001)

	b := users[1]
	assert.EqualValues(t, 300, b.SubscriptionTotalQuota)
	assert.EqualValues(t, 200, b.SubscriptionRemainQuota)
	assert.EqualValues(t, 100, b.SubscriptionUsedQuota)
	assert.False(t, b.SubscriptionUnlimited)
	// B remain value = 10 x 200/300 = 6.6667
	assert.InDelta(t, 6.6667, b.SubscriptionRemainValue, 0.001)

	d := users[2]
	assert.EqualValues(t, 0, d.SubscriptionTotalQuota, "expired sub must not count")
	assert.EqualValues(t, 0, d.SubscriptionUsedQuota, "expired sub must not count")
}

// Empty list: no query, no panic.
func TestFillUsersSubscriptionQuotaStats_Empty(t *testing.T) {
	setupSubQuotaTestDB(t)

	FillUsersSubscriptionQuotaStats(nil)
	FillUsersSubscriptionQuotaStats([]*User{})
}

// When running against PostgreSQL the aggregation SQL must be valid. The
// * 1.0 division is dialect-portable, but SQLite unit tests cannot cover a
// live PG run, so verify once against a real PG when TEST_PG_DSN is provided.
func TestFillUsersSubscriptionQuotaStats_PostgresDialect(t *testing.T) {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN not set; skip PostgreSQL dialect check")
	}
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	previousDB := DB
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("open test pg: %v", err)
	}
	require.NoError(t, db.AutoMigrate(&User{}, &UserSubscription{}, &SubscriptionPlan{}), "migrate pg tables")
	DB = db
	t.Cleanup(func() { DB = previousDB })

	uid := seedUserRaw(t, fmt.Sprintf("pgd%d", time.Now().UnixNano()%1_000_000))
	plan := seedPlanRaw(t, fmt.Sprintf("pg-plan-%d", time.Now().UnixNano()), 10.0)
	seedSubscriptionRaw(t, uid, plan, 1000, 250, "active", time.Now().Unix()+10000)

	users := []*User{{Id: uid}}
	FillUsersSubscriptionQuotaStats(users)

	u := users[0]
	assert.EqualValues(t, 750, u.SubscriptionRemainQuota)
	// 10 x 750/1000 = 7.5
	assert.InDelta(t, 7.5, u.SubscriptionRemainValue, 0.001)
}
