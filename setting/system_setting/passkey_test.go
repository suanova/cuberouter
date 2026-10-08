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
package system_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ServerAddress is the deployment's public base URL and may carry the
// deployment prefix, but WebAuthn works in origins and RP IDs: an origin has no
// path, and an RP ID has neither path nor port. Deriving them by copying the
// configured value verbatim made passkey registration fail on exactly the
// deployments that publish under a prefix.
func TestGetPasskeySettingsDerivesWebAuthnFieldsFromServerAddress(t *testing.T) {
	previousAddress := ServerAddress
	previousSettings := defaultPasskeySettings
	t.Cleanup(func() {
		ServerAddress = previousAddress
		defaultPasskeySettings = previousSettings
	})

	ServerAddress = "http://127.0.0.1:5000/cuberouter"

	// Both spellings of "nothing configured"; an explicit value is respected.
	for _, unset := range []string{"", "[]"} {
		defaultPasskeySettings.RPID = ""
		defaultPasskeySettings.Origins = unset

		settings := GetPasskeySettings()

		assert.Equal(t, "127.0.0.1", settings.RPID)
		assert.Equal(t, "http://127.0.0.1:5000", settings.Origins)
	}
}

func TestGetPasskeySettingsKeepsConfiguredOrigins(t *testing.T) {
	previousAddress := ServerAddress
	previousSettings := defaultPasskeySettings
	t.Cleanup(func() {
		ServerAddress = previousAddress
		defaultPasskeySettings = previousSettings
	})

	ServerAddress = "http://127.0.0.1:5000/cuberouter"
	defaultPasskeySettings.Origins = "https://passkey.example.com"

	settings := GetPasskeySettings()

	assert.Equal(t, "https://passkey.example.com", settings.Origins)
}
