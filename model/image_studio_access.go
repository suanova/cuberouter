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
	"strings"
)

// ImageStudioGroupAllowed applies to account membership, never a caller-selected
// token group. An unset allowlist defaults to the dedicated trial group.
func ImageStudioGroupAllowed(group string) bool {
	allowed, configured := os.LookupEnv("IMAGE_STUDIO_ALLOWED_GROUPS")
	if !configured {
		allowed = "image-studio"
	}
	if group == "" {
		return false
	}
	for _, item := range strings.Split(allowed, ",") {
		if candidate := strings.TrimSpace(item); candidate != "" && candidate == group {
			return true
		}
	}
	return false
}
