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

import "testing"

// Port from develop (58cf985/#94, 2c55d2e): quota basis column metadata.
func TestGetUserColumnsList(t *testing.T) {
	expectedOrder := []string{
		"select", "id", "username", "status",
		"quota", "money_balance", "group", "role",
		"invite_info", "created_at", "last_login_at", "actions",
	}
	if len(userColumns) != len(expectedOrder) {
		t.Fatalf("userColumns length = %d, want %d", len(userColumns), len(expectedOrder))
	}
	for i, want := range expectedOrder {
		if userColumns[i].Key != want {
			t.Errorf("userColumns[%d].Key = %q, want %q", i, userColumns[i].Key, want)
		}
	}

	// No history-money column in the subscription-basis layout.
	for _, c := range userColumns {
		if c.Key == "money_used" {
			t.Errorf("column %q should have been removed", c.Key)
		}
	}

	required := map[string]bool{}
	for _, c := range userColumns {
		if c.Required {
			required[c.Key] = true
		}
	}
	for _, k := range []string{"select", "id", "username", "actions"} {
		if !required[k] {
			t.Errorf("column %q should be required", k)
		}
	}
	if len(required) != 4 {
		t.Errorf("required column count = %d, want 4", len(required))
	}

	// Labels must be non-empty and use the subscription-basis wording.
	labels := map[string]string{}
	for _, c := range userColumns {
		if c.Label == "" {
			t.Errorf("column %q has empty label", c.Key)
		}
		labels[c.Key] = c.Label
	}
	if labels["money_balance"] != "Subscription Balance" {
		t.Errorf("money_balance label = %q, want Subscription Balance", labels["money_balance"])
	}
	if labels["quota"] != "Remaining/Total Quota" {
		t.Errorf("quota label = %q, want Remaining/Total Quota", labels["quota"])
	}
}
