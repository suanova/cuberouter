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
package security_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSwaggerAPIIPWhitelist(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty list is valid", value: "[]"},
		{name: "single IP", value: `["203.0.113.7"]`},
		{name: "IPv6 and CIDR entries", value: `["2001:db8::1","10.0.0.0/8","::1/128"]`},
		{name: "invalid CIDR is rejected", value: `["10.0.0.0/99"]`, wantErr: true},
		{name: "hostname is rejected", value: `["example.com"]`, wantErr: true},
		{name: "empty entry is rejected", value: `[""]`, wantErr: true},
		{name: "non-string element is rejected", value: `[80]`, wantErr: true},
		{name: "non-array value is rejected", value: `"1.2.3.4"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSwaggerAPIIPWhitelist(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
