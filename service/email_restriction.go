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
	"errors"
	"strings"

	emailverifier "github.com/AfterShip/email-verifier"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var (
	ErrEmailDomainNotWhitelisted = errors.New("email domain not in whitelist")
	ErrEmailDomainBlacklisted    = errors.New("email domain is blacklisted")
	ErrEmailDomainDisposable     = errors.New("email domain is disposable")
	ErrEmailAliasRestricted      = errors.New("email alias contains restricted characters")
)

var disposableEmailVerifier = emailverifier.NewVerifier()

// ValidateEmailRestrictions applies the configured email restrictions to an
// address. Malformed addresses are left to the caller's syntax validation and
// return nil here.
func ValidateEmailRestrictions(email string) error {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	email = model.NormalizeEmail(email)
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 || strings.Contains(email[:at], "@") {
		return nil
	}
	localPart := email[:at]
	domain := email[at+1:]

	if common.EmailAliasRestrictionEnabled &&
		(strings.Contains(localPart, "+") || strings.Contains(localPart, ".")) {
		return ErrEmailAliasRestricted
	}

	if common.EmailDomainRestrictionEnabled {
		for _, allowed := range common.EmailDomainWhitelist {
			if allowed == domain {
				return nil
			}
		}
		return ErrEmailDomainNotWhitelisted
	}

	if common.EmailDomainBlacklistEnabled {
		for _, blocked := range common.EmailDomainBlacklist {
			if common.EmailDomainMatches(domain, blocked) {
				return ErrEmailDomainBlacklisted
			}
		}
	}

	if common.EmailDisposableCheckEnabled && disposableEmailVerifier.IsDisposable(domain) {
		return ErrEmailDomainDisposable
	}

	return nil
}
