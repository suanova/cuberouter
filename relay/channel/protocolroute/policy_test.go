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
package protocolroute

import (
	"bytes"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allModelProtocols 是四条文本协议的固定顺序, 用于逐条断言策略判定。
var allModelProtocols = []string{
	dto.ModelProtocolChat,
	dto.ModelProtocolResponses,
	dto.ModelProtocolMessages,
	dto.ModelProtocolGemini,
}

func newInfo(channelID int, model string, protocols map[string][]string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:            channelID,
			UpstreamModelName:    model,
			ChannelOtherSettings: dto.ChannelOtherSettings{ModelProtocols: protocols},
		},
		OriginModelName: model,
	}
}

// TestUseNativeProtocolBaseline 锁定"基线是策略参数"这件事本身: 同一个判定函数,
// AstraFlow 的未声明基线只有 responses(保持历史行为), CubeStack 的是三条文本协议
// 全部原生。未声明该模型、以及 ChannelMeta 缺失(手工构造的 RelayInfo)都按基线走。
func TestUseNativeProtocolBaseline(t *testing.T) {
	tests := []struct {
		name       string
		policy     Policy
		wantNative []string
	}{
		{
			name:       "astraflow keeps the responses-only baseline",
			policy:     Policy{NativeWhenUndeclared: []string{dto.ModelProtocolResponses}},
			wantNative: []string{dto.ModelProtocolResponses},
		},
		{
			name: "cubestack serves every text protocol natively",
			policy: Policy{NativeWhenUndeclared: []string{
				dto.ModelProtocolChat, dto.ModelProtocolResponses, dto.ModelProtocolMessages,
			}},
			wantNative: []string{
				dto.ModelProtocolChat, dto.ModelProtocolResponses, dto.ModelProtocolMessages,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unconfigured := newInfo(101, "qwen38-27b", nil)
			// 声明了别的模型, 本次模型未命中声明, 同样回到基线。
			otherDeclared := newInfo(102, "qwen38-27b", map[string][]string{"glm-*": {dto.ModelProtocolChat}})

			for _, protocol := range allModelProtocols {
				want := slices.Contains(tt.wantNative, protocol)
				assert.Equal(t, want, tt.policy.useNativeProtocol(unconfigured, protocol),
					"an unconfigured channel must follow the baseline for %s", protocol)
				assert.Equal(t, want, tt.policy.useNativeProtocol(otherDeclared, protocol),
					"a model missing from the declaration must follow the baseline for %s", protocol)
				assert.Equal(t, want, tt.policy.useNativeProtocol(&relaycommon.RelayInfo{}, protocol),
					"a RelayInfo without channel meta must follow the baseline for %s", protocol)
			}
		})
	}
}

// TestUseNativeProtocolDeclarationOverridesBaseline 锁定: 一旦模型命中声明, 判定
// 只看声明——包括声明把某个协议从基线里**拿掉**(astraflow 的 responses 基线也能被
// 覆盖), 以及空数组表示该模型没有文本协议。
func TestUseNativeProtocolDeclarationOverridesBaseline(t *testing.T) {
	ResetDowngradeMemoForTest()
	t.Cleanup(ResetDowngradeMemoForTest)

	astraflow := Policy{NativeWhenUndeclared: []string{dto.ModelProtocolResponses}}
	cubestack := Policy{NativeWhenUndeclared: []string{
		dto.ModelProtocolChat, dto.ModelProtocolResponses, dto.ModelProtocolMessages,
	}}

	chatOnly := newInfo(103, "qwen38-27b", map[string][]string{"qwen38-27b": {dto.ModelProtocolChat}})
	for _, policy := range []Policy{astraflow, cubestack} {
		assert.True(t, policy.useNativeProtocol(chatOnly, dto.ModelProtocolChat))
		assert.False(t, policy.useNativeProtocol(chatOnly, dto.ModelProtocolMessages))
		assert.False(t, policy.useNativeProtocol(chatOnly, dto.ModelProtocolResponses),
			"a declaration must be able to remove a protocol the baseline would send natively")
	}

	noProtocols := newInfo(104, "qwen38-27b", map[string][]string{"qwen38-27b": {}})
	assert.False(t, cubestack.useNativeProtocol(noProtocols, dto.ModelProtocolChat),
		"an empty declaration means the model has no text protocol")
}

// TestDowngradeWatermarkKeepsInFlightRequestsNative 锁定水位线语义: 降级只对
// "记录之后才开始"的请求生效, 记录之前已在途的请求保持它发出时的原生决定。
func TestDowngradeWatermarkKeepsInFlightRequestsNative(t *testing.T) {
	ResetDowngradeMemoForTest()
	t.Cleanup(ResetDowngradeMemoForTest)

	policy := Policy{NativeWhenUndeclared: []string{dto.ModelProtocolMessages}}
	info := newInfo(105, "glm-5.3", map[string][]string{"glm-*": {dto.ModelProtocolMessages}})
	require.True(t, policy.useNativeProtocol(info, dto.ModelProtocolMessages))

	// 另一个请求刚刚因协议拒绝写入记忆。
	nativeProtocolDowngradeMemo.Store(nativeProtocolMemoKey(info, dto.ModelProtocolMessages), time.Now())

	info.StartTime = time.Now().Add(time.Second)
	assert.False(t, policy.useNativeProtocol(info, dto.ModelProtocolMessages),
		"a request started after the recording must downgrade")

	info.StartTime = time.Now().Add(-time.Second)
	assert.True(t, policy.useNativeProtocol(info, dto.ModelProtocolMessages),
		"a request already in flight when the recording happened must keep the native protocol")

	info.StartTime = time.Time{}
	assert.False(t, policy.useNativeProtocol(info, dto.ModelProtocolMessages),
		"with a zero StartTime the recording applies immediately")
}

// TestDowngradeUndeclaredGovernsTheBaseline 锁定 DowngradeUndeclared 这一处策略差异:
// 未声明模型的基线对 AstraFlow 是硬承诺(历史语义,拒绝不改变路由),对 CubeStack 是
// "上游应该原生支持"的推断(拒绝即证伪,该组合转 chat)。两个策略的 NativeWhenUndeclared
// 取值相同,差别只在这一个开关上。
func TestDowngradeUndeclaredGovernsTheBaseline(t *testing.T) {
	ResetDowngradeMemoForTest()
	t.Cleanup(ResetDowngradeMemoForTest)

	promise := Policy{NativeWhenUndeclared: []string{dto.ModelProtocolResponses}}
	inference := Policy{NativeWhenUndeclared: []string{dto.ModelProtocolResponses}, DowngradeUndeclared: true}

	undeclared := newInfo(108, "qwen38-27b", nil)
	nativeProtocolDowngradeMemo.Store(nativeProtocolMemoKey(undeclared, dto.ModelProtocolResponses), time.Now())

	assert.True(t, promise.useNativeProtocol(undeclared, dto.ModelProtocolResponses),
		"a baseline that is a promise must ignore the downgrade memo")
	assert.False(t, inference.useNativeProtocol(undeclared, dto.ModelProtocolResponses),
		"a baseline that is an inference must downgrade once the upstream rejected it")
	assert.False(t, inference.useNativeProtocol(undeclared, dto.ModelProtocolMessages),
		"a protocol outside the baseline stays non-native regardless of the memo")
}

// TestDowngradeMemoIsScopedToTheChannel 锁定降级记忆的键含渠道 ID: 这正是多个渠道
// 类型能共用同一张表的原因——一个渠道上的协议拒绝不得影响另一个渠道。
func TestDowngradeMemoIsScopedToTheChannel(t *testing.T) {
	ResetDowngradeMemoForTest()
	t.Cleanup(ResetDowngradeMemoForTest)

	policy := Policy{NativeWhenUndeclared: []string{dto.ModelProtocolMessages}}
	declared := map[string][]string{"glm-*": {dto.ModelProtocolMessages}}

	rejected := newInfo(106, "glm-5.3", declared)
	nativeProtocolDowngradeMemo.Store(nativeProtocolMemoKey(rejected, dto.ModelProtocolMessages), time.Now())
	assert.False(t, policy.useNativeProtocol(rejected, dto.ModelProtocolMessages))

	other := newInfo(107, "glm-5.3", declared)
	assert.True(t, policy.useNativeProtocol(other, dto.ModelProtocolMessages),
		"another channel serving the same model must stay native")
}

// TestGetRequestURLMessagesPathPrefix 锁定 MessagesPathPrefix 的作用范围: 只有
// 原生 messages 会话在渠道 base URL 后追加前缀(保留客户端查询串), chat、
// responses 以及被声明降级成 chat 的 messages 仍走渠道 base URL。
func TestGetRequestURLMessagesPathPrefix(t *testing.T) {
	ResetDowngradeMemoForTest()
	t.Cleanup(ResetDowngradeMemoForTest)

	policy := Policy{
		ChannelLabel:           "cubestack adaptor",
		NativeWhenUndeclared:   []string{dto.ModelProtocolChat, dto.ModelProtocolResponses, dto.ModelProtocolMessages},
		AllowCleartextUpstream: true,
		MessagesPathPrefix:     "/anthropic",
	}
	messagesInfo := func(channelID int, protocols map[string][]string) *relaycommon.RelayInfo {
		info := newInfo(channelID, "qwen38-27b", protocols)
		info.ChannelBaseUrl = "http://model.internal:5001"
		info.RelayFormat = types.RelayFormatClaude
		info.RequestURLPath = "/v1/messages?beta=true"
		return info
	}

	t.Run("native messages get the prefix", func(t *testing.T) {
		url, err := policy.GetRequestURL(messagesInfo(113, nil))
		require.NoError(t, err)
		assert.Equal(t, "http://model.internal:5001/anthropic/v1/messages?beta=true", url)
	})

	t.Run("an empty prefix keeps the client path", func(t *testing.T) {
		plain := policy
		plain.MessagesPathPrefix = ""
		url, err := plain.GetRequestURL(messagesInfo(114, nil))
		require.NoError(t, err)
		assert.Equal(t, "http://model.internal:5001/v1/messages?beta=true", url)
	})

	t.Run("chat is unaffected by the prefix", func(t *testing.T) {
		info := newInfo(115, "qwen38-27b", nil)
		info.ChannelBaseUrl = "http://model.internal:5001"
		info.RelayFormat = types.RelayFormatOpenAI
		info.RequestURLPath = "/v1/chat/completions"
		url, err := policy.GetRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "http://model.internal:5001/v1/chat/completions", url)
	})

	t.Run("downgraded messages are unaffected", func(t *testing.T) {
		info := messagesInfo(116, map[string][]string{"qwen38-27b": {dto.ModelProtocolChat}})
		url, err := policy.GetRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "http://model.internal:5001/v1/chat/completions", url)
	})
}

// TestDetectRejectedNativeProtocolTreatsNotFoundAsRejection 锁定 404 的语义: 原生
// 协议端点不存在时, FastAPI 类上游只回 {"detail":"Not Found"} 这类文案, 不命中任何
// 错误签名。404 只可能来自本次按原生协议打出的端点, 因此它本身就是"该协议不存在"的
// 证据, 必须记入降级记忆, 否则 Claude Code 这类只用 /v1/messages 的客户端会每次撞 404。
func TestDetectRejectedNativeProtocolTreatsNotFoundAsRejection(t *testing.T) {
	ResetDowngradeMemoForTest()
	t.Cleanup(ResetDowngradeMemoForTest)

	policy := Policy{
		NativeWhenUndeclared: []string{dto.ModelProtocolMessages},
		DowngradeUndeclared:  true,
	}
	newMessagesInfo := func() *relaycommon.RelayInfo {
		info := newInfo(109, "qwen38-27b", nil)
		info.RelayFormat = types.RelayFormatClaude
		info.RequestURLPath = "/v1/messages"
		return info
	}

	t.Run("404 records the downgrade and restores the body", func(t *testing.T) {
		ResetDowngradeMemoForTest()
		body := []byte(`{"detail":"Not Found"}`)
		resp := &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader(body))}

		policy.DetectRejectedNativeProtocol(newMessagesInfo(), resp)

		assert.False(t, policy.useNativeProtocol(newInfo(109, "qwen38-27b", nil), dto.ModelProtocolMessages))
		got, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, body, got, "the peek must not consume the upstream body")
	})

	t.Run("an unrelated 400 does not record", func(t *testing.T) {
		ResetDowngradeMemoForTest()
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"invalid temperature"}`))),
		}

		policy.DetectRejectedNativeProtocol(newMessagesInfo(), resp)

		assert.True(t, policy.useNativeProtocol(newInfo(109, "qwen38-27b", nil), dto.ModelProtocolMessages))
	})

	t.Run("a feature-level rejection does not record", func(t *testing.T) {
		ResetDowngradeMemoForTest()
		// "does not support tools" 是模型能力/参数级错误, 不是"端点不存在";
		// 一次这样的请求不得把整个组合永久降级。
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body: io.NopCloser(bytes.NewReader([]byte(
				`{"error":{"message":"This model does not support tools"}}`,
			))),
		}

		policy.DetectRejectedNativeProtocol(newMessagesInfo(), resp)

		assert.True(t, policy.useNativeProtocol(newInfo(109, "qwen38-27b", nil), dto.ModelProtocolMessages))
	})
}

// TestDetectRejectedNativeProtocolSkipsDeclaredPromiseBaseline 锁定: 未声明模型且
// 基线不受记忆约束(AstraFlow 的 responses)时, 上游拒绝不写入记忆。否则这条记录会在
// 运营者事后声明该协议时立刻生效, 让新配置背上一次并不存在的"已拒绝"。
func TestDetectRejectedNativeProtocolSkipsDeclaredPromiseBaseline(t *testing.T) {
	ResetDowngradeMemoForTest()
	t.Cleanup(ResetDowngradeMemoForTest)

	policy := Policy{NativeWhenUndeclared: []string{dto.ModelProtocolResponses}}
	info := newInfo(117, "qwen38-27b", nil)
	info.RelayFormat = types.RelayFormatOpenAIResponses
	info.RelayMode = relayconstant.RelayModeResponses
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"not implemented"}`))),
	}

	policy.DetectRejectedNativeProtocol(info, resp)

	declared := newInfo(117, "qwen38-27b", map[string][]string{"qwen38-27b": {dto.ModelProtocolResponses}})
	assert.True(t, policy.useNativeProtocol(declared, dto.ModelProtocolResponses),
		"a rejection of a promise baseline must not leak into a later declaration")
}

// TestDowngradeMemoAppliesToRetries 锁定重试轮次不受水位线约束: 重试会重新发送请求
// 并按新决定重建报文, 不存在"在途报文被中途翻转"的问题。否则刚被 404 拒过的请求每次
// 重试都会重新走原生协议, 同一个 404 撞满重试次数。
func TestDowngradeMemoAppliesToRetries(t *testing.T) {
	ResetDowngradeMemoForTest()
	t.Cleanup(ResetDowngradeMemoForTest)

	policy := Policy{NativeWhenUndeclared: []string{dto.ModelProtocolMessages}, DowngradeUndeclared: true}
	info := newInfo(110, "qwen38-27b", nil)
	info.StartTime = time.Now()
	nativeProtocolDowngradeMemo.Store(nativeProtocolMemoKey(info, dto.ModelProtocolMessages), time.Now())

	info.RetryIndex = 0
	assert.True(t, policy.useNativeProtocol(info, dto.ModelProtocolMessages),
		"an in-flight request keeps the protocol it was sent with")

	info.RetryIndex = 1
	assert.False(t, policy.useNativeProtocol(info, dto.ModelProtocolMessages),
		"a retried attempt is rebuilt and may downgrade")
}
