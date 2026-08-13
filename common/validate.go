package common

import (
	"unicode"

	"github.com/go-playground/validator/v10"
)

var Validate *validator.Validate

// validatePasswordStrength 校验密码强度：
//   - 长度 8-20（由 min/max tag 覆盖，此处不重复判断）
//   - 至少包含一个大写字母
//   - 至少包含一个小写字母
//   - 至少包含一个数字
func validatePasswordStrength(fl validator.FieldLevel) bool {
	v := fl.Field().String()
	if v == "" {
		return true // 空密码由其他规则处理
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range v {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

func init() {
	Validate = validator.New()
	_ = Validate.RegisterValidation("passwordStrength", validatePasswordStrength)
}

// IsValidPasswordStrength 公开函数：校验密码是否满足强度要求
// 要求至少包含大写字母、小写字母和数字
func IsValidPasswordStrength(password string) bool {
	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

// GetValidationI18nKey 将 validator 的首个失败 tag 映射为对应的 i18n 消息 key。
// 调用方可据此返回友好的本地化错误，避免把 validator 的英文原始错误暴露给用户。
// 返回空字符串表示无可匹配的 tag，调用方应回退到通用错误处理。
func GetValidationI18nKey(err error) string {
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		return ""
	}
	for _, fe := range errs {
		switch fe.Tag() {
		case "passwordStrength":
			return "user.password_strength_invalid"
		case "max":
			// 按字段返回友好的超长提示，避免暴露 validator 的英文原始错误
			switch fe.Field() {
			case "Username":
				return "user.username_too_long"
			case "DisplayName":
				return "user.display_name_too_long"
			case "Email":
				return "user.email_too_long"
			}
		}
	}
	return ""
}
