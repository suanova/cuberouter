package model_setting

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/relayconvert/reasoning"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type ChatCompletionsToResponsesPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

func (p ChatCompletionsToResponsesPolicy) IsChannelEnabled(channelID int, channelType int) bool {
	if !p.Enabled {
		return false
	}
	if p.AllChannels {
		return true
	}

	if channelID > 0 && len(p.ChannelIDs) > 0 && slices.Contains(p.ChannelIDs, channelID) {
		return true
	}
	if channelType > 0 && len(p.ChannelTypes) > 0 && slices.Contains(p.ChannelTypes, channelType) {
		return true
	}
	return false
}

type GlobalSettings struct {
	PassThroughRequestEnabled        bool                             `json:"pass_through_request_enabled"`
	ThinkingModelBlacklist           []string                         `json:"thinking_model_blacklist"`
	EffortTailModelIDs               []string                         `json:"effort_tail_model_ids"`
	ChatCompletionsToResponsesPolicy ChatCompletionsToResponsesPolicy `json:"chat_completions_to_responses_policy"`
}

// 默认配置
var defaultOpenaiSettings = GlobalSettings{
	PassThroughRequestEnabled: false,
	ThinkingModelBlacklist: []string{
		"moonshotai/kimi-k2-thinking",
		"kimi-k2-thinking",
	},
	EffortTailModelIDs: []string{
		"gpt-5.1-codex-max",
		"qwen-image-edit-max",
		"qwen-max",
		"stable-diffusion-3-medium",
		"yi-medium",
	},
	ChatCompletionsToResponsesPolicy: ChatCompletionsToResponsesPolicy{
		Enabled:     false,
		AllChannels: true,
	},
}

// 全局实例
var globalSettings = defaultOpenaiSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("global", &globalSettings)
}

func GetGlobalSettings() *GlobalSettings {
	return &globalSettings
}

// ShouldPreserveThinkingSuffix 判断模型是否配置为保留 thinking/-nothinking/-low/-high/-medium 后缀
func ShouldPreserveThinkingSuffix(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}

	for _, entry := range globalSettings.ThinkingModelBlacklist {
		if strings.TrimSpace(entry) == target {
			return true
		}
	}
	return false
}

// ambiguousEffortTokens are effort tails that are also ordinary words in real
// model IDs (qwen-max, yi-medium, stable-diffusion-3-medium). They are read as
// reasoning aliases only when the base name they leave behind is a registered
// model. The remaining effort tokens (-high, -low, -minimal, -none, -xhigh) are
// reasoning-specific and are always read as aliases.
var ambiguousEffortTokens = []string{"-max", "-medium"}

// ShouldPreserveEffortTail reports whether a model name ending in an
// effort-like token (for example -max or -high) must be sent upstream verbatim
// instead of being read as a reasoning alias.
//
// Rewriting is only safe when we can tell an alias apart from a real model ID.
// For an ambiguous tail that means the base name must be a registered model:
// gpt-5.6-sol-max resolves to the registered base gpt-5.6-sol, whereas
// qwen3.8-max would resolve to qwen3.8, which nobody registered — that is a
// real model ID that merely looks like an alias. EffortTailModelIDs remains as
// an explicit override for real model IDs whose base name is registered too.
func ShouldPreserveEffortTail(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}
	bare := target
	if slash := strings.LastIndex(bare, "/"); slash >= 0 {
		bare = bare[slash+1:]
	}
	for _, entry := range globalSettings.EffortTailModelIDs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == target || entry == bare {
			return true
		}
	}

	base, _, found := reasoning.TrimEffortSuffixWithSuffixes(target, ambiguousEffortTokens)
	if !found {
		return false
	}
	return !ratio_setting.IsRegisteredModel(base)
}
