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

// SelfUsageData is the response of GET /api/usage/self.
type SelfUsageData struct {
	BackfillComplete bool                   `json:"backfill_complete"`
	Range            SelfUsageRange         `json:"range"`
	Overview         SelfUsageOverview      `json:"overview"`
	Rows             SelfUsageRows          `json:"rows"`
	Totals           SelfUsageTotals        `json:"totals"`
	Charts           []SelfUsageChartPoint  `json:"charts"`
	FilterOptions    SelfUsageFilterOptions `json:"filter_options"`
}

type SelfUsageRange struct {
	Start          string `json:"start"`
	End            string `json:"end"`
	StartTimestamp int64  `json:"start_timestamp"`
	EndTimestamp   int64  `json:"end_timestamp"`
}

type SelfUsageAccountStat struct {
	RequestCount     int `json:"request_count"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CacheTokens      int `json:"cache_tokens"`
	Quota            int `json:"quota"`
	RemainingQuota   int `json:"remaining_quota"`
}

type SelfUsageOrganizationStat struct {
	SelfUsageAccountStat
	Id                    int    `json:"id"`
	Name                  string `json:"name"`
	MemberQuotaLimit      int    `json:"member_quota_limit"`
	MemberUsedQuota       int    `json:"member_used_quota"`
	OrganizationQuota     int    `json:"organization_quota"`
	OrganizationUsedQuota int    `json:"organization_used_quota"`
}

type SelfUsageOverview struct {
	Personal     SelfUsageAccountStat       `json:"personal"`
	Organization *SelfUsageOrganizationStat `json:"organization"`
}

type SelfUsageRow struct {
	Date               string `json:"date"`
	Type               string `json:"type"`
	BillingAccountType string `json:"billing_account_type"`
	OrganizationId     int    `json:"organization_id"`
	OrganizationName   string `json:"organization_name"`
	TokenId            int    `json:"token_id"`
	TokenName          string `json:"token_name"`
	ModelName          string `json:"model_name"`
	RequestCount       int    `json:"request_count"`
	SuccessCount       int    `json:"success_count"`
	FailureCount       int    `json:"failure_count"`
	PromptTokens       int    `json:"prompt_tokens"`
	CompletionTokens   int    `json:"completion_tokens"`
	CacheTokens        int    `json:"cache_tokens"`
	Quota              int    `json:"quota"`
}

type SelfUsageRows struct {
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int            `json:"total"`
	Items    []SelfUsageRow `json:"items"`
}

type SelfUsageTotals struct {
	RequestCount     int `json:"request_count"`
	SuccessCount     int `json:"success_count"`
	FailureCount     int `json:"failure_count"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CacheTokens      int `json:"cache_tokens"`
	Quota            int `json:"quota"`
}

// SelfUsageChartPoint is one day/model bucket of the filtered usage rows. The
// chart series shares the table's filters and accounting scope so the two
// never disagree.
type SelfUsageChartPoint struct {
	Date         string `json:"date"`
	ModelName    string `json:"model_name"`
	RequestCount int    `json:"request_count"`
	Quota        int    `json:"quota"`
}

// SelfUsageFilterOptions lists the models the caller can see and the API keys
// the caller can use (personal keys plus keys of organizations they belong to).
// Organization keys carry the organization so the UI can label them
// "key (organization)" and tell same-named keys apart.
type SelfUsageFilterOptions struct {
	Models []string               `json:"models"`
	Tokens []SelfUsageTokenOption `json:"tokens"`
}

type SelfUsageTokenOption struct {
	Id               int    `json:"id"`
	Name             string `json:"name"`
	OrganizationId   int    `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
}
