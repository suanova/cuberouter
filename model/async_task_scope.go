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
package model

import "gorm.io/gorm"

type AsyncTaskScope struct {
	UserId    int
	ScopeType string
	ScopeId   int
}

func (scope AsyncTaskScope) Apply(db *gorm.DB) *gorm.DB {
	if scope.UserId <= 0 || scope.ScopeId <= 0 ||
		(scope.ScopeType != AccountContextTypePersonal && scope.ScopeType != AccountContextTypeOrganization) {
		return db.Where("1 = 0")
	}

	query := db.Where(
		"user_id = ? AND scope_type = ? AND scope_id = ? AND billing_account_type = ? AND billing_account_id = ?",
		scope.UserId, scope.ScopeType, scope.ScopeId, scope.ScopeType, scope.ScopeId,
	)
	if scope.ScopeType == AccountContextTypeOrganization {
		return query.Where("organization_id = ?", scope.ScopeId)
	}
	return query.Where("organization_id = 0")
}

// personalAsyncTaskScopeCondition 把「我的任务」类列表收窄到个人账单的记录。
//
// 空作用域也算个人：历史行写在作用域列存在之前，除了升级回填，滚动发布期间仍
// 可能由旧节点写入空值。少这一支，用户的视频/绘图列表会凭空少一截。
//
// 只比对 billing_account_type、不比对 billing_account_id：这里要回答的是
// 「这笔钱算不算他自己的」。user_id 已经把范围限定在他名下的行，组织任务即使把
// 他记为责任人，账单类型也是 organization，据此排除即可。
const personalAsyncTaskScopeCondition = "(billing_account_type = ? OR billing_account_type = '' OR billing_account_type IS NULL)" +
	" AND (scope_type = ? OR scope_type = '' OR scope_type IS NULL)"

func applyPersonalAsyncTaskScope(db *gorm.DB) *gorm.DB {
	return db.Where(personalAsyncTaskScopeCondition, AccountContextTypePersonal, AccountContextTypePersonal)
}
