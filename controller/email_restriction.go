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

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
)

func emailRestrictionI18nKey(err error) string {
	switch {
	case errors.Is(err, service.ErrEmailDomainNotWhitelisted):
		return i18n.MsgUserEmailDomainNotAllowed
	case errors.Is(err, service.ErrEmailDomainBlacklisted):
		return i18n.MsgUserEmailDomainBlacklisted
	case errors.Is(err, service.ErrEmailDomainDisposable):
		return i18n.MsgUserEmailDomainDisposable
	case errors.Is(err, service.ErrEmailAliasRestricted):
		return i18n.MsgUserEmailAliasRestricted
	default:
		return i18n.MsgInvalidParams
	}
}
