package common

import (
	"bytes"
	"fmt"
	"text/template"
)

// WrapBilingualContent wraps email content with both English and Chinese versions.
// English content is placed at the top, followed by Chinese content.
// A horizontal rule separates the two sections.
func WrapBilingualContent(enContent string, zhContent string) string {
	return fmt.Sprintf(
		`%s`+
			`<hr style="border:none;border-top:1px solid #ddd;margin:20px 0;">`+
			`%s`,
		enContent, zhContent,
	)
}

// WrapBilingualSubject returns a bilingual email subject with English first.
func WrapBilingualSubject(enSubject string, zhSubject string) string {
	return fmt.Sprintf("%s / %s", enSubject, zhSubject)
}

// ============================================================
// Password Reset Email — Default Templates
// ============================================================

// DefaultPasswordResetEmailSubjectEn is the default English subject for the
// password-reset *link* email. Variables: {{.SystemName}}
var DefaultPasswordResetEmailSubjectEn = `{{.SystemName}} Password Reset`

// DefaultPasswordResetEmailSubjectZh is the default Chinese subject.
var DefaultPasswordResetEmailSubjectZh = `{{.SystemName}}密码重置`

// DefaultPasswordResetEmailContentEn is the default English HTML body.
// Variables: {{.SystemName}}, {{.Link}}, {{.ValidMinutes}}
var DefaultPasswordResetEmailContentEn = `<p>Hello, you are resetting your password for {{.SystemName}}.</p>` +
	`<p>Click <a href='{{.Link}}'>here</a> to reset your password.</p>` +
	`<p>If the link does not work, please copy and paste the following URL into your browser:<br> {{.Link}} </p>` +
	`<p>This reset link is valid for {{.ValidMinutes}} minutes. If you did not request this, please ignore this email.</p>`

// DefaultPasswordResetEmailContentZh is the default Chinese HTML body.
var DefaultPasswordResetEmailContentZh = `<p>您好，你正在进行{{.SystemName}}密码重置。</p>` +
	`<p>点击 <a href='{{.Link}}'>此处</a> 进行密码重置。</p>` +
	`<p>如果链接无法点击，请尝试点击下面的链接或将其复制到浏览器中打开：<br> {{.Link}} </p>` +
	`<p>重置链接 {{.ValidMinutes}} 分钟内有效，如果不是本人操作，请忽略。</p>`

// ============================================================
// Password Reset Success Email — Default Templates
// ============================================================

// DefaultPasswordResetSuccessEmailSubjectEn is the default English subject for
// the password-reset *success* email. Variables: {{.SystemName}}
var DefaultPasswordResetSuccessEmailSubjectEn = `{{.SystemName}} Password Reset Successfully`

// DefaultPasswordResetSuccessEmailSubjectZh is the default Chinese subject.
var DefaultPasswordResetSuccessEmailSubjectZh = `{{.SystemName}}密码重置成功`

// DefaultPasswordResetSuccessEmailContentEn is the default English HTML body.
// Variables: {{.SystemName}}, {{.NewPassword}}
var DefaultPasswordResetSuccessEmailContentEn = `<p>Hello, your password for {{.SystemName}} has been reset.</p>` +
	`<p>Your new password is: <strong>{{.NewPassword}}</strong></p>` +
	`<p>Please log in with the new password and change it as soon as possible. ` +
	`If this was not your action, please contact the administrator immediately.</p>`

// DefaultPasswordResetSuccessEmailContentZh is the default Chinese HTML body.
var DefaultPasswordResetSuccessEmailContentZh = `<p>您好，您的 {{.SystemName}} 密码已重置。</p>` +
	`<p>您的新密码为：<strong>{{.NewPassword}}</strong></p>` +
	`<p>请使用新密码登录并及时修改密码。如果不是本人操作，请立即联系管理员。</p>`

// ============================================================
// Template rendering helpers
// ============================================================

// PasswordResetEmailTemplateData holds variables for password reset link email.
type PasswordResetEmailTemplateData struct {
	SystemName   string
	Link         string
	ValidMinutes int
}

// PasswordResetSuccessEmailTemplateData holds variables for password reset success email.
type PasswordResetSuccessEmailTemplateData struct {
	SystemName  string
	NewPassword string
}

// renderTemplate safely renders a Go text/template, returning an error on
// parse or execute failure so the caller can fall back to a default template
// (instead of emailing the raw template with unresolved {{.Var}} placeholders).
func renderTemplate(tmplStr string, data interface{}) (string, error) {
	if tmplStr == "" {
		return "", nil
	}
	tmpl, err := template.New("email").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderTemplateWithFallback renders the configured template; on parse or
// execute failure it logs and falls back to the given default template, so a
// misconfigured admin template never mails unresolved {{.Link}}/{{.NewPassword}}
// placeholders (which would hide the reset link or the new password).
func renderTemplateWithFallback(configured, def string, data interface{}) string {
	out, err := renderTemplate(configured, data)
	if err == nil {
		return out
	}
	SysError("failed to render email template, falling back to default: " + err.Error())
	out, err = renderTemplate(def, data)
	if err != nil {
		// 默认模板为静态常量，理论上不会失败；兜底仍返回默认模板原文。
		return def
	}
	return out
}

// RenderPasswordResetEmail renders subject + content for the password-reset
// *link* email using the configurable templates stored in the package-level
// variables (which are loaded from OptionMap at startup / when settings change).
// If a configurable template is empty, it falls back to the default.
func RenderPasswordResetEmail(systemName, link string, validMinutes int) (subject, content string) {
	data := PasswordResetEmailTemplateData{
		SystemName:   systemName,
		Link:         link,
		ValidMinutes: validMinutes,
	}
	subjEnTmpl := PasswordResetEmailSubjectEn
	if subjEnTmpl == "" {
		subjEnTmpl = DefaultPasswordResetEmailSubjectEn
	}
	subjZhTmpl := PasswordResetEmailSubjectZh
	if subjZhTmpl == "" {
		subjZhTmpl = DefaultPasswordResetEmailSubjectZh
	}
	subjectEn := renderTemplateWithFallback(subjEnTmpl, DefaultPasswordResetEmailSubjectEn, data)
	subjectZh := renderTemplateWithFallback(subjZhTmpl, DefaultPasswordResetEmailSubjectZh, data)
	subject = WrapBilingualSubject(subjectEn, subjectZh)

	contentEnTmpl := PasswordResetEmailContentEn
	if contentEnTmpl == "" {
		contentEnTmpl = DefaultPasswordResetEmailContentEn
	}
	contentZhTmpl := PasswordResetEmailContentZh
	if contentZhTmpl == "" {
		contentZhTmpl = DefaultPasswordResetEmailContentZh
	}
	contentEn := renderTemplateWithFallback(contentEnTmpl, DefaultPasswordResetEmailContentEn, data)
	contentZh := renderTemplateWithFallback(contentZhTmpl, DefaultPasswordResetEmailContentZh, data)
	content = WrapBilingualContent(contentEn, contentZh)
	return subject, content
}

// RenderPasswordResetSuccessEmail renders subject + content for the
// password-reset *success* email.
// If a configurable template is empty, it falls back to the default.
func RenderPasswordResetSuccessEmail(systemName, newPassword string) (subject, content string) {
	data := PasswordResetSuccessEmailTemplateData{
		SystemName:  systemName,
		NewPassword: newPassword,
	}
	subjEnTmpl := PasswordResetSuccessEmailSubjectEn
	if subjEnTmpl == "" {
		subjEnTmpl = DefaultPasswordResetSuccessEmailSubjectEn
	}
	subjZhTmpl := PasswordResetSuccessEmailSubjectZh
	if subjZhTmpl == "" {
		subjZhTmpl = DefaultPasswordResetSuccessEmailSubjectZh
	}
	subjectEn := renderTemplateWithFallback(subjEnTmpl, DefaultPasswordResetSuccessEmailSubjectEn, data)
	subjectZh := renderTemplateWithFallback(subjZhTmpl, DefaultPasswordResetSuccessEmailSubjectZh, data)
	subject = WrapBilingualSubject(subjectEn, subjectZh)

	contentEnTmpl := PasswordResetSuccessEmailContentEn
	if contentEnTmpl == "" {
		contentEnTmpl = DefaultPasswordResetSuccessEmailContentEn
	}
	contentZhTmpl := PasswordResetSuccessEmailContentZh
	if contentZhTmpl == "" {
		contentZhTmpl = DefaultPasswordResetSuccessEmailContentZh
	}
	contentEn := renderTemplateWithFallback(contentEnTmpl, DefaultPasswordResetSuccessEmailContentEn, data)
	contentZh := renderTemplateWithFallback(contentZhTmpl, DefaultPasswordResetSuccessEmailContentZh, data)
	content = WrapBilingualContent(contentEn, contentZh)
	return subject, content
}
