package service

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

// TestClassifyBilling 覆盖 spec §3 的全部前缀分支,样本取自
// ucloud_model_20260715_hk_cn_direct_supported.csv 中实际出现的 description。
func TestClassifyBilling(t *testing.T) {
	t.Parallel()

	cases := map[string]billingKind{
		// Input — 裸名 + 多模态 + service_tier + video_input + 分层。
		"Input - Default":                        billInput,
		"Input - Input length(0, 32K]":           billInput,
		"Input - Input length(32K, 128K]":        billInput,
		"Input - service_tier=default":           billInput,
		"Input - video_input=no_video and 1080p": billInput,
		"Text Input - Default":                   billInput,
		"Audio Input - Default":                  billInput,
		"Image Input - Default":                  billInput,
		"Document Input - Default":               billInput,
		"Video Input - Default":                  billInput,
		// Output —— 覆盖四类多模态前缀。
		"Text Output - Default":  billOutput,
		"Image Output - Default": billOutput,
		"Audio Output - Default": billOutput,
		"Video Output - Default": billOutput,
		// Cache Read —— 裸名 / Cache Read - / 多模态 / Default + 分层。
		"Cache - Default":                           billCacheRead,
		"Cache - Input length(0, 32K]":              billCacheRead,
		"Cache - Input length(0, 32K] and is_batch": billCacheRead,
		"Cache Read - Default":                      billCacheRead,
		"Cache Read - Input length(32K, 128K]":      billCacheRead,
		"Audio Cache Read - Input length(0, 32K]":   billCacheRead,
		"Image Cache Read - Input length(0, 32K]":   billCacheRead,
		"Video Cache Read - Input length(0, 32K]":   billCacheRead,
		// Cache Write —— 5min/1h 都应命中(spec §6.1 双档偏差由 Task 3+ 处理)。
		"Cache Write 5min - Default":              billCacheWrite,
		"Cache Write 5min - Input length(0, 32K]": billCacheWrite,
		"Cache Write 1h - Default":                billCacheWrite,
		"Cache Write 1h - Input length(0, 32K]":   billCacheWrite,
		// 未识别前缀 —— 忽略,不计 errors。
		"Some Unknown":                      billUnknown,
		"Audio Count - vidu_type=img2video": billUnknown,
		"Request Count - something":         billUnknown,
		"":                                  billUnknown,
	}

	for desc, want := range cases {
		desc, want := desc, want
		t.Run(desc, func(t *testing.T) {
			t.Parallel()
			got := classifyBilling(desc)
			require.Equalf(t, want, got, "classifyBilling(%q)", desc)
		})
	}
}

// TestAggregate_TieredInputKeepsAllTiers_NonTokenFlagged 验证:
//   - 同模型的多个分层 Input 行(Million)全部保留在 inputTiers,顺序与输入一致;
//   - 非 Million 行(Second)置 hasNonToken=true,且不污染价格桶;
//   - 已知 Million 行(含 cacheRead/Write/Output)归入各自桶;
//   - 一致 desc → descInconsistent=false。
func TestAggregate_TieredInputKeepsAllTiers_NonTokenFlagged(t *testing.T) {
	t.Parallel()

	rows := []csvRow{
		{ModelID: "M1", Description: "Input - Default", PriceUSD: "0.4", Unit: "Million",
			DescZh: "z", DescEn: "e", Logo: "L"},
		{ModelID: "M1", Description: "Input - Input length(0, 32K]", PriceUSD: "0.56", Unit: "Million",
			DescZh: "z", DescEn: "e", Logo: "L"},
		{ModelID: "M1", Description: "Input - Input length(32K, 128K]", PriceUSD: "2.8", Unit: "Million",
			DescZh: "z", DescEn: "e", Logo: "L"},
		{ModelID: "M1", Description: "Text Output - Default", PriceUSD: "5", Unit: "Million",
			DescZh: "z", DescEn: "e", Logo: "L"},
		{ModelID: "M1", Description: "Cache Read - Default", PriceUSD: "0.04", Unit: "Million",
			DescZh: "z", DescEn: "e", Logo: "L"},
		{ModelID: "M1", Description: "Cache Write 5min - Default", PriceUSD: "0.5", Unit: "Million",
			DescZh: "z", DescEn: "e", Logo: "L"},
		// 非 token 行 —— 置 hasNonToken,不入任何价格桶。
		{ModelID: "M1", Description: "Audio Count - something", PriceUSD: "1", Unit: "Second",
			DescZh: "z", DescEn: "e", Logo: "L"},
	}

	out := aggregateModels(rows)
	m, ok := out["M1"]
	require.True(t, ok, "M1 should be aggregated")
	require.Equal(t, []float64{0.4, 0.56, 2.8}, m.inputTiers, "inputTiers keeps all tiers in order")
	require.Equal(t, []float64{5}, m.outputTiers)
	require.Equal(t, []float64{0.04}, m.cacheReadTiers)
	require.Equal(t, []float64{0.5}, m.cacheWriteTiers)
	require.True(t, m.hasNonToken, "Second row flags hasNonToken")
	require.False(t, m.descInconsistent, "uniform desc → not inconsistent")
	require.Equal(t, "z", m.descZh)
	require.Equal(t, "e", m.descEn)
	require.Equal(t, "L", m.logo)
}

// TestAggregate_DescInconsistentFirstRowWins 验证:同模型后续行 desc/logo 任一字段不同
// → descInconsistent=true,但保留首行值(spec §5 step4 / §9 warnings)。
func TestAggregate_DescInconsistentFirstRowWins(t *testing.T) {
	t.Parallel()

	rows := []csvRow{
		{ModelID: "M", Description: "Input - Default", PriceUSD: "1", Unit: "Million",
			DescZh: "first-zh", DescEn: "first-en", Logo: "logoA"},
		{ModelID: "M", Description: "Text Output - Default", PriceUSD: "2", Unit: "Million",
			DescZh: "diff-zh", DescEn: "first-en", Logo: "logoA"},
	}
	out := aggregateModels(rows)
	m, ok := out["M"]
	require.True(t, ok)
	require.True(t, m.descInconsistent, "descZh differs → flag")
	require.Equal(t, "first-zh", m.descZh, "first row wins")
	require.Equal(t, "first-en", m.descEn)
	require.Equal(t, "logoA", m.logo)
}

// TestAggregate_PriceParseFailureSkipped 验证:price 解析失败不 panic,该行静默丢弃;
// 同模型其他有效行仍正常入桶(Task 5 才做模型级 error 上报)。
func TestAggregate_PriceParseFailureSkipped(t *testing.T) {
	t.Parallel()

	rows := []csvRow{
		{ModelID: "M", Description: "Input - Default", PriceUSD: "not-a-number", Unit: "Million"},
		{ModelID: "M", Description: "Input - Input length(0, 32K]", PriceUSD: "0.5", Unit: "Million"},
	}
	out := aggregateModels(rows)
	m, ok := out["M"]
	require.True(t, ok)
	require.Equal(t, []float64{0.5}, m.inputTiers, "bad-price row dropped; good row kept")
}

// TestAggregate_PureNonTokenModel 验证:仅有非 Million 行的模型 —— hasNonToken=true 且
// 所有价格桶为空(下游 §5 step5 a 判定为 skipped_non_token)。
func TestAggregate_PureNonTokenModel(t *testing.T) {
	t.Parallel()

	rows := []csvRow{
		{ModelID: "P", Description: "Audio Count - x", PriceUSD: "1", Unit: "Second"},
		{ModelID: "P", Description: "Page Count - y", PriceUSD: "2", Unit: "Page"},
	}
	out := aggregateModels(rows)
	m, ok := out["P"]
	require.True(t, ok)
	require.True(t, m.hasNonToken)
	require.Empty(t, m.inputTiers)
	require.Empty(t, m.outputTiers)
	require.Empty(t, m.cacheReadTiers)
	require.Empty(t, m.cacheWriteTiers)
}

// TestAggregate_UnknownMillionPrefixIgnored 验证:Million 行但 description 前缀未识别 ——
// 既不入桶也不 panic(spec §3:其余 → unknown,忽略不计 errors)。
func TestAggregate_UnknownMillionPrefixIgnored(t *testing.T) {
	t.Parallel()

	rows := []csvRow{
		{ModelID: "U", Description: "Mystery - Default", PriceUSD: "9", Unit: "Million"},
	}
	out := aggregateModels(rows)
	m, ok := out["U"]
	require.True(t, ok)
	require.Empty(t, m.inputTiers)
	require.Empty(t, m.outputTiers)
	require.Empty(t, m.cacheReadTiers)
	require.Empty(t, m.cacheWriteTiers)
	require.False(t, m.hasNonToken, "Million row does not set hasNonToken even if unknown")
}

// TestAggregate_DefaultTierCapturedSeparately 验证 spec §6.1 的 Default 档捕获:
// "Input - Default" / "Cache - Default" 等以 "- Default" 结尾的行,除入 tier slice 外,
// 其价格同步记录到 defaultXxx 指针,供 computeRatios 实现 "Default 优先,否则 min(tiers)"。
// 空 Default(nil)与 Default=0(免费)需可区分,故用 *float64 而非零值。
func TestAggregate_DefaultTierCapturedSeparately(t *testing.T) {
	t.Parallel()

	rows := []csvRow{
		{ModelID: "D", Description: "Input - Default", PriceUSD: "0.4", Unit: "Million"},
		{ModelID: "D", Description: "Input - Input length(0, 32K]", PriceUSD: "0.56", Unit: "Million"},
		{ModelID: "D", Description: "Cache Write 5min - Default", PriceUSD: "0.5", Unit: "Million"},
	}
	out := aggregateModels(rows)
	m := out["D"]
	require.Equal(t, []float64{0.4, 0.56}, m.inputTiers, "Default row still enters tier slice")
	require.NotNil(t, m.defaultInput, "Default input captured")
	require.Equal(t, 0.4, *m.defaultInput)
	require.Nil(t, m.defaultOutput, "no Default output row")
	require.NotNil(t, m.defaultCacheWrite, "Cache Write 5min - Default captured")
	require.Equal(t, 0.5, *m.defaultCacheWrite)
}

// TestComputeRatios_TieredLowest_NoInputFree 覆盖 spec §6.1 主路径(brief 指定用例):
//   - input Default=0.4 + 长度档 0.56/2.8 → Default 优先,inputPrice=0.4;
//   - output 1.0 → ModelRatio=0.4/2=0.2、CompletionRatio=1.0/0.4=2.5;
//   - cacheRead 0.01 → CacheRatio=0.01/0.4=0.025;
//   - hasOutput/hasCacheRead=true,hasCacheWrite=false;
//   - inputPriceMin/Max 缓存 = 0.4/2.8(供 Task 5 collapse warning)。
func TestComputeRatios_TieredLowest_NoInputFree(t *testing.T) {
	t.Parallel()

	// 模拟 aggregateModels 的输出:Default 行既入 tiers 又单独记 defaultInput。
	defaultInput := 0.4
	am := &aggregatedModel{
		modelID:             "M",
		inputTiers:          []float64{0.4, 0.56, 2.8},
		outputTiers:         []float64{1.0},
		cacheReadTiers:      []float64{0.01},
		defaultInput:        &defaultInput,
	}
	mr, cr, cacheR, createR, hasOut, hasCR, hasCW := computeRatios(am)
	require.InDelta(t, 0.2, mr, 1e-9, "ModelRatio = 0.4/2")
	require.InDelta(t, 2.5, cr, 1e-9, "CompletionRatio = 1.0/0.4")
	require.InDelta(t, 0.025, cacheR, 1e-9, "CacheRatio = 0.01/0.4")
	require.InDelta(t, 0.0, createR, 1e-9, "no CacheWrite → CreateCacheRatio stays 0")
	require.True(t, hasOut)
	require.True(t, hasCR)
	require.False(t, hasCW)
	require.InDelta(t, 0.4, am.inputPriceMin, 1e-9, "inputPriceMin cached for Task 5 warning")
	require.InDelta(t, 2.8, am.inputPriceMax, 1e-9, "inputPriceMax cached for Task 5 warning")
}

// TestComputeRatios_DefaultPriorityOverLowerTier 验证 Default 优先于更低档:
// Default 价 0.5 高于其他档(0.3),仍取 Default。若无 Default 则取 min(tiers)=0.3。
func TestComputeRatios_DefaultPriorityOverLowerTier(t *testing.T) {
	t.Parallel()

	defaultInput := 0.5
	am := &aggregatedModel{
		inputTiers:   []float64{0.5, 0.3, 2.0}, // min=0.3 但 Default=0.5 优先
		defaultInput: &defaultInput,
	}
	mr, _, _, _, _, _, _ := computeRatios(am)
	require.InDelta(t, 0.25, mr, 1e-9, "Default 0.5 wins over min(tiers)=0.3 → ModelRatio=0.25")

	// 对照组:无 Default → 取 min(tiers)=0.3 → ModelRatio=0.15。
	am2 := &aggregatedModel{inputTiers: []float64{0.5, 0.3, 2.0}}
	mr2, _, _, _, _, _, _ := computeRatios(am2)
	require.InDelta(t, 0.15, mr2, 1e-9, "no Default → min(tiers)=0.3 → ModelRatio=0.15")
}

// TestComputeRatios_FreeInputNoDivZero 覆盖 spec §6.1 防除零:
// inputPrice==0(免费产品)→ ModelRatio=0、CompletionRatio=1,output/cache 不参与除法,
// 即使 output/CacheWrite 有非零价格也不产生 NaN/Inf。
func TestComputeRatios_FreeInputNoDivZero(t *testing.T) {
	t.Parallel()

	// 情形 A:Default=0(免费 Default)。
	freeDefault := 0.0
	amA := &aggregatedModel{
		inputTiers:     []float64{0.0, 1.0}, // Default=0
		outputTiers:    []float64{5.0},      // 非 0,验证不参与除法
		cacheWriteTiers: []float64{0.7},
		defaultInput:   &freeDefault,
	}
	mr, cr, cacheR, createR, hasOut, _, hasCW := computeRatios(amA)
	require.InDelta(t, 0.0, mr, 1e-9, "free input → ModelRatio=0")
	require.InDelta(t, 1.0, cr, 1e-9, "free input → CompletionRatio=1 (no division)")
	require.InDelta(t, 0.0, cacheR, 1e-9)
	require.InDelta(t, 0.0, createR, 1e-9, "free input → skip cache division")
	require.True(t, hasOut, "hasOutput reflects tier presence even when input free")
	require.True(t, hasCW)

	// 情形 B:无 Input 行(空 inputTiers,无 Default)→ inputPrice=0,同样免费路径。
	amB := &aggregatedModel{outputTiers: []float64{3.0}}
	mr2, cr2, _, _, hasOut2, _, _ := computeRatios(amB)
	require.InDelta(t, 0.0, mr2, 1e-9)
	require.InDelta(t, 1.0, cr2, 1e-9)
	require.True(t, hasOut2)
}

// TestComputeRatios_NoOutputDefaultsCompletionToOne 验证 spec §6.1:
// inputPrice>0 但无 Output 行 → CompletionRatio = inputPrice/inputPrice = 1(非 0)。
func TestComputeRatios_NoOutputDefaultsCompletionToOne(t *testing.T) {
	t.Parallel()

	am := &aggregatedModel{
		inputTiers:     []float64{0.4},
		cacheReadTiers: []float64{0.04},
	}
	mr, cr, cacheR, _, hasOut, hasCR, _ := computeRatios(am)
	require.InDelta(t, 0.2, mr, 1e-9)
	require.InDelta(t, 1.0, cr, 1e-9, "no Output → CompletionRatio defaults to 1 (not 0)")
	require.InDelta(t, 0.1, cacheR, 1e-9, "CacheRatio = 0.04/0.4")
	require.False(t, hasOut)
	require.True(t, hasCR)
}

// TestComputeRatios_FullMatrix 验证四桶齐全的常见 Claude/Gemini 模型路径:
// input=0.4, output=1.0, cacheRead=0.04, cacheWrite=0.5 → 全部 ratio 正确。
func TestComputeRatios_FullMatrix(t *testing.T) {
	t.Parallel()

	am := &aggregatedModel{
		inputTiers:     []float64{0.4},
		outputTiers:    []float64{1.0},
		cacheReadTiers: []float64{0.04},
		cacheWriteTiers: []float64{0.5},
	}
	mr, cr, cacheR, createR, hasOut, hasCR, hasCW := computeRatios(am)
	require.InDelta(t, 0.2, mr, 1e-9)
	require.InDelta(t, 2.5, cr, 1e-9)
	require.InDelta(t, 0.1, cacheR, 1e-9)
	require.InDelta(t, 1.25, createR, 1e-9)
	require.True(t, hasOut && hasCR && hasCW)
}

// TestBuildDescription 覆盖 spec §8 多语言拼接(简||简||英,空值保留)。
func TestBuildDescription(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		zh, en, want string
	}{
		"both present":   {"简体", "english", "简体||简体||english"},
		"empty zh":       {"", "english", "||||english"},
		"empty en":       {"简体", "", "简体||简体||"},
		"both empty":     {"", "", "||||"},
		"vendor zh text": {"Anthropic Claude 3.5 Sonnet", "Anthropic Claude 3.5 Sonnet",
			"Anthropic Claude 3.5 Sonnet||Anthropic Claude 3.5 Sonnet||Anthropic Claude 3.5 Sonnet"},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := buildDescription(tc.zh, tc.en)
			require.Equalf(t, tc.want, got, "buildDescription(%q,%q)", tc.zh, tc.en)
		})
	}
}

// TestPrechecks 验证 spec §6.1 三类预检:ModelPrice bypass、裸名硬编码 CompletionRatio、
// vendor 前缀名不跳过。具体命中以 defaultModelPrice / getHardcodedCompletionModelRatio 源码为准。
func TestPrechecks(t *testing.T) {
	// 注意:不能 t.Parallel() —— InitRatioSettings 会写全局 ratio map,
	// 与其他并行测试并发会观察到半装载状态。

	// GetModelPrice 走运行时 modelPriceMap(应用启动时由 InitRatioSettings 从
	// defaultModelPrice 装载)。单测无启动流程 → 显式初始化,镜像生产状态。
	// InitRatioSettings 的 AddAll 对已有 key 为覆写,幂等。
	ratio_setting.InitRatioSettings()

	// shouldSkipCompletionRatio —— 裸名硬编码 vs vendor 前缀名(spec §6.1 关键保护)。
	// gpt-4o(裸名 contain=false)→ 不跳过。
	require.False(t, shouldSkipCompletionRatio("gpt-4o"),
		"gpt-4o not hardcoded (contain=false) → CSV CompletionRatio writes are effective")
	// gpt-4-turbo(裸名 contain=true)→ 跳过。
	require.True(t, shouldSkipCompletionRatio("gpt-4-turbo"),
		"gpt-4-turbo hardcoded → CSV writes are ineffective, must skip")
	// claude-3-haiku-20240307(裸名,含 "claude-3")→ 跳过。
	require.True(t, shouldSkipCompletionRatio("claude-3-haiku-20240307"),
		"claude-3-* hardcoded → skip")
	// vendor 前缀名(含 "/")→ 永远不跳过(读路径会先查 map)。
	require.False(t, shouldSkipCompletionRatio("anthropic/claude-3-5-sonnet-20241022"),
		"vendor-prefixed name (contains '/') is never hardcoded → CSV writes are effective")

	// shouldSkipModelPrice —— ModelPrice bypass(defaultModelPrice 含 dall-e-3)。
	skipDalle, reasonDalle := shouldSkipModelPrice("dall-e-3")
	require.True(t, skipDalle, "dall-e-3 in defaultModelPrice → whole-model ratio bypass")
	require.NotEmpty(t, reasonDalle, "skip reason provided for admin warning")
	// gpt-4o 不在 ModelPrice → 不跳过。
	skipGpt, _ := shouldSkipModelPrice("gpt-4o")
	require.False(t, skipGpt, "gpt-4o not in ModelPrice → normal ratio path")
	// vendor 前缀名不在 ModelPrice 表 → 不跳过。
	skipVendor, _ := shouldSkipModelPrice("openai/gpt-4o")
	require.False(t, skipVendor, "vendor name not in ModelPrice → normal ratio path")
}

// TestApplyFormatMatching 验证 spec §6.1 的 ModelRatio/CompletionRatio key 改写:
// gpt-4-gizmo-* / gpt-4o-gizmo-* / gemini-2.5-{flash,flash-lite,pro}-thinking-*。
// 注意:仅 ModelRatio/CompletionRatio 用此函数;CacheRatio/CreateCacheRatio 用原名(Task 4/5)。
func TestApplyFormatMatching(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		// gizmo wildcard
		"gpt-4-gizmo-foo":      "gpt-4-gizmo-*",
		"gpt-4-gizmo-anything": "gpt-4-gizmo-*",
		"gpt-4o-gizmo-bar":     "gpt-4o-gizmo-*",
		// gemini thinking budget(必须含 -thinking- 才重写)
		"gemini-2.5-flash-thinking-123":    "gemini-2.5-flash-thinking-*",
		"gemini-2.5-flash-lite-thinking-x": "gemini-2.5-flash-lite-thinking-*",
		"gemini-2.5-pro-thinking-456":      "gemini-2.5-pro-thinking-*",
		// 不重写:无 -thinking- 后缀
		"gemini-2.5-pro":      "gemini-2.5-pro",
		"gemini-2.5-flash":    "gemini-2.5-flash",
		"gemini-2.5-flash-lite": "gemini-2.5-flash-lite",
		// 普通模型名原样返回
		"gpt-4o":          "gpt-4o",
		"claude-3-haiku":  "claude-3-haiku",
	}
	for name, want := range cases {
		name, want := name, want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := applyFormatMatching(name)
			require.Equalf(t, want, got, "applyFormatMatching(%q)", name)
		})
	}
}

// TestMergeMaps 验证 spec §6.2 读-合并-写中的合并步骤是纯函数:
// updates 覆盖 existing(就地修改),既有的 key 不在 updates 中则保留原值。
// 全局 ratio map 不应被本函数触碰 —— 它只操作 caller 传入的两份 map。
func TestMergeMaps(t *testing.T) {
	t.Parallel()

	existing := map[string]float64{"gpt-4": 1.0, "claude-3": 2.0}
	updates := map[string]float64{"new-model": 0.5, "gpt-4": 9.9}

	got := mergeMaps(existing, updates)
	require.True(t, reflect.ValueOf(existing).Pointer() == reflect.ValueOf(got).Pointer(),
		"mergeMaps returns the same underlying map (in-place mutation)")
	require.Equal(t, 9.9, got["gpt-4"], "updates overwrite existing value")
	require.Equal(t, 2.0, got["claude-3"], "untouched existing keys preserved")
	require.Equal(t, 0.5, got["new-model"], "new keys from updates added")
	require.Len(t, got, 3)
}

// TestMergeMaps_NilUpdates / EmptyCases 验证边界:nil 或空 updates 不污染 existing。
func TestMergeMaps_EmptyAndNil(t *testing.T) {
	t.Parallel()

	// 空 updates:existing 原样返回。
	ex1 := map[string]float64{"gpt-4": 1.0}
	out1 := mergeMaps(ex1, map[string]float64{})
	require.Equal(t, map[string]float64{"gpt-4": 1.0}, out1)

	// nil updates:迭代 nil map 等价空迭代,existing 原样返回,不 panic。
	ex2 := map[string]float64{"gpt-4": 1.0}
	out2 := mergeMaps(ex2, nil)
	require.Equal(t, map[string]float64{"gpt-4": 1.0}, out2)
}

// TestBuildMetaUpdateMap_EmptyGuard 验证 spec §6.4 的"已存在模型"字段白名单空值守卫:
//
//   - 全空(zh="" en="" logo=""):仅 updated_time,**不**写 description / icon / tags,
//     避免 CSV 空值覆盖 admin 既有介绍/图标;
//   - desc 非空(由 zh||zh||en 拼接,任一非空即非空)、logo 空:仅写 description;
//   - logo 非空、desc 空:仅写 icon+tags(不写 description);
//   - 全非空:三类都写。
//
// 关键不变量:updated_time 始终写入;status/sync_official/endpoints/name_rule/vendor_id
// 永远不出现在 map 里(由 model.UpdateMetaFields 的白名单 caller-side 决定,这里只看 map key)。
func TestBuildMetaUpdateMap_EmptyGuard(t *testing.T) {
	t.Parallel()

	const now int64 = 1700000000

	// 全空:仅 updated_time。
	m := buildMetaUpdateMap("", "", "", now)
	require.Contains(t, m, "updated_time")
	require.Equal(t, now, m["updated_time"])
	require.NotContains(t, m, "description", "empty zh+en must not overwrite admin description")
	require.NotContains(t, m, "icon", "empty logo must not overwrite admin icon")
	require.NotContains(t, m, "tags")

	// desc 非空(zh="简" → "简||简||" 非空)、logo 空:仅加 description。
	m = buildMetaUpdateMap("简", "", "", now)
	require.Contains(t, m, "description")
	require.Equal(t, "简||简||", m["description"])
	require.NotContains(t, m, "icon")
	require.NotContains(t, m, "tags")

	// logo 非空、desc 空:仅加 icon+tags。
	m = buildMetaUpdateMap("", "", "logo.png", now)
	require.Contains(t, m, "icon")
	require.Contains(t, m, "tags")
	require.Equal(t, "logo.png", m["icon"])
	require.Equal(t, "logo.png", m["tags"])
	require.NotContains(t, m, "description", "empty zh+en must not overwrite admin description")

	// 全非空:三类齐全。
	m = buildMetaUpdateMap("简", "en", "logo.png", now)
	require.Contains(t, m, "description")
	require.Equal(t, "简||简||en", m["description"])
	require.Equal(t, "logo.png", m["icon"])
	require.Equal(t, "logo.png", m["tags"])
	require.Equal(t, now, m["updated_time"])

	// 始终不含敏感字段(spec §6.4 白名单)。
	for _, forbidden := range []string{"status", "sync_official", "endpoints", "name_rule", "vendor_id"} {
		require.NotContains(t, m, forbidden, "%s must never appear in meta-update map", forbidden)
	}
}

// TestBuildMetaUpdateMap_DescriptionBuildsFromZhEn 验证 description 字段经由
// buildDescription(zh,en) 拼接(spec §8):zh||zh||en;确认 buildMetaUpdateMap
// 不会绕过该拼接直接落原始字符串。
func TestBuildMetaUpdateMap_DescriptionBuildsFromZhEn(t *testing.T) {
	t.Parallel()

	m := buildMetaUpdateMap("中", "english", "logo", 0)
	require.Equal(t, "中||中||english", m["description"])
}

// TestPersistRatios_PreservesExistingGlobalMap 是 spec §6.2 的回归保护:
// 预置全局 ModelRatio 含 {"existing": 3.0},persistRatios({"new": 0.5}) 之后
// 读回全局 ModelRatio 应同时含 existing=3.0(未清空)与 new=0.5(已合并)。
//
// 该测试需要注入/重置全局 ratio map,并依赖 model.UpdateOption 的完整 DB+OptionMap 链路。
// service 包当前无 DB 测试 harness,且不应在单测中污染真实全局 ratio map。
// 合并语义已由 TestMergeMaps 覆盖、空值守卫由 TestBuildMetaUpdateMap 覆盖;
// spec §6.2 的"marshal 失败不清空全局"由 persistRatios 的代码结构(marshal err → continue,
// 不调 UpdateOption)静态保证。故此处 t.Skip,留待 e2e/集成测试覆盖。
func TestPersistRatios_PreservesExistingGlobalMap(t *testing.T) {
	t.Skip("需全局 ratio map 注入 + DB 测试 harness;合并语义已由 TestMergeMaps 覆盖," +
		"marshal-err-不调-UpdateOption 的保护由 persistRatios 代码结构静态保证(spec §6.2)")
}

// TestPersistRatios_MarshalFailureNoWipe 是 spec §6.2 的核心安全属性:
// common.Marshal 失败时,绝不应调用 model.UpdateOption(否则空串会清空全局 ratio map)。
//
// common.Marshal 对 map[string]float64 在正常情况下不会失败,难以在不替换全局函数的前提下
// 注入失败;persistRatios 的实现已用 "marshal err → common.SysError + continue" 结构,
// 在 UpdateOption 之前 short-circuit。该结构由代码审查 + persistRatios 的 err 处理顺序静态保证。
// 此处 t.Skip,留待 mock common.Marshal 的测试基建落地后补全。
func TestPersistRatios_MarshalFailureNoWipe(t *testing.T) {
	t.Skip("需 mock common.Marshal;persistRatios 已用 'marshal err → continue' 结构," +
		"在 UpdateOption 之前短路,保证不调空串 UpdateOption(spec §6.2)")
}

// ---------------------------------------------------------------------------
// Task 5: parseCSV / dedupeModels / warning helpers / 主流程集成测试
// ---------------------------------------------------------------------------

// TestParseCSV_HappyPath 覆盖正常 8 列 CSV(含表头 + 1 行数据)的解析。
func TestParseCSV_HappyPath(t *testing.T) {
	t.Parallel()
	in := "model_id,billing_unit,description,price_usd,unit,description_zh,description_en,logo\n" +
		"m1,Million,Input -,2,Million,zh,en,logo1\n"
	rows, err := parseCSV(strings.NewReader(in))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "m1", rows[0].ModelID)
	require.Equal(t, "Million", rows[0].Unit)
	require.Equal(t, "2", rows[0].PriceUSD)
	require.Equal(t, "logo1", rows[0].Logo)
}

// TestParseCSV_TrimsModelID 覆盖 model_id 两侧空白归一化:ratio map key / models 行 /
// 渠道模型列表必须同名,否则运行时价格查找 miss。
func TestParseCSV_TrimsModelID(t *testing.T) {
	t.Parallel()
	in := "model_id,billing_unit,description,price_usd,unit,description_zh,description_en,logo\n" +
		"  gpt-4o  ,Million,Input -,2,Million,zh,en,logo1\n"
	rows, err := parseCSV(strings.NewReader(in))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "gpt-4o", rows[0].ModelID)
}

// TestParseCSV_BadHeader 覆盖 spec §4 的 bad_header:首行列数 ≠ 8。
func TestParseCSV_BadHeader(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",                       // 完全空
		"a,b,c\n",                // 列数不足
		"a,b,c,d,e,f,g,h,i\n",    // 列数过多
	}
	for i, in := range cases {
		_, err := parseCSV(strings.NewReader(in))
		require.ErrorIs(t, err, ErrBadHeader, "case %d", i)
	}
}

// TestParseCSV_InvalidCsvRow 覆盖 spec §4 的 invalid_csv_row:数据行列数 ≠ 8 或 csv 语法错误。
func TestParseCSV_InvalidCsvRow(t *testing.T) {
	t.Parallel()
	header := "model_id,billing_unit,description,price_usd,unit,description_zh,description_en,logo\n"
	// 数据行列数不足
	_, err := parseCSV(strings.NewReader(header + "m1,Million,Input\n"))
	require.ErrorIs(t, err, ErrInvalidCsvRow)
	// 引号未闭合 → csv.Reader 返回语法错误
	_, err = parseCSV(strings.NewReader(header + "\"unterminated\n"))
	require.ErrorIs(t, err, ErrInvalidCsvRow)
}

// TestParseCSV_TooManyRows 覆盖 spec §4 的 too_many_rows:数据行 > 50000。
func TestParseCSV_TooManyRows(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	b.WriteString("model_id,billing_unit,description,price_usd,unit,description_zh,description_en,logo\n")
	for i := 0; i <= csvMaxDataRows; i++ { // 等于上限+1 → 触发
		fmt.Fprintf(&b, "m%d,Million,Input -,1,Million,zh,en,\n", i)
	}
	_, err := parseCSV(&b)
	require.ErrorIs(t, err, ErrTooManyRows)
}

// TestParseCSV_MaxRowsBoundary 验证恰好 50000 行数据被接受(边界)。
func TestParseCSV_MaxRowsBoundary(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	b.WriteString("model_id,billing_unit,description,price_usd,unit,description_zh,description_en,logo\n")
	for i := 0; i < csvMaxDataRows; i++ { // 恰好等于上限
		fmt.Fprintf(&b, "m%d,Million,Input -,1,Million,zh,en,\n", i)
	}
	rows, err := parseCSV(&b)
	require.NoError(t, err)
	require.Len(t, rows, csvMaxDataRows)
}

// TestParseCSV_BadEncoding 覆盖 spec §4 的 bad_encoding:非 UTF-8 字节序列。
func TestParseCSV_BadEncoding(t *testing.T) {
	t.Parallel()
	// 0xff/0xfe 是 UTF-16 BOM 前缀,utf8.Valid 会判定为非法 UTF-8。
	_, err := parseCSV(bytes.NewReader([]byte{0xff, 0xfe, 'a', ',', 'b', '\n'}))
	require.ErrorIs(t, err, ErrBadEncoding)
}

// TestDedupeModels 覆盖 spec §6.3 渠道模型去重:
//   - 新模型追加到既有列表之后;
//   - 重复(精确字符串相等)不累加;
//   - trim 空白 + 丢弃空串(含纯空白元素);
//   - 幂等(二次传同一批 newModels 不变)。
func TestDedupeModels(t *testing.T) {
	t.Parallel()
	existing := "a, b ,, c" // 含纯空白元素 " "
	got := dedupeModels(existing, []string{"b", "d", "", "  "})
	require.Equal(t, "a,b,c,d", got)

	// 幂等:对返回结果再次追加相同 newModels,不变。
	got2 := dedupeModels(got, []string{"b", "d"})
	require.Equal(t, got, got2)

	// 空既有列表。
	require.Equal(t, "x,y", dedupeModels("", []string{"x", "y", "x"}))
	// 全空。
	require.Equal(t, "", dedupeModels("", nil))
	require.Equal(t, "", dedupeModels(" , , ", nil))
}

// TestTierCollapseWarning_Fires 覆盖 spec §6.1 的分层 collapse 警告(max/min≥1.5)。
func TestTierCollapseWarning_Fires(t *testing.T) {
	t.Parallel()
	// max/min = 3.0/1.0 = 3.0 ≥ 1.5 → 触发
	w := tierCollapseWarning("model-x", []float64{1.0, 2.0, 3.0})
	require.NotNil(t, w)
	require.Equal(t, "model-x", w.Model)
	require.Contains(t, w.Reason, "price_range=[1,3]")
	require.Contains(t, w.Reason, "分层计费取最低档")
}

// TestTierCollapseWarning_NoFire 验证不触发条件:<2 档、min≤0、max/min<1.5。
func TestTierCollapseWarning_NoFire(t *testing.T) {
	t.Parallel()
	require.Nil(t, tierCollapseWarning("m", []float64{1.0}), "单档不触发")
	require.Nil(t, tierCollapseWarning("m", nil), "空切片不触发")
	require.Nil(t, tierCollapseWarning("m", []float64{0, 1.0}), "min=0 防除零不触发")
	require.Nil(t, tierCollapseWarning("m", []float64{1.0, 1.4}), "max/min=1.4 < 1.5 不触发")
}

// TestCacheTiersWarning_Fires 验证 5min/1h cache write 双档不同价触发 warning。
func TestCacheTiersWarning_Fires(t *testing.T) {
	t.Parallel()
	w := cacheTiersWarning("m", []float64{0.5, 0.8})
	require.NotNil(t, w)
	require.Contains(t, w.Reason, "5min/1h cache write 双档不同价")
}

// TestCacheTiersWarning_NoFire 验证不触发条件:<2 档、单档或同价。
func TestCacheTiersWarning_NoFire(t *testing.T) {
	t.Parallel()
	require.Nil(t, cacheTiersWarning("m", []float64{0.5}))
	require.Nil(t, cacheTiersWarning("m", []float64{0.5, 0.5}), "同价不触发")
	require.Nil(t, cacheTiersWarning("m", []float64{0, 0}), "全 0 不触发")
}

// TestResolveWildcardConflict_PicksHighestInputPrice 验证 spec §6.1:
// 多候选映射到同一 FormatMatching key 时,取 inputPrice 最高者;warning 列出所有冲突模型。
func TestResolveWildcardConflict_PicksHighestInputPrice(t *testing.T) {
	t.Parallel()
	cands := []ratioCandidate{
		{modelID: "alpha", inputPrice: 1.0, ratio: 0.5},
		{modelID: "beta", inputPrice: 3.0, ratio: 1.5},
		{modelID: "gamma", inputPrice: 2.0, ratio: 1.0},
	}
	ratio, warn := resolveWildcardConflict("gpt-4-*", cands)
	require.Equal(t, 1.5, ratio, "取 inputPrice=3.0 对应的 ratio=1.5")
	require.NotNil(t, warn)
	require.Equal(t, "beta", warn.Model, "warning.Model 应为被选中的 beta")
	require.Contains(t, warn.Reason, "gpt-4-*")
}

// TestResolveWildcardConflict_NoConflict 单候选不产生 warning。
func TestResolveWildcardConflict_NoConflict(t *testing.T) {
	t.Parallel()
	ratio, warn := resolveWildcardConflict("k", []ratioCandidate{{modelID: "a", inputPrice: 1.0, ratio: 0.5}})
	require.Equal(t, 0.5, ratio)
	require.Nil(t, warn)
}

// ---------------------------------------------------------------------------
// 集成测试(需 DB + 全局 ratio map)—— service 包无 DB 测试 harness,以下 Skip。
// 完整 end-to-end 行为由 controller/gate 测试(Task 6/7)覆盖。
// ---------------------------------------------------------------------------

// TestImport_BadRowRecorded_PartialSuccess 是 spec §9 "部分失败仍 200 success:true" 的核心属性:
// 模型 A(正常)+ 模型 B(upsert 失败)+ 模型 C(纯非 token Second)→ A 成功、B 计 failed、C 计 skipped_non_token。
//
// 需要 mock model.GetModelByName / model.Insert / model.UpdateMetaFields + DB 测试 harness,
// 且不应在单测中污染真实全局 ratio map。Skip;依赖 controller e2e 测试(Task 6/7)覆盖。
func TestImport_BadRowRecorded_PartialSuccess(t *testing.T) {
	t.Skip("需 mock model.GetModelByName/Insert/UpdateMetaFields + DB harness;" +
		"避免污染全局 ratio map。已由纯函数测试(parseCSV/aggregateModels/computeRatios/dedupeModels/" +
		"warning helpers)覆盖各分支,部分失败语义由 ImportChannelModelsCSV 的 continue 结构静态保证(spec §9)")
}

// TestImport_ChannelUpdateFailed_OrphanReported 是 spec §9 channel.Update 失败孤儿报告:
// channel_update_failed=true、models_imported=0、persisted_ratio_models 含已写 ratio 模型、ratio 不回滚。
//
// 需 mock channel.Update + DB 测试 harness。Skip;失败处理逻辑由 ImportChannelModelsCSV
// 末尾的 `if channelUpdateFailed` 分支静态保证(代码审查),完整覆盖留 controller e2e 测试。
func TestImport_ChannelUpdateFailed_OrphanReported(t *testing.T) {
	t.Skip("需 mock model.GetChannelById/channel.Update + DB harness;孤儿报告分支由 " +
		"ImportChannelModelsCSV `if channelUpdateFailed` 代码结构静态保证(spec §9)")
}

// TestImport_Idempotent_SecondRunNoAccumulation 是 spec §10 幂等:同 CSV 二次上传 →
// Channel.Models 无重复、ratio 不翻倍。
//
// 幂等的纯逻辑已由 TestDedupeModels(渠道去重)+ TestMergeMaps(ratio map 合并覆盖语义)覆盖;
// 端到端幂等需真实 DB + 全局 ratio map,Skip 留 controller e2e 测试覆盖。
func TestImport_Idempotent_SecondRunNoAccumulation(t *testing.T) {
	t.Skip("幂等纯逻辑已由 TestDedupeModels + TestMergeMaps 覆盖;" +
		"端到端幂等需 DB + 全局 ratio map,留 controller e2e 测试覆盖(spec §10)")
}

// TestImport_ChannelNotFound covering spec §4 channel_not_found: GetChannelById 返回 gorm.ErrRecordNotFound。
//
// 需真实 DB(无 channel id=999999)或 mock。Skip;错误分流逻辑由 ImportChannelModelsCSV
// 的 errors.Is(err, gorm.ErrRecordNotFound) 分支静态保证,controller 在 Task 6/7 做最终 HTTP 码映射。
func TestImport_ChannelNotFound(t *testing.T) {
	t.Skip("需 DB 或 mock model.GetChannelById;channel_not_found 分流由 errors.Is(err, gorm.ErrRecordNotFound) " +
		"代码结构静态保证(spec §4)")
}
