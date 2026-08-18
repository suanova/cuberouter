package common

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClaimVerificationCodeWithKeySingleWinner 保护重置 token claim 的原子性：
// 同一 token 被并发请求同时索取时，恰好一个请求成功——这正是"先验证后投递"
// 会放行两个请求、各自投递不同新密码的并发漏洞的防线。
func TestClaimVerificationCodeWithKeySingleWinner(t *testing.T) {
	email := "claim-atomic@test.com"
	code := GenerateVerificationCode(0)
	RegisterVerificationCodeWithKey(email, code, PasswordResetPurpose)
	t.Cleanup(func() { DeleteKey(email, PasswordResetPurpose) })

	const workers = 16
	results := make(chan bool, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- ClaimVerificationCodeWithKey(email, code, PasswordResetPurpose)
		}()
	}
	wg.Wait()
	close(results)

	winners := 0
	for ok := range results {
		if ok {
			winners++
		}
	}
	assert.Equal(t, 1, winners, "并发索取同一 token 时必须有且仅有一个请求胜出")
}

// TestClaimReleaseConsumeTokenLifecycle 覆盖 claim 的三个状态迁移：
// claim 后 token 不可再用（投递中）；release 后恢复可用（投递/落库失败可重试）；
// DeleteKey 消费后彻底失效（密码已提交，禁止重放）。
func TestClaimReleaseConsumeTokenLifecycle(t *testing.T) {
	email := "claim-lifecycle@test.com"
	code := GenerateVerificationCode(0)
	RegisterVerificationCodeWithKey(email, code, PasswordResetPurpose)
	t.Cleanup(func() { DeleteKey(email, PasswordResetPurpose) })

	require.True(t, ClaimVerificationCodeWithKey(email, code, PasswordResetPurpose))
	assert.False(t, VerifyCodeWithKey(email, code, PasswordResetPurpose), "被 claim 的 token 在释放前必须不可验证")
	assert.False(t, ClaimVerificationCodeWithKey(email, code, PasswordResetPurpose), "重复 claim 必须失败")

	ReleaseVerificationCodeClaim(email, code, PasswordResetPurpose)
	assert.True(t, VerifyCodeWithKey(email, code, PasswordResetPurpose), "释放后 token 必须可再次验证")

	require.True(t, ClaimVerificationCodeWithKey(email, code, PasswordResetPurpose))
	DeleteKey(email, PasswordResetPurpose)
	assert.False(t, VerifyCodeWithKey(email, code, PasswordResetPurpose), "消费后 token 必须失效")
	assert.False(t, ClaimVerificationCodeWithKey(email, code, PasswordResetPurpose), "消费后 claim 必须失败")
}

// TestReleaseVerificationCodeClaimDoesNotTouchRotatedToken 保护"旧 claim 在途时
// token 被重新签发"的场景：过期请求释放 claim 不能把新 token 也标记为已占用。
func TestReleaseVerificationCodeClaimDoesNotTouchRotatedToken(t *testing.T) {
	email := "claim-rotated@test.com"
	oldCode := GenerateVerificationCode(0)
	RegisterVerificationCodeWithKey(email, oldCode, PasswordResetPurpose)
	require.True(t, ClaimVerificationCodeWithKey(email, oldCode, PasswordResetPurpose))

	newCode := GenerateVerificationCode(0)
	RegisterVerificationCodeWithKey(email, newCode, PasswordResetPurpose)

	ReleaseVerificationCodeClaim(email, oldCode, PasswordResetPurpose)
	assert.True(t, ClaimVerificationCodeWithKey(email, newCode, PasswordResetPurpose),
		"旧 claim 的释放不得影响重签发的 token")

	t.Cleanup(func() { DeleteKey(email, PasswordResetPurpose) })
}
