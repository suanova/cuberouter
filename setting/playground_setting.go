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
package setting

import "github.com/QuantumNous/new-api/common"

const (
	playgroundOrgFallbackThresholdCNY = 0.2
	playgroundOrgFallbackUSD2CNY      = 7.3
)

// PlaygroundOrgFallbackThreshold 返回游乐场组织额度兜底的「钱包视为耗尽」阈值（quota）。
//
// 语义：展示汇率 7.3 下的 ¥0.2，按「存储与计费永远 USD」折算为美元锚（≈ $0.0274）。
// 展示汇率只影响界面显示，阈值不随其变化；QuotaPerUnit 在调用时参与换算，
// 避免包初始化顺序踩到 QuotaPerUnit 的加载时机。
func PlaygroundOrgFallbackThreshold() int {
	return common.QuotaFromFloat(playgroundOrgFallbackThresholdCNY / playgroundOrgFallbackUSD2CNY * common.QuotaPerUnit)
}
