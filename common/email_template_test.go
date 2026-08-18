package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPasswordResetEmail_DefaultsRenderCorrectly(t *testing.T) {
	subject, content := RenderPasswordResetEmail("TestSys", "https://example.com/reset", 30)

	assert.Contains(t, subject, "Password Reset")
	assert.Contains(t, subject, "密码重置")
	assert.Contains(t, content, "reset your password")
	assert.Contains(t, content, "https://example.com/reset")
	// 所有 {{.X}} 占位符都必须被替换掉
	assert.NotContains(t, content, "{{.")
	assert.NotContains(t, subject, "{{.")
}

// 配置模板损坏（解析失败）时，必须回退到默认模板渲染，
// 不允许把未解析的 {{.Link}}/{{.NewPassword}} 原样发出去。
func TestRenderPasswordResetEmail_FallsBackToDefaultOnBrokenTemplate(t *testing.T) {
	old := PasswordResetEmailContentEn
	PasswordResetEmailContentEn = "broken {{.Link"
	t.Cleanup(func() { PasswordResetEmailContentEn = old })

	_, content := RenderPasswordResetEmail("TestSys", "https://example.com/reset", 30)

	assert.NotContains(t, content, "broken")
	assert.NotContains(t, content, "{{.", "损坏模板不得原样输出")
	assert.Contains(t, content, "reset your password", "应回退到默认英文模板")
}

// 配置模板执行失败（引用了不存在的变量）时同样回退默认模板。
func TestRenderPasswordResetSuccessEmail_FallsBackToDefaultOnBrokenTemplate(t *testing.T) {
	old := PasswordResetSuccessEmailContentEn
	PasswordResetSuccessEmailContentEn = "hello {{.DoesNotExist}} {{.Missing}}"
	t.Cleanup(func() { PasswordResetSuccessEmailContentEn = old })

	_, content := RenderPasswordResetSuccessEmail("TestSys", "s3cr3t")

	assert.NotContains(t, content, "DoesNotExist")
	assert.NotContains(t, content, "{{.", "损坏模板不得原样输出")
	require.True(t, strings.Contains(content, "new password"),
		"应回退到默认英文成功邮件模板，实际内容: %s", content)
	// 默认模板应渲染出真实新密码
	assert.Contains(t, content, "s3cr3t")
}
