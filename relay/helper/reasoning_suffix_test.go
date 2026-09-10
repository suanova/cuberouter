package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyReasoningModelSuffixTrimsUpstreamAndAttachesState(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet-thinking",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet-thinking",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "claude-3-7-sonnet", info.UpstreamModelName)
	require.NotNil(t, info.ReasoningConversion)
	assert.Equal(t, "enabled", info.ReasoningConversion.Mode)
}

func TestApplyReasoningModelSuffixRetryKeepsEquivalentState(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-opus-4-8-high",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-8-high",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	require.NotNil(t, info.ReasoningConversion)
	firstMode := info.ReasoningConversion.Mode
	firstEffort := info.ReasoningConversion.Effort

	info.UpstreamModelName = info.OriginModelName
	require.NoError(t, ApplyReasoningModelSuffix(info))
	require.NotNil(t, info.ReasoningConversion)
	assert.Equal(t, firstMode, info.ReasoningConversion.Mode)
	assert.Equal(t, firstEffort, info.ReasoningConversion.Effort)
	assert.Equal(t, "claude-opus-4-8", info.UpstreamModelName)
}

func TestApplyReasoningModelSuffixRetryClearsStateWhenNewChannelHasNoSuffix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := &dto.ClaudeRequest{Model: "claude-3-7-sonnet"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet",
		Request:         req,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet-thinking",
			IsModelMapped:     true,
		},
	}
	require.NoError(t, ApplyReasoningModelSuffix(info))
	require.NotNil(t, info.ReasoningState())

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "claude-3-7-sonnet")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	info.InitChannelMeta(ctx)
	assert.Nil(t, info.ReasoningState())

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Nil(t, info.ReasoningState())
}

func TestApplyReasoningModelSuffixPassThroughDoesNotTrim(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := settings.PassThroughRequestEnabled
	t.Cleanup(func() { settings.PassThroughRequestEnabled = original })
	settings.PassThroughRequestEnabled = true

	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet-thinking",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet-thinking",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "claude-3-7-sonnet-thinking", info.UpstreamModelName)
	assert.Nil(t, info.ReasoningConversion)
}

func TestApplyReasoningModelSuffixBlacklistDoesNotTrim(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := append([]string(nil), settings.ThinkingModelBlacklist...)
	t.Cleanup(func() { settings.ThinkingModelBlacklist = original })
	settings.ThinkingModelBlacklist = append(settings.ThinkingModelBlacklist, "claude-3-7-sonnet-thinking")

	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet-thinking",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet-thinking",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "claude-3-7-sonnet-thinking", info.UpstreamModelName)
	assert.Nil(t, info.ReasoningConversion)
}

func TestApplyReasoningModelSuffixRejectsExplicitSuffixConflict(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet-thinking",
		Request: &dto.ClaudeRequest{
			Model:    "claude-3-7-sonnet-thinking",
			Thinking: &dto.Thinking{Type: "disabled"},
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet-thinking",
		},
	}

	err := ApplyReasoningModelSuffix(info)
	require.Error(t, err)
}

func TestApplyReasoningModelSuffixGeminiNoThinkingWhenAdapterEnabled(t *testing.T) {
	settings := model_setting.GetGeminiSettings()
	original := settings.ThinkingAdapterEnabled
	t.Cleanup(func() { settings.ThinkingAdapterEnabled = original })
	settings.ThinkingAdapterEnabled = true

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-2.5-flash-nothinking",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash-nothinking",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "gemini-2.5-flash", info.UpstreamModelName)
	require.NotNil(t, info.ReasoningConversion)
	assert.Equal(t, "disabled", info.ReasoningConversion.Mode)
	assert.Equal(t, "none", info.ReasoningConversion.Effort)
}

func TestApplyReasoningModelSuffixPreservesEffortTailModelID(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "qwen-max",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "qwen-max",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "qwen-max", info.UpstreamModelName)
	assert.Nil(t, info.ReasoningConversion)
}

func TestApplyReasoningModelSuffixLeavesDeepSeekV4SuffixForAdaptor(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "deepseek-v4-chat-max",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeDeepSeek,
			UpstreamModelName: "deepseek-v4-chat-max",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "deepseek-v4-chat-max", info.UpstreamModelName)
	assert.Nil(t, info.ReasoningConversion)
}

func TestApplyReasoningModelSuffixLeavesVolcengineDeepSeekThinkingForAdaptor(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "deepseek-r1-thinking",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeVolcEngine,
			UpstreamModelName: "deepseek-r1-thinking",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "deepseek-r1-thinking", info.UpstreamModelName)
	assert.Nil(t, info.ReasoningConversion)
}

func TestApplyReasoningModelSuffixStillParsesOpenAIEffortTail(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1-high",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-5.1-high",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "gpt-5.1", info.UpstreamModelName)
	require.NotNil(t, info.ReasoningConversion)
	assert.Equal(t, "enabled", info.ReasoningConversion.Mode)
	assert.Equal(t, "high", info.ReasoningConversion.Effort)
}

// useModelRatios installs a known pricing registry for the duration of a test.
// The registry is the "is this name a real model?" source for effort-tail
// detection, so tests must seed it explicitly instead of inheriting whatever
// the process happens to hold.
func useModelRatios(t *testing.T, ratios map[string]float64) {
	t.Helper()
	savedRatios := ratio_setting.ModelRatio2JSONString()
	savedPrices := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
	})

	payload, err := common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(payload)))
}

// A model ID that merely ends in an effort token must survive when it reached
// the request through a channel model_mapping. The operator configured an
// upstream identifier there, not a client-facing reasoning alias, so trimming
// it rewrites a real model into a nonexistent one (qwen3.8-max -> qwen3.8).
func TestApplyReasoningModelSuffixPreservesMappedRealModelID(t *testing.T) {
	useModelRatios(t, map[string]float64{"gpt-5.6-sol": 2.5})

	info := &relaycommon.RelayInfo{
		OriginModelName: "qwen3.8-max-a",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "qwen3.8-max", // channel model_mapping target
			IsModelMapped:     true,
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "qwen3.8-max", info.UpstreamModelName)
	assert.Nil(t, info.ReasoningConversion)
}

// An ambiguous tail (-max/-medium, tokens real model IDs use too) still trims
// when the base name it leaves behind is a registered model.
func TestApplyReasoningModelSuffixTrimsAmbiguousTailWhenBaseIsRegistered(t *testing.T) {
	useModelRatios(t, map[string]float64{"gpt-5.6-sol": 2.5})

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.6-sol-max",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.6-sol-max",
		},
	}

	require.NoError(t, ApplyReasoningModelSuffix(info))
	assert.Equal(t, "gpt-5.6-sol", info.UpstreamModelName)
	require.NotNil(t, info.ReasoningConversion)
	assert.Equal(t, "enabled", info.ReasoningConversion.Mode)
	assert.Equal(t, "max", info.ReasoningConversion.Effort)
}

func TestApplyReasoningModelSuffixTrimsOpenRouterThinkingOnly(t *testing.T) {
	openRouter := &relaycommon.RelayInfo{
		OriginModelName: "some-model-thinking",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: "some-model-thinking",
		},
	}
	require.NoError(t, ApplyReasoningModelSuffix(openRouter))
	assert.Equal(t, "some-model", openRouter.UpstreamModelName)
	require.NotNil(t, openRouter.ReasoningConversion)
	assert.Equal(t, "enabled", openRouter.ReasoningConversion.Mode)
}
