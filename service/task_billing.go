package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo, task *model.Task) {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	// 支持任务仅按次计费
	if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		logContent = fmt.Sprintf("%s，按次计费", logContent)
	} else {
		var contents []string
		if otherRatios := info.PriceData.OtherRatios(); len(otherRatios) > 0 {
			for key, ra := range otherRatios {
				if 1.0 != ra {
					contents = append(contents, fmt.Sprintf("%s: %.2f", key, ra))
				}
			}
		}
		if snap := info.TieredBillingSnapshot; snap != nil {
			for key, value := range snap.UsageFacts {
				contents = append(contents, fmt.Sprintf("%s: %v", key, value))
			}
		}
		if len(contents) > 0 {
			logContent = fmt.Sprintf("%s, 计算参数：%s", logContent, strings.Join(contents, ", "))
		}
	}
	other := make(map[string]interface{})
	other["is_task"] = true
	other["request_path"] = c.Request.URL.Path
	other["model_price"] = info.PriceData.ModelPrice
	if info.PriceData.ModelRatio > 0 {
		other["model_ratio"] = info.PriceData.ModelRatio
	}
	other["group_ratio"] = info.PriceData.GroupRatioInfo.GroupRatio
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	if info.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = info.UpstreamModelName
	}
	if snap := info.TieredBillingSnapshot; snap != nil {
		other["billing_mode"] = "tiered_expr"
		other["expr_b64"] = base64.StdEncoding.EncodeToString([]byte(snap.ExprString))
		other["matched_tier"] = snap.EstimatedTier
		if len(snap.UsageFacts) > 0 {
			other["usage_facts"] = snap.UsageFacts
		}
	}
	appendTaskLogInfo(task, other)
	attachQuotaSaturation(c, info, other)
	model.RecordConsumeLog(c, info.UserId, RelayConsumeLogParams(info, model.RecordConsumeLogParams{
		ChannelId: info.ChannelId,
		ModelName: info.OriginModelName,
		TokenName: tokenName,
		Quota:     info.PriceData.Quota,
		Content:   logContent,
		TokenId:   info.TokenId,
		Group:     info.UsingGroup,
		Other:     other,
	}))
	// 组织计费的用量在组织账本里结算，个人看板保持零增长。
	if !IsOrganizationBilling(info) {
		model.UpdateUserUsedQuotaAndRequestCount(info.UserId, info.PriceData.Quota)
	}
	model.UpdateChannelUsedQuota(info.ChannelId, info.PriceData.Quota)
}

// ---------------------------------------------------------------------------
// 异步任务计费辅助函数
// ---------------------------------------------------------------------------

// resolveTokenKey 通过 TokenId 运行时获取令牌 Key（用于 Redis 缓存操作）。
// 如果令牌已被删除或查询失败，返回空字符串。
func resolveTokenKey(ctx context.Context, tokenId int, taskID string) string {
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("获取令牌 key 失败 (tokenId=%d, task=%s): %s", tokenId, taskID, err.Error()))
		return ""
	}
	return token.Key
}

// taskIsSubscription 判断任务是否通过订阅计费。
func taskIsSubscription(task *model.Task) bool {
	return task.PrivateData.BillingSource == BillingSourceSubscription && task.PrivateData.SubscriptionId > 0
}

// taskAdjustFunding 调整任务的资金来源（钱包或订阅），delta > 0 表示扣费，delta < 0 表示退还。
func taskAdjustFunding(task *model.Task, delta int) error {
	if taskIsSubscription(task) {
		return model.PostConsumeUserSubscriptionDelta(task.PrivateData.SubscriptionId, int64(delta))
	}
	if delta > 0 {
		return model.DecreaseUserQuota(task.UserId, delta, false)
	}
	return model.IncreaseUserQuota(task.UserId, -delta, false)
}

// taskAdjustTokenQuota 调整任务的令牌额度，delta > 0 表示扣费，delta < 0 表示退还。
// 需要通过 resolveTokenKey 运行时获取 key（不从 PrivateData 中读取）。
func taskAdjustTokenQuota(ctx context.Context, task *model.Task, delta int) {
	if task.PrivateData.TokenId <= 0 || delta == 0 {
		return
	}
	tokenKey := resolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		return
	}
	var err error
	if delta > 0 {
		err = model.DecreaseTokenQuota(task.PrivateData.TokenId, tokenKey, delta)
	} else {
		err = model.IncreaseTokenQuota(task.PrivateData.TokenId, tokenKey, -delta)
	}
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("调整令牌额度失败 (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
	}
}

// taskBillingOther 从 task 的 BillingContext 构建日志 Other 字段。
func taskBillingOther(task *model.Task) map[string]interface{} {
	other := make(map[string]interface{})
	if bc := task.PrivateData.BillingContext; bc != nil {
		other["model_price"] = bc.ModelPrice
		if bc.ModelRatio > 0 {
			other["model_ratio"] = bc.ModelRatio
		}
		other["group_ratio"] = bc.GroupRatio
		if priceData := taskBillingContextPriceData(bc); priceData != nil {
			for k, v := range priceData.OtherRatios() {
				other[k] = v
			}
		}
		if snap := bc.TieredSnapshot; snap != nil {
			other["billing_mode"] = "tiered_expr"
			other["expr_b64"] = base64.StdEncoding.EncodeToString([]byte(snap.ExprString))
			other["matched_tier"] = snap.EstimatedTier
			if len(snap.UsageFacts) > 0 {
				other["usage_facts"] = snap.UsageFacts
			}
		}
	}
	props := task.Properties
	if props.UpstreamModelName != "" && props.UpstreamModelName != props.OriginModelName {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = props.UpstreamModelName
	}
	appendTaskLogInfo(task, other)
	return other
}

func appendTaskLogInfo(task *model.Task, other map[string]interface{}) {
	if task == nil || other == nil {
		return
	}
	if task.TaskID != "" {
		other["task_id"] = task.TaskID
	}
	if task.PrivateData.Execution != nil {
		AppendTaskPluginAuditInfo(other, task.PrivateData.Execution.TaskPlugin)
	}
	if task.PrivateData.UpstreamTaskID == "" && task.PrivateData.NodeName == "" {
		return
	}
	rootInfo, ok := other["root_info"].(map[string]interface{})
	if !ok || rootInfo == nil {
		rootInfo = map[string]interface{}{}
		other["root_info"] = rootInfo
	}
	if task.PrivateData.UpstreamTaskID != "" {
		rootInfo["upstream_task_id"] = task.PrivateData.UpstreamTaskID
	}
	if task.PrivateData.NodeName != "" {
		rootInfo["node_name"] = task.PrivateData.NodeName
	}
}

func taskBillingContextPriceData(bc *model.TaskBillingContext) *types.PriceData {
	if bc == nil || len(bc.OtherRatios) == 0 {
		return nil
	}
	priceData := &types.PriceData{}
	if !priceData.ReplaceOtherRatios(bc.OtherRatios) {
		return nil
	}
	return priceData
}

// taskModelName 从 BillingContext 或 Properties 中获取模型名称。
func taskModelName(task *model.Task) string {
	if bc := task.PrivateData.BillingContext; bc != nil && bc.OriginModelName != "" {
		return bc.OriginModelName
	}
	return task.Properties.OriginModelName
}

// RefundTaskQuota 统一的任务失败退款逻辑。
// 当异步任务失败时，退还资金与令牌额度，并回减用户和渠道用量。
// 返回资金来源是否已成功退还；失败时保留 quota，供显式重试或人工对账。
func RefundTaskQuota(ctx context.Context, task *model.Task, reason string) bool {
	quota := task.Quota
	if quota == 0 {
		return true
	}

	// 0. 组织计费走组织账本：它的预扣是账本会话，钱和组织用量在同一个事务里
	//    回滚，个人钱包/订阅一分未动，下面那三步必须整体跳过。账目痕迹由账本
	//    自己的 refund 记录给出，调用方还会再写一条系统日志。
	if handled, err := RefundTaskOrganizationQuota(task); handled {
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("退还组织账本失败 task %s: %s", task.TaskID, err.Error()))
			return false
		}
		return true
	}

	// 1. 退还资金来源（钱包或订阅）
	if err := taskAdjustFunding(task, -quota); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("退还资金来源失败 task %s: %s", task.TaskID, err.Error()))
		return false
	}

	// 2. 退还令牌额度
	taskAdjustTokenQuota(ctx, task, -quota)

	// 3. 回减预扣时累计的用户和渠道用量，请求次数保持不变
	model.UpdateUserUsedQuota(task.UserId, -quota)
	model.UpdateChannelUsedQuota(task.ChannelId, -quota)

	// 4. 记录日志
	recordTaskRefundLog(task, reason, quota)

	// 5. 资金退款完成后再清除持久化标记。
	// 回写失败必须显式告警，避免漏掉潜在的重复退款风险。
	task.Quota = 0
	if err := task.UpdateQuota(); err != nil {
		logger.LogError(ctx, fmt.Sprintf("退款成功但清除 task quota 失败 task %s: %s", task.TaskID, err.Error()))
	}
	return true
}

// recordTaskRefundLog 记录一条异步任务退款日志。
func recordTaskRefundLog(task *model.Task, reason string, quota int) {
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	params := model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     quota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	}
	// 退款日志必须继承任务的作用域：组织任务的退款在个人的日志列表里
	// 会显示成一条来路不明的进账。
	applyTaskBillingScope(&params, task)
	model.RecordTaskBillingLog(params)
}

// applyTaskBillingScope 把任务在提交阶段落库的作用域与账单归属补进日志参数。
//
// 结算和退款都发生在轮询阶段，那时请求上下文已经没了，只有任务记录能说明
// 这笔钱原本记在谁头上。漏掉这一步，日志会以空作用域落库、被当成个人记录。
func applyTaskBillingScope(params *model.RecordTaskBillingLogParams, task *model.Task) {
	model.NormalizeTaskBillingScope(task)
	params.ScopeType = task.ScopeType
	params.ScopeId = task.ScopeId
	params.BillingAccountType = task.BillingAccountType
	params.BillingAccountId = task.BillingAccountId
	params.OrganizationId = task.OrganizationId
	params.ResponsibleUserId = task.ResponsibleUserId
}

// applyMidjourneyBillingScope 与 applyTaskBillingScope 同理，作用在 Midjourney 任务上。
func applyMidjourneyBillingScope(params *model.RecordTaskBillingLogParams, task *model.Midjourney) {
	if task == nil {
		return
	}
	model.NormalizeMidjourneyBillingScope(task)
	params.ScopeType = task.ScopeType
	params.ScopeId = task.ScopeId
	params.BillingAccountType = task.BillingAccountType
	params.BillingAccountId = task.BillingAccountId
	params.OrganizationId = task.OrganizationId
	params.ResponsibleUserId = task.ResponsibleUserId
}

// RecalculateTaskQuota 通用的异步差额结算。
// actualQuota 是任务完成后的实际应扣额度，与预扣额度 (task.Quota) 做差额结算。
// reason 用于日志记录（例如 "token重算" 或 "adaptor调整"）。
// clamps 可选：若计算 actualQuota 时发生额度饱和，将其记入日志 admin_info（仅管理员可见）。
func RecalculateTaskQuota(ctx context.Context, task *model.Task, actualQuota int, reason string, clamps ...*common.QuotaClamp) {
	if actualQuota < 0 {
		return
	}

	// 组织计费没有「资金来源差额」这一步：预扣就是一个账本会话，按实际金额
	// 结算即可，多退少补都发生在组织那个事务里。个人额度与渠道用量同理，
	// 组织请求从提交起就没碰过它们。
	if handled, err := SettleAsyncTaskOrganizationQuota(task, actualQuota); handled {
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("组织账本差额结算失败 task %s: %s", task.TaskID, err.Error()))
			return
		}
		if err := task.UpdateQuota(); err != nil {
			logger.LogError(ctx, fmt.Sprintf("组织差额结算回写 quota 失败 task %s: %s", task.TaskID, err.Error()))
		}
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 组织账本结算：%s（%s）", task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}

	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota

	if quotaDelta == 0 {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）",
			task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))

	// 调整资金来源
	//
	// 组织任务如果没留下账本会话（提交阶段没走到 PreConsumeBilling 的老任务），
	// 差额也只能记到组织账上。绝不能落进 taskAdjustFunding——那条路走的是个人
	// 钱包和订阅，会把组织的开销记到个人的账上。
	if IsOrganizationBilling(TaskBillingRelayInfo(task)) {
		if err := PostConsumeQuotaWithRequestCount(TaskBillingRelayInfo(task), quotaDelta, task.Quota, false, false); err != nil {
			logger.LogError(ctx, fmt.Sprintf("组织差额结算失败 task %s: %s", task.TaskID, err.Error()))
			return
		}
	} else {
		if err := taskAdjustFunding(task, quotaDelta); err != nil {
			logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
			return
		}

		// 调整令牌额度
		taskAdjustTokenQuota(ctx, task, quotaDelta)

		// 提交阶段已经累计过一次请求；结算阶段只调整最终用量。
		model.UpdateUserUsedQuota(task.UserId, quotaDelta)
	}

	task.Quota = actualQuota
	if err := task.UpdateQuota(); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算回写 quota 失败 task %s: %s", task.TaskID, err.Error()))
	}

	model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)

	var logType int
	var logQuota int
	if quotaDelta > 0 {
		logType = model.LogTypeConsume
		logQuota = quotaDelta
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["pre_consumed_quota"] = preConsumedQuota
	other["actual_quota"] = actualQuota
	for _, clamp := range clamps {
		attachQuotaSaturationToOther(other, clamp)
	}
	billingLogParams := model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   logType,
		Content:   reason,
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     logQuota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
		NodeName:  task.PrivateData.NodeName,
	}
	applyTaskBillingScope(&billingLogParams, task)
	model.RecordTaskBillingLog(billingLogParams)
}

// RecalculateTaskQuotaByTokens 根据实际 token 消耗重新计费（异步差额结算）。
// 当任务成功且返回了 totalTokens 时，根据模型倍率和分组倍率重新计算实际扣费额度，
// 与预扣费的差额进行补扣或退还。支持钱包和订阅计费来源。
func RecalculateTaskQuotaByTokens(ctx context.Context, task *model.Task, totalTokens int) bool {
	if totalTokens <= 0 {
		return false
	}

	modelName := taskModelName(task)

	// 获取模型价格和倍率
	modelRatio, hasRatioSetting, _ := ratio_setting.GetModelRatio(modelName)
	// 只有配置了倍率(非固定价格)时才按 token 重新计费
	if !hasRatioSetting || modelRatio <= 0 {
		return false
	}

	// 获取用户和组的倍率信息
	group := task.Group
	if group == "" {
		user, err := model.GetUserById(task.UserId, false)
		if err == nil {
			group = user.Group
		}
	}
	if group == "" {
		return false
	}

	groupRatio := ratio_setting.GetGroupRatio(group)
	userGroupRatio, hasUserGroupRatio := ratio_setting.GetGroupGroupRatio(group, group)

	var finalGroupRatio float64
	if hasUserGroupRatio {
		finalGroupRatio = userGroupRatio
	} else {
		finalGroupRatio = groupRatio
	}

	// 计算 OtherRatios 乘积（视频折扣、时长等）
	otherMultiplier := 1.0
	if priceData := taskBillingContextPriceData(task.PrivateData.BillingContext); priceData != nil {
		otherMultiplier = priceData.OtherRatioMultiplier()
	}

	// 计算实际应扣费额度: totalTokens * modelRatio * groupRatio * otherMultiplier（饱和转换，防止溢出成负数）
	actualQuota, clamp := common.QuotaFromFloatChecked(float64(totalTokens) * modelRatio * finalGroupRatio * otherMultiplier)

	reason := fmt.Sprintf("token重算：tokens=%d, modelRatio=%.2f, groupRatio=%.2f, otherMultiplier=%.4f", totalTokens, modelRatio, finalGroupRatio, otherMultiplier)
	RecalculateTaskQuota(ctx, task, actualQuota, reason, clamp)
	return true
}
