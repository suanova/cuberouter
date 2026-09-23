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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeJoinRulePattern(t *testing.T) {
	cases := []struct {
		name      string
		matchType string
		raw       string
		want      string
		wantErr   bool
	}{
		{name: "bare domain lowercased", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "  Enterprise.COM ", want: "enterprise.com"},
		{name: "trailing dot is one fqdn dot", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "enterprise.com.", want: "enterprise.com"},
		{name: "wildcard keeps its prefix", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "*.Enterprise.com", want: "*.enterprise.com"},
		{name: "bare single label rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "com", wantErr: true},
		{name: "wildcard on a bare tld rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "*.com", wantErr: true},
		{name: "wildcard in the middle rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "mail.*.enterprise.com", wantErr: true},
		{name: "bare star rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "*", wantErr: true},
		{name: "email in a domain rule rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "user@enterprise.com", wantErr: true},
		{name: "unicode domain rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "企業.com", wantErr: true},
		{name: "empty label rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "enterprise..com", wantErr: true},
		{name: "leading hyphen label rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "-enterprise.com", wantErr: true},
		{name: "address normalized", matchType: model.OrganizationJoinRuleMatchTypeEmail, raw: " User-A@Enterprise.com ", want: "user-a@enterprise.com"},
		{name: "invalid address rejected", matchType: model.OrganizationJoinRuleMatchTypeEmail, raw: "not-an-email", wantErr: true},
		{name: "unknown match type rejected", matchType: "wildcard", raw: "enterprise.com", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeJoinRulePattern(tc.matchType, tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNormalizeJoinRulePatternRejectsOverlongPattern(t *testing.T) {
	long := strings.Repeat("a", 190) + ".com" // 194 字节，超过唯一索引列宽
	_, err := NormalizeJoinRulePattern(model.OrganizationJoinRuleMatchTypeDomain, long)
	require.Error(t, err)
}

func TestJoinRuleMatchTypeFor(t *testing.T) {
	assert.Equal(t, model.OrganizationJoinRuleMatchTypeEmail, joinRuleMatchTypeFor("user-a@enterprise.com"))
	assert.Equal(t, model.OrganizationJoinRuleMatchTypeDomain, joinRuleMatchTypeFor("*.enterprise.com"))
	assert.Equal(t, model.OrganizationJoinRuleMatchTypeDomain, joinRuleMatchTypeFor("enterprise.com"))
}
