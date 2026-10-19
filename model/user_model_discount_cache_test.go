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
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupUserModelDiscountTestDB installs an isolated in-memory SQLite database
// for the discount column tests and restores the global handles on cleanup.
func setupUserModelDiscountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&User{}))

	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createUserModelDiscountTestUser(t *testing.T, username string) *User {
	t.Helper()
	user := User{
		Username:    username,
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     "aff-" + username, // uniqueIndex — must be unique per user
	}
	require.NoError(t, DB.Create(&user).Error)
	return &user
}

func TestUserModelDiscountsColumnRoundTrip(t *testing.T) {
	setupUserModelDiscountTestDB(t)

	stored := createUserModelDiscountTestUser(t, "discount-stored")
	raw := `{"gpt-5":0.5,"claude-4":0.9}`
	require.NoError(t, DB.Model(&User{}).Where("id = ?", stored.Id).Update("model_discounts", raw).Error)
	discounts, err := GetUserModelDiscounts(stored.Id, true)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"gpt-5": 0.5, "claude-4": 0.9}, discounts)

	empty := createUserModelDiscountTestUser(t, "discount-empty")
	discounts, err = GetUserModelDiscounts(empty.Id, true)
	require.NoError(t, err)
	assert.Empty(t, discounts)

	// Rows that predate the column hold NULL rather than an empty string on all
	// three supported databases. An upgrade must read them as "no discounts"
	// instead of failing the lookup.
	legacy := createUserModelDiscountTestUser(t, "discount-legacy")
	require.NoError(t, DB.Exec("UPDATE users SET model_discounts = NULL WHERE id = ?", legacy.Id).Error)
	discounts, err = GetUserModelDiscounts(legacy.Id, true)
	require.NoError(t, err)
	assert.Empty(t, discounts)
}

func TestUserBaseGetModelDiscounts(t *testing.T) {
	base := &UserBase{ModelDiscounts: `{"m":0.5}`}
	assert.Equal(t, map[string]float64{"m": 0.5}, base.GetModelDiscounts())

	assert.Nil(t, (&UserBase{}).GetModelDiscounts())
}

func TestUserBaseWriteContextSetsModelDiscounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	(&UserBase{ModelDiscounts: `{"m":0.5}`}).WriteContext(c)
	discounts, ok := common.GetContextKeyType[map[string]float64](c, constant.ContextKeyUserModelDiscounts)
	require.True(t, ok)
	assert.Equal(t, map[string]float64{"m": 0.5}, discounts)

	// A user without discounts must still publish the key, so downstream
	// consumers never depend on its absence.
	emptyContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	(&UserBase{}).WriteContext(emptyContext)
	discounts, ok = common.GetContextKeyType[map[string]float64](emptyContext, constant.ContextKeyUserModelDiscounts)
	require.True(t, ok)
	assert.Empty(t, discounts)
}

func TestRefreshUserModelDiscountsCacheWithoutRedis(t *testing.T) {
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
	})

	require.NoError(t, RefreshUserModelDiscountsCache(1))
}

func TestToBaseUserCarriesModelDiscounts(t *testing.T) {
	user := User{Id: 1, ModelDiscounts: `{"m":0.5}`}
	assert.Equal(t, `{"m":0.5}`, user.ToBaseUser().ModelDiscounts)
}

func TestUserJSONDoesNotExposeModelDiscounts(t *testing.T) {
	raw, err := common.Marshal(User{Id: 7, Username: "discount-json", ModelDiscounts: `{"m":0.5}`})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"username":"discount-json"`)
	assert.NotContains(t, string(raw), "model_discounts")
}
