package common

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type verificationValue struct {
	code    string
	time    time.Time
	claimed bool
}

const (
	EmailVerificationPurpose = "v"
	PasswordResetPurpose     = "r"
)

var verificationMutex sync.Mutex
var verificationMap map[string]verificationValue
var verificationMapMaxSize = 10
var VerificationValidMinutes = 10

// Verification codes are read by a different process than the one that wrote
// them whenever more than one instance serves traffic, so with Redis configured
// they live there instead of in verificationMap. Expiry is the key's TTL, and
// the value is a hash so claiming a code does not reset that TTL.
func verificationRedisKey(purpose string, key string) string {
	return "verification:" + purpose + ":" + key
}

func verificationTTL() time.Duration {
	return time.Duration(VerificationValidMinutes) * time.Minute
}

// RedisEnabled defaults to true and only InitRedisClient may clear it, so the
// flag alone does not mean a client exists. Both must hold before the shared
// store can be used, otherwise a process that has not initialised Redis yet
// would take the Redis branch with a nil client.
func verificationUsesRedis() bool {
	return RedisEnabled && RDB != nil
}

// watchVerificationValue runs fn under a WATCH on the key and retries when a
// concurrent writer invalidates the transaction, which is how the claim and
// release transitions stay atomic across instances.
func watchVerificationValue(fullKey string, fn func(tx *redis.Tx, values map[string]string) error) error {
	ctx := context.Background()
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = RDB.Watch(ctx, func(tx *redis.Tx) error {
			values, err := tx.HGetAll(ctx, fullKey).Result()
			if err != nil {
				return err
			}
			return fn(tx, values)
		}, fullKey)
		if !errors.Is(err, redis.TxFailedErr) {
			return err
		}
	}
	return err
}

// setClaimed rewrites only the claimed field so the key keeps its remaining TTL.
func setClaimed(tx *redis.Tx, fullKey string, claimed bool) error {
	claimedValue := "0"
	if claimed {
		claimedValue = "1"
	}
	ctx := context.Background()
	_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, fullKey, "claimed", claimedValue)
		return nil
	})
	return err
}

func GenerateVerificationCode(length int) string {
	code := uuid.New().String()
	code = strings.Replace(code, "-", "", -1)
	if length == 0 {
		return code
	}
	return code[:length]
}

// RegisterVerificationCodeWithKey stores a code and reports whether it reached
// the store. A code that was never stored can never be accepted by any instance,
// so callers that are not hiding account existence must surface this instead of
// reporting a successful send.
func RegisterVerificationCodeWithKey(key string, code string, purpose string) error {
	if verificationUsesRedis() {
		return registerVerificationCodeInRedis(verificationRedisKey(purpose, key), code)
	}
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap[purpose+key] = verificationValue{
		code: code,
		time: time.Now(),
	}
	if len(verificationMap) > verificationMapMaxSize {
		removeExpiredPairs()
	}
	return nil
}

func registerVerificationCodeInRedis(fullKey string, code string) error {
	ctx := context.Background()
	_, err := RDB.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, fullKey, "code", code, "claimed", "0")
		pipe.Expire(ctx, fullKey, verificationTTL())
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to store verification code in redis: %w", err)
	}
	return nil
}

func VerifyCodeWithKey(key string, code string, purpose string) bool {
	if verificationUsesRedis() {
		return verifyCodeInRedis(verificationRedisKey(purpose, key), code)
	}
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	value, okay := verificationMap[purpose+key]
	now := time.Now()
	if !okay || int(now.Sub(value.time).Seconds()) >= VerificationValidMinutes*60 {
		return false
	}
	if value.claimed {
		return false
	}
	return code == value.code
}

func verifyCodeInRedis(fullKey string, code string) bool {
	ctx := context.Background()
	values, err := RDB.HGetAll(ctx, fullKey).Result()
	if err != nil {
		SysError("failed to read verification code from redis: " + err.Error())
		return false
	}
	// An empty hash means the key is missing or its TTL has elapsed.
	if len(values) == 0 || values["claimed"] == "1" {
		return false
	}
	return code == values["code"]
}

// ClaimVerificationCodeWithKey atomically verifies a code and reserves it for
// the caller. Exactly one concurrent caller wins: the first to lock sees an
// unclaimed, valid code and marks it claimed; every later caller — including a
// second request racing while the first is still mid-flight — gets false. This
// is the guard for password reset, where a plain verify-then-act would let two
// concurrent requests both pass and deliver different generated passwords. The
// winner releases the claim with ReleaseVerificationCodeClaim when the follow-up
// work fails, or consumes it permanently with DeleteKey once it commits.
func ClaimVerificationCodeWithKey(key string, code string, purpose string) bool {
	if verificationUsesRedis() {
		return claimVerificationCodeInRedis(verificationRedisKey(purpose, key), code)
	}
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	value, okay := verificationMap[purpose+key]
	now := time.Now()
	if !okay || int(now.Sub(value.time).Seconds()) >= VerificationValidMinutes*60 {
		return false
	}
	if value.claimed || code != value.code {
		return false
	}
	value.claimed = true
	verificationMap[purpose+key] = value
	return true
}

func claimVerificationCodeInRedis(fullKey string, code string) bool {
	claimed := false
	err := watchVerificationValue(fullKey, func(tx *redis.Tx, values map[string]string) error {
		if len(values) == 0 || values["claimed"] == "1" || values["code"] != code {
			return nil
		}
		if err := setClaimed(tx, fullKey, true); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	if err != nil {
		SysError("failed to claim verification code in redis: " + err.Error())
		return false
	}
	return claimed
}

// ReleaseVerificationCodeClaim reverses a prior claim so the same token becomes
// usable again without deleting it. It is a no-op unless the stored code still
// matches the claimed one: if the token was re-issued (rotated) while the claim
// was in flight, the new token must not be marked claimed.
func ReleaseVerificationCodeClaim(key string, code string, purpose string) {
	if verificationUsesRedis() {
		releaseVerificationCodeClaimInRedis(verificationRedisKey(purpose, key), code)
		return
	}
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	value, okay := verificationMap[purpose+key]
	if !okay || value.code != code {
		return
	}
	value.claimed = false
	verificationMap[purpose+key] = value
}

func releaseVerificationCodeClaimInRedis(fullKey string, code string) {
	err := watchVerificationValue(fullKey, func(tx *redis.Tx, values map[string]string) error {
		if len(values) == 0 || values["code"] != code {
			return nil
		}
		return setClaimed(tx, fullKey, false)
	})
	if err != nil {
		SysError("failed to release verification code claim in redis: " + err.Error())
	}
}

func DeleteKey(key string, purpose string) {
	if verificationUsesRedis() {
		if err := RDB.Del(context.Background(), verificationRedisKey(purpose, key)).Err(); err != nil {
			SysError("failed to delete verification code in redis: " + err.Error())
		}
		return
	}
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	delete(verificationMap, purpose+key)
}

// no lock inside, so the caller must lock the verificationMap before calling!
func removeExpiredPairs() {
	now := time.Now()
	for key := range verificationMap {
		if int(now.Sub(verificationMap[key].time).Seconds()) >= VerificationValidMinutes*60 {
			delete(verificationMap, key)
		}
	}
}

func init() {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap = make(map[string]verificationValue)
}
