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
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

type restrictionSettings struct {
	whitelistEnabled  bool
	whitelist         []string
	blacklistEnabled  bool
	blacklist         []string
	aliasEnabled      bool
	disposableEnabled bool
}

func applyRestrictionSettings(t *testing.T, s restrictionSettings) {
	t.Helper()
	prevWhitelistEnabled := common.EmailDomainRestrictionEnabled
	prevWhitelist := common.EmailDomainWhitelist
	prevBlacklistEnabled := common.EmailDomainBlacklistEnabled
	prevBlacklist := common.EmailDomainBlacklist
	prevAliasEnabled := common.EmailAliasRestrictionEnabled
	prevDisposableEnabled := common.EmailDisposableCheckEnabled
	t.Cleanup(func() {
		common.EmailDomainRestrictionEnabled = prevWhitelistEnabled
		common.EmailDomainWhitelist = prevWhitelist
		common.EmailDomainBlacklistEnabled = prevBlacklistEnabled
		common.EmailDomainBlacklist = prevBlacklist
		common.EmailAliasRestrictionEnabled = prevAliasEnabled
		common.EmailDisposableCheckEnabled = prevDisposableEnabled
	})
	common.EmailDomainRestrictionEnabled = s.whitelistEnabled
	common.EmailDomainWhitelist = s.whitelist
	common.EmailDomainBlacklistEnabled = s.blacklistEnabled
	common.EmailDomainBlacklist = s.blacklist
	common.EmailAliasRestrictionEnabled = s.aliasEnabled
	common.EmailDisposableCheckEnabled = s.disposableEnabled
}

func TestValidateEmailRestrictions(t *testing.T) {
	cases := []struct {
		name     string
		settings restrictionSettings
		email    string
		wantErr  error
	}{
		{"no restrictions", restrictionSettings{}, "user@example.com", nil},
		{"alias plus rejected", restrictionSettings{aliasEnabled: true}, "user+tag@example.com", ErrEmailAliasRestricted},
		{"alias dot rejected", restrictionSettings{aliasEnabled: true}, "user.name@example.com", ErrEmailAliasRestricted},
		{"plain alias allowed", restrictionSettings{aliasEnabled: true}, "username@example.com", nil},
		{"blacklist exact", restrictionSettings{blacklistEnabled: true, blacklist: []string{"mailinator.com"}}, "x@mailinator.com", ErrEmailDomainBlacklisted},
		{"blacklist wildcard subdomain", restrictionSettings{blacklistEnabled: true, blacklist: []string{"*.mailinator.com"}}, "x@a.mailinator.com", ErrEmailDomainBlacklisted},
		{"blacklist wildcard does not match apex", restrictionSettings{blacklistEnabled: true, blacklist: []string{"*.mailinator.com"}}, "x@mailinator.com", nil},
		{"blacklist entry is trimmed and lowercased", restrictionSettings{blacklistEnabled: true, blacklist: []string{" *.MAILINATOR.COM "}}, "x@a.mailinator.com", ErrEmailDomainBlacklisted},
		{"blacklist enabled with empty list is a no-op", restrictionSettings{blacklistEnabled: true}, "x@example.com", nil},
		{"whitelist rejects unknown", restrictionSettings{whitelistEnabled: true, whitelist: []string{"example.com"}}, "x@other.com", ErrEmailDomainNotWhitelisted},
		{"whitelist overrides blacklist", restrictionSettings{whitelistEnabled: true, whitelist: []string{"example.com"}, blacklistEnabled: true, blacklist: []string{"example.com"}}, "x@example.com", nil},
		{"whitelist overrides disposable", restrictionSettings{whitelistEnabled: true, whitelist: []string{"mailinator.com"}, disposableEnabled: true}, "x@mailinator.com", nil},
		{"disposable list", restrictionSettings{disposableEnabled: true}, "x@mailinator.com", ErrEmailDomainDisposable},
		{"non-disposable passes", restrictionSettings{disposableEnabled: true}, "x@gmail.com", nil},
		{"email without at is ignored", restrictionSettings{disposableEnabled: true}, "not-an-email", nil},
		{"email with empty local part is ignored", restrictionSettings{disposableEnabled: true}, "@example.com", nil},
		{"email with empty domain is ignored", restrictionSettings{disposableEnabled: true}, "a@", nil},
		{"email with multiple at is ignored", restrictionSettings{disposableEnabled: true}, "a@b@c.com", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			applyRestrictionSettings(t, tc.settings)
			err := ValidateEmailRestrictions(tc.email)
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestValidateEmailRestrictionsConcurrentOptionUpdate(t *testing.T) {
	applyRestrictionSettings(t, restrictionSettings{blacklistEnabled: true})

	var wg sync.WaitGroup
	stop := make(chan struct{})
	started := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(started)
		for {
			select {
			case <-stop:
				return
			default:
				common.OptionMapRWMutex.Lock()
				common.EmailDomainBlacklist = []string{"mailinator.com"}
				common.OptionMapRWMutex.Unlock()
				time.Sleep(time.Microsecond)
			}
		}
	}()
	<-started
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = ValidateEmailRestrictions("x@example.com")
	}
	close(stop)
	wg.Wait()
}
