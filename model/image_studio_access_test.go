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
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioGroupAllowlist(t *testing.T) {
	t.Setenv("IMAGE_STUDIO_ALLOWED_GROUPS", "")
	require.NoError(t, os.Unsetenv("IMAGE_STUDIO_ALLOWED_GROUPS"))
	require.True(t, ImageStudioGroupAllowed("image-studio"))
	require.False(t, ImageStudioGroupAllowed("default"))
	for _, tc := range []struct {
		allow, group string
		want         bool
	}{
		{"", "image-studio", false},
		{"image-studio", "", false},
		{"image-studio", "image-studio-extra", false},
		{"image-studio", "IMAGE-STUDIO", false},
		{" image-studio, approved-trial ", "approved-trial", true},
		{"*", "default", false},
	} {
		t.Setenv("IMAGE_STUDIO_ALLOWED_GROUPS", tc.allow)
		require.Equal(t, tc.want, ImageStudioGroupAllowed(tc.group), "%q / %q", tc.allow, tc.group)
	}
}
