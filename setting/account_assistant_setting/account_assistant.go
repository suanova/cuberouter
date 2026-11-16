/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

// Package account_assistant_setting 承载「账户助手」的运行时配置。
//
// 账户助手只负责把用户问题路由到只读工具，再用工具结果组织回答；
// 它不能凭空回答。所有开关与限额都在这里集中配置，默认关闭，未配置
// 模型时接口直接拒绝（fail-closed），避免误用运营方 token。
package account_assistant_setting

import "github.com/QuantumNous/new-api/setting/config"

// AccountAssistantSetting 是账户助手的全局配置。
//
// BillingUserId 是调用模型时使用的账号；为空时退回提问者本人。
// 生产环境建议指向一个低额度、只允许 Group 内模型的专用账号。
type AccountAssistantSetting struct {
	Enabled bool   `json:"enabled"`
	Model   string `json:"model"`
	// BillingUserId 为 0 时使用提问者本人计费。
	BillingUserId int `json:"billing_user_id"`
	// Group 为 0 时使用计费账号的默认分组。
	Group string `json:"group"`
	// MaxTokens 是单次模型补全的 max_tokens 上限。
	MaxTokens int `json:"max_tokens"`
	// MaxToolCalls 是单轮问答内允许执行的工具调用总数。
	MaxToolCalls int `json:"max_tool_calls"`
	// MaxRounds 是允许模型请求工具的补全轮数上限；走完这些轮次后仍会
	// 额外执行一次不提供工具的强制收尾补全，所以总补全次数最多为 MaxRounds+1。
	MaxRounds int `json:"max_rounds"`
	// HourlyLimit / DailyLimit 是单用户提问次数限制。
	HourlyLimit int `json:"hourly_limit"`
	DailyLimit  int `json:"daily_limit"`
	// DailyTokenBudget 是单用户每日模型 token 预算，超过后拒绝服务。
	DailyTokenBudget int `json:"daily_token_budget"`
	// FAQJSON 是运营维护的帮助文档数组，元素形如
	// {"title": "...", "content": "..."}。search_help_docs 工具只读这里。
	FAQJSON string `json:"faq_json"`
	// SupportURL 是「联系人工」按钮打开的人工支持地址；为空时前端跳到关于页。
	SupportURL string `json:"support_url"`
}

const (
	defaultMaxTokens        = 800
	defaultMaxToolCalls     = 3
	defaultMaxRounds        = 2
	defaultHourlyLimit      = 20
	defaultDailyLimit       = 100
	defaultDailyTokenBudget = 500000
)

var defaultAccountAssistantSetting = AccountAssistantSetting{
	Enabled:          false,
	Model:            "",
	BillingUserId:    0,
	Group:            "",
	MaxTokens:        defaultMaxTokens,
	MaxToolCalls:     defaultMaxToolCalls,
	MaxRounds:        defaultMaxRounds,
	HourlyLimit:      defaultHourlyLimit,
	DailyLimit:       defaultDailyLimit,
	DailyTokenBudget: defaultDailyTokenBudget,
	FAQJSON:          "[]",
}

func init() {
	config.GlobalConfig.Register("account_assistant", &defaultAccountAssistantSetting)
}

// GetAccountAssistantSetting 返回归一化后的配置副本。
func GetAccountAssistantSetting() AccountAssistantSetting {
	setting := defaultAccountAssistantSetting
	setting.MaxTokens = clamp(setting.MaxTokens, 64, 4096, defaultMaxTokens)
	setting.MaxToolCalls = clamp(setting.MaxToolCalls, 1, 8, defaultMaxToolCalls)
	setting.MaxRounds = clamp(setting.MaxRounds, 1, 4, defaultMaxRounds)
	setting.HourlyLimit = clamp(setting.HourlyLimit, 1, 1000, defaultHourlyLimit)
	setting.DailyLimit = clamp(setting.DailyLimit, 1, 10000, defaultDailyLimit)
	setting.DailyTokenBudget = clamp(setting.DailyTokenBudget, 1000, 50_000_000, defaultDailyTokenBudget)
	if setting.FAQJSON == "" {
		setting.FAQJSON = "[]"
	}
	return setting
}

func clamp(value, minValue, maxValue, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
