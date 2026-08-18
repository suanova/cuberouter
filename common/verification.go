package common

import (
	"strings"
	"sync"
	"time"

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

func GenerateVerificationCode(length int) string {
	code := uuid.New().String()
	code = strings.Replace(code, "-", "", -1)
	if length == 0 {
		return code
	}
	return code[:length]
}

func RegisterVerificationCodeWithKey(key string, code string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap[purpose+key] = verificationValue{
		code: code,
		time: time.Now(),
	}
	if len(verificationMap) > verificationMapMaxSize {
		removeExpiredPairs()
	}
}

func VerifyCodeWithKey(key string, code string, purpose string) bool {
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

// ClaimVerificationCodeWithKey atomically verifies a code and reserves it for
// the caller. Exactly one concurrent caller wins: the first to lock sees an
// unclaimed, valid code and marks it claimed; every later caller — including a
// second request racing while the first is still mid-flight — gets false. This
// is the guard for password reset, where a plain verify-then-act would let two
// concurrent requests both pass and deliver different generated passwords. The
// winner releases the claim with ReleaseVerificationCodeClaim when the follow-up
// work fails, or consumes it permanently with DeleteKey once it commits.
func ClaimVerificationCodeWithKey(key string, code string, purpose string) bool {
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

// ReleaseVerificationCodeClaim reverses a prior claim so the same token becomes
// usable again without deleting it. It is a no-op unless the stored code still
// matches the claimed one: if the token was re-issued (rotated) while the claim
// was in flight, the new token must not be marked claimed.
func ReleaseVerificationCodeClaim(key string, code string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	value, okay := verificationMap[purpose+key]
	if !okay || value.code != code {
		return
	}
	value.claimed = false
	verificationMap[purpose+key] = value
}

func DeleteKey(key string, purpose string) {
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
