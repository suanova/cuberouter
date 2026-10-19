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
package billingexpr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRegisterAndLookupTaskUsageSchema 锁定注册表契约：命中 matcher 的模型返回
// 注册的 schema，未命中返回 false；nil matcher / 空 schema 静默忽略。
func TestRegisterAndLookupTaskUsageSchema(t *testing.T) {
	RegisterTaskUsageSchema(func(model string) bool { return model == "unit-test-facts-model" }, map[string]UsageFieldSchema{
		"seconds": {Type: "number", Unit: "second"},
	})

	schema, ok := LookupTaskUsageSchema("unit-test-facts-model")
	require.True(t, ok)
	require.Contains(t, schema, "seconds")

	_, ok = LookupTaskUsageSchema("unit-test-unregistered-model")
	require.False(t, ok)

	RegisterTaskUsageSchema(nil, map[string]UsageFieldSchema{"seconds": {Type: "number"}})
	RegisterTaskUsageSchema(func(string) bool { return true }, nil)
	_, ok = LookupTaskUsageSchema("unit-test-unregistered-model")
	require.False(t, ok, "invalid registrations must be ignored")
}
