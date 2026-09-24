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

// Port from develop (58cf985/#94, 2c55d2e): subscription-basis column
// metadata; the standalone used_quota column is removed (used quota moves
// into the quota column tooltip).
func TestGetOpsUserColumnsList(t *testing.T) {
	expectedOrder := []string{
		"id", "username", "display_name", "phone",
		"role", "status", "group", "quota",
		"money_balance", "request_count",
		"total_prompt_tokens", "total_completion_tokens",
		"aff_code", "aff_count", "created_at",
	}
	if len(opsUserColumns) != len(expectedOrder) {
		t.Fatalf("opsUserColumns length = %d, want %d", len(opsUserColumns), len(expectedOrder))
	}
	for i, want := range expectedOrder {
		if opsUserColumns[i].Key != want {
			t.Errorf("opsUserColumns[%d].Key = %q, want %q", i, opsUserColumns[i].Key, want)
		}
	}

	for _, c := range opsUserColumns {
		if c.Key == "used_quota" || c.Key == "money_used" {
			t.Errorf("column %q should have been removed", c.Key)
		}
	}

	required := map[string]bool{}
	for _, c := range opsUserColumns {
		if c.Required {
			required[c.Key] = true
		}
	}
	for _, k := range []string{"id", "username"} {
		if !required[k] {
			t.Errorf("column %q should be required", k)
		}
	}
	if len(required) != 2 {
		t.Errorf("required column count = %d, want 2", len(required))
	}

	labels := map[string]string{}
	for _, c := range opsUserColumns {
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
