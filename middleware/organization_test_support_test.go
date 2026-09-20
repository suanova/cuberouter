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
package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// snapshotMiddlewareTestGlobals 记录包级全局，并在测试结束时还原。
// 这些测试会替换 model.DB / LOG_DB / SQLitePath 等进程级变量，同一个测试
// 二进制里的其他用例共用它们，不还原就会互相污染。
func snapshotMiddlewareTestGlobals(t *testing.T) {
	t.Helper()
	db, logDB := model.DB, model.LOG_DB
	mainType, logType := common.MainDatabaseType(), common.LogDatabaseType()
	redisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	t.Cleanup(func() {
		if db != nil {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB, model.LOG_DB = db, logDB
		common.SetDatabaseTypes(mainType, logType)
		common.RedisEnabled = redisEnabled
		common.SQLitePath = originalSQLitePath
		if hadSQLDSN {
			_ = os.Setenv("SQL_DSN", originalSQLDSN)
		} else {
			_ = os.Unsetenv("SQL_DSN")
		}
	})
}

// setupOrganizationMiddlewareTestDB 打开一个测试专属的进程内 SQLite，并把给定模型建表。
//
// 这里走 model.InitDB() 而不是自己 gorm.Open：InitDB 会顺带执行 initCol()，
// 后者填的是 `key` / `group` 这两个保留字的引用格式。跳过它的话，
// GetTokenByKey 会生成 `WHERE  = ?` 这种列名为空的坏 SQL，令牌查询直接 500。
func setupOrganizationMiddlewareTestDB(t *testing.T, models ...any) {
	t.Helper()
	snapshotMiddlewareTestGlobals(t)
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(30000)", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(models...))
}

// translated 返回某个 i18n key 在当前进程里的实际渲染结果。
//
// middleware 包内没有任何测试调用 i18n.Init()（那是个 sync.Once，会永久替换
// common.TranslateMessage），所以这里拿到的是原始 key；用这个辅助函数断言而不是
// 硬编码字符串，将来有人在该包里初始化 i18n 也不会让测试变脆。
func translated(key string) string {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	// TranslateMessage 可能是内置桩，也可能已被包内某个测试初始化过的 i18n.T 取代；
	// 后者会读 Accept-Language，所以必须挂一个真实的 *http.Request。
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return common.TranslateMessage(ctx, key)
}
