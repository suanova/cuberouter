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
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	// DshAllowedOrganizationIdsKey stores the JSON array of organization IDs
	// whose active members may use the DSH Agent. An organization must also be
	// bound to a channel before it counts as enabled.
	DshAllowedOrganizationIdsKey = "DshAllowedOrganizationIds"
	// DshOrganizationChannelsKey stores the JSON object binding organization
	// IDs to their dedicated DSH task-plugin channel: {"12": 34}.
	DshOrganizationChannelsKey = "DshOrganizationChannels"
	// DshOrganizationDeepseekSecretKey stores the JSON object holding each
	// organization's DeepSeek API key: {"12": "sk-..."}. The option NAME must
	// keep the "Key" suffix so controller.GetOptions filters it out; the Go
	// constant avoids colliding with GetDshOrganizationDeepseekKey.
	DshOrganizationDeepseekSecretKey = "DshOrganizationDeepseekKey"
	// DshTaskPluginKey is the factory key of the bundled DSH task plugin.
	DshTaskPluginKey = "dsh"

	maxDshAllowedOrganizationIds  = 1000
	maxDshOrganizationSecretBytes = 512
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

// GetDshOrganizationChannels returns the organization -> channel binding map.
// Invalid stored values fail closed: callers get an empty map, never a partial
// one, so a typo cannot silently enable the wrong organization.
func GetDshOrganizationChannels() map[int]int {
	common.OptionMapRWMutex.RLock()
	raw := ""
	if common.OptionMap != nil {
		raw = common.OptionMap[DshOrganizationChannelsKey]
	}
	common.OptionMapRWMutex.RUnlock()

	bindings, err := parseDshOrganizationChannels(raw)
	if err != nil {
		common.SysLog(fmt.Sprintf("dsh setting: invalid %s: %s", DshOrganizationChannelsKey, err.Error()))
		return map[int]int{}
	}
	return bindings
}

func parseDshOrganizationChannels(raw string) (map[int]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[int]int{}, nil
	}
	var bindings map[int]int
	if err := common.UnmarshalJsonStr(raw, &bindings); err != nil {
		return nil, err
	}
	if len(bindings) > maxDshAllowedOrganizationIds {
		return nil, fmt.Errorf("too many organization bindings: %d (max %d)", len(bindings), maxDshAllowedOrganizationIds)
	}
	for organizationId, channelId := range bindings {
		if organizationId <= 0 {
			return nil, fmt.Errorf("organization id must be positive: %d", organizationId)
		}
		if channelId <= 0 {
			return nil, fmt.Errorf("channel id must be positive: %d", channelId)
		}
	}
	return bindings, nil
}

// ValidateDshOrganizationChannels rejects malformed values before they are
// persisted. Empty strings and "{}" are both valid (no bindings yet).
func ValidateDshOrganizationChannels(value string) error {
	_, err := parseDshOrganizationChannels(value)
	return err
}

// GetDshOrganizationDeepseekKey returns one organization's stored DeepSeek key,
// or "" when it has none.
func GetDshOrganizationDeepseekKey(organizationId int) string {
	if organizationId <= 0 {
		return ""
	}
	return GetDshOrganizationDeepseekKeys()[organizationId]
}

// GetDshOrganizationDeepseekKeys returns every stored organization key. Callers
// that only need one entry (or none) should use GetDshOrganizationDeepseekKey.
func GetDshOrganizationDeepseekKeys() map[int]string {
	common.OptionMapRWMutex.RLock()
	raw := ""
	if common.OptionMap != nil {
		raw = common.OptionMap[DshOrganizationDeepseekSecretKey]
	}
	common.OptionMapRWMutex.RUnlock()

	keys, err := parseDshOrganizationDeepseekKeys(raw)
	if err != nil {
		common.SysLog(fmt.Sprintf("dsh setting: invalid %s: %s", DshOrganizationDeepseekSecretKey, err.Error()))
		return map[int]string{}
	}
	return keys
}

func parseDshOrganizationDeepseekKeys(raw string) (map[int]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[int]string{}, nil
	}
	var keys map[int]string
	if err := common.UnmarshalJsonStr(raw, &keys); err != nil {
		return nil, err
	}
	if len(keys) > maxDshAllowedOrganizationIds {
		return nil, fmt.Errorf("too many organization secrets: %d (max %d)", len(keys), maxDshAllowedOrganizationIds)
	}
	for organizationId, key := range keys {
		if organizationId <= 0 {
			return nil, fmt.Errorf("organization id must be positive: %d", organizationId)
		}
		if err := validateDshOrganizationSecret(key); err != nil {
			return nil, fmt.Errorf("organization %d: %w", organizationId, err)
		}
	}
	return keys, nil
}

func validateDshOrganizationSecret(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("secret must not be empty")
	}
	if len(key) > maxDshOrganizationSecretBytes {
		return fmt.Errorf("secret is too long: %d bytes (max %d)", len(key), maxDshOrganizationSecretBytes)
	}
	for _, char := range key {
		if char < 0x20 || char == 0x7f {
			return fmt.Errorf("secret must not contain control characters")
		}
	}
	return nil
}

// ValidateDshOrganizationSecretKey validates a single organization DeepSeek key
// before it is stored in the per-organization map.
func ValidateDshOrganizationSecretKey(key string) error {
	return validateDshOrganizationSecret(key)
}

// ValidateDshOrganizationDeepseekKey rejects malformed values before they are
// persisted. Values that store at least one secret must pass the per-secret
// rules above.
func ValidateDshOrganizationDeepseekKey(value string) error {
	_, err := parseDshOrganizationDeepseekKeys(value)
	return err
}

// DshEnabledOrganizationIds is the single definition of "enabled": the
// organization is on the administrator allowlist AND bound to a channel.
// Returns the intersection sorted ascending; either side empty disables DSH.
func DshEnabledOrganizationIds() []int {
	allowed := GetDshAllowedOrganizationIds()
	if len(allowed) == 0 {
		return []int{}
	}
	bindings := GetDshOrganizationChannels()
	if len(bindings) == 0 {
		return []int{}
	}
	enabled := make([]int, 0, len(allowed))
	for _, organizationId := range allowed {
		if _, ok := bindings[organizationId]; ok {
			enabled = append(enabled, organizationId)
		}
	}
	sort.Ints(enabled)
	return enabled
}
