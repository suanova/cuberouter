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
	"fmt"
	"strings"
	"sync"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// nativeProtocolDowngradeMemo 记住"已声明原生、但上游明确拒绝"的 (渠道, 模型, 协议):
// 进程内生效,命中后该组合自动降级为转换,避免同一个配置错误每次都撞上游。
// 重启即失效;不写库、不改渠道配置。值是记录时刻,见 nativeProtocolDowngraded。
// 键含渠道 ID,而渠道 ID 是渠道行主键、一行只有一个渠道类型,所以多个渠道类型
// 共用这一张表也不会串味。
var nativeProtocolDowngradeMemo sync.Map // key: channelID|model|protocol, value: time.Time

const nativeProtocolBodyPeekLimit = 4 * 1024

// protocolUnsupportedMarkers 是上游拒绝某协议时的错误文案特征(小写子串)。
// 只收能指明"协议/端点本身不支持"的措辞: 裸 "unsupported" 会把参数级 4xx
// (如 "unsupported parameter: temperature") 也算作协议拒绝, 让该组合在进程内
// 被永久降级。实测: claude 模型打 /v1/responses 返回 500 "not implemented";
// 图像模型返回 400 "The requested operation is unsupported."。
// 不收 "Invalid param: get request mode": 星图的 gemini 端点会整体抖动——同一模型
// 同一请求在几分钟内 200/400 交替(实测 2026-09-27,期间所有模型一起 400), 那是上游
// 瞬时故障而非"该模型没有 gemini 协议", 收进签名会让一次抖动把原生 gemini 永久降级。
var protocolUnsupportedMarkers = []string{
	"not implemented",
	"operation is unsupported",
	"does not support",
	"unsupported protocol",
	"unsupported endpoint",
}

func nativeProtocolMemoKey(info *relaycommon.RelayInfo, protocol string) string {
	return fmt.Sprintf("%d|%s|%s", info.ChannelId, upstreamModelID(info), protocol)
}

// nativeProtocolDowngraded 报告该 (渠道, 模型, 协议) 是否已被上游拒绝过。
// 记录带水位线: 只对"记录之后才开始"的请求生效。否则已经在途的请求会被中途翻转
// ——它按原生协议发出,响应却会被降级后的处理器按 chat 解析,把成功的原生报文判成
// 错误。StartTime 在 RelayInfo 构建时写入, 早于任何适配器调用,故以它为界:
// 记录时刻早于 StartTime 才降级, 即记录之前发出的请求保持它发出时的决定。
// StartTime 为零(手工构造的 RelayInfo)时按"命中即降级"处理。
func nativeProtocolDowngraded(info *relaycommon.RelayInfo, protocol string) bool {
	recordedAt, downgraded := nativeProtocolDowngradeMemo.Load(nativeProtocolMemoKey(info, protocol))
	if !downgraded {
		return false
	}
	if info.StartTime.IsZero() {
		return true
	}
	recorded, ok := recordedAt.(time.Time)
	if !ok {
		return true
	}
	return recorded.Before(info.StartTime)
}

func upstreamRejectsProtocol(message string) bool {
	lower := strings.ToLower(message)
	for _, marker := range protocolUnsupportedMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
