package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// newModelTestDB 构造测试用的 in-memory SQLite + 已迁移的 Model 表。
// 仿照 user_created_at_test.go 中的 newTestDB，但 AutoMigrate Model 而非 User。
func newModelTestDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&Model{}); err != nil {
		t.Fatalf("migrate model: %v", err)
	}
	return db
}

// TestUpdateMetaFields_PreservesStatusVendorID 验证 UpdateMetaFields 只更新传入字段，
// 不会意外覆盖 status / sync_official / endpoints / name_rule / vendor_id 等敏感列。
func TestUpdateMetaFields_PreservesStatusVendorID(t *testing.T) {
	prev := DB
	defer func() { DB = prev }()
	DB = newModelTestDB(t)

	m := &Model{
		ModelName:    "gpt-4o-test",
		Description:  "old intro",
		VendorID:     5,
		Endpoints:    "chat",
		Status:       1,
		SyncOfficial: 1,
		NameRule:     NameRuleExact,
	}
	if err := m.Insert(); err != nil {
		t.Fatalf("insert: %v", err)
	}

	now := time.Now().Unix()
	if err := m.UpdateMetaFields(map[string]interface{}{
		"description":  "new intro",
		"updated_time": now,
	}); err != nil {
		t.Fatalf("UpdateMetaFields: %v", err)
	}

	var got Model
	if err := DB.Where("id = ?", m.Id).First(&got).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if got.Description != "new intro" {
		t.Errorf("description: got %q, want %q", got.Description, "new intro")
	}
	if got.Status != 1 {
		t.Errorf("status mutated: got %d, want 1", got.Status)
	}
	if got.SyncOfficial != 1 {
		t.Errorf("sync_official mutated: got %d, want 1", got.SyncOfficial)
	}
	if got.VendorID != 5 {
		t.Errorf("vendor_id mutated: got %d, want 5", got.VendorID)
	}
	if got.Endpoints != "chat" {
		t.Errorf("endpoints mutated: got %q, want %q", got.Endpoints, "chat")
	}
	if got.NameRule != NameRuleExact {
		t.Errorf("name_rule mutated: got %d, want %d", got.NameRule, NameRuleExact)
	}
}

// TestGetModelByName_BasicAndNotFound 验证按名查询命中与 not-found 返回 gorm.ErrRecordNotFound。
func TestGetModelByName_BasicAndNotFound(t *testing.T) {
	prev := DB
	defer func() { DB = prev }()
	DB = newModelTestDB(t)

	m := &Model{ModelName: "claude-3-haiku", Status: 1}
	if err := m.Insert(); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := GetModelByName("claude-3-haiku")
	if err != nil {
		t.Fatalf("GetModelByName hit: %v", err)
	}
	if got.Id != m.Id {
		t.Errorf("returned id: got %d, want %d", got.Id, m.Id)
	}

	if _, err := GetModelByName("does-not-exist"); err != gorm.ErrRecordNotFound {
		t.Errorf("not-found err: got %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

// TestGetModelByName_SoftDeleteFiltered 验证软删除记录不被返回。
func TestGetModelByName_SoftDeleteFiltered(t *testing.T) {
	prev := DB
	defer func() { DB = prev }()
	DB = newModelTestDB(t)

	m := &Model{ModelName: "gemini-2.5-pro-softdel", Status: 1}
	if err := m.Insert(); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := DB.Where("id = ?", m.Id).Delete(&Model{}).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := GetModelByName("gemini-2.5-pro-softdel"); err != gorm.ErrRecordNotFound {
		t.Errorf("soft-deleted should not be returned: got err=%v, want %v", err, gorm.ErrRecordNotFound)
	}
}
