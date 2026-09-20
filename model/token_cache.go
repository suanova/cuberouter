package model

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
)

func getTokenCacheKey(key string) string {
	return fmt.Sprintf("token:%s", common.GenerateHMAC(key))
}

func getTokenCacheFenceKey(key string) string {
	return fmt.Sprintf("token:fence:%s", common.GenerateHMAC(key))
}

func tokenCacheTTLSeconds() int {
	ttl := common.RedisKeyCacheSeconds()
	if ttl <= 0 {
		return 60
	}
	return ttl
}

// tokenCacheFenceSeconds must outlive a token mutation's database write plus
// any in-flight reader's DB-read-to-cache-init gap. The fence is not deleted
// after commit; it expires naturally so a reader holding a pre-mutation
// snapshot cannot publish it right after the mutation cleared the cache.
// While the fence exists readers simply serve the database without caching.
const tokenCacheFenceSeconds = 10

// invalidateTokenCacheForMutation is called before a token metadata mutation
// writes to the database: it raises the fence and drops the cached hash so no
// reader can act on (or re-publish) the pre-mutation state.
func invalidateTokenCacheForMutation(key string) error {
	if !common.RedisEnabled || key == "" {
		return nil
	}
	ctx := context.Background()
	err := common.RDB.Set(ctx, getTokenCacheFenceKey(key), 1, time.Duration(tokenCacheFenceSeconds)*time.Second).Err()
	if err != nil {
		return err
	}
	return common.RDB.Del(ctx, getTokenCacheKey(key)).Err()
}

// tokenCacheScopeVersion 是缓存哈希里组织作用域字段的模式版本。版本之前的哈希没有这些字段，
// 读出来是零值（组织令牌会被当成个人令牌，进而错误地扣个人钱包），因此必须拒绝。
// InvalidateTokenCache 让令牌缓存立即失效，下一次读取回落到数据库。
//
// 组织封禁状态变化后必须调用：缓存里存的是封禁前的快照，不失效就会继续放行。
// 这里复用写路径的 fence 机制而不是直接删 key——删完仍可能有读者把旧快照写回去。
func InvalidateTokenCache(key string) error {
	return invalidateTokenCacheForMutation(key)
}

const tokenCacheScopeVersion = 1

// cacheInitToken publishes a database snapshot only when no mutation fence is
// active and the hash is cold. An existing hash only gets its TTL refreshed:
// its RemainQuota may already be ahead of this snapshot because atomic
// pre-consume decrements Redis first, so a snapshot must never overwrite any
// field of a live hash.
// 返回值：0=被 fence 拦截，1=完成初始化，2=哈希已存在，仅刷新 TTL。
func cacheInitToken(token Token) (int, error) {
	if !common.RedisEnabled {
		return 0, nil
	}
	allowIps := ""
	if token.AllowIps != nil {
		allowIps = *token.AllowIps
	}
	// 哈希字段名必须与 Token 的 Go 字段名一致：RedisHGetObj 是按字段名回填的。
	const script = `
if redis.call('EXISTS', KEYS[2]) == 1 then
  return 0
end
if redis.call('EXISTS', KEYS[1]) == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[30])
  return 2
end
redis.call('HSET', KEYS[1],
  'Id', ARGV[1], 'UserId', ARGV[2], 'Status', ARGV[3], 'Name', ARGV[4],
  'CreatedTime', ARGV[5], 'AccessedTime', ARGV[6], 'ExpiredTime', ARGV[7],
  'UnlimitedQuota', ARGV[8], 'ModelLimitsEnabled', ARGV[9], 'ModelLimits', ARGV[10],
  'AllowIps', ARGV[11], 'Group', ARGV[12], 'CrossGroupRetry', ARGV[13],
  'AutoGroups', ARGV[14], 'RemainQuota', ARGV[15], 'UsedQuota', ARGV[16],
  'ScopeType', ARGV[17], 'ScopeId', ARGV[18], 'Visibility', ARGV[19],
  'OrganizationId', ARGV[20], 'CreatorUserId', ARGV[21], 'ResponsibleUserId', ARGV[22],
  'TransferReason', ARGV[23], 'DisabledBySystems', ARGV[24], 'SystemDisabledReason', ARGV[25],
  'SystemDisabledRefId', ARGV[26], 'SystemDisabledAt', ARGV[27], 'PreviousStatus', ARGV[28],
  'CacheScopeVersion', ARGV[29])
redis.call('EXPIRE', KEYS[1], ARGV[30])
return 1`

	return common.RDB.Eval(context.Background(), script, []string{
		getTokenCacheKey(token.Key), getTokenCacheFenceKey(token.Key),
	},
		token.Id, token.UserId, token.Status, token.Name,
		token.CreatedTime, token.AccessedTime, token.ExpiredTime,
		strconv.FormatBool(token.UnlimitedQuota), strconv.FormatBool(token.ModelLimitsEnabled),
		token.ModelLimits, allowIps, token.Group, strconv.FormatBool(token.CrossGroupRetry),
		token.AutoGroups, token.RemainQuota, token.UsedQuota,
		token.ScopeType, token.ScopeId, token.Visibility,
		token.OrganizationId, token.CreatorUserId, token.ResponsibleUserId,
		token.TransferReason, strconv.FormatBool(token.DisabledBySystems), token.SystemDisabledReason,
		token.SystemDisabledRefId, token.SystemDisabledAt, token.PreviousStatus,
		tokenCacheScopeVersion,
		tokenCacheTTLSeconds(),
	).Int()
}

// cacheGetTokenByKey 从缓存读取 token；不完整的哈希（如仅有配额字段）会被拒绝。
func cacheGetTokenByKey(key string) (*Token, error) {
	if !common.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	var token Token
	if err := common.RedisHGetObj(getTokenCacheKey(key), &token); err != nil {
		return nil, err
	}
	if token.Id <= 0 {
		return nil, fmt.Errorf("token cache is incomplete")
	}
	// 旧快照只能被丢弃，不能就地改写：cacheInitToken 对已存在的哈希只刷 TTL，
	// 不会覆盖字段。这里返回错误让调用方回落到数据库读，等哈希自然过期后重建。
	if token.CacheScopeVersion < tokenCacheScopeVersion {
		return nil, fmt.Errorf("token cache predates the scope schema, version %d", token.CacheScopeVersion)
	}
	token.Key = key
	return &token, nil
}
