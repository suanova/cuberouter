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
package common

import (
	"fmt"
	"strings"
)

const (
	maxEmailDomainListEntries = 10000
	maxEmailDomainLength      = 253
)

// ParseEmailDomainList splits a comma-separated domain list, normalizes each
// entry (trim + lowercase), drops empty entries, and deduplicates while
// preserving first-seen order.
func ParseEmailDomainList(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	entries := make([]string, 0, len(parts))
	for _, part := range parts {
		entry := strings.ToLower(strings.TrimSpace(part))
		if entry == "" {
			continue
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		entries = append(entries, entry)
	}
	return entries
}

// ValidateEmailDomainList validates entries as exact domains or *.suffix
// wildcards. Empty entries are ignored and an empty list is valid.
func ValidateEmailDomainList(entries []string) error {
	if len(entries) > maxEmailDomainListEntries {
		return fmt.Errorf("邮箱域名黑名单最多允许 %d 条", maxEmailDomainListEntries)
	}
	for _, raw := range entries {
		entry := strings.ToLower(strings.TrimSpace(raw))
		if entry == "" {
			continue
		}
		domain := entry
		if strings.HasPrefix(domain, "*.") {
			domain = domain[2:]
		}
		if strings.Contains(domain, "*") {
			return fmt.Errorf("邮箱域名黑名单条目无效: %q", entry)
		}
		if len(domain) == 0 || len(domain) > maxEmailDomainLength || !strings.Contains(domain, ".") {
			return fmt.Errorf("邮箱域名黑名单条目无效: %q", entry)
		}
		for _, label := range strings.Split(domain, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return fmt.Errorf("邮箱域名黑名单条目无效: %q", entry)
			}
			for _, r := range label {
				if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
					return fmt.Errorf("邮箱域名黑名单条目无效: %q", entry)
				}
			}
		}
	}
	return nil
}

// EmailDomainMatches reports whether domain matches entry. entry may be an
// exact domain or a "*.suffix" wildcard that matches subdomains only.
func EmailDomainMatches(domain, entry string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	entry = strings.ToLower(strings.TrimSpace(entry))
	if domain == "" || entry == "" {
		return false
	}
	if strings.HasPrefix(entry, "*.") {
		suffix := entry[1:]
		return len(domain) > len(suffix) && strings.HasSuffix(domain, suffix)
	}
	return domain == entry
}
