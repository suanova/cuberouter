package security_setting

import (
	"fmt"
	"net"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// SecuritySetting 传输安全相关设置。
type SecuritySetting struct {
	// RequireHTTPSChannelBaseURL 要求渠道上游 base URL 使用 HTTPS（或指向回环地址），
	// 避免把渠道鉴权凭证明文发送到公网上游（CWE-319）。本地/E2E 使用明文 mock
	// 上游时可显式关闭。
	RequireHTTPSChannelBaseURL bool `json:"require_https_channel_base_url"`
	// SwaggerAPIIPWhitelist 限制 OpenAPI 文档与 /api/v2 对外接口的客户端 IP
	// （单个 IP 或 CIDR）。为空表示不限制，保持未配置时的既有行为。
	SwaggerAPIIPWhitelist []string `json:"swagger_api_ip_whitelist"`
}

var defaultSecuritySetting = SecuritySetting{
	RequireHTTPSChannelBaseURL: true, // 默认开启，拒绝明文渠道上游
	SwaggerAPIIPWhitelist:      []string{},
}

// ValidateSwaggerAPIIPWhitelist 校验白名单的 JSON 字符串数组表示。每一项必须是
// 合法 IP 或 CIDR：非法项在匹配时会被静默跳过，管理员会误以为已经放行，所以保存时
// 直接拒绝。非字符串元素同样拒绝，避免后端 []string 解析失败后整份配置被丢弃。
func ValidateSwaggerAPIIPWhitelist(value string) error {
	var entries []string
	if err := common.UnmarshalJsonStr(value, &entries); err != nil {
		return fmt.Errorf("IP whitelist must be a JSON array of strings: %w", err)
	}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return fmt.Errorf("IP whitelist contains an empty entry")
		}
		if net.ParseIP(entry) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(entry); err != nil {
			return fmt.Errorf("invalid IP or CIDR in whitelist: %s", entry)
		}
	}
	return nil
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("security_setting", &defaultSecuritySetting)
}

func GetSecuritySetting() *SecuritySetting {
	return &defaultSecuritySetting
}
