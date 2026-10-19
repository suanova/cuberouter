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
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	// MaxMemberModelAccessEntries bounds the number of models accepted for one organization member.
	MaxMemberModelAccessEntries = 2000
	// MaxMemberModelAccessBytes bounds the serialized size of one member's model
	// access list. MySQL stores the column and the audit snapshots as TEXT,
	// which holds at most 65535 bytes, so the cap keeps headroom below that
	// limit instead of accepting a list the database cannot store (a strict
	// write would fail; a non-strict one would truncate it into "unrestricted").
	MaxMemberModelAccessBytes = 60 * 1024
	// MaxMemberModelNameLen bounds the byte length of a model name in a member model access list.
	MaxMemberModelNameLen = 255
)

// cleanMemberModelAccess trims every entry, drops blanks and duplicates, and
// returns the unique names in their input order. Callers sort the result when a
// stable payload is needed.
func cleanMemberModelAccess(models []string) []string {
	cleaned := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		name := strings.TrimSpace(model)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		cleaned = append(cleaned, name)
	}
	return cleaned
}

// ParseMemberModelAccess decodes a stored member model access list for the read
// path. It never fails the caller: the empty string and the JSON null yield nil,
// and a value that cannot be decoded at all yields nil with a log line. Entries
// are trimmed, blanks and duplicates are dropped while unique names keep their
// stored order, and names longer than MaxMemberModelNameLen are skipped. A list
// longer than MaxMemberModelAccessEntries is truncated, so a corrupted row can
// never hand an unbounded set to the caller.
func ParseMemberModelAccess(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var parsed []string
	if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
		common.SysError("failed to parse member model access: " + err.Error())
		return nil
	}
	models := make([]string, 0, len(parsed))
	for _, model := range cleanMemberModelAccess(parsed) {
		if len(model) > MaxMemberModelNameLen {
			common.SysError("skipped member model access entry with overlong model name")
			continue
		}
		models = append(models, model)
	}
	if len(models) > MaxMemberModelAccessEntries {
		common.SysError(fmt.Sprintf("member model access has %d entries, truncated to %d", len(models), MaxMemberModelAccessEntries))
		models = models[:MaxMemberModelAccessEntries]
	}
	if len(models) == 0 {
		return nil
	}
	return models
}

// MarshalMemberModelAccess serializes a member model access list. Entries are
// trimmed, blanks and duplicates are dropped, and the result is sorted so the
// stored payload is stable. The empty list is the empty string, matching the
// stored column value for "no restriction".
func MarshalMemberModelAccess(models []string) string {
	cleaned := cleanMemberModelAccess(models)
	if len(cleaned) == 0 {
		return ""
	}
	sort.Strings(cleaned)
	raw, err := common.Marshal(cleaned)
	if err != nil {
		common.SysError("failed to marshal member model access: " + err.Error())
		return ""
	}
	return string(raw)
}

// ValidateMemberModelAccess validates a member model access list for the write
// path. Every entry must be non-blank and at most MaxMemberModelNameLen bytes
// after trimming, the input must hold at most MaxMemberModelAccessEntries
// entries, and the serialized list must fit MaxMemberModelAccessBytes. The
// empty list is valid and means "no restriction". Errors are plain values for
// the controller to map onto i18n keys.
func ValidateMemberModelAccess(models []string) error {
	if len(models) > MaxMemberModelAccessEntries {
		return fmt.Errorf("too many member model access entries: %d (max %d)", len(models), MaxMemberModelAccessEntries)
	}
	for i, model := range models {
		name := strings.TrimSpace(model)
		if name == "" {
			return fmt.Errorf("member model access entry %d: model name must not be empty", i)
		}
		if len(name) > MaxMemberModelNameLen {
			return fmt.Errorf("member model access entry %d: model name too long (%d bytes, max %d): %s", i, len(name), MaxMemberModelNameLen, name)
		}
	}
	if payload := MarshalMemberModelAccess(models); len(payload) > MaxMemberModelAccessBytes {
		return fmt.Errorf("member model access payload too large: %d bytes (max %d)", len(payload), MaxMemberModelAccessBytes)
	}
	return nil
}
