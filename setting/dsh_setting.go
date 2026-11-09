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
package setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	// DshAllowedOrganizationIdsKey stores the JSON array of organization IDs
	// whose active members may use the DSH Agent. An empty list closes the
	// feature for everyone except platform administrators.
	DshAllowedOrganizationIdsKey = "DshAllowedOrganizationIds"
	// DshTaskPluginKey is the factory key of the bundled DSH task plugin.
	DshTaskPluginKey = "dsh"

	maxDshAllowedOrganizationIds = 1000
)

// GetDshAllowedOrganizationIds returns the configured allowlist. Invalid
// stored values fail closed: callers get an empty list, never a partial one.
func GetDshAllowedOrganizationIds() []int {
	common.OptionMapRWMutex.RLock()
	raw := ""
	if common.OptionMap != nil {
		raw = common.OptionMap[DshAllowedOrganizationIdsKey]
	}
	common.OptionMapRWMutex.RUnlock()

	ids, err := parseDshAllowedOrganizationIds(raw)
	if err != nil {
		return []int{}
	}
	return ids
}

func parseDshAllowedOrganizationIds(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int{}, nil
	}
	var ids []int
	if err := common.UnmarshalJsonStr(raw, &ids); err != nil {
		return nil, err
	}
	seen := make(map[int]struct{}, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("organization id must be positive: %d", id)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

// ValidateDshAllowedOrganizationIds rejects malformed values before they are
// persisted through the option API.
func ValidateDshAllowedOrganizationIds(value string) error {
	ids, err := parseDshAllowedOrganizationIds(value)
	if err != nil {
		return err
	}
	if len(ids) > maxDshAllowedOrganizationIds {
		return fmt.Errorf("too many organization ids: %d (max %d)", len(ids), maxDshAllowedOrganizationIds)
	}
	return nil
}
