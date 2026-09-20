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
