package service

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"gorm.io/gorm"
)

// CSV 导入错误哨量(spec §4 错误码:bad_header/invalid_csv_row/too_many_rows/bad_encoding/channel_not_found)。
// 导出供 controller 用 errors.Is 分流 HTTP 状态码。controller 负责 file_too_large(413)等其他错误码。
var (
	ErrBadHeader       = errors.New("bad_header")
	ErrInvalidCsvRow   = errors.New("invalid_csv_row")
	ErrTooManyRows     = errors.New("too_many_rows")
	ErrBadEncoding     = errors.New("bad_encoding")
	ErrChannelNotFound = errors.New("channel_not_found")
)

const (
	csvMaxDataRows     = 50000 // spec §4:行数上限 50000(不含表头)
	csvExpectedColumns = 8     // spec §3:8 列固定表头
)

// billingKind 表示 CSV description 经前缀归类后的计费类型(spec §3)。
// 仅区分 token 计费的四大类;非 token(unit != "Million")行不入 tier。
type billingKind int

const (
	billUnknown    billingKind = iota // 未识别前缀,忽略不计 errors
	billInput                         // 输入(Input - / Text/Audio/Image/Document/Video Input -)
	billOutput                        // 输出(Text/Image/Audio/Video Output -)
	billCacheRead                     // 读缓存(Cache - / Cache Read - / {Audio,Image,Video} Cache Read -)
	billCacheWrite                    // 写缓存(Cache Write)
)

// csvRow 是 UCloud CSV 8 列解析后的单行结构(spec §3)。
//
// 字段顺序与 CSV 表头一致:
//
//	model_id, billing_unit, description, price_usd, unit, description_zh, description_en, logo
type csvRow struct {
	ModelID     string
	BillingUnit string
	Description string
	PriceUSD    string
	Unit        string
	DescZh      string
	DescEn      string
	Logo        string
}

// aggregatedModel 是按 model_id 聚合后的中间结构(spec §5 step4)。
//
//   - *Tiers 仅收集 unit=="Million" 行按 classifyBilling 分桶后的价格(保留**所有档**;
//     取最低档是 Task 3 的工作,这里不折叠)。
//   - defaultInput/Output/CacheRead/CacheWrite:指向 description 以 "- Default" 结尾
//     的行的价格(spec §6.1 Default 档优先取价)。指针为 nil 表示该分类无 Default 行;
//     非 nil 时即使值为 0(免费 Default)也优先于 min(tiers)。
//   - inputPriceMin/Max:computeRatios 计算并缓存 inputTiers 的 [min,max],供 Task 5
//     的分层 collapse warning(max/min≥1.5)使用。无 Input 行时均为 0。
//   - hasNonToken:出现任意 unit!="Million" 行即置位(下游区分 skipped_non_token)。
//   - descZh/descEn/logo 取首行;后续行任一字段与首行不同则 descInconsistent=true。
type aggregatedModel struct {
	modelID                                                  string
	inputTiers, outputTiers, cacheReadTiers, cacheWriteTiers []float64
	defaultInput, defaultOutput                              *float64
	defaultCacheRead, defaultCacheWrite                      *float64
	inputPriceMin, inputPriceMax                             float64
	hasNonToken                                              bool
	descZh, descEn, logo                                     string
	descInconsistent                                         bool
}

// classifyBilling 按 description 前缀归类(spec §3,实测 Million 行 100% 覆盖)。
//
// 顺序敏感:Cache Write 必须在 Cache - 之前判定(虽然字符不同,顺序依旧保留以易于阅读)。
// 多模态前缀(Audio/Image/Video)分别列出,避免误归入 Input - / Output -。
func classifyBilling(desc string) billingKind {
	switch {
	case strings.HasPrefix(desc, "Cache Write"):
		return billCacheWrite
	case strings.HasPrefix(desc, "Cache -"),
		strings.HasPrefix(desc, "Cache Read -"),
		strings.HasPrefix(desc, "Audio Cache Read -"),
		strings.HasPrefix(desc, "Image Cache Read -"),
		strings.HasPrefix(desc, "Video Cache Read -"):
		return billCacheRead
	case strings.HasPrefix(desc, "Text Output -"),
		strings.HasPrefix(desc, "Image Output -"),
		strings.HasPrefix(desc, "Audio Output -"),
		strings.HasPrefix(desc, "Video Output -"):
		return billOutput
	case strings.HasPrefix(desc, "Input -"),
		strings.HasPrefix(desc, "Text Input -"),
		strings.HasPrefix(desc, "Audio Input -"),
		strings.HasPrefix(desc, "Image Input -"),
		strings.HasPrefix(desc, "Document Input -"),
		strings.HasPrefix(desc, "Video Input -"):
		return billInput
	}
	return billUnknown
}

// aggregateModels 按 model_id 分组聚合(spec §5 step4)。
//
// 规则:
//   - unit=="Million" 行:按 classifyBilling 入对应 tier slice(保留所有档,不折叠);
//     若 description 以 "- Default" 结尾 → 同步记录到 defaultXxx 指针(spec §6.1 Default 优先)。
//   - unit!="Million" 行:置 hasNonToken=true(不参与价格分桶);
//   - descZh/descEn/logo 取**首行**,后续行任一字段不同则 descInconsistent=true;
//   - price 解析失败:跳过该行(Task 5 才做模型级 error 上报,这里不得 panic)。
//
// 下游(Task 3+)基于此结构做最低档选取、换算、ratio 预检与持久化。
func aggregateModels(rows []csvRow) map[string]*aggregatedModel {
	result := make(map[string]*aggregatedModel, len(rows))
	for _, row := range rows {
		m, exists := result[row.ModelID]
		if !exists {
			m = &aggregatedModel{modelID: row.ModelID}
			result[row.ModelID] = m
		}
		// desc/logo:首行落值,后续行不一致则置 flag(无论是否 token 行)。
		if !exists {
			m.descZh = row.DescZh
			m.descEn = row.DescEn
			m.logo = row.Logo
		} else if row.DescZh != m.descZh || row.DescEn != m.descEn || row.Logo != m.logo {
			m.descInconsistent = true
		}

		// 非 token 行:仅置 hasNonToken,不入价格桶。
		if row.Unit != "Million" {
			m.hasNonToken = true
			continue
		}

		// 价格解析失败:静默跳过(Task 5 做模型级 error 上报)。
		price, err := strconv.ParseFloat(row.PriceUSD, 64)
		if err != nil {
			continue
		}

		isDefault := isDefaultTier(row.Description)
		switch classifyBilling(row.Description) {
		case billInput:
			m.inputTiers = append(m.inputTiers, price)
			if isDefault {
				p := price
				m.defaultInput = &p
			}
		case billOutput:
			m.outputTiers = append(m.outputTiers, price)
			if isDefault {
				p := price
				m.defaultOutput = &p
			}
		case billCacheRead:
			m.cacheReadTiers = append(m.cacheReadTiers, price)
			if isDefault {
				p := price
				m.defaultCacheRead = &p
			}
		case billCacheWrite:
			m.cacheWriteTiers = append(m.cacheWriteTiers, price)
			if isDefault {
				p := price
				m.defaultCacheWrite = &p
			}
		case billUnknown:
			// 未识别前缀(spec §3:忽略不计 errors)
		}
	}
	return result
}

// isDefaultTier 判断该行 description 是否为 Default 档(spec §6.1 "Default 优先")。
// 实测 CSV 中 Default 档一律以 "- Default" 结尾,覆盖 Input/Output/Cache Read/Cache Write
// (含 5min/1h)及多模态前缀(Text/Audio/Image/Document/Video)。
func isDefaultTier(desc string) bool {
	return strings.HasSuffix(desc, "- Default")
}

// pickLowestDefaultFirst 按 spec §6.1 取档:Default 行价格优先,否则取 min(tiers)。
// 空切片且无 Default → 返回 0(下游 inputPrice==0 视为免费模型)。
// defaultPrice 非 nil 时即使值为 0(免费 Default)也优先,以保留 "Default=0" 的语义。
func pickLowestDefaultFirst(tiers []float64, defaultPrice *float64) float64 {
	if defaultPrice != nil {
		return *defaultPrice
	}
	if len(tiers) == 0 {
		return 0
	}
	min := tiers[0]
	for _, p := range tiers[1:] {
		if p < min {
			min = p
		}
	}
	return min
}

// tierRange 返回 tiers 的 (min, max);空切片返回 (0, 0)。供 Task 5 collapse warning。
func tierRange(tiers []float64) (min, max float64) {
	if len(tiers) == 0 {
		return 0, 0
	}
	min, max = tiers[0], tiers[0]
	for _, p := range tiers[1:] {
		if p < min {
			min = p
		}
		if p > max {
			max = p
		}
	}
	return min, max
}

// computeRatios 按 spec §6.1 公式将聚合价格换算为四种 ratio,先防除零。
//
// 返回:
//   - modelRatio, completionRatio, cacheRatio, createCacheRatio
//   - hasOutput/hasCacheRead/hasCacheWrite:对应桶是否有价格行(决定下游是否写该 ratio key)
//
// 规则:
//   - inputPrice==0(无 Input 行,或 Default=0)→ ModelRatio=0、CompletionRatio=1,
//     跳过 output/cache 除法(避免除零)。仍返回 hasOutput/... 供调用方决定 key 写入。
//   - 否则:ModelRatio=inputPrice/2.0;
//     CompletionRatio=(outputPrice>0?outputPrice:inputPrice)/inputPrice(无 Output 行→1);
//     CacheRatio/CreateCacheRatio 仅在对应价>0 时计算(否则保留 0,调用方按 hasXxx 决定是否写 key)。
//
// 副作用:缓存 inputPriceMin/Max 到 am(供 Task 5 collapse warning)。
func computeRatios(am *aggregatedModel) (modelRatio, completionRatio, cacheRatio, createCacheRatio float64, hasOutput, hasCacheRead, hasCacheWrite bool) {
	inputPrice := pickLowestDefaultFirst(am.inputTiers, am.defaultInput)
	outputPrice := pickLowestDefaultFirst(am.outputTiers, am.defaultOutput)
	cacheReadPrice := pickLowestDefaultFirst(am.cacheReadTiers, am.defaultCacheRead)
	cacheWritePrice := pickLowestDefaultFirst(am.cacheWriteTiers, am.defaultCacheWrite)

	// 缓存 inputTiers 范围(无论 Default 与否,所有 tier 的 min/max),供 Task 5 warning。
	am.inputPriceMin, am.inputPriceMax = tierRange(am.inputTiers)

	hasOutput = len(am.outputTiers) > 0
	hasCacheRead = len(am.cacheReadTiers) > 0
	hasCacheWrite = len(am.cacheWriteTiers) > 0

	if inputPrice == 0 {
		// 免费产品(spec §6.1):ModelRatio=0、CompletionRatio=1,跳过除法。
		return 0, 1, 0, 0, hasOutput, hasCacheRead, hasCacheWrite
	}

	modelRatio = inputPrice / 2.0
	if outputPrice > 0 {
		completionRatio = outputPrice / inputPrice
	} else {
		// 无 Output 行或 Output=0:用 inputPrice 自身作分母 → CompletionRatio=1。
		completionRatio = inputPrice / inputPrice
	}
	if cacheReadPrice > 0 {
		cacheRatio = cacheReadPrice / inputPrice
	}
	if cacheWritePrice > 0 {
		createCacheRatio = cacheWritePrice / inputPrice
	}
	return modelRatio, completionRatio, cacheRatio, createCacheRatio, hasOutput, hasCacheRead, hasCacheWrite
}

// buildDescription 拼接模型介绍多语言字符串(spec §8)。
// 格式:description_zh + "||" + description_zh + "||" + description_en
// (繁体位简体兜底;空字符串保留为空,不填默认值)。
func buildDescription(zh, en string) string {
	return zh + "||" + zh + "||" + en
}

// shouldSkipModelPrice 检测模型是否已配置按次计费(spec §6.1 ModelPrice bypass)。
// 若 GetModelPrice 命中(usePrice==true)→ 写 ratio 无计费效果,整模型跳过 ratio 写入。
// 返回 (skip, reason):skip=true 时 reason 为面向 admin 的中文说明,供 warning 使用。
func shouldSkipModelPrice(name string) (bool, string) {
	if _, usePrice := ratio_setting.GetModelPrice(name, false); usePrice {
		return true, "已在 ModelPrice 表中,写 ratio 无计费效果,需先从 ModelPrice 移除"
	}
	return false, ""
}

// shouldSkipCompletionRatio 判断模型的 CompletionRatio 是否被系统硬编码覆盖(spec §6.1)。
// 镜像源端 IsCompletionRatioHardcoded 的语义:
//   - vendor 前缀名(含 "/")→ false:读路径 GetCompletionRatio 先查 completionRatioMap,
//     写 map 有效,故不跳过(不能直接用 GetCompletionRatioInfo(name).Locked —— 它在
//     map miss 时会把 vendor 名落入 getHardcodedCompletionModelRatio 的 substring 判定,
//     对 "anthropic/claude-3-... " 这类名字误判为 hardcoded);
//   - 裸名 → GetCompletionRatioInfo(name).Locked:getHardcodedCompletionModelRatio
//     locked==true 即硬编码覆盖 map,写 map 无效,跳过。
func shouldSkipCompletionRatio(name string) bool {
	if strings.Contains(name, "/") {
		return false
	}
	return ratio_setting.GetCompletionRatioInfo(name).Locked
}

// applyFormatMatching 重写 ModelRatio/CompletionRatio 的 map key(spec §6.1)。
//
// ⚠️ 仅用于 ModelRatio/CompletionRatio 两个 map 的 key 改写;CacheRatio/CreateCacheRatio
// 的调用方必须传 model_id 原名 —— 读路径(GetCacheRatio/GetCreateCacheRatio)不调
// FormatMatchingModelName,若写时改写会造成读写 key 不一致。
// 实际的 key 处理(含 wildcard 冲突取 inputPrice 最高)在 Task 4/5 主流程,本函数仅做纯重写。
func applyFormatMatching(name string) string {
	return ratio_setting.FormatMatchingModelName(name)
}

// ratioWriteMu 串行化 service 层的 ratio 读-合并-写(spec §6.2 TOCTOU)。
// 仅进程内串行;与"价格编辑 UI"跨进程并发 last-writer-wins(完全强一致需 DB 行锁,本期不引入)。
var ratioWriteMu sync.Mutex

// mergeMaps 是 spec §6.2 读-合并-写中的"合并"步骤的纯函数实现:
// 将 updates 中每个 key 覆盖写入 existing(就地修改)并返回 existing。
//
// 抽成独立纯函数是为了让合并语义可单测(不触碰全局 ratio map);
// persistRatios 内部对每个 ratio map 都走 GetXxxCopy() → mergeMaps(copy, updates) → Marshal。
func mergeMaps(existing, updates map[string]float64) map[string]float64 {
	for k, v := range updates {
		existing[k] = v
	}
	return existing
}

// buildMetaUpdateMap 构造 spec §6.4 的"已存在模型"更新字段白名单 map。
//
// 空值守卫(empty-guard):
//   - updated_time:总是写入(刷新编辑时间);
//   - description:仅当 zh/en 至少一个非空时写入 —— buildDescription("","") 会产生
//     "||||"(四条竖线)而非空串,直接用 `desc != ""` 无法识别"双空"情形,会污染 admin 既有
//     介绍;故这里按 zh||en 的输入判空。空 CSV 值不覆盖 admin 既有介绍。
//   - icon/tags:仅当 logo != "" 时写入 —— 空 CSV 值不覆盖 admin 既有图标;
//   - 其余字段(status/sync_official/endpoints/name_rule/vendor_id)一律不动。
//
// 抽成纯函数是为了让空值守卫语义可单测(service 层无 DB 测试 harness);
// upsertModelMeta 在"已存在"分支直接复用本函数。
func buildMetaUpdateMap(zh, en, logo string, now int64) map[string]interface{} {
	fields := map[string]interface{}{"updated_time": now}
	if zh != "" || en != "" {
		fields["description"] = buildDescription(zh, en)
	}
	if logo != "" {
		fields["icon"] = logo
		fields["tags"] = logo
	}
	return fields
}

// upsertModelMeta 按 spec §6.4 实现模型介绍的 upsert(字段白名单 + 空值守卫)。
//
//   - 不存在(GetModelByName → gorm.ErrRecordNotFound):Insert 新记录,
//     Status=1 / SyncOfficial=1 / VendorID=0(spec §6.4 默认值);Description/Icon/Tags 用 CSV 值。
//   - 已存在:仅以 buildMetaUpdateMap 的白名单字段调 UpdateMetaFields,
//     保留 status/sync_official/endpoints/name_rule/vendor_id;空 CSV 值不覆盖既有介绍/图标。
//   - 其他查询错误:原样向上返回。
func upsertModelMeta(am *aggregatedModel) error {
	existing, err := model.GetModelByName(am.modelID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		m := &model.Model{
			ModelName:    am.modelID,
			Description:  buildDescription(am.descZh, am.descEn),
			Icon:         am.logo,
			Tags:         am.logo,
			VendorID:     0,
			Status:       1,
			SyncOfficial: 1,
		}
		return m.Insert()
	}
	if err != nil {
		return err
	}
	fields := buildMetaUpdateMap(am.descZh, am.descEn, am.logo, common.GetTimestamp())
	return existing.UpdateMetaFields(fields)
}

// persistRatios 按 spec §6.2 实现 ratio 读-合并-写(persistRatios 黄金路径):
//
//   - 在 package-level ratioWriteMu 下串行,避免 service 内 TOCTOU;
//   - 按 ModelRatio → CompletionRatio → CacheRatio → CreateCacheRatio 固定顺序写;
//   - 每个 map 独立"GetXxxCopy() → mergeMaps → common.Marshal → model.UpdateOption":
//     marshal 失败 → 记日志并跳过该 map(**绝不以空串调 UpdateOption**,避免清空全局),
//     UpdateOption 失败 → 记日志并跳过;成功才把名字追加到 persisted;
//   - 返回的 persisted 为本轮实际写成功的 ratio map 名列表(顺序与成功落盘顺序一致)。
//
// key 重写:caller 已对 ModelRatio/CompletionRatio 的 key 过 applyFormatMatching,
// CacheRatio/CreateCacheRatio 传 model_id 原名;本函数不做 key 改写。
//
// ⚠️ 严禁 ratio_setting.UpdateXxxByJSONString(csvOnly):其走 rw_map 替换语义会清空全局。
// ⚠️ inherited:UpdateOption 的 DB.FirstOrCreate/Save 错误未被其捕获,可能内存更新而 DB 未落盘,
// 重启后 ratio 回退 —— 本期接受(spec §6.2 已标注)。
func persistRatios(modelRatios, completionRatios, cacheRatios, createCacheRatios map[string]float64) ([]string, error) {
	ratioWriteMu.Lock()
	defer ratioWriteMu.Unlock()

	var persisted []string
	type pair struct {
		name string
		src  map[string]float64
		copy func() map[string]float64
	}
	order := []pair{
		{"ModelRatio", modelRatios, ratio_setting.GetModelRatioCopy},
		{"CompletionRatio", completionRatios, ratio_setting.GetCompletionRatioCopy},
		{"CacheRatio", cacheRatios, ratio_setting.GetCacheRatioCopy},
		{"CreateCacheRatio", createCacheRatios, ratio_setting.GetCreateCacheRatioCopy},
	}
	for _, p := range order {
		if len(p.src) == 0 {
			continue
		}
		merged := mergeMaps(p.copy(), p.src)
		js, err := common.Marshal(merged)
		if err != nil {
			// 不调 UpdateOption,避免空串/nil 清空全局 ratio map(spec §6.2 关键保护)。
			common.SysError(fmt.Sprintf("csv-import: marshal %s: %v", p.name, err))
			continue
		}
		if err := model.UpdateOption(p.name, string(js)); err != nil {
			common.SysError(fmt.Sprintf("csv-import: UpdateOption %s: %v", p.name, err))
			continue
		}
		persisted = append(persisted, p.name)
	}
	return persisted, nil
}

// parseCSV 解析 UCloud 8 列 CSV(spec §3/§4/§5 step2-3),返回数据行(不含表头)。
//
// 校验顺序与错误码:
//  1. UTF-8 有效性(`utf8.Valid`)→ 非 UTF-8 返回 `ErrBadEncoding`;
//  2. `csv.NewReader`(`FieldsPerRecord=-1` 关闭严格列数检查)读取全部记录;
//     `csv.Reader` 返回的语法错误(引号不闭合等)→ `ErrInvalidCsvRow`;
//  3. 表头严格 8 列(空文件或首行列数≠8)→ `ErrBadHeader`;
//  4. 数据行(除表头外)任一列数≠8 → `ErrInvalidCsvRow`;
//  5. 数据行数 > 50000 → `ErrTooManyRows`。
//
// 不做表头"字段名"匹配(spec §4 仅要求列数):字段语义错位由下游 classifyBilling
// 自然降级为 billUnknown(忽略),不会污染 ratio map。
//
// 注:header 列数错误用 `ErrBadHeader`(而非 FieldsPerRecord 的 `ErrInvalidCsvRow`),
// 是为了让 controller 能给出更精确的错误码(spec §4 区分两者)。
func parseCSV(r io.Reader) ([]csvRow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if !utf8Valid(data) {
		return nil, ErrBadEncoding
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1 // 关闭内置列数检查,由本函数手动分流 bad_header/invalid_csv_row
	reader.ReuseRecord = false

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCsvRow, err)
	}
	if len(records) == 0 || len(records[0]) != csvExpectedColumns {
		return nil, ErrBadHeader
	}
	dataRows := records[1:]
	if len(dataRows) > csvMaxDataRows {
		return nil, fmt.Errorf("%w: %d rows > %d", ErrTooManyRows, len(dataRows), csvMaxDataRows)
	}
	for i, rec := range dataRows {
		if len(rec) != csvExpectedColumns {
			return nil, fmt.Errorf("%w: row %d has %d cols", ErrInvalidCsvRow, i+2, len(rec))
		}
	}

	rows := make([]csvRow, 0, len(dataRows))
	for _, rec := range dataRows {
		rows = append(rows, csvRow{
			// ModelID 统一 TrimSpace:下游 ratio map key / models 行 / 渠道模型列表
			// 必须同名,否则渠道列表存 "gpt-4o" 而 ratio key 存 " gpt-4o",
			// 运行时价格查找会 miss(FormatMatchingModelName 不做 trim)。
			ModelID:     strings.TrimSpace(rec[0]),
			BillingUnit: rec[1],
			Description: rec[2],
			PriceUSD:    rec[3],
			Unit:        rec[4],
			DescZh:      rec[5],
			DescEn:      rec[6],
			Logo:        rec[7],
		})
	}
	return rows, nil
}

// utf8Valid 包装 unicode/utf8.Valid,便于潜在future mocking。当前直接转发。
func utf8Valid(b []byte) bool { return utf8.Valid(b) }

// dedupeModels 实现 spec §6.3 的渠道模型字符串去重(纯函数,便于单测)。
//
// 语义:
//   - 既有列表与新追加列表按出现顺序合并,去重(精确字符串相等,不去 prefix);
//   - 每个元素先 `strings.TrimSpace`,空串丢弃;
//   - 返回逗号分隔的去重字符串(保留既有顺序优先)。
//
// 幂等(spec §10):同一 CSV 二次上传,newModels 已在 existing 中 → 返回值不变。
func dedupeModels(existing string, add []string) string {
	seen := make(map[string]struct{})
	keys := make([]string, 0, len(add)+strings.Count(existing, ",")+1)
	addOne := func(m string) {
		m = strings.TrimSpace(m)
		if m == "" {
			return
		}
		if _, ok := seen[m]; ok {
			return
		}
		seen[m] = struct{}{}
		keys = append(keys, m)
	}
	for _, m := range strings.Split(existing, ",") {
		addOne(m)
	}
	for _, m := range add {
		addOne(m)
	}
	return strings.Join(keys, ",")
}

// tierCollapseWarning 检测 spec §6.1 的分层 collapse 警告:inputTiers 的 max/min≥1.5
// 即视为分层价差显著(长上下文少扣),返回面向 admin 的提示。无 2+ 档 / min≤0 时返回 nil。
//
// 注意:用原始 tiers(含 Default),而非 pickLowestDefaultFirst 后的单值。
func tierCollapseWarning(modelID string, inputTiers []float64) *dto.ImportWarning {
	if len(inputTiers) < 2 {
		return nil
	}
	minP, maxP := tierRange(inputTiers)
	if minP <= 0 {
		return nil
	}
	if maxP/minP < 1.5 {
		return nil
	}
	return &dto.ImportWarning{
		Model:  modelID,
		Reason: fmt.Sprintf("分层计费取最低档;price_range=[%g,%g],长上下文少扣,请复核", minP, maxP),
	}
}

// cacheTiersWarning 检测 spec §6.1 的 5min/1h cache write 双档不同价 warning。
// cacheWriteTiers 中存在 2 个以上互不相同的正价 → 返回 warning;否则 nil。
// 系统默认 1h = CreateCacheRatio×1.6,若 CSV 双档同价 → 1h 被高估,需 admin 复核。
func cacheTiersWarning(modelID string, cacheWriteTiers []float64) *dto.ImportWarning {
	if len(cacheWriteTiers) < 2 {
		return nil
	}
	seen := make(map[float64]struct{})
	var distinct []float64
	for _, p := range cacheWriteTiers {
		if p <= 0 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		distinct = append(distinct, p)
	}
	if len(distinct) < 2 {
		return nil
	}
	return &dto.ImportWarning{
		Model:  modelID,
		Reason: fmt.Sprintf("5min/1h cache write 双档不同价 %v;系统默认 1h=CreateCacheRatio×1.6,CSV 同价时 1h 被高估,请复核", distinct),
	}
}

// resolveWildcardConflict 按 spec §6.1 处理 FormatMatching wildcard 冲突:
// 同一 FormatMatching key 下多模型时,取 inputPrice 最高者为该 key 的最终 ratio;
// 若存在冲突(候选数>1)返回一条 warning,列出所有冲突模型名。
//
// 返回该 key 的最终 ratio 与(若有)warning。空候选集返回 (0, nil)。
func resolveWildcardConflict(key string, cands []ratioCandidate) (float64, *dto.ImportWarning) {
	if len(cands) == 0 {
		return 0, nil
	}
	sort.Slice(cands, func(i, j int) bool {
		// 稳定排序:inputPrice 高者优先;价格相同时按 modelID 字典序(确定性)
		if cands[i].inputPrice != cands[j].inputPrice {
			return cands[i].inputPrice > cands[j].inputPrice
		}
		return cands[i].modelID < cands[j].modelID
	})
	best := cands[0]
	if len(cands) == 1 {
		return best.ratio, nil
	}
	names := make([]string, len(cands))
	for i, c := range cands {
		names[i] = c.modelID
	}
	return best.ratio, &dto.ImportWarning{
		Model:  best.modelID,
		Reason: fmt.Sprintf("FormatMatching wildcard 冲突 (key=%s),按 inputPrice 最高者取值;冲突模型: %s", key, strings.Join(names, ",")),
	}
}

// ratioCandidate 是 ModelRatio/CompletionRatio 的 wildcard 冲突候选(spec §6.1)。
type ratioCandidate struct {
	modelID    string
	inputPrice float64
	ratio      float64
}

// ImportChannelModelsCSV 是 CSV 渠道-模型导入的 service 主入口(spec §5 数据流)。
//
// 流程:
//  1. `GetChannelById(channelID, false)`(导入不需 Key);`gorm.ErrRecordNotFound` → ErrChannelNotFound,其他 → 原样上抛。
//  2. `parseCSV(r)`(UTF-8/列数/行数/语法校验,错误码见 parseCSV 文档)。
//  3. `aggregateModels(rows)` → 按 model_id 聚合的中间结构。
//  4. 逐模型 best-effort:skip 判定(no_input/non_token)→ upsert intro → ratio 预检 + 累积 map → 收集到 newModels。
//  5. `persistRatios(...)` 批量落盘 ratio(spec §6.2,顺序 ModelRatio→CompletionRatio→CacheRatio→CreateCacheRatio)。
//  6. `GetChannelPollingLock(channelID)` 包裹 `channel.Update()`(spec §6.3):dedupeModels 后 Update。
//  7. 失败处理:`channel.Update` 失败 → `channel_update_failed=true`、`models_imported=0`、`persisted_ratio_models=<已写 ratio 的模型>`,**不回滚 ratio**(孤儿状态由 admin 重跑补救)。
//
// 幂等(spec §10):二次同 CSV → model upsert 复用、Channel.Models 去重、ratio map 合并覆盖(以 CSV 值)。
func ImportChannelModelsCSV(channelID int, r io.Reader) (*dto.ImportCsvResult, error) {
	res := &dto.ImportCsvResult{
		ChannelID:            channelID,
		RatioPersisted:       []string{},
		PersistedRatioModels: []string{},
		Errors:               []dto.ImportError{},
		Warnings:             []dto.ImportWarning{},
	}

	// Step 1: Load channel(spec §5 step1)。
	if _, err := model.GetChannelById(channelID, false); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: channel %d", ErrChannelNotFound, channelID)
		}
		return nil, fmt.Errorf("load channel %d: %w", channelID, err)
	}

	// Step 2-3: Parse + aggregate(spec §5 step2-4)。
	rows, err := parseCSV(r)
	if err != nil {
		return nil, err
	}
	aggregated := aggregateModels(rows)
	res.ModelsInCSV = len(aggregated)

	// Step 4: Per-model best-effort processing。
	modelRatioCands := make(map[string][]ratioCandidate)   // key=applyFormatMatching(id)
	completionRatioCands := make(map[string][]ratioCandidate)
	cacheRatioMap := make(map[string]float64)
	createCacheRatioMap := make(map[string]float64)
	newModels := make([]string, 0, len(aggregated))
	ratioWrittenModels := make([]string, 0, len(aggregated)) // 提交 ratio 写入的模型(channel 失败时填充孤儿列表)

	// 确定性顺序(map iteration 在 Go 中随机)。
	ids := make([]string, 0, len(aggregated))
	for id := range aggregated {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		// 跳过空 model_id(防御性,CSV 正常不会出现)。
		if strings.TrimSpace(id) == "" {
			continue
		}
		am := aggregated[id]

		// (a) Skip 判定(spec §5 step5a)。
		//     完全非 token(所有行 unit!="Million")→ skipped_non_token;否则无 Input tier → skipped_no_input。
		//     二者均属"silent skip",不 upsert intro、不追加渠道、不计 errors。
		hasAnyTokenTier := len(am.inputTiers)+len(am.outputTiers)+len(am.cacheReadTiers)+len(am.cacheWriteTiers) > 0
		hasInputTier := len(am.inputTiers) > 0 || am.defaultInput != nil
		if am.hasNonToken && !hasAnyTokenTier {
			res.ModelsSkippedNonToken++
			res.ModelsSkipped++
			continue
		}
		if !hasInputTier {
			res.ModelsSkippedNoInput++
			res.ModelsSkipped++
			continue
		}

		// (b) upsert intro(spec §5 step5c)。
		//     放在 ratio 处理之前:upsert 失败 → errors + models_failed++,不进入 ratio 写入。
		if err := upsertModelMeta(am); err != nil {
			res.ModelsFailed++
			res.Errors = append(res.Errors, dto.ImportError{
				Model:  id,
				Reason: fmt.Sprintf("upsert 模型介绍失败: %v", err),
			})
			continue
		}
		res.IntroUpdated++

		// (c) ratio 预检 + 累积(spec §5 step5d,§6.1)。
		skipPrice, reason := shouldSkipModelPrice(id)
		if skipPrice {
			// ModelPrice bypass:整模型跳过全部 ratio 写入 + warning;intro 仍已 upsert(上一步)。
			res.PriceSkipped++
			res.Warnings = append(res.Warnings, dto.ImportWarning{Model: id, Reason: reason})
		} else {
			mr, cr, cacheR, createCacheR, _, hasCacheRead, hasCacheWrite := computeRatios(am)
			inputPrice := pickLowestDefaultFirst(am.inputTiers, am.defaultInput)
			fmKey := applyFormatMatching(id)
			// ModelRatio 进候选池(spec §6.1 wildcard 冲突取 inputPrice 最高)。
			modelRatioCands[fmKey] = append(modelRatioCands[fmKey], ratioCandidate{
				modelID: id, inputPrice: inputPrice, ratio: mr,
			})
			res.PriceUpdated++
			// CompletionRatio:裸名硬编码 → 跳过 + warning + count;否则进候选池。
			if shouldSkipCompletionRatio(id) {
				res.CompletionRatioSkipped++
				res.Warnings = append(res.Warnings, dto.ImportWarning{
					Model:  id,
					Reason: "裸名硬编码 CompletionRatio 覆盖,CSV 写入无效,已跳过",
				})
			} else {
				completionRatioCands[fmKey] = append(completionRatioCands[fmKey], ratioCandidate{
					modelID: id, inputPrice: inputPrice, ratio: cr,
				})
			}
			// CacheRatio/CreateCacheRatio 用 model_id 原名(spec §6.1 读路径不调 FormatMatching)。
			if hasCacheRead && cacheR > 0 {
				cacheRatioMap[id] = cacheR
			}
			if hasCacheWrite && createCacheR > 0 {
				createCacheRatioMap[id] = createCacheR
			}
			ratioWrittenModels = append(ratioWrittenModels, id)
		}

		// (d) 非致命 warnings:分层 collapse、5min/1h cache 双档、desc 不一致。
		if w := tierCollapseWarning(id, am.inputTiers); w != nil {
			res.Warnings = append(res.Warnings, *w)
		}
		if w := cacheTiersWarning(id, am.cacheWriteTiers); w != nil {
			res.Warnings = append(res.Warnings, *w)
		}
		if am.descInconsistent {
			res.Warnings = append(res.Warnings, dto.ImportWarning{
				Model:  id,
				Reason: "多行 desc/logo 不一致,已取首行",
			})
		}

		// (e) 收集到 newModels(待追加渠道,spec §5 step5e)。
		newModels = append(newModels, id)
	}

	// ModelsRecognized = ModelsInCSV - ModelsSkipped(spec §4 语义:recognized = imported + failed)。
	res.ModelsRecognized = res.ModelsInCSV - res.ModelsSkipped

	// 解析 wildcard 冲突 + 填充最终 ratio map(spec §6.1)。
	modelRatioMap := make(map[string]float64, len(modelRatioCands))
	for key, cands := range modelRatioCands {
		ratio, warn := resolveWildcardConflict(key, cands)
		modelRatioMap[key] = ratio
		if warn != nil {
			res.Warnings = append(res.Warnings, *warn)
		}
	}
	completionRatioMap := make(map[string]float64, len(completionRatioCands))
	for key, cands := range completionRatioCands {
		ratio, warn := resolveWildcardConflict(key, cands)
		completionRatioMap[key] = ratio
		if warn != nil {
			res.Warnings = append(res.Warnings, *warn)
		}
	}

	// Step 5: 批量持久化 ratio(spec §5 step6,§6.2)。
	persistedMaps, _ := persistRatios(modelRatioMap, completionRatioMap, cacheRatioMap, createCacheRatioMap)
	res.RatioPersisted = persistedMaps

	// Step 6: 渠道去重更新(spec §5 step7,§6.3):GetChannelPollingLock 包裹。
	channelUpdateFailed := false
	lock := model.GetChannelPollingLock(channelID)
	lock.Lock()
	defer lock.Unlock()
	fresh, err := model.GetChannelById(channelID, false)
	if err != nil {
		common.SysError(fmt.Sprintf("csv-import: reload channel %d before Update failed: %v", channelID, err))
		channelUpdateFailed = true
	} else {
		fresh.Models = dedupeModels(fresh.Models, newModels)
		if err := fresh.Update(); err != nil {
			common.SysError(fmt.Sprintf("csv-import: channel %d Update failed: %v", channelID, err))
			channelUpdateFailed = true
		} else {
			res.ModelsImported = len(newModels)
		}
	}

	// Step 7: 失败处理(spec §9:channel.Update 失败 → channel_update_failed=true、models_imported=0、persisted_ratio_models=<孤儿>)。
	if channelUpdateFailed {
		res.ChannelUpdateFailed = true
		res.ModelsImported = 0
		res.PersistedRatioModels = ratioWrittenModels
	}

	return res, nil
}
