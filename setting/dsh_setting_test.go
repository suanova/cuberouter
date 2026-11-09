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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func setupDshSettingTest(t *testing.T) {
	t.Helper()
	originalMap := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalMap
		common.OptionMapRWMutex.Unlock()
	})
}

func TestValidateDshAllowedOrganizationIds(t *testing.T) {
	for _, value := range []string{"", "[]", "[12]", "[12,13,12]"} {
		assert.NoError(t, ValidateDshAllowedOrganizationIds(value), value)
	}
	for _, value := range []string{"12", "{}", "[0]", "[-3]", `["12"]`, "[12"} {
		assert.Error(t, ValidateDshAllowedOrganizationIds(value), value)
	}
}

func TestValidateDshOrganizationChannels(t *testing.T) {
	for _, value := range []string{"", "{}", `{"12":34}`, `{"12":34,"13":35}`} {
		assert.NoError(t, ValidateDshOrganizationChannels(value), value)
	}
	for _, value := range []string{
		"[]",
		`{"0":34}`,
		`{"12":0}`,
		`{"12":-1}`,
		`{"12":"34"}`,
		`{"12":34`,
	} {
		assert.Error(t, ValidateDshOrganizationChannels(value), value)
	}
}

func TestValidateDshOrganizationDeepseekKey(t *testing.T) {
	assert.NoError(t, ValidateDshOrganizationDeepseekKey(""))
	assert.NoError(t, ValidateDshOrganizationDeepseekKey("{}"))
	assert.NoError(t, ValidateDshOrganizationDeepseekKey(`{"12":"sk-organization-secret"}`))

	for _, value := range []string{
		"[]",
		`{"0":"sk-value"}`,
		`{"12":""}`,
		`{"12":"   "}`,
		`{"12":"sk-value\nmore"}`,
		`{"12":12}`,
	} {
		assert.Error(t, ValidateDshOrganizationDeepseekKey(value), value)
	}

	tooLong := `{"12":"` + strings.Repeat("a", maxDshOrganizationSecretBytes+1) + `"}`
	assert.Error(t, ValidateDshOrganizationDeepseekKey(tooLong))
}

func TestValidateDshOrganizationSecretKey(t *testing.T) {
	assert.NoError(t, ValidateDshOrganizationSecretKey("sk-organization-secret"))
	assert.NoError(t, ValidateDshOrganizationSecretKey("  sk-with-padding  "))
	assert.Error(t, ValidateDshOrganizationSecretKey(""))
	assert.Error(t, ValidateDshOrganizationSecretKey("   "))
	assert.Error(t, ValidateDshOrganizationSecretKey("sk-value\nmore"))
	assert.Error(t, ValidateDshOrganizationSecretKey(strings.Repeat("a", maxDshOrganizationSecretBytes+1)))
}

func TestDshEnabledOrganizationIds(t *testing.T) {
	setupDshSettingTest(t)

	// Nothing configured: DSH stays closed.
	assert.Empty(t, DshEnabledOrganizationIds())

	// Allowlisted but not bound to a channel: still not enabled.
	common.OptionMapRWMutex.Lock()
	common.OptionMap[DshAllowedOrganizationIdsKey] = "[13,12]"
	common.OptionMapRWMutex.Unlock()
	assert.Empty(t, DshEnabledOrganizationIds())

	// Bound but not allowlisted: also not enabled.
	common.OptionMapRWMutex.Lock()
	common.OptionMap[DshAllowedOrganizationIdsKey] = "[]"
	common.OptionMap[DshOrganizationChannelsKey] = `{"12":34}`
	common.OptionMapRWMutex.Unlock()
	assert.Empty(t, DshEnabledOrganizationIds())

	// The intersection is returned sorted, regardless of allowlist order.
	common.OptionMapRWMutex.Lock()
	common.OptionMap[DshAllowedOrganizationIdsKey] = "[13,12,99]"
	common.OptionMap[DshOrganizationChannelsKey] = `{"13":35,"12":34}`
	common.OptionMapRWMutex.Unlock()
	assert.Equal(t, []int{12, 13}, DshEnabledOrganizationIds())

	// A malformed binding map fails closed instead of enabling a subset.
	common.OptionMapRWMutex.Lock()
	common.OptionMap[DshOrganizationChannelsKey] = `{"12":34`
	common.OptionMapRWMutex.Unlock()
	assert.Empty(t, DshEnabledOrganizationIds())
}

func TestGetDshOrganizationDeepseekKey(t *testing.T) {
	setupDshSettingTest(t)

	common.OptionMapRWMutex.Lock()
	common.OptionMap[DshOrganizationDeepseekSecretKey] = `{"12":"sk-organization-secret"}`
	common.OptionMapRWMutex.Unlock()

	assert.Equal(t, "sk-organization-secret", GetDshOrganizationDeepseekKey(12))
	assert.Empty(t, GetDshOrganizationDeepseekKey(13))
	assert.Empty(t, GetDshOrganizationDeepseekKey(0))
}
