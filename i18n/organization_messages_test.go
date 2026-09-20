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
package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrganizationMessages verifies that every organization.* message renders
// with complete per-locale text through the same path the app uses
// (i18n.Translate, which backs common.ApiErrorI18n).
//
// The organization API deliberately returns machine-readable error_code values
// plus a stable English message for most failures — the frontend maps the code
// to its own copy. Localized strings are added here only where the backend
// itself has to phrase a message, so this table is small on purpose; a missing
// key would surface to the caller as the raw key name.
func TestOrganizationMessages(t *testing.T) {
	require.NoError(t, Init())

	cases := []struct {
		name string
		lang string
		key  string
		want string
	}{
		{name: "en quota data time span", lang: LangEn, key: MsgOrganizationQuotaDataTimeSpanTooLong, want: "Time span cannot exceed 1 month"},
		{name: "zh-CN quota data time span", lang: LangZhCN, key: MsgOrganizationQuotaDataTimeSpanTooLong, want: "时间跨度不能超过 1 个月"},
		{name: "zh-TW quota data time span", lang: LangZhTW, key: MsgOrganizationQuotaDataTimeSpanTooLong, want: "時間跨度不能超過 1 個月"},
		{name: "unknown locale falls back to en", lang: "fr", key: MsgOrganizationQuotaDataTimeSpanTooLong, want: "Time span cannot exceed 1 month"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Translate(tc.lang, tc.key))
		})
	}
}
