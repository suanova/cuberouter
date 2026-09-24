package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newSubscriptionOverdrawTestDB 内存 SQLite 测试库（订阅表）
func newSubscriptionOverdrawTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	common.RedisEnabled = false
	if commonGroupCol == "" {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&SubscriptionPlan{}, &UserSubscription{}); err != nil {
		t.Fatalf("migrate subscription tables: %v", err)
	}
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
	if err := DB.Create(sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	if err := PostConsumeUserSubscriptionDelta(sub.Id, 500); err != nil {
		t.Fatalf("expect overdraw settle to succeed, got error: %v", err)
	}

	var got UserSubscription
	if err := DB.First(&got, sub.Id).Error; err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	if got.AmountUsed != 1400 {
		t.Fatalf("expect amount_used=1400 (total 1000), got %d", got.AmountUsed)
	}
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
	if err := DB.Create(sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	if err := PostConsumeUserSubscriptionDelta(sub.Id, -200); err != nil {
		t.Fatalf("expect refund settle to succeed, got error: %v", err)
	}

	var got UserSubscription
	if err := DB.First(&got, sub.Id).Error; err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	if got.AmountUsed != 0 {
		t.Fatalf("expect amount_used clamped to 0, got %d", got.AmountUsed)
	}
}
