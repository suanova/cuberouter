package system_setting

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type PasskeySettings struct {
	Enabled              bool   `json:"enabled"`
	RPDisplayName        string `json:"rp_display_name"`
	RPID                 string `json:"rp_id"`
	Origins              string `json:"origins"`
	AllowInsecureOrigin  bool   `json:"allow_insecure_origin"`
	UserVerification     string `json:"user_verification"`
	AttachmentPreference string `json:"attachment_preference"`
}

var defaultPasskeySettings = PasskeySettings{
	Enabled:              false,
	RPDisplayName:        common.SystemName,
	RPID:                 "",
	Origins:              "",
	AllowInsecureOrigin:  false,
	UserVerification:     "preferred",
	AttachmentPreference: "",
}

func init() {
	config.GlobalConfig.Register("passkey", &defaultPasskeySettings)
}

func GetPasskeySettings() *PasskeySettings {
	if defaultPasskeySettings.RPID == "" && ServerAddress != "" {
		// 从ServerAddress提取域名作为RPID
		// ServerAddress可能是 "https://newapi.pro" 这种格式
		// RPID 是域名，不带端口：url.Host 含端口，必须用 Hostname。
		serverAddr := strings.TrimSpace(ServerAddress)
		if parsed, err := url.Parse(serverAddr); err == nil && parsed.Hostname() != "" {
			defaultPasskeySettings.RPID = parsed.Hostname()
		} else {
			defaultPasskeySettings.RPID = serverAddr
		}
	}
	if defaultPasskeySettings.Origins == "" || defaultPasskeySettings.Origins == "[]" {
		// Origins 必须是 scheme://host[:port]：WebAuthn 不接受带路径的 origin，
		// 而 ServerAddress 可能带着部署前缀（如 http://127.0.0.1:5000/cuberouter）。
		// 解析不出来就留空，让 resolveOrigins 改用请求自身的 origin。
		serverAddr := strings.TrimSpace(ServerAddress)
		if parsed, err := url.Parse(serverAddr); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			defaultPasskeySettings.Origins = parsed.Scheme + "://" + parsed.Host
		}
	}
	return &defaultPasskeySettings
}
