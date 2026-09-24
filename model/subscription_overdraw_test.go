package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newSubscriptionOverdrawTestDB 内存 SQLite 测试库（订阅表）
func newSubscriptionOverdrawTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })
	if commonGroupCol == "" {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "open test db")
	require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}, &UserSubscription{}), "migrate subscription tables")
	return db
}

// 超额结算应允许透支：补扣使 used 超过 total 时不报错，额度记为负余额（remain = total - used < 0），
// 由预扣校验（remain < amount）自然拦截后续请求。
func TestPostConsumeUserSubscriptionDeltaAllowsOverdraw(t *testing.T) {
	prev := DB
	defer func() { DB = prev }()
	DB = newSubscriptionOverdrawTestDB(t)

	sub := &UserSubscription{
		UserId:      1,
		PlanId:      1,
		AmountTotal: 1000,
		AmountUsed:  900,
		Status:      "active",
	}
	require.NoError(t, DB.Create(sub).Error, "create subscription")

	require.NoError(t, PostConsumeUserSubscriptionDelta(sub.Id, 500), "expect overdraw settle to succeed")

	var got UserSubscription
	require.NoError(t, DB.First(&got, sub.Id).Error, "reload subscription")
	assert.EqualValues(t, 1400, got.AmountUsed, "want amount_used=1400 (total 1000)")
}

// 退款（负 delta）仍不允许把 used 打到 0 以下 —— 下界 clamp 保持不变。
func TestPostConsumeUserSubscriptionDeltaClampsAtZero(t *testing.T) {
	prev := DB
	defer func() { DB = prev }()
	DB = newSubscriptionOverdrawTestDB(t)

	sub := &UserSubscription{
		UserId:      1,
		PlanId:      1,
		AmountTotal: 1000,
		AmountUsed:  100,
		Status:      "active",
	}
	require.NoError(t, DB.Create(sub).Error, "create subscription")

	require.NoError(t, PostConsumeUserSubscriptionDelta(sub.Id, -200), "expect refund settle to succeed")

	var got UserSubscription
	require.NoError(t, DB.First(&got, sub.Id).Error, "reload subscription")
	assert.EqualValues(t, 0, got.AmountUsed, "want amount_used clamped to 0")
}
