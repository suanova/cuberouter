package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TokenDetails struct {
	TextTokens  int
	AudioTokens int
}

type QuotaInfo struct {
	InputDetails  TokenDetails
	OutputDetails TokenDetails
	ModelName     string
	UsePrice      bool
	ModelPrice    float64
	ModelRatio    float64
	GroupRatio    float64
}

func hasCustomModelRatio(modelName string, currentRatio float64) bool {
	defaultRatio, exists := ratio_setting.GetDefaultModelRatioMap()[modelName]
	if !exists {
		return true
	}
	return currentRatio != defaultRatio
}

func calculateAudioQuota(info QuotaInfo) (int, *common.QuotaClamp) {
	if info.UsePrice {
		modelPrice := decimal.NewFromFloat(info.ModelPrice)
		quotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		groupRatio := decimal.NewFromFloat(info.GroupRatio)

		quota := modelPrice.Mul(quotaPerUnit).Mul(groupRatio)
		return common.QuotaFromDecimalChecked(quota)
	}

	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(info.ModelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(info.ModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(info.ModelName))

	groupRatio := decimal.NewFromFloat(info.GroupRatio)
	modelRatio := decimal.NewFromFloat(info.ModelRatio)
	ratio := groupRatio.Mul(modelRatio)

	inputTextTokens := decimal.NewFromInt(int64(info.InputDetails.TextTokens))
	outputTextTokens := decimal.NewFromInt(int64(info.OutputDetails.TextTokens))
	inputAudioTokens := decimal.NewFromInt(int64(info.InputDetails.AudioTokens))
	outputAudioTokens := decimal.NewFromInt(int64(info.OutputDetails.AudioTokens))

	quota := decimal.Zero
	quota = quota.Add(inputTextTokens)
	quota = quota.Add(outputTextTokens.Mul(completionRatio))
	quota = quota.Add(inputAudioTokens.Mul(audioRatio))
	quota = quota.Add(outputAudioTokens.Mul(audioRatio).Mul(audioCompletionRatio))

	quota = quota.Mul(ratio)

	// If ratio is not zero and quota is less than or equal to zero, set quota to 1
	if !ratio.IsZero() && quota.LessThanOrEqual(decimal.Zero) {
		quota = decimal.NewFromInt(1)
	}

	return common.QuotaFromDecimalChecked(quota)
}

func PreWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage) error {
	if relayInfo.UsePrice {
		return nil
	}
	userQuota, err := model.GetUserQuota(relayInfo.UserId, false)
	if err != nil {
		return err
	}

	token, err := model.GetTokenByKey(strings.TrimPrefix(relayInfo.TokenKey, "sk-"), false)
	if err != nil {
		return err
	}

	modelName := relayInfo.OriginModelName
	textInputTokens := usage.InputTokenDetails.TextTokens
	textOutTokens := usage.OutputTokenDetails.TextTokens
	audioInputTokens := usage.InputTokenDetails.AudioTokens
	audioOutTokens := usage.OutputTokenDetails.AudioTokens
	groupRatio := ratio_setting.GetGroupRatio(relayInfo.UsingGroup)
	modelRatio, _, _ := ratio_setting.GetModelRatio(modelName)

	autoGroup, exists := common.GetContextKey(ctx, constant.ContextKeyAutoGroup)
	if exists {
		groupRatio = ratio_setting.GetGroupRatio(autoGroup.(string))
		logger.LogDebug(ctx, "final group ratio: %f", groupRatio)
		relayInfo.UsingGroup = autoGroup.(string)
	}

	actualGroupRatio := groupRatio
	userGroupRatio, ok := ratio_setting.GetGroupGroupRatio(relayInfo.UserGroup, relayInfo.UsingGroup)
	if ok {
		actualGroupRatio = userGroupRatio
	}

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  modelName,
		UsePrice:   relayInfo.UsePrice,
		ModelRatio: modelRatio,
		GroupRatio: actualGroupRatio,
	}

	quota, clamp := calculateAudioQuota(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)

	if userQuota < quota {
		return fmt.Errorf("user quota is not enough, user quota: %s, need quota: %s", logger.FormatQuota(userQuota), logger.FormatQuota(quota))
	}

	if !token.UnlimitedQuota && token.RemainQuota < quota {
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(token.RemainQuota), logger.FormatQuota(quota))
	}

	err = PostConsumeQuota(relayInfo, quota, 0, false)
	if err != nil {
		return err
	}
	logger.LogInfo(ctx, "realtime streaming consume quota success, quota: "+fmt.Sprintf("%d", quota))
	return nil
}

func PostWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelName string,
	usage *dto.RealtimeUsage, extraContent string) {

	var tieredResult *billingexpr.TieredResult
	tieredOk, tieredQuota, tieredRes := TryTieredSettle(relayInfo, billingexpr.TokenParams{
		P:   float64(usage.InputTokens),
		C:   float64(usage.OutputTokens),
		Len: float64(usage.InputTokens),
	})
	if tieredOk {
		tieredResult = tieredRes
	}

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	textInputTokens := usage.InputTokenDetails.TextTokens
	textOutTokens := usage.OutputTokenDetails.TextTokens

	audioInputTokens := usage.InputTokenDetails.AudioTokens
	audioOutTokens := usage.OutputTokenDetails.AudioTokens

	tokenName := ctx.GetString("token_name")
	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(modelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(relayInfo.OriginModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(modelName))

	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	usePrice := relayInfo.PriceData.UsePrice

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  modelName,
		UsePrice:   usePrice,
		ModelRatio: modelRatio,
		GroupRatio: groupRatio,
	}

	quota, clamp := calculateAudioQuota(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)
	if tieredOk {
		quota = tieredQuota
	}

	totalTokens := usage.TotalTokens
	var logContent string
	if !usePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，音频倍率 %.2f，音频补全倍率 %.2f，分组倍率 %.2f",
			modelRatio, completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), groupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", modelPrice, groupRatio)
	}

	// record all the consume log even if quota is 0
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
		logContent += "（可能是上游超时）"
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		// 组织请求的个人用量在组织结算事务里不落账：调用者的钱包、订阅与
		// 个人看板都不参与，只有渠道用量仍按请求计一次。
		if !IsOrganizationBilling(relayInfo) {
			model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		}
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		logger.LogError(ctx, "error settling billing: "+err.Error())
	}

	logModel := modelName
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	other := GenerateWssOtherInfo(ctx, relayInfo, usage, modelRatio, groupRatio,
		completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), modelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	if tieredResult != nil {
		InjectTieredBillingInfo(other, relayInfo, tieredResult)
	}
	attachQuotaSaturation(ctx, relayInfo, other)
	model.RecordConsumeLog(ctx, relayInfo.UserId, RelayConsumeLogParams(relayInfo, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(useTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	}))
}

func CalcOpenRouterCacheCreateTokens(usage dto.Usage, priceData types.PriceData) int {
	if priceData.CacheCreationRatio == 1 {
		return 0
	}
	quotaPrice := priceData.ModelRatio / common.QuotaPerUnit
	promptCacheCreatePrice := quotaPrice * priceData.CacheCreationRatio
	promptCacheReadPrice := quotaPrice * priceData.CacheRatio
	completionPrice := quotaPrice * priceData.CompletionRatio

	cost, _ := usage.Cost.(float64)
	totalPromptTokens := float64(usage.PromptTokens)
	completionTokens := float64(usage.CompletionTokens)
	promptCacheReadTokens := float64(usage.PromptTokensDetails.CachedTokens)

	value := (cost -
		totalPromptTokens*quotaPrice +
		promptCacheReadTokens*(quotaPrice-promptCacheReadPrice) -
		completionTokens*completionPrice) /
		(promptCacheCreatePrice - quotaPrice)
	quota, clamp := common.QuotaRoundChecked(value)
	if clamp != nil {
		return -1
	}
	return quota
}

func PostAudioConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent string) {

	var tieredUsedVars map[string]bool
	if snap := relayInfo.TieredBillingSnapshot; snap != nil {
		tieredUsedVars = billingexpr.UsedVars(snap.ExprString)
	}
	var tieredResult *billingexpr.TieredResult
	tieredOk, tieredQuota, tieredRes := TryTieredSettle(relayInfo, BuildTieredTokenParams(usage, false, tieredUsedVars))
	if tieredOk {
		tieredResult = tieredRes
	}

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	textInputTokens := usage.PromptTokensDetails.TextTokens
	textOutTokens := usage.CompletionTokenDetails.TextTokens

	audioInputTokens := usage.PromptTokensDetails.AudioTokens
	audioOutTokens := usage.CompletionTokenDetails.AudioTokens

	tokenName := ctx.GetString("token_name")
	billingModelName := relayInfo.GetBillingModelName()
	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(billingModelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(billingModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(billingModelName))

	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	usePrice := relayInfo.PriceData.UsePrice

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  billingModelName,
		UsePrice:   usePrice,
		ModelRatio: modelRatio,
		GroupRatio: groupRatio,
	}

	quota, clamp := calculateAudioQuota(quotaInfo)
	noteQuotaClamp(relayInfo, clamp)
	if tieredOk {
		quota = tieredQuota
	}

	totalTokens := usage.TotalTokens
	var logContent string
	if !usePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，音频倍率 %.2f，音频补全倍率 %.2f，分组倍率 %.2f",
			modelRatio, completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), groupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", modelPrice, groupRatio)
	}

	// record all the consume log even if quota is 0
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
		logContent += "（可能是上游超时）"
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, billingModelName, relayInfo.FinalPreConsumedQuota))
	} else {
		// 组织请求的个人用量在组织结算事务里不落账：调用者的钱包、订阅与
		// 个人看板都不参与，只有渠道用量仍按请求计一次。
		if !IsOrganizationBilling(relayInfo) {
			model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		}
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		logger.LogError(ctx, "error settling billing: "+err.Error())
	}

	logModel := billingModelName
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	other := GenerateAudioOtherInfo(ctx, relayInfo, usage, modelRatio, groupRatio,
		completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), modelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	if tieredResult != nil {
		InjectTieredBillingInfo(other, relayInfo, tieredResult)
	}
	attachQuotaSaturation(ctx, relayInfo, other)
	model.RecordConsumeLog(ctx, relayInfo.UserId, RelayConsumeLogParams(relayInfo, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(useTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	}))
	goBackgroundWork(func() {
		perfmetrics.RecordRelaySample(relayInfo, true, int64(usage.CompletionTokens))
	})
}

// loadTokenForQuotaTx 在事务里把令牌捞出来，顺便把作用域对清楚。
//
// 组织令牌的 id 与个人令牌同处一张表，只按 id 查会让一个组织令牌的 id
// 撞上另一个作用域下行——所以组织作用域必须带上 organization_id 一起匹配。
// 找不到令牌时返回 (nil, nil)：调用方把它当成「无需扣令牌额度」，而不是错误。
func loadTokenForQuotaTx(tx *gorm.DB, relayInfo *relaycommon.RelayInfo) (*model.Token, error) {
	if tx == nil {
		tx = model.DB
	}
	if relayInfo == nil {
		return nil, nil
	}
	if relayInfo.TokenId == 0 && relayInfo.TokenKey == "" {
		return nil, nil
	}
	var token model.Token
	// 必须另起一个 statement：tx 往往已经执行过（clone 归零），直接 Where 会把作用域条件
	// 写回调用方的 Statement，调用方随后的 Updates 会带上这些条件。MySQL/SQLite 只是把
	// UPDATE 收窄，PostgreSQL 的 UpdateClauses 含 FROM，会渲染出
	// `UPDATE ... FROM "tokens"` 直接报 42712。
	query := tx.Session(&gorm.Session{NewDB: true})
	// NewDB 会重建 Statement，而 Unscoped 默认不跨 Statement 继承（除非配置
	// PropagateUnscoped）。结算路径故意传 tx.Unscoped() 来读被软删的令牌，这里必须带上。
	if tx.Statement.Unscoped {
		query = query.Unscoped()
	}
	if relayInfo.ScopeType == model.TokenScopeOrganization {
		if relayInfo.OrganizationId <= 0 {
			return nil, errors.New("organization token quota scope is invalid")
		}
		query = query.Where(
			"scope_type = ? AND organization_id = ?",
			model.TokenScopeOrganization,
			relayInfo.OrganizationId,
		)
	}
	if relayInfo.TokenId > 0 {
		if err := query.Where("id = ?", relayInfo.TokenId).First(&token).Error; err != nil {
			return nil, err
		}
	} else {
		tokenKey := strings.TrimPrefix(relayInfo.TokenKey, "sk-")
		if err := query.Where("key = ?", tokenKey).First(&token).Error; err != nil {
			return nil, err
		}
	}
	model.NormalizeTokenScope(&token)
	relayInfo.TokenId = token.Id
	relayInfo.TokenKey = token.Key
	relayInfo.TokenUnlimited = token.UnlimitedQuota
	return &token, nil
}

// DecreaseTokenQuotaTx 在给定事务里扣减令牌额度。
// 返回值是「实际扣掉的额度」；返回 0 表示额度由无限额令牌承担（只累加用量）。
func DecreaseTokenQuotaTx(tx *gorm.DB, relayInfo *relaycommon.RelayInfo, quota int) (int, error) {
	return decreaseTokenQuotaTx(tx, relayInfo, quota, nil)
}

// decreaseTokenQuotaWithSnapshotTx 用调用方固化的 unlimitedQuota 快照扣减，
// 而不是重新读一遍令牌当前值：组织账本在预扣与结算之间要保证口径一致。
func decreaseTokenQuotaWithSnapshotTx(tx *gorm.DB, relayInfo *relaycommon.RelayInfo, quota int, unlimitedQuota bool) (int, error) {
	return decreaseTokenQuotaTx(tx, relayInfo, quota, &unlimitedQuota)
}

func decreaseTokenQuotaTx(tx *gorm.DB, relayInfo *relaycommon.RelayInfo, quota int, unlimitedQuota *bool) (int, error) {
	if quota < 0 {
		return 0, errors.New("quota 不能为负数！")
	}
	if tx == nil {
		tx = model.DB
	}
	if relayInfo == nil || relayInfo.IsPlayground || quota == 0 {
		return 0, nil
	}
	token, err := loadTokenForQuotaTx(tx, relayInfo)
	if err != nil {
		return 0, err
	}
	if token == nil {
		return 0, nil
	}
	isUnlimited := token.UnlimitedQuota
	if unlimitedQuota != nil {
		isUnlimited = *unlimitedQuota
	}
	if isUnlimited {
		result := tx.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"accessed_time": common.GetTimestamp(),
		})
		return 0, result.Error
	}
	if token.RemainQuota < quota {
		return 0, fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(token.RemainQuota), logger.FormatQuota(quota))
	}
	// WHERE 里再带一次 remain_quota >= ?：并发下这个守卫才是真正的原子闸门，
	// 前面那次比较只用来给出可读的错误信息。
	result := tx.Model(&model.Token{}).Where("id = ? AND remain_quota >= ?", token.Id, quota).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota - ?", quota),
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"accessed_time": common.GetTimestamp(),
		},
	)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(token.RemainQuota), logger.FormatQuota(quota))
	}
	return quota, nil
}

// IncreaseTokenQuotaTx 在给定事务里退还令牌额度，返回值是实际退回的额度。
func IncreaseTokenQuotaTx(tx *gorm.DB, relayInfo *relaycommon.RelayInfo, quota int) (int, error) {
	return increaseTokenQuotaTx(tx, relayInfo, quota, nil)
}

func increaseTokenQuotaWithSnapshotTx(tx *gorm.DB, relayInfo *relaycommon.RelayInfo, quota int, unlimitedQuota bool) (int, error) {
	return increaseTokenQuotaTx(tx, relayInfo, quota, &unlimitedQuota)
}

func increaseTokenQuotaTx(tx *gorm.DB, relayInfo *relaycommon.RelayInfo, quota int, unlimitedQuota *bool) (int, error) {
	if quota < 0 {
		return 0, errors.New("quota 不能为负数！")
	}
	if tx == nil {
		tx = model.DB
	}
	if relayInfo == nil || relayInfo.IsPlayground || quota == 0 {
		return 0, nil
	}
	token, err := loadTokenForQuotaTx(tx, relayInfo)
	if err != nil {
		return 0, err
	}
	if token == nil {
		return 0, nil
	}
	isUnlimited := token.UnlimitedQuota
	if unlimitedQuota != nil {
		isUnlimited = *unlimitedQuota
	}
	if isUnlimited {
		result := tx.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]interface{}{
			"used_quota":    gorm.Expr("CASE WHEN used_quota - ? < 0 THEN 0 ELSE used_quota - ? END", quota, quota),
			"accessed_time": common.GetTimestamp(),
		})
		return 0, result.Error
	}
	// 退款同样要在 SQL 里夹住 used_quota 的下界，否则重试/补偿路径能把用量退成负数。
	result := tx.Model(&model.Token{}).Where("id = ?", token.Id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota + ?", quota),
			"used_quota":    gorm.Expr("CASE WHEN used_quota - ? < 0 THEN 0 ELSE used_quota - ? END", quota, quota),
			"accessed_time": common.GetTimestamp(),
		},
	)
	if result.Error != nil {
		return 0, result.Error
	}
	return quota, nil
}

func PreConsumeTokenQuota(relayInfo *relaycommon.RelayInfo, quota int) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if relayInfo.IsPlayground {
		return nil
	}
	// 原子预扣：检查与扣减在同一操作中完成，并发请求不可能同时通过检查后超扣。
	reserved, err := model.TryReserveTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota, relayInfo.TokenUnlimited)
	if err != nil {
		return err
	}
	if !reserved {
		remainQuota := 0
		if token, tokenErr := model.GetTokenByKey(relayInfo.TokenKey, false); tokenErr == nil && token != nil {
			remainQuota = token.RemainQuota
		}
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(remainQuota), logger.FormatQuota(quota))
	}
	return nil
}

type postConsumeQuotaResult struct {
	FundingApplied bool
	TokenApplied   bool
}

// RelayConsumeLogParams 把请求上的作用域与归属信息一次性带进消费日志。
//
// 日志是唯一同时展示「谁发起、算在哪个组织、由哪个令牌扣费」的地方，
// 散在十几个调用点手填容易漏；统一在这里从 RelayInfo 抄一遍，
// 个人请求抄到的就是 personal/自己，不需要额外的分支。
func RelayConsumeLogParams(relayInfo *relaycommon.RelayInfo, params model.RecordConsumeLogParams) model.RecordConsumeLogParams {
	if relayInfo == nil {
		return params
	}
	params.ScopeType = relayInfo.ScopeType
	params.ScopeId = relayInfo.ScopeId
	params.BillingAccountType = relayInfo.BillingAccountType
	params.BillingAccountId = relayInfo.BillingAccountId
	params.OrganizationId = relayInfo.OrganizationId
	params.ActorUserId = relayInfo.ActorUserId
	params.CreatorUserId = relayInfo.CreatorUserId
	params.ResponsibleUserId = relayInfo.ResponsibleUserId
	params.OrganizationBillingSessionId = relayInfo.OrganizationBillingSessionId
	params.OrganizationBillingSessionKey = relayInfo.OrganizationBillingSessionKey
	return params
}

func PostConsumeQuota(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool) error {
	_, err := postConsumeQuotaWithResult(relayInfo, quota, preConsumedQuota, sendEmail)
	return err
}

// PostConsumeQuotaWithRequestCount 与 PostConsumeQuota 相同，但显式决定是否计入请求数。
// 按次计费（比如部分音频接口）会传 false，避免组织面板把一次调用算成两次。
func PostConsumeQuotaWithRequestCount(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool, incrementRequestCount bool) error {
	_, err := postConsumeQuotaWithResultCount(relayInfo, quota, preConsumedQuota, sendEmail, incrementRequestCount)
	return err
}

func postConsumeQuotaWithResult(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool) (result postConsumeQuotaResult, err error) {
	return postConsumeQuotaWithResultCount(relayInfo, quota, preConsumedQuota, sendEmail, quota > 0)
}

func postConsumeQuotaWithResultCount(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool, incrementRequestCount bool) (result postConsumeQuotaResult, err error) {

	// 组织计费一次性动组织用量和令牌额度，不走下面那两条个人路径。
	// FundingApplied / TokenApplied 仍要如实回报：调用方（比如 Midjourney 的
	// 落库路径）靠它决定是否保留任务的 quota 标记，报错会把它当成扣费失败清掉。
	if IsOrganizationBilling(relayInfo) {
		orgErr := consumeOrganizationAndTokenQuotaWithRequestCount(relayInfo, quota, incrementRequestCount)
		result.FundingApplied = orgErr == nil
		result.TokenApplied = orgErr == nil && !relayInfo.IsPlayground && relayInfo.TokenId > 0 && !relayInfo.TokenUnlimited
		return result, orgErr
	}

	// 1) Consume from wallet quota OR subscription item
	if relayInfo != nil && relayInfo.BillingSource == BillingSourceSubscription {
		if relayInfo.SubscriptionId == 0 {
			return result, errors.New("subscription id is missing")
		}
		delta := int64(quota)
		if delta != 0 {
			if err := model.PostConsumeUserSubscriptionDelta(relayInfo.SubscriptionId, delta); err != nil {
				return result, err
			}
			relayInfo.SubscriptionPostDelta += delta
		}
	} else {
		// Wallet
		if quota > 0 {
			err = model.DecreaseUserQuota(relayInfo.UserId, quota, false)
		} else {
			err = model.IncreaseUserQuota(relayInfo.UserId, -quota, false)
		}
		if err != nil {
			return result, err
		}
	}
	result.FundingApplied = true

	if !relayInfo.IsPlayground {
		if quota > 0 {
			err = model.DecreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		} else {
			err = model.IncreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota)
		}
		if err != nil {
			return result, err
		}
		result.TokenApplied = true
	}

	if sendEmail {
		if (quota + preConsumedQuota) != 0 {
			checkAndSendQuotaNotify(relayInfo, quota, preConsumedQuota)
		}
	}

	return result, nil
}

func checkAndSendQuotaNotify(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int) {
	goBackgroundWork(func() {
		userSetting := relayInfo.UserSetting
		threshold := common.QuotaRemindThreshold
		if userSetting.QuotaWarningThreshold != 0 {
			threshold = int(userSetting.QuotaWarningThreshold)
		}

		//noMoreQuota := userCache.Quota-(quota+preConsumedQuota) <= 0
		quotaTooLow := false
		consumeQuota := quota + preConsumedQuota
		if relayInfo.UserQuota-consumeQuota < threshold {
			quotaTooLow = true
		}
		if quotaTooLow {
			prompt := "您的额度即将用尽"
			topUpLink := PaymentReturnURL("/wallet")

			// 根据通知方式生成不同的内容格式
			var content string
			var values []interface{}

			notifyType := userSetting.NotifyType
			if notifyType == "" {
				notifyType = dto.NotifyTypeEmail
			}

			if notifyType == dto.NotifyTypeBark {
				// Bark推送使用简短文本，不支持HTML
				content = "{{value}}，剩余额度：{{value}}，请及时充值"
				values = []interface{}{prompt, logger.FormatQuota(relayInfo.UserQuota)}
			} else if notifyType == dto.NotifyTypeGotify {
				content = "{{value}}，当前剩余额度为 {{value}}，请及时充值。"
				values = []interface{}{prompt, logger.FormatQuota(relayInfo.UserQuota)}
			} else {
				// 默认内容格式，适用于Email和Webhook（支持HTML）
				content = "{{value}}，当前剩余额度为 {{value}}，为了不影响您的使用，请及时充值。<br/>充值链接：<a href='{{value}}'>{{value}}</a>"
				values = []interface{}{prompt, logger.FormatQuota(relayInfo.UserQuota), topUpLink, topUpLink}
			}

			err := NotifyUser(relayInfo.UserId, relayInfo.UserEmail, relayInfo.UserSetting, dto.NewNotify(dto.NotifyTypeQuotaExceed, prompt, content, values))
			if err != nil {
				common.SysError(fmt.Sprintf("failed to send quota notify to user %d: %s", relayInfo.UserId, err.Error()))
			}
		}
	})
}

func checkAndSendSubscriptionQuotaNotify(relayInfo *relaycommon.RelayInfo) {
	goBackgroundWork(func() {
		if relayInfo == nil {
			return
		}
		if relayInfo.SubscriptionId == 0 || relayInfo.SubscriptionAmountTotal <= 0 {
			return
		}

		userSetting := relayInfo.UserSetting
		threshold := common.QuotaRemindThreshold
		if userSetting.QuotaWarningThreshold != 0 {
			threshold = int(userSetting.QuotaWarningThreshold)
		}

		usedAfter := relayInfo.SubscriptionAmountUsedAfterPreConsume + relayInfo.SubscriptionPostDelta
		remaining := relayInfo.SubscriptionAmountTotal - usedAfter
		if remaining >= int64(threshold) {
			return
		}

		prompt := "您的订阅额度即将用尽"
		topUpLink := PaymentReturnURL("/wallet")

		var content string
		var values []interface{}
		notifyType := userSetting.NotifyType
		if notifyType == "" {
			notifyType = dto.NotifyTypeEmail
		}

		if notifyType == dto.NotifyTypeBark {
			content = "{{value}}，剩余额度：{{value}}，请及时充值"
			values = []interface{}{prompt, logger.FormatQuota(int(remaining))}
		} else if notifyType == dto.NotifyTypeGotify {
			content = "{{value}}，当前剩余额度为 {{value}}，请及时充值。"
			values = []interface{}{prompt, logger.FormatQuota(int(remaining))}
		} else {
			content = "{{value}}，当前剩余额度为 {{value}}，为了不影响您的使用，请及时充值。<br/>充值链接：<a href='{{value}}'>{{value}}</a>"
			values = []interface{}{prompt, logger.FormatQuota(int(remaining)), topUpLink, topUpLink}
		}

		if err := NotifyUser(relayInfo.UserId, relayInfo.UserEmail, relayInfo.UserSetting, dto.NewNotify(dto.NotifyTypeQuotaExceed, prompt, content, values)); err != nil {
			common.SysError(fmt.Sprintf("failed to send subscription quota notify to user %d: %s", relayInfo.UserId, err.Error()))
		}
	})
}
