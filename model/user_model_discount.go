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
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	// MaxUserModelDiscountEntries bounds the number of model discount entries accepted for one user.
	MaxUserModelDiscountEntries = 500
	// MaxUserModelDiscountBytes bounds the serialized size of one user's model discount table.
	MaxUserModelDiscountBytes = 64 * 1024
	// MaxUserModelDiscountNameLen bounds the byte length of a discount model name.
	MaxUserModelDiscountNameLen = 255
)

var (
	// ErrUserModelDiscountRange reports a discount outside (0, 1] or not finite.
	ErrUserModelDiscountRange = errors.New("user model discount out of range")
	// ErrUserModelDiscountName reports an empty or overlong model name.
	ErrUserModelDiscountName = errors.New("invalid user model discount model name")
	// ErrUserModelDiscountCount reports a table with too many entries.
	ErrUserModelDiscountCount = errors.New("too many user model discount entries")
	// ErrUserModelDiscountSize reports a serialized table that is too large.
	ErrUserModelDiscountSize = errors.New("user model discount payload too large")
)

// isValidUserModelDiscount reports whether a discount may scale billing: it must
// be finite and within (0, 1]. A zero discount is rejected so no model can be
// turned free by a discount entry.
func isValidUserModelDiscount(discount float64) bool {
	return !math.IsNaN(discount) && !math.IsInf(discount, 0) && discount > 0 && discount <= 1
}

// NormalizeUserModelDiscounts validates a discount table for the write path.
// Model names are trimmed, discounts are rounded to four decimal places, and
// full price entries (d == 1) are dropped because they are equivalent to no
// entry at all. Validation applies to the rounded value, which is what gets
// stored, so a discount that rounds to zero is rejected and one that rounds to
// full price is dropped. The result is never nil. Structural limits are checked
// before per-entry validation, so an oversized payload is refused as a whole.
func NormalizeUserModelDiscounts(input map[string]float64) (map[string]float64, error) {
	if len(input) > MaxUserModelDiscountEntries {
		return nil, ErrUserModelDiscountCount
	}
	if serialized := MarshalUserModelDiscounts(input); len(serialized) > MaxUserModelDiscountBytes {
		return nil, ErrUserModelDiscountSize
	}
	normalized := make(map[string]float64, len(input))
	for name, discount := range input {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > MaxUserModelDiscountNameLen {
			return nil, ErrUserModelDiscountName
		}
		discount = math.Round(discount*10000) / 10000
		if !isValidUserModelDiscount(discount) {
			return nil, fmt.Errorf("%w: %s", ErrUserModelDiscountRange, name)
		}
		if discount == 1 {
			continue
		}
		normalized[name] = discount
	}
	return normalized, nil
}

// ParseUserModelDiscounts decodes a stored discount table for the read path.
// It never fails the caller: a table that cannot be decoded at all yields nil,
// and a single invalid entry is skipped with a log line while the rest of the
// table still applies. Valid entries are returned as stored, without rounding.
func ParseUserModelDiscounts(raw string) map[string]float64 {
	if raw == "" {
		return nil
	}
	parsed := make(map[string]float64)
	if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
		common.SysError("failed to parse user model discounts: " + err.Error())
		return nil
	}
	discounts := make(map[string]float64, len(parsed))
	for name, discount := range parsed {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > MaxUserModelDiscountNameLen {
			common.SysError("skipped user model discount entry with invalid model name")
			continue
		}
		if !isValidUserModelDiscount(discount) {
			common.SysError("skipped invalid user model discount for model " + name)
			continue
		}
		discounts[name] = discount
	}
	return discounts
}

// MarshalUserModelDiscounts serializes a discount table. The empty table is the
// empty string, matching the stored column value for "no discounts".
func MarshalUserModelDiscounts(discounts map[string]float64) string {
	if len(discounts) == 0 {
		return ""
	}
	raw, err := common.Marshal(discounts)
	if err != nil {
		common.SysError("failed to marshal user model discounts: " + err.Error())
		return ""
	}
	return string(raw)
}

// LookupModelDiscount returns the discount that applies to modelName, or 1.0
// when no entry matches. Stored values are re-validated here, so an invalid
// entry can never scale billing; it falls back to full price with a log line.
func LookupModelDiscount(discounts map[string]float64, modelName string) float64 {
	discount, ok := discounts[modelName]
	if !ok {
		return 1.0
	}
	if !isValidUserModelDiscount(discount) {
		common.SysError("invalid user model discount for model " + modelName + ", using full price")
		return 1.0
	}
	return discount
}
