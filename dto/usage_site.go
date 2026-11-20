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

package dto

// SiteUsageData is the response of GET /api/usage/site. It shares the table,
// totals, and chart shapes with SelfUsageData but has no personal/organization
// overview: the site view is an aggregate, not an account posture.
type SiteUsageData struct {
	BackfillComplete bool                   `json:"backfill_complete"`
	Range            SelfUsageRange         `json:"range"`
	Rows             SelfUsageRows          `json:"rows"`
	Totals           SelfUsageTotals        `json:"totals"`
	Charts           []SelfUsageChartPoint  `json:"charts"`
	FilterOptions    SiteUsageFilterOptions `json:"filter_options"`
}

type SiteUsageFilterOptions struct {
	Models        []string                      `json:"models"`
	Tokens        []SelfUsageTokenOption        `json:"tokens"`
	Users         []SiteUsageUserOption         `json:"users"`
	Organizations []SiteUsageOrganizationOption `json:"organizations"`
}

type SiteUsageUserOption struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type SiteUsageOrganizationOption struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}
