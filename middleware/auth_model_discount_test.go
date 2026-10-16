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
	"bytes"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupAuthModelDiscountTest installs an isolated in-memory SQLite database for
// the actor discount tests and restores the global handles on cleanup. The SQL
// logger is silenced so the intentional missing-user lookups stay out of the
// test output.
func setupAuthModelDiscountTest(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

func createAuthModelDiscountUser(t *testing.T, username string) *model.User {
	t.Helper()
	user := &model.User{
		Username: username, Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
		AffCode: "auth-discount-" + username,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func TestSetActorModelDiscountContextLoadsActorDiscounts(t *testing.T) {
	setupAuthModelDiscountTest(t)
	gin.SetMode(gin.TestMode)

	actor := createAuthModelDiscountUser(t, "discount-actor")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", actor.Id).
		Update("model_discounts", `{"m":0.4}`).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	setActorModelDiscountContext(c, actor.Id)

	discounts, ok := common.GetContextKeyType[map[string]float64](c, constant.ContextKeyUserModelDiscounts)
	require.True(t, ok)
	assert.Equal(t, map[string]float64{"m": 0.4}, discounts)
	assert.Equal(t, 0.4, model.LookupModelDiscount(discounts, "m"))
}

func TestSetActorModelDiscountContextEmptyWhenNoDiscounts(t *testing.T) {
	setupAuthModelDiscountTest(t)
	gin.SetMode(gin.TestMode)

	actor := createAuthModelDiscountUser(t, "discount-actor-empty")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	setActorModelDiscountContext(c, actor.Id)

	// A nil or empty table both mean "loaded, no discounts": the key must be
	// present so downstream consumers never fall back to a second lookup.
	discounts, ok := common.GetContextKeyType[map[string]float64](c, constant.ContextKeyUserModelDiscounts)
	require.True(t, ok)
	assert.Empty(t, discounts)
	assert.Equal(t, 1.0, model.LookupModelDiscount(discounts, "m"))
}

func TestSetActorModelDiscountContextFailsOpen(t *testing.T) {
	setupAuthModelDiscountTest(t)
	gin.SetMode(gin.TestMode)

	// The failure path logs one SysError line by design; capture it so the test
	// output stays clean.
	var errorLog bytes.Buffer
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &errorLog
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = previousWriter
		common.LogWriterMu.Unlock()
	})

	// A non-positive actor id never touches the database.
	zeroContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	setActorModelDiscountContext(zeroContext, 0)
	discounts, ok := common.GetContextKeyType[map[string]float64](zeroContext, constant.ContextKeyUserModelDiscounts)
	require.True(t, ok)
	assert.Empty(t, discounts)
	assert.Equal(t, 1.0, model.LookupModelDiscount(discounts, "m"))

	// A user that cannot be loaded fails open to full price with the key still
	// marked loaded.
	missingContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	setActorModelDiscountContext(missingContext, 999999)
	discounts, ok = common.GetContextKeyType[map[string]float64](missingContext, constant.ContextKeyUserModelDiscounts)
	require.True(t, ok)
	assert.Empty(t, discounts)
	assert.Equal(t, 1.0, model.LookupModelDiscount(discounts, "m"))
}
