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
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the wire shape of the user hash for the model discount table:
// the Lua field name must match UserBase.ModelDiscounts (common.RedisHGetObj
// matches on the Go field name), and the script's argument list must keep TTL on
// ARGV[12] with ModelDiscounts appended last.

func TestUserModelDiscountsRedisCacheWriteShape(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)

	user := createUserModelDiscountTestUser(t, "discount-redis-write")
	raw := `{"m":0.5}`
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("model_discounts", raw).Error)
	user.ModelDiscounts = raw
	require.NoError(t, populateUserCache(*user))

	field, err := common.RDB.HGet(t.Context(), getUserCacheKey(user.Id), "ModelDiscounts").Result()
	require.NoError(t, err)
	assert.Equal(t, raw, field)

	schema, err := common.RDB.HGet(t.Context(), getUserCacheKey(user.Id), "CacheSchema").Result()
	require.NoError(t, err)
	assert.Equal(t, "3", schema)

	// EXPIRE must still read the TTL argument after the new field was appended.
	ttl, err := common.RDB.TTL(t.Context(), getUserCacheKey(user.Id)).Result()
	require.NoError(t, err)
	assert.Greater(t, int(ttl.Seconds()), 0)
	quota, err := common.RDB.HGet(t.Context(), getUserCacheKey(user.Id), "Quota").Result()
	require.NoError(t, err)
	assert.Equal(t, "0", quota)

	cached, err := cacheGetUserBase(user.Id)
	require.NoError(t, err)
	assert.Equal(t, raw, cached.ModelDiscounts)
	assert.Equal(t, map[string]float64{"m": 0.5}, cached.GetModelDiscounts())
}

func TestRefreshUserModelDiscountsCacheUpdatesHashField(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)

	user := createUserModelDiscountTestUser(t, "discount-redis-refresh")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("model_discounts", `{"m":0.5}`).Error)
	user.ModelDiscounts = `{"m":0.5}`
	require.NoError(t, populateUserCache(*user))

	// The cache-first read serves the hash value, not the database value.
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("model_discounts", `{"m":0.25}`).Error)
	require.NoError(t, updateUserCacheFieldAtVersion(user.Id, "ModelDiscounts", `{"m":0.75}`, user.AuthVersion))
	read, err := GetUserModelDiscounts(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"m": 0.75}, read)

	require.NoError(t, RefreshUserModelDiscountsCache(user.Id))
	field, err := common.RDB.HGet(t.Context(), getUserCacheKey(user.Id), "ModelDiscounts").Result()
	require.NoError(t, err)
	assert.Equal(t, `{"m":0.25}`, field)
	read, err = GetUserModelDiscounts(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"m": 0.25}, read)
}

func TestUserModelDiscountsCacheStaleSchemaFallsBackToDB(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)

	user := createUserModelDiscountTestUser(t, "discount-redis-stale")
	raw := `{"m":0.5}`
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("model_discounts", raw).Error)
	user.ModelDiscounts = raw
	require.NoError(t, populateUserCache(*user))

	// An entry written by the previous schema version is rejected as stale.
	require.NoError(t, common.RDB.HSet(t.Context(), getUserCacheKey(user.Id), "CacheSchema", 2).Err())
	require.NoError(t, common.RDB.HDel(t.Context(), getUserCacheKey(user.Id), "ModelDiscounts").Err())
	_, err := cacheGetUserBase(user.Id)
	require.ErrorContains(t, err, "stale")

	// The read path falls back to the database and refills the hash.
	read, err := GetUserModelDiscounts(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"m": 0.5}, read)

	WaitForQuotaCacheWorkers()
	schema, err := common.RDB.HGet(t.Context(), getUserCacheKey(user.Id), "CacheSchema").Result()
	require.NoError(t, err)
	assert.Equal(t, "3", schema)
	field, err := common.RDB.HGet(t.Context(), getUserCacheKey(user.Id), "ModelDiscounts").Result()
	require.NoError(t, err)
	assert.Equal(t, raw, field)
}
