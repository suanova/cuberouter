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

package channelconfig

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pointerTo[T any](value T) *T {
	return &value
}

func TestParseNormalizesAndConvertsPrices(t *testing.T) {
	t.Setenv("TEST_CUBESTACK_KEY", "sk-from-env")
	raw := `
version: 1
channels:
  - name: cubestack-local
    type: cubestack
    base_url: http://127.0.0.1:8000
    key_env: TEST_CUBESTACK_KEY
    models:
      fetch: true
      include: ["qwen*", "glm-*"]
      exclude: ["*-embedding"]
    overrides:
      temperature: 0.7
    pricing:
      defaults:
        input: 0.5
        output: 1.5
        cache_read: 0.05
pricing:
  models:
    qwen3-32b:
      input: 0.2
      output: 0.8
`
	config, err := Parse([]byte(raw))
	require.NoError(t, err)

	require.Len(t, config.Channels, 1)
	channel := config.Channels[0]
	assert.Equal(t, "cubestack-local", channel.Name)
	assert.Equal(t, constant.ChannelTypeCubeStack, mustChannelType(t, channel.Type))
	assert.Equal(t, "default", channel.Group)
	require.NotNil(t, channel.Models.Fetch)
	assert.True(t, *channel.Models.Fetch)
	assert.Equal(t, []string{"sk-from-env"}, channel.ChannelKeys())

	plan, unknown, err := BuildPricePlan(
		[]string{"qwen3-32b", "glm-4.6"},
		[]PriceLayer{
			{Defaults: config.Channels[0].Pricing.Defaults},
			{Models: config.Pricing.Models},
		},
	)
	require.NoError(t, err)
	assert.Empty(t, unknown)

	qwen := plan["qwen3-32b"]
	assert.Equal(t, 0.1, qwen.ModelRatio)
	assert.Equal(t, 4.0, qwen.CompletionRatio)
	assert.Nil(t, qwen.CacheRatio)

	glm := plan["glm-4.6"]
	assert.Equal(t, 0.25, glm.ModelRatio)
	assert.Equal(t, 3.0, glm.CompletionRatio)
	require.NotNil(t, glm.CacheRatio)
	assert.Equal(t, 0.1, *glm.CacheRatio)
}

func mustChannelType(t *testing.T, value any) int {
	t.Helper()
	channelType, err := ResolveChannelType(value)
	require.NoError(t, err)
	return channelType
}

func TestResolveChannelType(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    int
		wantErr bool
	}{
		{name: "numeric id", value: 64, want: constant.ChannelTypeCubeStack},
		{name: "numeric string", value: "64", want: constant.ChannelTypeCubeStack},
		{name: "name", value: "CubeStack", want: constant.ChannelTypeCubeStack},
		{name: "normalized name", value: "cube-stack", want: constant.ChannelTypeCubeStack},
		{name: "provider name", value: "OpenAI", want: constant.ChannelTypeOpenAI},
		{name: "alias", value: "claude", want: constant.ChannelTypeAnthropic},
		{name: "json number", value: float64(1), want: constant.ChannelTypeOpenAI},
		{name: "missing", value: nil, wantErr: true},
		{name: "unknown name", value: "not-a-provider", wantErr: true},
		{name: "zero", value: 0, wantErr: true},
		{name: "out of range", value: 99, wantErr: true},
		{name: "fractional", value: 1.5, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveChannelType(test.value)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestBuildPricePlanPrecedence(t *testing.T) {
	globalDefault := &Price{Input: pointerTo(1.0), Output: pointerTo(2.0)}
	channelDefault := &Price{Input: pointerTo(2.0), Output: pointerTo(6.0)}
	channelModel := Price{Input: pointerTo(3.0), Output: pointerTo(15.0)}
	globalModel := Price{Input: pointerTo(4.0), Output: pointerTo(20.0)}

	models := []string{"alpha", "beta", "gamma"}
	plan, unknown, err := BuildPricePlan(models, []PriceLayer{
		{Defaults: globalDefault},
		{Defaults: channelDefault},
		{Models: map[string]Price{"beta": channelModel}},
		{Models: map[string]Price{"gamma": globalModel, "absent": globalModel}},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"absent"}, unknown)
	require.Len(t, plan, 3)

	assert.Equal(t, 1.0, plan["alpha"].ModelRatio)
	assert.Equal(t, 1.5, plan["beta"].ModelRatio)
	assert.Equal(t, 5.0, plan["beta"].CompletionRatio)
	assert.Equal(t, 2.0, plan["gamma"].ModelRatio)
	assert.Equal(t, 5.0, plan["gamma"].CompletionRatio)
}

func TestModelPriceToPriceRoundTrip(t *testing.T) {
	price := Price{
		Input:      pointerTo(0.5),
		Output:     pointerTo(1.5),
		CacheRead:  pointerTo(0.05),
		CacheWrite: pointerTo(0.125),
	}
	converted, err := price.toModelPrice()
	require.NoError(t, err)
	restored := converted.ToPrice()
	require.NotNil(t, restored.Input)
	require.NotNil(t, restored.Output)
	require.NotNil(t, restored.CacheRead)
	require.NotNil(t, restored.CacheWrite)
	assert.InDelta(t, 0.5, *restored.Input, 1e-12)
	assert.InDelta(t, 1.5, *restored.Output, 1e-12)
	assert.InDelta(t, 0.05, *restored.CacheRead, 1e-12)
	assert.InDelta(t, 0.125, *restored.CacheWrite, 1e-12)

	perRequest, err := (Price{PerRequest: pointerTo(0.04)}).toModelPrice()
	require.NoError(t, err)
	restored = perRequest.ToPrice()
	require.NotNil(t, restored.PerRequest)
	assert.Equal(t, 0.04, *restored.PerRequest)
	assert.Nil(t, restored.Input)
}

func TestPerRequestPriceConversion(t *testing.T) {
	price := Price{PerRequest: pointerTo(0.04)}
	converted, err := price.toModelPrice()
	require.NoError(t, err)
	require.NotNil(t, converted.PerRequest)
	assert.Equal(t, 0.04, *converted.PerRequest)
	assert.Zero(t, converted.ModelRatio)

	conflicting := Price{PerRequest: pointerTo(0.04), Input: pointerTo(1.0), Output: pointerTo(2.0)}
	_, err = conflicting.toModelPrice()
	require.ErrorContains(t, err, "per_request cannot be combined")

	negative := Price{PerRequest: pointerTo(-1.0)}
	_, err = negative.toModelPrice()
	require.ErrorContains(t, err, "positive")
}

func TestFilterModels(t *testing.T) {
	fetched := []string{"qwen3-32b", "qwen3-embedding", "glm-4.6", "gpt-4o", " qwen3-32b "}
	explicit := []string{"manual-only", "gpt-4o"}

	models, err := FilterModels(fetched, explicit, []string{"qwen*", "gpt-*"}, []string{"*-embedding"})
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-4o", "manual-only", "qwen3-32b"}, models)

	models, err = FilterModels(nil, explicit, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-4o", "manual-only"}, models)
}

func TestParseRejectsInvalidConfigs(t *testing.T) {
	base := `
version: 1
channels:
  - name: cubestack-local
    type: cubestack
    base_url: http://127.0.0.1:8000
    key: sk-test
    models:
      fetch: false
      list: [qwen3-32b]
`
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name:    "unknown field",
			raw:     base + "    unknown_field: true\n",
			wantErr: "unknown_field",
		},
		{
			name:    "missing key",
			raw:     "version: 1\nchannels:\n  - name: no-key\n    type: cubestack\n    base_url: http://127.0.0.1:8000\n    models:\n      fetch: false\n      list: [a]\n",
			wantErr: "key is required",
		},
		{
			name:    "missing base url",
			raw:     "version: 1\nchannels:\n  - name: no-url\n    type: cubestack\n    key: sk-test\n    models:\n      fetch: false\n      list: [a]\n",
			wantErr: "base_url is required",
		},
		{
			name:    "key and keys",
			raw:     "version: 1\nchannels:\n  - name: both\n    type: openai\n    key: sk-a\n    keys: [sk-b]\n    models:\n      fetch: false\n      list: [a]\n",
			wantErr: "use only one of key, keys, key_env",
		},
		{
			name:    "no models",
			raw:     "version: 1\nchannels:\n  - name: empty\n    type: openai\n    key: sk-a\n    models:\n      fetch: false\n",
			wantErr: "models.list is required",
		},
		{
			name:    "duplicate names",
			raw:     "version: 1\nchannels:\n  - name: same\n    type: openai\n    key: sk-a\n    models:\n      fetch: false\n      list: [a]\n  - name: same\n    type: openai\n    key: sk-b\n    models:\n      fetch: false\n      list: [a]\n",
			wantErr: "duplicate channel name",
		},
		{
			name:    "operations not a list",
			raw:     base + "    overrides:\n      operations: {}\n",
			wantErr: "operations must be a list",
		},
		{
			name:    "operation missing mode",
			raw:     base + "    overrides:\n      operations:\n        - path: output_config.effort\n          value: low\n",
			wantErr: "operations[0].mode is required",
		},
		{
			name:    "operation bad logic",
			raw:     base + "    overrides:\n      operations:\n        - path: output_config.effort\n          mode: set\n          logic: XOR\n",
			wantErr: "logic must be AND or OR",
		},
		{
			name:    "condition missing path",
			raw:     base + "    overrides:\n      operations:\n        - path: output_config.effort\n          mode: set\n          conditions:\n            - mode: full\n              value: high\n",
			wantErr: "conditions[0].path is required",
		},
		{
			name:    "incomplete price",
			raw:     base + "    pricing:\n      defaults:\n        input: 1\n",
			wantErr: "input and output prices are required",
		},
		{
			name:    "unsupported version",
			raw:     "version: 9\n",
			wantErr: "unsupported config version",
		},
		{
			name:    "no channels",
			raw:     "version: 1\n",
			wantErr: "config defines no channels",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.raw))
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestParseOperationsOverrides(t *testing.T) {
	raw := `
version: 1
channels:
  - name: cubestack-local
    type: cubestack
    base_url: http://127.0.0.1:8000
    key: sk-test
    models:
      fetch: false
      list: [qwen38-27b]
    overrides:
      operations:
        - mode: set
          path: output_config.effort
          value: low
          logic: AND
          conditions:
            - path: model
              mode: full
              value: qwen38-27b
            - path: output_config.effort
              mode: full
              value: high
`
	config, err := Parse([]byte(raw))
	require.NoError(t, err)
	override := config.Channels[0].Overrides
	require.NotNil(t, override)
	operations, ok := override["operations"].([]any)
	require.True(t, ok)
	require.Len(t, operations, 1)
	operation, ok := operations[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "set", operation["mode"])
	assert.Equal(t, "output_config.effort", operation["path"])
	assert.Equal(t, "AND", operation["logic"])
}

func TestExampleConfigParses(t *testing.T) {
	t.Setenv("CUBESTACK_API_KEY", "sk-example")
	data, err := os.ReadFile("../../docs/channel/channels.example.yaml")
	require.NoError(t, err)
	config, err := Parse(data)
	require.NoError(t, err)
	require.NotEmpty(t, config.Channels)
}

func TestAllChannelTypeNamesResolve(t *testing.T) {
	for channelType, name := range constant.ChannelTypeNames {
		if channelType == constant.ChannelTypeUnknown {
			continue
		}
		got, err := ResolveChannelType(name)
		require.NoError(t, err, name)
		assert.Equal(t, channelType, got, name)
	}
}
