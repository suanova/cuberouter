package common

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useVerificationRedis points the verification-code store at an in-process
// Redis, mirroring the rate limiter's test fixture. The returned server lets a
// test advance time past the code TTL.
func useVerificationRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	previousRedisEnabled := RedisEnabled
	previousRedisClient := RDB
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, client.Ping(context.Background()).Err())

	RedisEnabled = true
	RDB = client
	t.Cleanup(func() {
		_ = client.Close()
		RedisEnabled = previousRedisEnabled
		RDB = previousRedisClient
	})

	return server
}

// verificationStorageKey mirrors the namespace the store must use so codes for
// different purposes cannot collide in a Redis shared with other features.
func verificationStorageKey(purpose, key string) string {
	return "verification:" + purpose + ":" + key
}

// TestVerificationCodeReachesSharedRedis is the multi-replica contract: because
// a code is registered by one instance and verified by whichever instance
// serves the next request, it must live in the shared Redis rather than in the
// registering process. A second client stands in for the sibling node, which is
// the only way to observe this from inside a single process.
func TestVerificationCodeReachesSharedRedis(t *testing.T) {
	useVerificationRedis(t)
	email := "multi-node@test.com"
	code := GenerateVerificationCode(6)
	RegisterVerificationCodeWithKey(email, code, EmailVerificationPurpose)
	t.Cleanup(func() { DeleteKey(email, EmailVerificationPurpose) })

	siblingNode := redis.NewClient(&redis.Options{Addr: RDB.Options().Addr})
	t.Cleanup(func() { _ = siblingNode.Close() })

	stored, err := siblingNode.HGet(context.Background(), verificationStorageKey(EmailVerificationPurpose, email), "code").Result()
	require.NoError(t, err, "验证码必须写在共享 Redis 上，否则多副本下另一节点看不到它")
	assert.Equal(t, code, stored, "共享 Redis 里存的必须就是这个验证码")
	assert.Equal(t, "0", siblingNode.HGet(context.Background(), verificationStorageKey(EmailVerificationPurpose, email), "claimed").Val())
}

// TestVerificationCodeExpiresInRedis pins expiry to the key's TTL, so an
// instance that never saw the registering request still rejects a stale code.
func TestVerificationCodeExpiresInRedis(t *testing.T) {
	server := useVerificationRedis(t)
	email := "ttl@test.com"
	code := GenerateVerificationCode(6)
	RegisterVerificationCodeWithKey(email, code, EmailVerificationPurpose)
	t.Cleanup(func() { DeleteKey(email, EmailVerificationPurpose) })

	require.True(t, VerifyCodeWithKey(email, code, EmailVerificationPurpose))
	server.FastForward(time.Duration(VerificationValidMinutes)*time.Minute + time.Second)
	assert.False(t, VerifyCodeWithKey(email, code, EmailVerificationPurpose), "TTL 过后验证码必须失效")
}

// verificationBackends returns the two stores the code must behave identically
// on: the in-process map (Redis unconfigured) and the shared Redis.
func verificationBackends(t *testing.T) map[string]func(*testing.T) {
	t.Helper()
	return map[string]func(*testing.T){
		"memory": func(t *testing.T) {
			previousRedisEnabled := RedisEnabled
			previousRedisClient := RDB
			RedisEnabled = false
			t.Cleanup(func() {
				RedisEnabled = previousRedisEnabled
				RDB = previousRedisClient
			})
		},
		"redis": func(t *testing.T) { useVerificationRedis(t) },
	}
}

// TestRegisterVerificationCodeWithKeyReportsStorageFailure keeps the send
// endpoint honest: if the code never reaches the shared store, no instance will
// ever accept it, so the caller must not report a successful send.
func TestRegisterVerificationCodeWithKeyReportsStorageFailure(t *testing.T) {
	server := useVerificationRedis(t)
	email := "storage-failure@test.com"

	require.NoError(t, RegisterVerificationCodeWithKey(email, "abc123", EmailVerificationPurpose),
		"写入成功时不得报错")

	server.Close()
	require.Error(t, RegisterVerificationCodeWithKey(email, "abc123", EmailVerificationPurpose),
		"验证码存不进去时必须报错，否则发码接口会谎报成功")
}
