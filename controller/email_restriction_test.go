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
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
)

func TestEmailRestrictionI18nKey(t *testing.T) {
	assert.Equal(t, i18n.MsgUserEmailDomainNotAllowed, emailRestrictionI18nKey(service.ErrEmailDomainNotWhitelisted))
	assert.Equal(t, i18n.MsgUserEmailDomainBlacklisted, emailRestrictionI18nKey(service.ErrEmailDomainBlacklisted))
	assert.Equal(t, i18n.MsgUserEmailDomainDisposable, emailRestrictionI18nKey(service.ErrEmailDomainDisposable))
	assert.Equal(t, i18n.MsgUserEmailAliasRestricted, emailRestrictionI18nKey(service.ErrEmailAliasRestricted))
	assert.Equal(t, i18n.MsgInvalidParams, emailRestrictionI18nKey(errors.New("other")))
}
