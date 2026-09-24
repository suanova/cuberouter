package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetPricingEndpointTestTables(t *testing.T) {
	t.Helper()
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Model{}, &Vendor{}, &Option{}))
	for _, table := range []string{"abilities", "channels", "models", "vendors", "options"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
	InitChannelCache()
	InvalidatePricingCache()
	t.Cleanup(func() {
		for _, table := range []string{"abilities", "channels", "models", "vendors", "options"} {
			require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
		}
		InitChannelCache()
		InvalidatePricingCache()
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
}

func insertPricingEndpointChannel(t *testing.T, channelID int, channelType int, settings dto.ChannelOtherSettings) {
	t.Helper()
	channel := &Channel{
		Id:     channelID,
		Type:   channelType,
		Key:    fmt.Sprintf("key-%d", channelID),
		Status: common.ChannelStatusEnabled,
		Name:   fmt.Sprintf("channel-%d", channelID),
	}
	if settings.AdvancedCustom != nil {
		channel.SetOtherSettings(settings)
	}
	require.NoError(t, DB.Create(channel).Error)
}

func insertPricingEndpointAbility(t *testing.T, channelID int, modelName string) {
	t.Helper()
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     modelName,
		ChannelId: channelID,
		Enabled:   true,
	}).Error)
}

func pricingEndpointAdvancedCustomConfig(routes ...dto.AdvancedCustomRoute) dto.ChannelOtherSettings {
	return dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{
			Routes: routes,
		},
	}
}

func pricingEndpointTypesByModel(t *testing.T) map[string][]constant.EndpointType {
	t.Helper()
	InitChannelCache()
	return pricingEndpointTypesFromPricing(GetPricing())
}

func pricingEndpointTypesFromPricing(pricings []Pricing) map[string][]constant.EndpointType {
	byModel := make(map[string][]constant.EndpointType)
	for _, pricing := range pricings {
		byModel[pricing.ModelName] = pricing.SupportedEndpointTypes
	}
	return byModel
}

func TestPricingAdvancedCustomUsesConfiguredEndpointTypes(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 101, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/v1/chat/completions",
		},
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	insertPricingEndpointAbility(t, 101, "gemini-2.5-flash")
	insertPricingEndpointAbility(t, 101, "gpt-4o")

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, byModel["gemini-2.5-flash"])
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
	}, byModel["gpt-4o"])
}

func TestPricingModelMetadataEndpointsMergeWithAdvancedCustomInference(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 103, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	insertPricingEndpointAbility(t, 103, "gemini-2.5-flash")
	require.NoError(t, DB.Create(&Model{
		ModelName: "gemini-2.5-flash",
		Endpoints: `{
			"openai": "/v1/chat/completions"
		}`,
		Status:   1,
		NameRule: NameRuleExact,
	}).Error)

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeOpenAI,
	}, byModel["gemini-2.5-flash"])
}

func TestPricingModelMetadataEndpointsCanProvideEndpointWithoutChannelInference(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 104, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	insertPricingEndpointAbility(t, 104, "metadata-only-model")
	require.NoError(t, DB.Create(&Model{
		ModelName: "metadata-only-model",
		Endpoints: `{
			"openai": "/v1/chat/completions"
		}`,
		Status:   1,
		NameRule: NameRuleExact,
	}).Error)

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, byModel["metadata-only-model"])
}

func TestPricingAdvancedCustomMissingConfigFallsBackToChannelType(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 102, constant.ChannelTypeAdvancedCustom, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 102, "gpt-4o")

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, byModel["gpt-4o"])
}

func TestPricingNativeChannelEndpointTypesUnchanged(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 201, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	insertPricingEndpointChannel(t, 202, constant.ChannelTypeGemini, dto.ChannelOtherSettings{})
	insertPricingEndpointChannel(t, 203, constant.ChannelTypeAnthropic, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 201, "gpt-4o")
	insertPricingEndpointAbility(t, 202, "gemini-2.5-flash")
	insertPricingEndpointAbility(t, 203, "claude-3-5-sonnet")

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, byModel["gpt-4o"])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeGemini, constant.EndpointTypeOpenAI}, byModel["gemini-2.5-flash"])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeAnthropic, constant.EndpointTypeOpenAI}, byModel["claude-3-5-sonnet"])
}

func TestInitChannelCacheInvalidatesPricingCache(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 301, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/v1/chat/completions",
		},
	))
	insertPricingEndpointAbility(t, 301, "gemini-3.5-flash")
	InitChannelCache()

	initial := pricingEndpointTypesByModel(t)
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, initial["gemini-3.5-flash"])

	var channel Channel
	require.NoError(t, DB.First(&channel, "id = ?", 301).Error)
	channel.SetOtherSettings(pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/v1/chat/completions",
		},
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 301).Update("settings", channel.OtherSettings).Error)
	InitChannelCache()

	updated := pricingEndpointTypesByModel(t)
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, updated["gemini-3.5-flash"])
}

func TestInitChannelCacheInvalidatesStartupPricingBuiltBeforeChannelCache(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 302, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/v1/chat/completions",
		},
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	insertPricingEndpointAbility(t, 302, "gemini-3.5-flash")

	staleByModel := pricingEndpointTypesFromPricing(GetPricing())
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, staleByModel["gemini-3.5-flash"])

	InitChannelCache()

	rebuiltByModel := pricingEndpointTypesFromPricing(GetPricing())
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, rebuiltByModel["gemini-3.5-flash"])
}

func TestCacheUpdateChannelSyncsAdvancedCustomConfig(t *testing.T) {
	resetPricingEndpointTestTables(t)

	channel := &Channel{
		Id:     401,
		Type:   constant.ChannelTypeAdvancedCustom,
		Key:    "key-401",
		Status: common.ChannelStatusEnabled,
		Name:   "channel-401",
	}
	channel.SetOtherSettings(pricingEndpointAdvancedCustomConfig(dto.AdvancedCustomRoute{
		IncomingPath: "/v1/responses",
		UpstreamPath: "/v1beta/models/{model}:generateContent",
		Converter:    "openai_responses_to_gemini_generate_content",
	}))
	CacheUpdateChannel(channel)

	require.NotNil(t, channel2advancedCustomConfig[401])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIResponse}, channel2advancedCustomConfig[401].SupportedEndpointTypesForModel("gemini-3.5-flash"))

	channel.SetOtherSettings(pricingEndpointAdvancedCustomConfig(dto.AdvancedCustomRoute{
		IncomingPath: "/v1/chat/completions",
		UpstreamPath: "/v1/chat/completions",
	}))
	CacheUpdateChannel(channel)

	require.NotNil(t, channel2advancedCustomConfig[401])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, channel2advancedCustomConfig[401].SupportedEndpointTypesForModel("gemini-3.5-flash"))

	channel.Type = constant.ChannelTypeOpenAI
	CacheUpdateChannel(channel)

	assert.Nil(t, channel2advancedCustomConfig[401])
}

func TestPricingVideoPricesPopulatedFromVideoPriceOption(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 501, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 501, "viduq3-pro")
	insertPricingEndpointAbility(t, 501, "viduq3-turbo")

	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		// 恢复 OptionMap 与 ratio_setting 全局状态:本包仅此用例写入视频价格表,清空即恢复
		require.NoError(t, ratio_setting.UpdateVideoPriceByJSONString("{}"))
		common.OptionMap = previousOptionMap
	})

	require.NoError(t, UpdateOption("VideoPrice", `{
		"viduq3-pro": {
			"rows": [
				{"resolution": "1080p", "normal_price": 0.75, "off_peak_price": 0.5},
				{"resolution": "4k", "normal_price": 1.5, "off_peak_price": 1.0}
			]
		}
	}`))

	InvalidatePricingCache()
	pricings := GetPricing()

	var pro, turbo *Pricing
	for i := range pricings {
		switch pricings[i].ModelName {
		case "viduq3-pro":
			pro = &pricings[i]
		case "viduq3-turbo":
			turbo = &pricings[i]
		}
	}
	require.NotNil(t, pro, "viduq3-pro should appear in pricing")
	require.NotNil(t, turbo, "viduq3-turbo should appear in pricing")

	require.NotNil(t, pro.VideoPrices)
	require.Len(t, pro.VideoPrices.Rows, 2)
	assert.Equal(t, "1080p", pro.VideoPrices.Rows[0].Resolution)
	assert.Equal(t, 0.75, pro.VideoPrices.Rows[0].NormalPrice)
	assert.Equal(t, 0.5, pro.VideoPrices.Rows[0].OffPeakPrice)
	assert.Equal(t, "4k", pro.VideoPrices.Rows[1].Resolution)
	assert.Equal(t, 1.5, pro.VideoPrices.Rows[1].NormalPrice)
	assert.Equal(t, 1.0, pro.VideoPrices.Rows[1].OffPeakPrice)

	assert.Nil(t, turbo.VideoPrices, "unconfigured model must not expose video_prices")
}

func TestPricingImagePricesPopulatedFromImagePriceOption(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 502, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 502, "img-pro")
	insertPricingEndpointAbility(t, 502, "img-turbo")

	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		// 恢复 OptionMap 与 ratio_setting 全局状态:本包仅此用例写入图片价格表,清空即恢复
		require.NoError(t, ratio_setting.UpdateImagePriceByJSONString("{}"))
		common.OptionMap = previousOptionMap
	})

	require.NoError(t, UpdateOption("ImagePrice", `{
		"img-pro": {
			"rows": [
				{"tier": "fast", "price": 0.02},
				{"tier": "high", "price": 0.04}
			]
		}
	}`))

	InvalidatePricingCache()
	pricings := GetPricing()

	var pro, turbo *Pricing
	for i := range pricings {
		switch pricings[i].ModelName {
		case "img-pro":
			pro = &pricings[i]
		case "img-turbo":
			turbo = &pricings[i]
		}
	}
	require.NotNil(t, pro, "img-pro should appear in pricing")
	require.NotNil(t, turbo, "img-turbo should appear in pricing")

	require.NotNil(t, pro.ImagePrices)
	require.Len(t, pro.ImagePrices.Rows, 2)
	assert.Equal(t, "fast", pro.ImagePrices.Rows[0].Tier)
	assert.Equal(t, 0.02, pro.ImagePrices.Rows[0].Price)
	assert.Equal(t, "high", pro.ImagePrices.Rows[1].Tier)
	assert.Equal(t, 0.04, pro.ImagePrices.Rows[1].Price)

	assert.Nil(t, turbo.ImagePrices, "unconfigured model must not expose image_prices")
}

func TestOffPeakWindowOptionUpdatesWindow(t *testing.T) {
	resetPricingEndpointTestTables(t)

	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	prev := ratio_setting.GetOffPeakWindow()
	t.Cleanup(func() {
		restored, err := common.Marshal(prev)
		require.NoError(t, err)
		require.NoError(t, ratio_setting.UpdateOffPeakWindowByJSONString(string(restored)))
		common.OptionMap = previousOptionMap
	})

	require.NoError(t, UpdateOption("OffPeakWindow", `{"start_hour":23,"end_hour":7,"timezone":"Asia/Shanghai"}`))

	w := ratio_setting.GetOffPeakWindow()
	assert.Equal(t, 23, w.StartHour)
	assert.Equal(t, 7, w.EndHour)
}

// TestPricingModelTagDeclaresImageGenerationEndpoint 锁定模型标签到端点类型的契约：
// 只有 text-to-image 会补上 image-generation，且位于列表首位——端点列表首项是各
// 处的默认示例端点，定价页据此选默认展示的示例。image-to-image 是仅编辑模型
// （走 /v1/images/edits，没有对应端点类型），刻意不声明该端点，否则它又会像
// "qwen-image" 子串那样被当成文生图模型被调用并收到上游 404 响应。
// 标签必须整项相等：text-to-imagex 不算命中。
func TestPricingModelTagDeclaresImageGenerationEndpoint(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 601, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	models := []string{
		"qwen-image-2512",
		"qwen-image-edit-2511",
		"plain-chat-model",
		"both-modes-model",
	}
	for _, name := range models {
		insertPricingEndpointAbility(t, 601, name)
	}
	for _, meta := range []Model{
		{ModelName: "qwen-image-2512", Tags: "text-to-image,hot", Status: 1, NameRule: NameRuleExact},
		{ModelName: "qwen-image-edit-2511", Tags: "image-to-image", Status: 1, NameRule: NameRuleExact},
		{ModelName: "plain-chat-model", Tags: "text-to-imagex", Status: 1, NameRule: NameRuleExact},
		{ModelName: "both-modes-model", Tags: "text-to-image,image-to-image", Status: 1, NameRule: NameRuleExact},
	} {
		require.NoError(t, DB.Create(&meta).Error)
	}

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeImageGeneration,
		constant.EndpointTypeOpenAI,
	}, byModel["qwen-image-2512"])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, byModel["qwen-image-edit-2511"])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, byModel["plain-chat-model"])
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeImageGeneration,
		constant.EndpointTypeOpenAI,
	}, byModel["both-modes-model"])
}
