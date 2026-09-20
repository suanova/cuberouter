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
	"context"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 保护契约：组织令牌经 Redis 缓存读回后作用域字段必须完整，否则会被当成个人令牌扣个人钱包。
func TestTokenScopeRoundTripsThroughRedisHashCache(t *testing.T) {
	useUserCacheMiniRedis(t)
	token := Token{
		Id:                   42,
		UserId:               7,
		Key:                  "token-scope-cache-key",
		Name:                 "org-key",
		ScopeType:            TokenScopeOrganization,
		ScopeId:              3,
		Visibility:           TokenVisibilityPublic,
		OrganizationId:       3,
		CreatorUserId:        7,
		ResponsibleUserId:    8,
		TransferReason:       "member-exit",
		DisabledBySystems:    true,
		SystemDisabledReason: "organization_disabled",
		SystemDisabledRefId:  3,
		SystemDisabledAt:     1700000000,
		PreviousStatus:       common.TokenStatusEnabled,
	}

	require.NoError(t, cacheSetTokenForTest(token))
	cached, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.Equal(t, TokenScopeOrganization, cached.ScopeType)
	assert.Equal(t, 3, cached.ScopeId)
	assert.Equal(t, TokenVisibilityPublic, cached.Visibility)
	assert.Equal(t, 3, cached.OrganizationId)
	assert.Equal(t, 7, cached.CreatorUserId)
	assert.Equal(t, 8, cached.ResponsibleUserId)
	assert.Equal(t, "member-exit", cached.TransferReason)
	assert.True(t, cached.DisabledBySystems)
	assert.Equal(t, "organization_disabled", cached.SystemDisabledReason)
	assert.Equal(t, 3, cached.SystemDisabledRefId)
	assert.EqualValues(t, 1700000000, cached.SystemDisabledAt)
	assert.Equal(t, common.TokenStatusEnabled, cached.PreviousStatus)
}

// 保护契约：组织作用域上线之前写入的哈希没有作用域字段，读出来是零值。宁可回落数据库，
// 也不能把组织令牌当成个人令牌。
func TestTokenCacheWithoutScopeVersionIsRejected(t *testing.T) {
	useUserCacheMiniRedis(t)
	const key = "token-pre-scope-cache-key"
	require.NoError(t, common.RDB.HSet(context.Background(), getTokenCacheKey(key), map[string]any{
		"Id":        strconv.Itoa(42),
		"UserId":    strconv.Itoa(7),
		"Status":    strconv.Itoa(common.TokenStatusEnabled),
		"ScopeType": "",
	}).Err())

	_, err := cacheGetTokenByKey(key)
	require.Error(t, err, "a pre-scope cache snapshot must not be served")
	assert.Contains(t, err.Error(), "predates the scope schema")
}
