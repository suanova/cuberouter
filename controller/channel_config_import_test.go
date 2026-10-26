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

package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/channelconfig"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildChannelFromConfigMapsDeclarativeFields(t *testing.T) {
	raw := `
version: 1
channels:
  - name: cube-stack-main
    type: cubestack
    base_url: https://cube.example.com
    keys: [sk-one, sk-two]
    group: default,vip
    priority: 7
    weight: 3
    status: 0
    tag: local
    remark: main channel
    test_model: qwen3-32b
    auto_ban: 0
    multi_key_mode: polling
    model_mapping:
      public-model: upstream-model
    overrides:
      temperature: 0.7
      operations:
        - mode: set
          path: output_config.effort
          value: low
          logic: AND
          conditions:
            - path: model
              mode: full
              value: qwen3-32b
    header_override:
      X-Test: abc
    proxy: http://127.0.0.1:7890
    settings:
      force_format: true
    other_settings:
      upstream_model_update_check_enabled: true
    models:
      fetch: false
      list: [qwen3-32b]
`
	config, err := channelconfig.Parse([]byte(raw))
	require.NoError(t, err)
	require.Len(t, config.Channels, 1)

	channel, err := buildChannelFromConfig(&config.Channels[0])
	require.NoError(t, err)

	assert.Equal(t, constant.ChannelTypeCubeStack, channel.Type)
	assert.Equal(t, "cube-stack-main", channel.Name)
	assert.Equal(t, "https://cube.example.com", channel.GetBaseURL())
	assert.Equal(t, "sk-one\nsk-two", channel.Key)
	assert.Equal(t, "default,vip", channel.Group)
	assert.EqualValues(t, 7, *channel.Priority)
	assert.EqualValues(t, 3, *channel.Weight)
	assert.Equal(t, 0, channel.Status)
	require.NotNil(t, channel.Tag)
	assert.Equal(t, "local", *channel.Tag)
	require.NotNil(t, channel.Remark)
	assert.Equal(t, "main channel", *channel.Remark)
	require.NotNil(t, channel.TestModel)
	assert.Equal(t, "qwen3-32b", *channel.TestModel)
	require.NotNil(t, channel.AutoBan)
	assert.Equal(t, 0, *channel.AutoBan)

	assert.True(t, channel.ChannelInfo.IsMultiKey)
	assert.Equal(t, 2, channel.ChannelInfo.MultiKeySize)
	assert.Equal(t, constant.MultiKeyModePolling, channel.ChannelInfo.MultiKeyMode)

	require.NotNil(t, channel.ModelMapping)
	var mapping map[string]string
	require.NoError(t, common.UnmarshalJsonStr(*channel.ModelMapping, &mapping))
	assert.Equal(t, "upstream-model", mapping["public-model"])

	require.NotNil(t, channel.ParamOverride)
	var paramOverride map[string]any
	require.NoError(t, common.UnmarshalJsonStr(*channel.ParamOverride, &paramOverride))
	assert.Equal(t, 0.7, paramOverride["temperature"])
	operations, ok := paramOverride["operations"].([]any)
	require.True(t, ok)
	require.Len(t, operations, 1)

	require.NotNil(t, channel.HeaderOverride)
	var headerOverride map[string]any
	require.NoError(t, common.UnmarshalJsonStr(*channel.HeaderOverride, &headerOverride))
	assert.Equal(t, "abc", headerOverride["X-Test"])

	require.NotNil(t, channel.Setting)
	var settings map[string]any
	require.NoError(t, common.UnmarshalJsonStr(*channel.Setting, &settings))
	assert.Equal(t, "http://127.0.0.1:7890", settings["proxy"])
	assert.Equal(t, true, settings["force_format"])

	assert.NotEmpty(t, channel.OtherSettings)
}

func TestBuildChannelFromConfigSingleKey(t *testing.T) {
	raw := `
version: 1
channels:
  - name: single
    type: openai
    key: sk-test
    models:
      fetch: false
      list: [gpt-4o]
`
	config, err := channelconfig.Parse([]byte(raw))
	require.NoError(t, err)
	channel, err := buildChannelFromConfig(&config.Channels[0])
	require.NoError(t, err)

	assert.False(t, channel.ChannelInfo.IsMultiKey)
	assert.Equal(t, constant.ChannelTypeOpenAI, channel.Type)
	require.NotNil(t, channel.BaseURL)
	assert.Equal(t, constant.GetChannelBaseURL(constant.ChannelTypeOpenAI), *channel.BaseURL)
	require.NotNil(t, channel.ParamOverride)
	assert.Equal(t, "{}", *channel.ParamOverride)
}

func TestBuildPricePlanResolvesConfiguredPrices(t *testing.T) {
	ratio_setting.InitRatioSettings()
	raw := `
version: 1
channels:
  - name: primary
    type: cubestack
    base_url: http://127.0.0.1:8000
    key: sk-test
    models:
      fetch: false
      list: [priced-model]
    pricing:
      defaults:
        input: 2
        output: 4
  - name: secondary
    type: cubestack
    base_url: http://127.0.0.1:8001
    key: sk-test
    models:
      fetch: false
      list: [unpriced-model-xyz]
pricing:
  models:
    priced-model:
      input: 4
      output: 8
`
	config, err := channelconfig.Parse([]byte(raw))
	require.NoError(t, err)
	prepared := []*preparedChannelConfig{
		{source: &config.Channels[0], models: []string{"priced-model"}},
		{source: &config.Channels[1], models: []string{"unpriced-model-xyz"}},
	}

	plan, warnings, err := buildPricePlan(config, prepared)
	require.NoError(t, err)
	require.Contains(t, plan, "priced-model")
	assert.Equal(t, 2.0, plan["priced-model"].ModelRatio)
	assert.Equal(t, 2.0, plan["priced-model"].CompletionRatio)
	assert.NotContains(t, plan, "unpriced-model-xyz")

	assert.Contains(t, warnings, `model "unpriced-model-xyz" has no configured price and no built-in price; requests for it fail with "model price is not configured"`)
}

func TestBuildPricePlanWarnsAboutCrossChannelPriceConflicts(t *testing.T) {
	ratio_setting.InitRatioSettings()
	raw := `
version: 1
channels:
  - name: first
    type: cubestack
    base_url: http://127.0.0.1:8000
    key: sk-test
    models:
      fetch: false
      list: [shared-model]
    pricing:
      defaults:
        input: 2
        output: 4
  - name: second
    type: cubestack
    base_url: http://127.0.0.1:8001
    key: sk-test
    models:
      fetch: false
      list: [shared-model]
    pricing:
      defaults:
        input: 3
        output: 6
`
	config, err := channelconfig.Parse([]byte(raw))
	require.NoError(t, err)
	prepared := []*preparedChannelConfig{
		{source: &config.Channels[0], models: []string{"shared-model"}},
		{source: &config.Channels[1], models: []string{"shared-model"}},
	}

	plan, warnings, err := buildPricePlan(config, prepared)
	require.NoError(t, err)
	assert.Equal(t, 1.5, plan["shared-model"].ModelRatio)
	assert.Contains(t, warnings, `model "shared-model" is priced differently by channels "first" and "second"; the later channel wins`)
}
