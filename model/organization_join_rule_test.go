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

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 唯一索引是跨组织互斥的最后一道防线，必须真的存在于库上，
// 而不是只写在标签里。
func TestOrganizationJoinRulePatternIsGloballyUnique(t *testing.T) {
	setupModelTestDB(t)

	first := OrganizationJoinRule{
		OrganizationId:    1,
		MatchType:         OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "enterprise.com",
		PatternNormalized: "enterprise.com",
		CreatedBy:         1,
		CreatedAt:         1,
		UpdatedAt:         1,
	}
	require.NoError(t, DB.Create(&first).Error)

	duplicate := first
	duplicate.Id = 0
	duplicate.OrganizationId = 2
	err := DB.Create(&duplicate).Error
	require.Error(t, err, "同一条 pattern_normalized 不允许被第二个组织占用")
}
