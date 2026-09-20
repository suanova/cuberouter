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
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"

	"gorm.io/gorm"
)

// IsDuplicateKeyError 判断 err 是否是一次唯一键冲突，不限具体是哪个约束。
// 日志落库用它做"幂等键已存在即视为成功"的判断。
func IsDuplicateKeyError(db *gorm.DB, err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	if db != nil {
		if translator, ok := db.Dialector.(gorm.ErrorTranslator); ok && errors.Is(translator.Translate(err), gorm.ErrDuplicatedKey) {
			return true
		}
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// IsOrganizationNameDuplicateError 判断 err 是否来自 organizations.name_normalized
// 上的唯一索引冲突。
//
// 这里不能像其它唯一键那样只看 "是不是重复键错误"：组织创建会同时写 name_normalized
// 和 slug 两个唯一列，而只有前者代表"组织重名"。冲突可能是 slug 撞了（重试即可），
// 也可能是名字撞了（要报 409 organization_name_conflict），所以必须按约束名区分，
// 三个驱动各自暴露约束名的方式都不一样。
func IsOrganizationNameDuplicateError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		_, key, found := strings.Cut(mysqlErr.Message, " for key ")
		if !found {
			return false
		}
		key = strings.Trim(strings.TrimSpace(key), "'`\"")
		if separator := strings.LastIndexByte(key, '.'); separator >= 0 {
			key = key[separator+1:]
		}
		return key == organizationNameNormalizedIndex
	}

	var postgresErr *pgconn.PgError
	if errors.As(err, &postgresErr) {
		return postgresErr.Code == "23505" && postgresErr.ConstraintName == organizationNameNormalizedIndex
	}

	var sqliteErr interface {
		error
		Code() int
	}
	return errors.As(err, &sqliteErr) &&
		sqliteErr.Code() == 2067 &&
		strings.Contains(sqliteErr.Error(), "UNIQUE constraint failed: organizations.name_normalized")
}
