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

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOptionValueEmailDomainBlacklist(t *testing.T) {
	require.NoError(t, validateOptionValue("EmailDomainBlacklist", ""))
	require.NoError(t, validateOptionValue("EmailDomainBlacklist", "mailinator.com,*.tempmail.com"))
	require.NoError(t, validateOptionValue("EmailDomainBlacklist", " Mailinator.com "))
	require.Error(t, validateOptionValue("EmailDomainBlacklist", "http://mailinator.com"))
	require.Error(t, validateOptionValue("EmailDomainBlacklist", "a@b.com"))
}

func TestUpdateOptionMapEmailDomainBlacklistNormalizes(t *testing.T) {
	oldMap := common.OptionMap
	oldList := common.EmailDomainBlacklist
	t.Cleanup(func() {
		common.OptionMap = oldMap
		common.EmailDomainBlacklist = oldList
	})
	common.OptionMap = map[string]string{}

	require.NoError(t, updateOptionMap("EmailDomainBlacklist", " Mailinator.com, ,*.TempMail.com "))
	assert.Equal(t, []string{"mailinator.com", "*.tempmail.com"}, common.EmailDomainBlacklist)
}

func TestNormalizeOptionValueEmailDomainBlacklist(t *testing.T) {
	assert.Equal(t, "mailinator.com,*.tempmail.com", normalizeOptionValue("EmailDomainBlacklist", " Mailinator.com, ,*.TempMail.com "))
	assert.Equal(t, "unchanged", normalizeOptionValue("Other", "unchanged"))
}
