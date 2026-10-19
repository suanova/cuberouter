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
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// modelAccessEntries builds a list of count unique model names that are
// nameLen bytes long (zero-padded decimal), for boundary tests.
func modelAccessEntries(count int, nameLen int) []string {
	models := make([]string, 0, count)
	for i := 0; i < count; i++ {
		models = append(models, fmt.Sprintf("%0*d", nameLen, i))
	}
	return models
}

func TestParseMemberModelAccess(t *testing.T) {
	overlong := strings.Repeat("a", MaxMemberModelNameLen+1)
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty string", raw: "", want: nil},
		{name: "blank string", raw: "   ", want: nil},
		{name: "json null", raw: "null", want: nil},
		{name: "empty array", raw: "[]", want: nil},
		{name: "invalid json", raw: "[bad", want: nil},
		{name: "wrong json type", raw: `{"a":1}`, want: nil},
		{name: "trims, drops blanks and dedupes keeping order", raw: `[" b ","a","a",""]`, want: []string{"b", "a"}},
		{name: "keeps stored order", raw: `["z","m","a"]`, want: []string{"z", "m", "a"}},
		{name: "skips overlong name", raw: `["a","` + overlong + `","b"]`, want: []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMemberModelAccess(tt.raw)
			if tt.want == nil {
				require.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("truncates entries beyond the cap", func(t *testing.T) {
		entries := modelAccessEntries(MaxMemberModelAccessEntries+5, 8)
		raw, err := common.Marshal(entries)
		require.NoError(t, err)

		got := ParseMemberModelAccess(string(raw))
		require.Len(t, got, MaxMemberModelAccessEntries)
		assert.Equal(t, entries[0], got[0])
		assert.Equal(t, entries[MaxMemberModelAccessEntries-1], got[len(got)-1])
	})
}

func TestMarshalMemberModelAccess(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		want   string
	}{
		{name: "nil list", models: nil, want: ""},
		{name: "empty list", models: []string{}, want: ""},
		{name: "blank entries only", models: []string{"", "  "}, want: ""},
		{name: "single model", models: []string{"gpt-4o"}, want: `["gpt-4o"]`},
		{name: "trims entries", models: []string{" gpt-4o "}, want: `["gpt-4o"]`},
		{name: "dedupes and sorts", models: []string{"b", "a", "b", " a "}, want: `["a","b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MarshalMemberModelAccess(tt.models))
		})
	}
}

func TestValidateMemberModelAccess(t *testing.T) {
	overlong := strings.Repeat("a", MaxMemberModelNameLen+1)
	tests := []struct {
		name            string
		models          []string
		wantErrContains string
	}{
		{name: "nil list is valid", models: nil},
		{name: "empty list is valid", models: []string{}},
		{name: "small list is valid", models: []string{"gpt-4o", "claude-3-5-sonnet"}},
		{name: "padded name is valid", models: []string{" gpt-4o "}},
		{name: "duplicate names are valid", models: []string{"gpt-4o", "gpt-4o"}},
		{name: "accepts name at the length limit", models: []string{strings.Repeat("a", MaxMemberModelNameLen)}},
		{name: "accepts the entry count limit", models: modelAccessEntries(MaxMemberModelAccessEntries, 8)},
		{name: "rejects empty entry", models: []string{""}, wantErrContains: "must not be empty"},
		{name: "rejects blank entry", models: []string{"gpt-4o", "   "}, wantErrContains: "must not be empty"},
		{name: "rejects overlong entry and names it", models: []string{overlong}, wantErrContains: overlong},
		{name: "rejects too many entries", models: modelAccessEntries(MaxMemberModelAccessEntries+1, 8), wantErrContains: "too many"},
		{name: "rejects oversized payload", models: modelAccessEntries(MaxMemberModelAccessEntries, MaxMemberModelNameLen), wantErrContains: "too large"},
		// MySQL stores the column (and the audit snapshots) as TEXT, whose
		// capacity is 65535 bytes. A list the validator accepts must fit there,
		// or a valid write fails (strict mode) or is truncated into "no
		// restriction" (non-strict mode).
		{name: "rejects payload beyond the MySQL TEXT limit", models: modelAccessEntries(MaxMemberModelAccessEntries, 40), wantErrContains: "too large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMemberModelAccess(tt.models)
			if tt.wantErrContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErrContains)
		})
	}
}
