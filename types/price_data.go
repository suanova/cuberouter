package types

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

type GroupRatioInfo struct {
	GroupRatio        float64
	GroupSpecialRatio float64
	HasSpecialRatio   bool
}

type PriceData struct {
	FreeModel            bool
	ModelPrice           float64
	ModelRatio           float64
	CompletionRatio      float64
	CacheRatio           float64
	CacheCreationRatio   float64
	CacheCreation5mRatio float64
	CacheCreation1hRatio float64
	ImageRatio           float64
	AudioRatio           float64
	AudioCompletionRatio float64
	otherRatios          map[string]float64
	UsePrice             bool
	Quota                int // 按次计费的最终额度（MJ / Task）
	QuotaToPreConsume    int // 按量计费的预消耗额度
	PreConsumeDetail     *PreConsumeDetail
	GroupRatioInfo       GroupRatioInfo
}

// PreConsumeDetail 记录预扣额度的计算过程，随消费日志输出，便于追溯预扣值来源。
type PreConsumeDetail struct {
	EstimatedPromptTokens int     `json:"estimated_prompt_tokens"` // 请求前本地估算的 prompt tokens（CountToken 关闭时为 0）
	FloorTokens           int     `json:"floor_tokens"`            // 后台 PreConsumedQuota 保底设置
	RequestMaxTokens      int     `json:"request_max_tokens"`      // 请求携带的 max_tokens（0=未携带）
	PreConsumedTokens     int     `json:"pre_consumed_tokens"`     // max(估算, 保底) + max_tokens
	ModelRatio            float64 `json:"model_ratio"`             // 模型倍率
	GroupRatio            float64 `json:"group_ratio"`             // 分组倍率
	UsePrice              bool    `json:"use_price"`               // true=按次计费（quota = 单价 × QuotaPerUnit × 分组倍率）
	Quota                 int     `json:"quota"`                   // 最终预扣额度
	// 估算构成分解（CountToken 关闭时为 nil）
	EstimateBreakdown *TokenEstimateBreakdown `json:"estimate_breakdown,omitempty"`
}

// TokenEstimateBreakdown 估算 tokens 的构成（与 service 层共享定义，避免循环依赖由 types 承载）。
type TokenEstimateBreakdown struct {
	MethodName       string `json:"method_name"`       // 估算方式：heuristic / rune_count
	TextTokens       int    `json:"text_tokens"`       // 文本分词（含 role 等拼接文本）
	MessagesOverhead int    `json:"messages_overhead"` // 每条消息格式化开销（MessagesCount × 3）
	ToolsOverhead    int    `json:"tools_overhead"`    // 工具定义开销（ToolsCount × 8）
	NamesOverhead    int    `json:"names_overhead"`    // 具名消息开销（NameCount × 3）
	BaseOverhead     int    `json:"base_overhead"`     // 基础开销（OpenAI 格式固定 +3）
	MediaTokens      int    `json:"media_tokens"`      // 图片/音频/文件等媒体 token
	Total            int    `json:"total"`             // 与估算返回值一致
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if !isValidOtherRatio(ratio) {
		return
	}
	if p.otherRatios == nil {
		p.otherRatios = make(map[string]float64)
	}
	p.otherRatios[key] = ratio
}

func (p *PriceData) ReplaceOtherRatios(ratios map[string]float64) bool {
	p.otherRatios = nil
	for key, ratio := range ratios {
		p.AddOtherRatio(key, ratio)
	}
	return len(p.otherRatios) > 0
}

func (p *PriceData) HasOtherRatio(key string) bool {
	ratio, ok := p.otherRatios[key]
	return ok && isValidOtherRatio(ratio)
}

func (p *PriceData) OtherRatios() map[string]float64 {
	if len(p.otherRatios) == 0 {
		return nil
	}
	ratios := make(map[string]float64, len(p.otherRatios))
	for key, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) {
			ratios[key] = ratio
		}
	}
	if len(ratios) == 0 {
		return nil
	}
	return ratios
}

func (p *PriceData) OtherRatioMultiplier() float64 {
	multiplier := 1.0
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			multiplier *= ratio
		}
	}
	return multiplier
}

func (p *PriceData) ApplyOtherRatiosToFloat(value float64) float64 {
	return value * p.OtherRatioMultiplier()
}

func (p *PriceData) ApplyOtherRatiosToDecimal(value decimal.Decimal) decimal.Decimal {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value = value.Mul(decimal.NewFromFloat(ratio))
		}
	}
	return value
}

func (p *PriceData) RemoveOtherRatiosFromFloat(value float64) float64 {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value /= ratio
		}
	}
	return value
}

func isValidOtherRatio(ratio float64) bool {
	return ratio > 0 && !math.IsInf(ratio, 1)
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio)
}
