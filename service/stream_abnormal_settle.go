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
package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

// SettleAbnormalStreamEnd 在流式异常结束（客户端断开 / 空闲超时 / 扫描器错误 /
// 不完整流）时按现有口径做部分结算。usage 是调用方已掌握的最好证据：上游已给的
// usage 优先，其次已收文本的本地估算。
//
// 完全没有可计费证据时 PostTextConsumeQuota 会走 Settle(0) 全额退回，资金结果与
// 旧行为一致，但仍留一条消费日志（消灭"异常结束连一条 type=2 都没有"的盲区）。
//
// 调用方必须保证随后返回的错误带 ErrOptionWithSkipRetry：结算完成后再换渠道重试
// 会复用同一份预扣二次结算。结算后 controller 的退款 defer 因 BillingSession 已
// 结算而成为 no-op。
func SettleAbnormalStreamEnd(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) {
	if info == nil {
		return
	}
	endReason := ""
	if info.StreamStatus != nil {
		endReason = string(info.StreamStatus.EndReason)
	}
	promptTokens, completionTokens := 0, 0
	if usage != nil {
		promptTokens, completionTokens = usage.PromptTokens, usage.CompletionTokens
	}
	info.AbnormalStreamSettled = true
	logger.LogWarn(c, fmt.Sprintf("流式异常结束（%s），按已掌握用量部分结算: prompt_tokens=%d, completion_tokens=%d",
		endReason, promptTokens, completionTokens))
	PostTextConsumeQuota(c, info, usage, []string{fmt.Sprintf("流式异常结束（%s），按已掌握用量部分结算", endReason)})
}
