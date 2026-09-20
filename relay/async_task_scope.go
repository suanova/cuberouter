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
package relay

import (
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// asyncTaskScopeFromRelayInfo 是 service.AsyncTaskScopeFromContext 的 RelayInfo 版本。
//
// 重试和异步轮询阶段已经没有 gin.Context，只能从 RelayInfo 还原作用域；
// 校验规则与上下文版本一致，作用域与计费归属不等就返回空作用域。
func asyncTaskScopeFromRelayInfo(info *relaycommon.RelayInfo) model.AsyncTaskScope {
	if info == nil || info.ScopeType != info.BillingAccountType || info.ScopeId != info.BillingAccountId {
		return model.AsyncTaskScope{}
	}
	return model.AsyncTaskScope{UserId: info.UserId, ScopeType: info.ScopeType, ScopeId: info.ScopeId}
}
