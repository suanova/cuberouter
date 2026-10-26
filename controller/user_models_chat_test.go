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

	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/require"
)

// TestIsChatCapableEndpoints 锁定 Playground 的模型过滤语义：图片/视频生成模型
// 即使挂在 OpenAI 渠道上带着 openai 端点也不能聊天，responses-only 与 embedding
// 模型同样走不了 chat/completions。
func TestIsChatCapableEndpoints(t *testing.T) {
	cases := []struct {
		name      string
		endpoints []constant.EndpointType
		want      bool
	}{
		{name: "openai chat", endpoints: []constant.EndpointType{constant.EndpointTypeOpenAI}, want: true},
		{name: "anthropic convertible", endpoints: []constant.EndpointType{constant.EndpointTypeAnthropic}, want: true},
		{name: "gemini convertible", endpoints: []constant.EndpointType{constant.EndpointTypeGemini}, want: true},
		{name: "multimodal chat stays", endpoints: []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse}, want: true},
		{
			name:      "image model on openai channel",
			endpoints: []constant.EndpointType{constant.EndpointTypeImageGeneration, constant.EndpointTypeOpenAI},
			want:      false,
		},
		{name: "video model", endpoints: []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, want: false},
		{name: "ark video model", endpoints: []constant.EndpointType{constant.EndpointTypeArkVideo}, want: false},
		{name: "task video model", endpoints: []constant.EndpointType{constant.EndpointTypeVideoGeneration}, want: false},
		{name: "responses only", endpoints: []constant.EndpointType{constant.EndpointTypeOpenAIResponse}, want: false},
		{name: "embeddings only", endpoints: []constant.EndpointType{constant.EndpointTypeEmbeddings}, want: false},
		{name: "rerank only", endpoints: []constant.EndpointType{constant.EndpointTypeJinaRerank}, want: false},
		{name: "unknown model", endpoints: nil, want: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isChatCapableEndpoints(tt.endpoints))
		})
	}
}
