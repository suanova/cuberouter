package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveModelProtocols(t *testing.T) {
	settings := ChannelOtherSettings{
		ModelProtocols: map[string][]string{
			"*":               {ModelProtocolChat},
			"claude-*":        {ModelProtocolChat, ModelProtocolMessages},
			"gpt-*":           {ModelProtocolChat, ModelProtocolResponses},
			"gpt-image-*":     {},
			"deepseek-v4-pro": {ModelProtocolChat, ModelProtocolResponses, ModelProtocolMessages},
			"re:^kimi-k[23]":  {ModelProtocolChat, ModelProtocolMessages},
			"glm-5.3":         {ModelProtocolChat, ModelProtocolResponses, ModelProtocolMessages},
		},
	}

	tests := []struct {
		name      string
		model     string
		want      []string
		wantFound bool
	}{
		{name: "exact match wins over wildcard", model: "glm-5.3", want: []string{ModelProtocolChat, ModelProtocolResponses, ModelProtocolMessages}, wantFound: true},
		{name: "longest prefix wins", model: "gpt-image-2.5-flare", want: []string{}, wantFound: true},
		{name: "prefix wildcard", model: "gpt-5.5", want: []string{ModelProtocolChat, ModelProtocolResponses}, wantFound: true},
		{name: "other prefix wildcard", model: "claude-opus-5", want: []string{ModelProtocolChat, ModelProtocolMessages}, wantFound: true},
		{name: "regex key", model: "kimi-k3", want: []string{ModelProtocolChat, ModelProtocolMessages}, wantFound: true},
		{name: "fallback wildcard", model: "unknown-model", want: []string{ModelProtocolChat}, wantFound: true},
		{name: "wildcard does not match non prefix", model: "my-claude-x", want: []string{ModelProtocolChat}, wantFound: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := settings.ResolveModelProtocols(tt.model)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveModelProtocolsWithoutConfig(t *testing.T) {
	got, found := ChannelOtherSettings{}.ResolveModelProtocols("gpt-5.5")
	assert.False(t, found)
	assert.Nil(t, got)
}

// TestResolveModelProtocolsOverlappingRegexIsDeterministic 锁定: 多个正则同时命中
// 同一模型时取 key 字典序最小的那个。Go 的 map 遍历顺序随机, 若直接返回"第一个
// 命中的键", 同一份配置在不同请求上会解析出不同协议; 因此这里重复解析确认稳定。
func TestResolveModelProtocolsOverlappingRegexIsDeterministic(t *testing.T) {
	settings := ChannelOtherSettings{ModelProtocols: map[string][]string{
		"re:^gpt":    {ModelProtocolChat},
		"re:-5$":     {ModelProtocolChat, ModelProtocolResponses},
		"re:^gpt-5$": {ModelProtocolChat, ModelProtocolResponses, ModelProtocolMessages},
	}}

	// 三个键都命中 gpt-5;"re:-5$" 字典序最小('-' 0x2D < '^' 0x5E), 即胜出者。
	want := []string{ModelProtocolChat, ModelProtocolResponses}
	for range 16 {
		got, found := settings.ResolveModelProtocols("gpt-5")
		require.True(t, found)
		require.Equal(t, want, got, "overlapping regex keys must resolve deterministically")
	}
}

func TestValidateModelProtocols(t *testing.T) {
	tests := []struct {
		name     string
		settings ChannelOtherSettings
		wantErr  string
	}{
		{
			name: "valid config",
			settings: ChannelOtherSettings{ModelProtocols: map[string][]string{
				"*":        {ModelProtocolChat},
				"claude-*": {ModelProtocolChat, ModelProtocolMessages},
				"re:^kimi": {ModelProtocolChat},
				"none":     {},
			}},
		},
		{
			name: "gemini protocol",
			settings: ChannelOtherSettings{ModelProtocols: map[string][]string{
				"gemini-*": {ModelProtocolGemini},
			}},
		},
		{
			name:     "unknown protocol",
			settings: ChannelOtherSettings{ModelProtocols: map[string][]string{"*": {"chatt"}}},
			wantErr:  "unsupported protocol",
		},
		{
			name:     "wildcard in the middle",
			settings: ChannelOtherSettings{ModelProtocols: map[string][]string{"cl*de": {ModelProtocolChat}}},
			wantErr:  "wildcard",
		},
		{
			name:     "invalid regex",
			settings: ChannelOtherSettings{ModelProtocols: map[string][]string{"re:[": {ModelProtocolChat}}},
			wantErr:  "invalid regex",
		},
		{
			name:     "empty key",
			settings: ChannelOtherSettings{ModelProtocols: map[string][]string{" ": {ModelProtocolChat}}},
			wantErr:  "must not be empty",
		},
		{
			name:     "exact key with surrounding whitespace",
			settings: ChannelOtherSettings{ModelProtocols: map[string][]string{" gpt-5 ": {ModelProtocolChat}}},
			wantErr:  "surrounding whitespace",
		},
		{
			name:     "fallback key with surrounding whitespace",
			settings: ChannelOtherSettings{ModelProtocols: map[string][]string{" * ": {ModelProtocolChat}}},
			wantErr:  "surrounding whitespace",
		},
		{
			name:     "empty config is valid",
			settings: ChannelOtherSettings{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.ValidateModelProtocols()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
