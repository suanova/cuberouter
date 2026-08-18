package controller

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

const billingMaxRangeSeconds = 31 * 24 * 3600 // 31 天(含起止两端)

// billingDateLayout 账单接口接受的日期格式。
const billingDateLayout = "2006-01-02"

var (
	errBillingDateRequired   = errors.New("start 和 end 不能为空,格式 YYYY-MM-DD")
	errBillingDateFormat     = errors.New("日期格式错误,需 YYYY-MM-DD")
	errBillingEndBeforeStart = errors.New("end 不能早于 start")
	errBillingRangeTooWide   = errors.New("时间跨度不能超过 31 天")
)

// parseBillingDateRange 把 "YYYY-MM-DD" 解析为本地时区的闭区间 Unix 秒。
// start 当日 00:00:00,end 当日 23:59:59。
func parseBillingDateRange(start, end string) (startTs, endTs int64, err error) {
	if start == "" || end == "" {
		return 0, 0, errBillingDateRequired
	}
	st, err := time.ParseInLocation(billingDateLayout, start, time.Local)
	if err != nil {
		return 0, 0, errBillingDateFormat
	}
	et, err := time.ParseInLocation(billingDateLayout, end, time.Local)
	if err != nil {
		return 0, 0, errBillingDateFormat
	}
	if et.Before(st) {
		return 0, 0, errBillingEndBeforeStart
	}
	// end 取当天 23:59:59
	et = et.Add(24*time.Hour - time.Second)
	if et.Unix()-st.Unix() > billingMaxRangeSeconds {
		return 0, 0, errBillingRangeTooWide
	}
	return st.Unix(), et.Unix(), nil
}

// quotaToDisplayAmount 把内部额度按站点展示类型换算为金额。
// 供账单报表与 OpenAI 兼容 billing 接口（controller/billing.go）共用。
// 支持 USD / CNY / TOKENS / CUSTOM 四种展示类型。
func quotaToDisplayAmount(quota int) float64 {
	amount := float64(quota)
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		amount = amount / common.QuotaPerUnit * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeCustom:
		rate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
		if rate <= 0 {
			rate = 1
		}
		amount = amount / common.QuotaPerUnit * rate
	case operation_setting.QuotaDisplayTypeTokens:
		// 保持 tokens 原值
	default:
		amount = amount / common.QuotaPerUnit
	}
	return amount
}

// buildBillingReportData 汇总指定用户计费账单：按模型汇总 + 按日明细。
// GetUserBillingReport 与 GetOpsBillingReport 共享，避免报表装配逻辑漂移。
func buildBillingReportData(user *model.User, startStr, endStr string, startTs, endTs int64) (dto.BillingReportData, error) {
	// 汇总(按模型)
	summaryRows, err := model.GetUserBillingAgg(user.Username, startTs, endTs, false)
	if err != nil {
		return dto.BillingReportData{}, err
	}
	// 按日(按模型)
	dailyRows, err := model.GetUserBillingAgg(user.Username, startTs, endTs, true)
	if err != nil {
		return dto.BillingReportData{}, err
	}

	// 组装汇总
	summary := make([]dto.BillingModelStat, 0, len(summaryRows))
	for _, r := range summaryRows {
		summary = append(summary, dto.BillingModelStat{
			ModelName:        r.ModelName,
			RequestCount:     r.RequestCount,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			CacheTokens:      r.CacheTokens,
			TotalTokens:      r.PromptTokens + r.CompletionTokens,
			Quota:            r.Quota,
			Amount:           quotaToDisplayAmount(r.Quota),
		})
	}

	// 组装按日(按 day_key 分组,每组内按模型)
	dailyMap := make(map[int64]*dto.BillingDailyStat)
	dailyOrder := make([]int64, 0)
	for _, r := range dailyRows {
		day, ok := dailyMap[r.DayKey]
		if !ok {
			day = &dto.BillingDailyStat{Date: model.BillingDayKeyToDate(r.DayKey)}
			dailyMap[r.DayKey] = day
			dailyOrder = append(dailyOrder, r.DayKey)
		}
		stat := dto.BillingModelStat{
			ModelName:        r.ModelName,
			RequestCount:     r.RequestCount,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			CacheTokens:      r.CacheTokens,
			TotalTokens:      r.PromptTokens + r.CompletionTokens,
			Quota:            r.Quota,
			Amount:           quotaToDisplayAmount(r.Quota),
		}
		day.Models = append(day.Models, stat)
		day.RequestCount += stat.RequestCount
		day.PromptTokens += stat.PromptTokens
		day.CompletionTokens += stat.CompletionTokens
		day.CacheTokens += stat.CacheTokens
		day.TotalTokens += stat.TotalTokens
		day.Quota += stat.Quota
		day.Amount += stat.Amount
	}
	daily := make([]dto.BillingDailyStat, 0, len(dailyOrder))
	for _, k := range dailyOrder {
		daily = append(daily, *dailyMap[k])
	}

	return dto.BillingReportData{
		User: dto.UserDashboardBrief{
			Id:           user.Id,
			Username:     user.Username,
			DisplayName:  user.DisplayName,
			Role:         user.Role,
			Group:        user.Group,
			Quota:        user.Quota,
			UsedQuota:    user.UsedQuota,
			RequestCount: user.RequestCount,
		},
		Currency: dto.BillingCurrency{
			Type:   operation_setting.GetQuotaDisplayType(),
			Symbol: operation_setting.GetCurrencySymbol(),
		},
		Range: dto.BillingRange{
			Start:          startStr,
			End:            endStr,
			StartTimestamp: startTs,
			EndTimestamp:   endTs,
		},
		Summary: summary,
		Daily:   daily,
	}, nil
}

// GetUserBillingReport 查询用户计费账单报表(管理员)。
// 按用户名 + 日期范围(YYYY-MM-DD) 查询 token 用量与费用,含汇总与按日明细,
// 均按 model_name 分组。最大跨度 31 天。
func GetUserBillingReport(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		common.ApiErrorMsg(c, "用户名不能为空")
		return
	}

	startStr := c.Query("start")
	endStr := c.Query("end")
	startTs, endTs, err := parseBillingDateRange(startStr, endStr)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	// 按用户名定位用户
	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	data, err := buildBillingReportData(&user, startStr, endStr, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// GetOpsBillingReport 查询用户计费账单报表(运营/管理员)。
// 运营人员仅可查询自己邀请的用户,管理员可查询任意用户。
func GetOpsBillingReport(c *gin.Context) {
	callerId := c.GetInt("id")
	callerRole := c.GetInt("role")
	// callerId 由 OpsAuth 解析的身份写入；为 0 说明身份缺失，直接拒绝，
	// 避免与 InviterId==0 的无邀请人用户误判为匹配而越权。
	if callerId <= 0 {
		common.ApiErrorMsg(c, "无效的身份信息")
		return
	}

	username := c.Query("username")
	if username == "" {
		common.ApiErrorMsg(c, "用户名不能为空")
		return
	}

	startStr := c.Query("start")
	endStr := c.Query("end")
	startTs, endTs, err := parseBillingDateRange(startStr, endStr)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	// 按用户名定位用户
	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	// 运营人员(role < admin)只能查询自己邀请的用户
	if callerRole < common.RoleAdminUser {
		if user.InviterId != callerId {
			common.ApiErrorMsg(c, "无权查询该用户的账单,仅可查询自己邀请的用户")
			return
		}
	}

	data, err := buildBillingReportData(&user, startStr, endStr, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// GetReconciliationReport 查询计费对账报表(管理员)。
// 按日期范围(YYYY-MM-DD) 汇总所有用户的计费总金额(无交易明细),每用户一行
// (用量/quota/换算金额/模型数),按金额降序,含全平台合计。最大跨度 31 天。
func GetReconciliationReport(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	startTs, endTs, err := parseBillingDateRange(startStr, endStr)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	rows, err := model.GetReconciliationReport(startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 批量补齐用户概要(display_name/group/role)。rows 按 user_id 聚合,
	// 用一条 IN 查询取所有相关用户。走 GORM User 模型让 GORM 按方言正确
	// 引用 "group" 保留字（不手写 SELECT 投影，避免 MySQL ANSI_QUOTES 问题）。
	userIds := make([]int, 0, len(rows))
	for _, r := range rows {
		if r.UserId > 0 {
			userIds = append(userIds, r.UserId)
		}
	}
	type userBrief struct {
		Id          int
		DisplayName string
		Group       string
		Role        int
	}
	briefMap := make(map[int]userBrief)
	if len(userIds) > 0 {
		var users2 []model.User
		if err := model.DB.Where("id IN ?", userIds).Find(&users2).Error; err != nil {
			// 用户概要仅用于回显：查询失败时降级为空白，而不是让整个对账报表失败
			common.SysError(fmt.Sprintf("GetReconciliationReport 补齐用户概要失败: %v", err))
		} else {
			for _, u := range users2 {
				briefMap[u.Id] = userBrief{
					Id:          u.Id,
					DisplayName: u.DisplayName,
					Group:       u.Group,
					Role:        u.Role,
				}
			}
		}
	}

	users := make([]dto.ReconciliationUserStat, 0, len(rows))
	var total dto.ReconciliationTotals
	for _, r := range rows {
		b := briefMap[r.UserId]
		totalTokens := r.PromptTokens + r.CompletionTokens
		amount := quotaToDisplayAmount(r.Quota)
		users = append(users, dto.ReconciliationUserStat{
			UserId:           r.UserId,
			Username:         r.Username,
			DisplayName:      b.DisplayName,
			Group:            b.Group,
			Role:             b.Role,
			RequestCount:     r.RequestCount,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			TotalTokens:      totalTokens,
			CacheTokens:      r.CacheTokens,
			Quota:            r.Quota,
			Amount:           amount,
			ModelCount:       r.ModelCount,
		})
		total.UserCount++
		total.RequestCount += r.RequestCount
		total.PromptTokens += r.PromptTokens
		total.CompletionTokens += r.CompletionTokens
		total.TotalTokens += totalTokens
		total.CacheTokens += r.CacheTokens
		total.Quota += r.Quota
		total.Amount += amount
	}

	data := dto.ReconciliationReportData{
		Currency: dto.BillingCurrency{
			Type:   operation_setting.GetQuotaDisplayType(),
			Symbol: operation_setting.GetCurrencySymbol(),
		},
		Range: dto.BillingRange{
			Start:          startStr,
			End:            endStr,
			StartTimestamp: startTs,
			EndTimestamp:   endTs,
		},
		Total: total,
		Users: users,
	}
	common.ApiSuccess(c, data)
}
