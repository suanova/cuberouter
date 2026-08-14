package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "open test db")
	require.NoError(t, db.AutoMigrate(&Model{}), "migrate model")
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
	require.NoError(t, m.Insert())

	now := time.Now().Unix()
	require.NoError(t, m.UpdateMetaFields(map[string]interface{}{
		"description":  "new intro",
		"updated_time": now,
	}))

	var got Model
	require.NoError(t, DB.Where("id = ?", m.Id).First(&got).Error)
	require.Equal(t, "new intro", got.Description)
	require.Equal(t, 1, got.Status, "status must not be mutated")
	require.Equal(t, 1, got.SyncOfficial, "sync_official must not be mutated")
	require.Equal(t, 5, got.VendorID, "vendor_id must not be mutated")
	require.Equal(t, "chat", got.Endpoints, "endpoints must not be mutated")
	require.Equal(t, NameRuleExact, got.NameRule, "name_rule must not be mutated")
}

// TestGetModelByName_BasicAndNotFound 验证按名查询命中与 not-found 返回 gorm.ErrRecordNotFound。
func TestGetModelByName_BasicAndNotFound(t *testing.T) {
	prev := DB
	defer func() { DB = prev }()
	DB = newModelTestDB(t)

	m := &Model{ModelName: "claude-3-haiku", Status: 1}
	require.NoError(t, m.Insert())

	got, err := GetModelByName("claude-3-haiku")
	require.NoError(t, err)
	require.Equal(t, m.Id, got.Id)

	_, err = GetModelByName("does-not-exist")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// TestGetModelByName_SoftDeleteFiltered 验证软删除记录不被返回。
func TestGetModelByName_SoftDeleteFiltered(t *testing.T) {
	prev := DB
	defer func() { DB = prev }()
	DB = newModelTestDB(t)

	m := &Model{ModelName: "gemini-2.5-pro-softdel", Status: 1}
	require.NoError(t, m.Insert())
	require.NoError(t, DB.Where("id = ?", m.Id).Delete(&Model{}).Error)

	_, err := GetModelByName("gemini-2.5-pro-softdel")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "soft-deleted model must not be returned")
}
