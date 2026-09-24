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
package toolconv

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Responses 客户端(如 Codex)会发出没有 parameters 的 function 工具。转到 chat 时
// 必须补成下游可用的 schema: 缺 parameters、空对象、以及只有 additionalProperties
// 这类不完整 schema，都会被要求非空 input_schema 的上游(Bedrock)拒绝。
func TestAttachOpenAIChatNormalizesFunctionToolParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		parameters any
		want       map[string]any
	}{
		{
			name:       "omitted parameters",
			parameters: nil,
			want: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			name:       "empty parameters object",
			parameters: map[string]any{},
			want: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			name:       "missing type and properties",
			parameters: map[string]any{"additionalProperties": false},
			want: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			name: "valid schema preserved",
			parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"q": map[string]any{"type": "string"}},
				"required":   []any{"q"},
			},
			want: map[string]any{
				"type":       "object",
				"properties": map[string]any{"q": map[string]any{"type": "string"}},
				"required":   []any{"q"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool := map[string]any{"type": "function", "name": "lookup", "description": "Lookup data"}
			if tt.parameters != nil {
				tool["parameters"] = tt.parameters
			}
			tools, err := kitutil.Marshal([]map[string]any{tool})
			require.NoError(t, err)

			_, set, err := ExtractRequest(types.RelayFormatOpenAIResponses, &dto.OpenAIResponsesRequest{
				Model: "claude-opus-5",
				Tools: tools,
			})
			require.NoError(t, err)

			target := &dto.GeneralOpenAIRequest{Model: "claude-opus-5"}
			out, _, err := AttachRequest(types.RelayFormatOpenAI, target, set, &convmeta.Options{})
			require.NoError(t, err)
			got, ok := out.(*dto.GeneralOpenAIRequest)
			require.True(t, ok)

			require.Len(t, got.Tools, 1)
			assert.Equal(t, "function", got.Tools[0].Type)
			assert.Equal(t, "lookup", got.Tools[0].Function.Name)
			assert.Equal(t, tt.want, got.Tools[0].Function.Parameters)
		})
	}
}

// 同上，但目标格式是 Responses: 缺 parameters 的工具会以 "parameters": null 落到报文里，
// 而 Responses 的 function 工具要求 parameters 是可用的 JSON Schema。
func TestAttachOpenAIResponsesNormalizesFunctionToolParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		parameters any
		want       map[string]any
	}{
		{
			name:       "omitted parameters",
			parameters: nil,
			want: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			name:       "empty parameters object",
			parameters: map[string]any{},
			want: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			name:       "missing type and properties",
			parameters: map[string]any{"additionalProperties": false},
			want: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			name: "valid schema preserved",
			parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"q": map[string]any{"type": "string"}},
				"required":   []any{"q"},
			},
			want: map[string]any{
				"type":       "object",
				"properties": map[string]any{"q": map[string]any{"type": "string"}},
				"required":   []any{"q"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, set, err := ExtractRequest(types.RelayFormatOpenAI, &dto.GeneralOpenAIRequest{
				Model:    "gpt-test",
				Messages: []dto.Message{{Role: "user", Content: "hi"}},
				Tools: []dto.ToolCallRequest{{
					Type: "function",
					Function: dto.FunctionRequest{
						Name:        "lookup",
						Description: "Lookup data",
						Parameters:  tt.parameters,
					},
				}},
			})
			require.NoError(t, err)

			target := &dto.OpenAIResponsesRequest{Model: "gpt-test"}
			out, _, err := AttachRequest(types.RelayFormatOpenAIResponses, target, set, &convmeta.Options{})
			require.NoError(t, err)
			got, ok := out.(*dto.OpenAIResponsesRequest)
			require.True(t, ok)

			var tools []map[string]any
			require.NoError(t, kitutil.Unmarshal(got.Tools, &tools))
			require.Len(t, tools, 1)
			assert.Equal(t, "function", tools[0]["type"])
			assert.Equal(t, "lookup", tools[0]["name"])
			assert.Equal(t, tt.want, tools[0]["parameters"])
		})
	}
}
