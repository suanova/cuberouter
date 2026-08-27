package common

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// ValidateRedirectURL validates that a redirect URL is safe to use.
// It checks that:
//   - The URL is properly formatted
//   - The scheme is either http or https
//   - The domain is in the trusted domains list (exact match or subdomain)
//
// Returns nil if the URL is valid and trusted, otherwise returns an error
// describing why the validation failed.
func ValidateRedirectURL(rawURL string) error {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %s", err.Error())
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: only http and https are allowed")
	}

	domain := strings.ToLower(parsedURL.Hostname())

	for _, trustedDomain := range constant.TrustedRedirectDomains {
		if domain == trustedDomain || strings.HasSuffix(domain, "."+trustedDomain) {
			return nil
		}
	}

	return fmt.Errorf("domain %s is not in the trusted domains list", domain)
}

// ValidateHTTPSChannelBaseURL 校验渠道上游 base URL 的传输安全（CWE-319）：
// 当 requireHTTPS 为真时，仅接受 https 上游或回环地址（本地开发/MockLLM），
// 拒绝其余明文上游，避免渠道鉴权凭证被明文发送到公网。空值视为安全——调用方
// 会回退到渠道内置默认 base URL（AstraFlow 默认为 https://api.modelverse.cn）。
func ValidateHTTPSChannelBaseURL(baseURL string, requireHTTPS bool) error {
	if !requireHTTPS {
		return nil
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("channel base URL is invalid: %s", baseURL)
	}
	if parsedURL.Scheme == "https" {
		return nil
	}
	if IsLoopbackHost(parsedURL.Hostname()) {
		return nil
	}
	return fmt.Errorf("channel base URL must use HTTPS to protect the channel API key, got %q", baseURL)
}
