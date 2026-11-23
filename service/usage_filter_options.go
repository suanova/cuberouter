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

package service

import "sort"

// usageModelFilterOptions returns the scope's model catalog plus every model
// that appears in the usage rows, sorted. The catalog keeps models the viewer
// can use today; the rows keep historical or retired models filterable from
// the table they appear in.
func usageModelFilterOptions(catalog []string, rows []usageRow) []string {
	seen := make(map[string]struct{}, len(catalog)+len(rows))
	for _, name := range catalog {
		if name != "" {
			seen[name] = struct{}{}
		}
	}
	for _, row := range rows {
		if row.ModelName != "" {
			seen[row.ModelName] = struct{}{}
		}
	}
	models := make([]string, 0, len(seen))
	for name := range seen {
		models = append(models, name)
	}
	sort.Strings(models)
	return models
}
