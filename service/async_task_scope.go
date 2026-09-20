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
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// AsyncTaskScopeFromContext 从请求上下文还原异步任务的查询作用域。
//
// 作用域与计费归属必须一致才算有效：两者由 TokenAuth 在同一次校验里写入，
// 一旦不相等说明上下文是拼出来的，这时返回空作用域让查询匹配不到任何行，
// 而不是退化成「按 user_id 查」——那样会把另一个作用域的任务暴露出去。
func AsyncTaskScopeFromContext(c *gin.Context) model.AsyncTaskScope {
	if c == nil {
		return model.AsyncTaskScope{}
	}

	scopeType := common.GetContextKeyString(c, constant.ContextKeyScopeType)
	scopeID := common.GetContextKeyInt(c, constant.ContextKeyScopeId)
	if scopeType != common.GetContextKeyString(c, constant.ContextKeyBillingAccountType) ||
		scopeID != common.GetContextKeyInt(c, constant.ContextKeyBillingAccountId) {
		return model.AsyncTaskScope{}
	}
	return model.AsyncTaskScope{
		UserId:    common.GetContextKeyInt(c, constant.ContextKeyUserId),
		ScopeType: scopeType,
		ScopeId:   scopeID,
	}
}
