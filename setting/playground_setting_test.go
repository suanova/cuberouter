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

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundOrgFallbackThresholdIsUSDAnchoredCNYValue(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	// ¥0.2 ÷ 7.3 × 500000 = 13698.63… → 截断 13698
	common.QuotaPerUnit = 500000
	require.Equal(t, 13698, PlaygroundOrgFallbackThreshold())

	// 随 QuotaPerUnit 换算，保持美元锚语义
	common.QuotaPerUnit = 1000000
	require.Equal(t, 27397, PlaygroundOrgFallbackThreshold())
}
