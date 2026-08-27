package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJsonRawMessageToString(t *testing.T) {
	tests := []struct {
		name string
		data json.RawMessage
		want string
	}{
		{
			name: "object",
			data: json.RawMessage(`{"city":"Paris","days":0,"strict":false}`),
			want: `{"city":"Paris","days":0,"strict":false}`,
		},
		{
			name: "string",
			data: json.RawMessage(`"{\"city\":\"Paris\",\"days\":0,\"strict\":false}"`),
			want: `{"city":"Paris","days":0,"strict":false}`,
		},
		{
			name: "null",
			data: json.RawMessage(`null`),
			want: "",
		},
		{
			name: "empty",
			data: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, JsonRawMessageToString(tt.data))
		})
	}
}

func TestMarshalDoesNotHTMLEscape(t *testing.T) {
	// 网关 JSON 只被 API 客户端解析、从不嵌入 HTML，因此 Marshal 不得把
	// &、<、> 转义成 &/</> —— 否则带签名的 video_url 等
	// 含查询串的 URL 在客户端按原样提取时会带上转义字符。
	value := struct {
		URL   string `json:"url"`
		Other string `json:"other"`
	}{
		URL:   "https://example.com/v.mp4?a=1&b=2&sig=x",
		Other: "a < b > c",
	}

	data, err := Marshal(value)
	require.NoError(t, err)
	s := string(data)
	// Go 默认 encoder 会把 &、<、> 输出为含反斜杠的 HTML 转义序列
	// （反斜杠 + "u0026"/"u003c"/"u003e"）。反斜杠用 rune(92) 构造，
	// 避免测试源码里的反斜杠序列在写入文件时被意外转义。
	backslash := string(rune(92))
	require.NotContains(t, s, backslash+"u0026")
	require.NotContains(t, s, backslash+"u003c")
	require.NotContains(t, s, backslash+"u003e")
	require.Contains(t, string(data), "a=1&b=2&sig=x")
	require.Contains(t, string(data), "a < b > c")

	// 输出仍是合法 JSON，解析后与原值一致。
	var got struct {
		URL   string `json:"url"`
		Other string `json:"other"`
	}
	require.NoError(t, Unmarshal(data, &got))
	require.Equal(t, value, got)
}
