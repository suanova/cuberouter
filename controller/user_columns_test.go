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
package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port from develop (58cf985/#94, 2c55d2e): quota basis column metadata.
func TestGetUserColumnsList(t *testing.T) {
	expectedOrder := []string{
		"select", "id", "username", "status",
		"quota", "money_balance", "group", "role",
		"invite_info", "created_at", "last_login_at", "actions",
	}
	require.Len(t, userColumns, len(expectedOrder))
	for i, want := range expectedOrder {
		assert.Equal(t, want, userColumns[i].Key, "userColumns[%d].Key", i)
	}

	// No history-money column in the subscription-basis layout.
	for _, c := range userColumns {
		assert.NotEqual(t, "money_used", c.Key, "column should have been removed")
	}

	required := map[string]bool{}
	for _, c := range userColumns {
		if c.Required {
			required[c.Key] = true
		}
	}
	for _, k := range []string{"select", "id", "username", "actions"} {
		assert.True(t, required[k], "column %q should be required", k)
	}
	assert.Len(t, required, 4)

	// Labels must be non-empty and use the subscription-basis wording.
	labels := map[string]string{}
	for _, c := range userColumns {
		assert.NotEmpty(t, c.Label, "column %q has empty label", c.Key)
		labels[c.Key] = c.Label
	}
	assert.Equal(t, "Subscription Balance", labels["money_balance"])
	assert.Equal(t, "Remaining/Total Quota", labels["quota"])
}
