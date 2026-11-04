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
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateLogoOptionValue(t *testing.T) {
	valid := []string{
		"",
		"https://cdn.example.com/logo.png",
		"http://127.0.0.1:8080/logo.svg",
		"data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png")),
		"data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte("<svg/>")),
	}
	for _, value := range valid {
		assert.NoError(t, validateLogoOptionValue(value), value)
	}

	invalid := []string{
		"/head.png",
		"logo.png",
		"ftp://example.com/logo.png",
		"https://",
		"data:text/html;base64,PHNjcmlwdD4=",
		"data:image/png,not-base64",
		"data:image/png;base64,!!!",
	}
	for _, value := range invalid {
		assert.Error(t, validateLogoOptionValue(value), value)
	}
}

func TestValidateLogoOptionValueSizeLimit(t *testing.T) {
	atLimit := "data:image/png;base64," +
		base64.StdEncoding.EncodeToString(make([]byte, maxLogoImageBytes))
	assert.NoError(t, validateLogoOptionValue(atLimit))

	overLimit := "data:image/png;base64," +
		base64.StdEncoding.EncodeToString(make([]byte, maxLogoImageBytes+1))
	assert.Error(t, validateLogoOptionValue(overLimit))
}
