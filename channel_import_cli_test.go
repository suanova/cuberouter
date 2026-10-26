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

package main

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelImportCLIRejectsPositionalArguments(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		unexpected string
	}{
		{"trailing positional arguments", []string{"-f", "channels.yaml", "first", "second"}, `["first" "second"]`},
		{"flag after positional argument", []string{"-f", "channels.yaml", "stray", "--dry-run"}, `["stray" "--dry-run"]`},
		{"arguments after terminator", []string{"-f", "channels.yaml", "--", "--dry-run"}, `["--dry-run"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			originalStderr := os.Stderr
			reader, writer, err := os.Pipe()
			require.NoError(t, err)
			t.Cleanup(func() {
				os.Stderr = originalStderr
				_ = reader.Close()
				_ = writer.Close()
			})
			os.Stderr = writer

			status := runChannelImportCLI(tc.args)
			require.NoError(t, writer.Close())
			output, err := io.ReadAll(reader)
			require.NoError(t, err)

			assert.Equal(t, 2, status)
			assert.Contains(t, string(output), "error: unexpected arguments: "+tc.unexpected)
			assert.Contains(t, string(output), "Usage: cuberouter channels import")
		})
	}
}
