package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockForUpdate makes the next query emit SELECT ... FOR UPDATE so the matched
// rows stay locked until the surrounding transaction ends.
//
// GORM v2 silently ignores the legacy `Set("gorm:query_option", "FOR UPDATE")`
// from GORM v1, so that form does not lock anything. Always use this helper
// instead.
//
// SQLite has no FOR UPDATE syntax (the clause would be a syntax error), so it
// is skipped there; SQLite's single-writer model makes one of two conflicting
// transactions fail instead of both committing.
func lockForUpdate(tx *gorm.DB) *gorm.DB {
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

// LockForUpdate 是 lockForUpdate 的导出形式，供 model 包外的代码使用
// （组织服务的事务都在 service 包里）。
//
// 直接写 clause.Locking{Strength: "UPDATE"} 会在 SQLite 上生成 FOR UPDATE，
// 那是语法错误；这里复用同一份判断，避免调用方漏掉 SQLite 分支。
func LockForUpdate(tx *gorm.DB) *gorm.DB {
	return lockForUpdate(tx)
}
