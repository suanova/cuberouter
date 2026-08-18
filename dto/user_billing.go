package dto

// BillingModelStat 账单报表中按模型维度的统计行。
// Amount 为按站点展示类型(USD/CNY/TOKENS)换算后的金额;Quota 为原始额度。
type BillingModelStat struct {
	ModelName        string  `json:"model_name"`
	RequestCount     int     `json:"request_count"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CacheTokens      int     `json:"cache_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Quota            int     `json:"quota"`
	Amount           float64 `json:"amount"`
}

// BillingDailyStat 按自然日聚合的账单行,内含当日按模型分组明细与日合计。
type BillingDailyStat struct {
	Date             string             `json:"date"`
	Models           []BillingModelStat `json:"models"`
	RequestCount     int                `json:"request_count"`
	PromptTokens     int                `json:"prompt_tokens"`
	CompletionTokens int                `json:"completion_tokens"`
	CacheTokens      int                `json:"cache_tokens"`
	TotalTokens      int                `json:"total_tokens"`
	Quota            int                `json:"quota"`
	Amount           float64            `json:"amount"`
}

// BillingCurrency 当前站点的货币展示信息。
type BillingCurrency struct {
	Type   string `json:"type"`
	Symbol string `json:"symbol"`
}

// BillingRange 时间范围回显。
type BillingRange struct {
	Start          string `json:"start"`
	End            string `json:"end"`
	StartTimestamp int64  `json:"start_timestamp"`
	EndTimestamp   int64  `json:"end_timestamp"`
}

// BillingReportData 用户计费账单报表响应体。
type BillingReportData struct {
	User     UserDashboardBrief `json:"user"`
	Currency BillingCurrency    `json:"currency"`
	Range    BillingRange       `json:"range"`
	Summary  []BillingModelStat `json:"summary"`
	Daily    []BillingDailyStat `json:"daily"`
}

// ReconciliationUserStat 计费对账报表:按用户汇总的总金额(无明细)。
type ReconciliationUserStat struct {
	UserId           int     `json:"user_id"`
	Username         string  `json:"username"`
	DisplayName      string  `json:"display_name"`
	Group            string  `json:"group"`
	Role             int     `json:"role"`
	RequestCount     int     `json:"request_count"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CacheTokens      int     `json:"cache_tokens"`
	Quota            int     `json:"quota"`
	Amount           float64 `json:"amount"`
	ModelCount       int     `json:"model_count"`
}

// ReconciliationTotals 对账报表全平台合计(便于核对总额)。
type ReconciliationTotals struct {
	UserCount        int     `json:"user_count"`
	RequestCount     int     `json:"request_count"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CacheTokens      int     `json:"cache_tokens"`
	Quota            int     `json:"quota"`
	Amount           float64 `json:"amount"`
}

// ReconciliationReportData 计费对账报表响应体:全量用户汇总,按金额降序。
type ReconciliationReportData struct {
	Currency BillingCurrency          `json:"currency"`
	Range    BillingRange             `json:"range"`
	Total    ReconciliationTotals     `json:"total"`
	Users    []ReconciliationUserStat `json:"users"`
}
