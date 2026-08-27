package security_setting

import "github.com/QuantumNous/new-api/setting/config"

// SecuritySetting 传输安全相关设置。
type SecuritySetting struct {
	// RequireHTTPSChannelBaseURL 要求渠道上游 base URL 使用 HTTPS（或指向回环地址），
	// 避免把渠道鉴权凭证明文发送到公网上游（CWE-319）。本地/E2E 使用明文 mock
	// 上游时可显式关闭。
	RequireHTTPSChannelBaseURL bool `json:"require_https_channel_base_url"`
}

var defaultSecuritySetting = SecuritySetting{
	RequireHTTPSChannelBaseURL: true, // 默认开启，拒绝明文渠道上游
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("security_setting", &defaultSecuritySetting)
}

func GetSecuritySetting() *SecuritySetting {
	return &defaultSecuritySetting
}
