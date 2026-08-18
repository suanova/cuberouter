package controller

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ============================================================
// Issue #61 — 9 个聚合 API（对外第三方收敛接口）
//
// 对外暴露的简化接口，封装内部用户管理、订阅计划、用户启停、
// 密码重置、额度调整与订阅绑定能力。鉴权通过 AdminAuth（access
// token 或 session）。响应格式与内部 API 不同，遵循 Issue 中
// 定义的 {status, ...} 格式。
// 审计不在此显式埋点：AdminAuth 已在鉴权链路内聚自动审计所有写操作，
// 显式调用反而重复。
// ============================================================

// ---- 统一响应辅助 ----

const (
	aggregatedStatusSuccess = "success"
	aggregatedStatusFail    = "fail"
	aggregatedStatusCodeOK  = 2000
)

func aggregatedSuccess(c *gin.Context, data gin.H) {
	resp := gin.H{"status": aggregatedStatusSuccess}
	for k, v := range data {
		resp[k] = v
	}
	c.JSON(http.StatusOK, resp)
}

func aggregatedFail(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, gin.H{
		"status":  aggregatedStatusFail,
		"message": msg,
	})
}

// ============================================================
// API 1: Create New User — POST /api/v1/users
// ============================================================

type AggregatedCreateUserRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	PlanId       int    `json:"plan_id"`
	Username     string `json:"username"`
	UserValidity string `json:"user_validity"` // ISO 8601 date, e.g. "2026-12-31"
	InviterId    int    `json:"inviter_id"`    // 运营用户邀请人 user id
}

// AggregatedCreateUser 创建用户（默认可用状态）并绑定订阅计划
//
// @Summary  创建用户（聚合 API，默认可用状态）
// @Tags     聚合API-用户管理
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    body body AggregatedCreateUserRequest true "创建用户请求"
// @Success  200 {object} map[string]interface{} "{status: success, user_id: string}"
// @Router   /users [post]
func AggregatedCreateUser(c *gin.Context) {
	var req AggregatedCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		aggregatedFail(c, "参数格式错误")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" || req.Password == "" || req.Email == "" {
		aggregatedFail(c, "username, password, email 不能为空")
		return
	}

	// 密码长度校验（与现有 User 模型 validate tag 一致）
	if len(req.Password) < 8 || len(req.Password) > 20 {
		aggregatedFail(c, "密码长度必须在 8-20 个字符之间")
		return
	}

	// 检查用户是否已存在
	exist, err := model.CheckUserExistOrDeleted(req.Username, req.Email)
	if err != nil {
		common.SysError(fmt.Sprintf("AggregatedCreateUser CheckUserExistOrDeleted error: %v", err))
		aggregatedFail(c, "数据库查询失败")
		return
	}
	if exist {
		aggregatedFail(c, "用户名或邮箱已存在")
		return
	}

	// 校验 user_validity 格式（ISO 8601 YYYY-MM-DD），无效直接拒绝，
	// 避免创建用户后才发现无效日期被静默忽略
	if req.UserValidity != "" {
		if _, err := time.Parse("2006-01-02", req.UserValidity); err != nil {
			aggregatedFail(c, fmt.Sprintf("无效的 user_validity: %s（需 YYYY-MM-DD 格式）", req.UserValidity))
			return
		}
	}

	// 创建用户（复用 model.User.Insert）
	// Issue #69: 支持通过 inviter_id 指定运营用户邀请人
	// Issue #75: 聚合 API 创建用户默认为可用状态
	myRole := c.GetInt("role")
	cleanUser := model.User{
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.Username,
		Email:       req.Email,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		InviterId:   req.InviterId,
	}
	if myRole <= common.RoleCommonUser {
		// 安全：确保不会创建比自己角色更高的用户（RoleCommonUser 已是最低）
		cleanUser.Role = common.RoleCommonUser
	}
	// 若指定了 inviter_id，验证邀请人存在并继承分组（与控制台注册逻辑一致）
	if req.InviterId > 0 {
		inviterUser, err := model.GetUserById(req.InviterId, false)
		if err != nil {
			aggregatedFail(c, fmt.Sprintf("邀请人(inviter_id=%d)不存在: %s", req.InviterId, err.Error()))
			return
		}
		if inviterUser.Group != "" {
			cleanUser.Group = inviterUser.Group
		}
	}
	if err := cleanUser.Insert(req.InviterId); err != nil {
		aggregatedFail(c, fmt.Sprintf("创建用户失败: %s", err.Error()))
		return
	}

	// Insert 后重新设置状态为可用（Insert 可能不保留我们设置的 Status）
	if err := model.DB.Model(&model.User{}).Where("id = ?", cleanUser.Id).Update("status", common.UserStatusEnabled).Error; err != nil {
		common.SysError(fmt.Sprintf("AggregatedCreateUser 设置用户可用状态失败: %v", err))
	}

	// 绑定订阅计划（如果提供了 plan_id）
	if req.PlanId > 0 {
		msg, err := model.AdminBindSubscription(cleanUser.Id, req.PlanId, "aggregated_api")
		if err != nil {
			common.SysError(fmt.Sprintf("AggregatedCreateUser AdminBindSubscription error: %v", err))
			// 用户已创建成功：返回 user_id 让调用方定位部分创建的账户
			// （无 user_id 时重试会撞"用户名或邮箱已存在"，账户沦为孤儿）
			c.JSON(http.StatusOK, gin.H{
				"status":  aggregatedStatusFail,
				"message": fmt.Sprintf("绑定订阅计划失败: %s", err.Error()),
				"user_id": strconv.Itoa(cleanUser.Id),
			})
			return
		}
		if msg != "" {
			common.SysLog(fmt.Sprintf("AggregatedCreateUser 绑定订阅提示: %s", msg))
		}
	}

	// 处理 user_validity：解析 ISO 8601 日期并记录到 remark（格式已在
	// Insert 前校验，此处解析不会失败）
	if req.UserValidity != "" {
		validityTime, _ := time.Parse("2006-01-02", req.UserValidity)
		remark := fmt.Sprintf("账户有效期至 %s", validityTime.Format("2006-01-02"))
		if err := model.DB.Model(&model.User{}).Where("id = ?", cleanUser.Id).Update("remark", remark).Error; err != nil {
			common.SysError(fmt.Sprintf("AggregatedCreateUser 更新 remark 失败: %v", err))
		}
	}

	aggregatedSuccess(c, gin.H{
		"user_id": strconv.Itoa(cleanUser.Id),
	})
}

// ============================================================
// API 2: Create New Plan — POST /api/v1/plans
// ============================================================

// Issue #63: 聚合 API 创建订阅计划的参数与逻辑必须和控制台一致。
// 支持两种请求格式：
//   1. 新格式（推荐，与控制台一致）：{"plan": { title, subtitle, price_amount, duration_unit, ... }}
//   2. 旧格式（向后兼容）：{ plan_name, token_amount, plan_validity, consume_priority, price }
// 如果同时提供了 plan 对象，以 plan 为准（新格式优先）。

type AggregatedCreatePlanRequest struct {
	// 新格式：完整 plan 对象，与控制台 AdminCreateSubscriptionPlan 一致
	Plan *model.SubscriptionPlan `json:"plan,omitempty"`

	// 旧格式（向后兼容简化字段）—— 仅用于运行时向后兼容，不对外暴露于 Swagger 文档
	PlanName        string  `json:"plan_name,omitempty" swaggerignore:"true"`
	TokenAmount     int64   `json:"token_amount,omitempty" swaggerignore:"true"`
	PlanValidity    string  `json:"plan_validity,omitempty" swaggerignore:"true"`    // monthly / none / 3_months / N_months / N_years / N_days
	ConsumePriority int     `json:"consume_priority,omitempty" swaggerignore:"true"` // 消耗优先级（映射到 sort_order）
	Price           float64 `json:"price,omitempty" swaggerignore:"true"`
}

// AggregatedCreatePlan 创建订阅计划
//
// @Summary  创建订阅计划（聚合 API）
// @Tags     聚合API-订阅管理
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    body body AggregatedCreatePlanRequest true "创建订阅计划请求（推荐使用 {plan: {...}} 格式，与控制台参数一致）"
// @Success  200 {object} map[string]interface{} "{status: success, plan_id: int}"
// @Router   /plans [post]
func AggregatedCreatePlan(c *gin.Context) {
	var req AggregatedCreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		aggregatedFail(c, "参数格式错误")
		return
	}

	var plan model.SubscriptionPlan
	if req.Plan != nil {
		// 新格式：直接使用 plan 对象（与控制台一致）
		plan = *req.Plan
	} else {
		// 旧格式：从简化字段映射到 SubscriptionPlan
		if strings.TrimSpace(req.PlanName) == "" {
			aggregatedFail(c, "plan_name 不能为空（或使用 plan 对象传入完整参数）")
			return
		}
		plan = model.SubscriptionPlan{
			Title:       strings.TrimSpace(req.PlanName),
			PriceAmount: req.Price,
			Enabled:     true,
			TotalAmount: req.TokenAmount,
			SortOrder:   req.ConsumePriority,
		}

		// 将 plan_validity 映射到内部 SubscriptionPlan 字段
		switch req.PlanValidity {
		case "monthly":
			plan.DurationUnit = model.SubscriptionDurationMonth
			plan.DurationValue = 1
			plan.QuotaResetPeriod = model.SubscriptionResetMonthly
		case "none":
			plan.DurationUnit = model.SubscriptionDurationCustom
			plan.DurationValue = 0
			plan.QuotaResetPeriod = model.SubscriptionResetNever
		case "3_months":
			plan.DurationUnit = model.SubscriptionDurationMonth
			plan.DurationValue = 3
			plan.QuotaResetPeriod = model.SubscriptionResetNever
		case "":
			// 未提供 plan_validity 时给合理默认值
			plan.DurationUnit = model.SubscriptionDurationMonth
			plan.DurationValue = 1
			plan.QuotaResetPeriod = model.SubscriptionResetNever
		default:
			// 尝试解析自定义格式（如 "6_months", "1_year" 等）
			parts := strings.SplitN(req.PlanValidity, "_", 2)
			if len(parts) == 2 {
				val, err := strconv.Atoi(parts[0])
				if err != nil || val <= 0 {
					aggregatedFail(c, fmt.Sprintf("无效的 plan_validity: %s", req.PlanValidity))
					return
				}
				switch parts[1] {
				case "month", "months":
					plan.DurationUnit = model.SubscriptionDurationMonth
					plan.DurationValue = val
				case "year", "years":
					plan.DurationUnit = model.SubscriptionDurationYear
					plan.DurationValue = val
				case "day", "days":
					plan.DurationUnit = model.SubscriptionDurationDay
					plan.DurationValue = val
				default:
					aggregatedFail(c, fmt.Sprintf("无效的 plan_validity 单位: %s", parts[1]))
					return
				}
				plan.QuotaResetPeriod = model.SubscriptionResetNever
			} else {
				aggregatedFail(c, fmt.Sprintf("无效的 plan_validity: %s", req.PlanValidity))
				return
			}
		}
	}

	// Issue #63: 以下验证逻辑与控制台 AdminCreateSubscriptionPlan 保持完全一致
	plan.Id = 0
	if strings.TrimSpace(plan.Title) == "" {
		aggregatedFail(c, "套餐标题不能为空")
		return
	}
	if plan.PriceAmount < 0 {
		aggregatedFail(c, "价格不能为负数")
		return
	}
	if plan.PriceAmount > 9999 {
		aggregatedFail(c, "价格不能超过9999")
		return
	}
	if plan.Currency == "" {
		plan.Currency = "USD"
	}
	// 与控制台一致：强制使用 USD
	plan.Currency = "USD"
	if plan.DurationUnit == "" {
		plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != model.SubscriptionDurationCustom {
		plan.DurationValue = 1
	}
	if plan.MaxPurchasePerUser < 0 {
		aggregatedFail(c, "购买上限不能为负数")
		return
	}
	if plan.TotalAmount < 0 {
		aggregatedFail(c, "总额度不能为负数")
		return
	}
	plan.UpgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
	if plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[plan.UpgradeGroup]; !ok {
			aggregatedFail(c, "升级分组不存在")
			return
		}
	}
	plan.QuotaResetPeriod = model.NormalizeResetPeriod(plan.QuotaResetPeriod)
	if plan.QuotaResetPeriod == model.SubscriptionResetCustom && plan.QuotaResetCustomSeconds <= 0 {
		aggregatedFail(c, "自定义重置周期需大于0秒")
		return
	}

	if err := model.DB.Create(&plan).Error; err != nil {
		aggregatedFail(c, fmt.Sprintf("创建订阅计划失败: %s", err.Error()))
		return
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)

	aggregatedSuccess(c, gin.H{
		"plan_id": plan.Id,
	})
}

// ============================================================
// API 3: Suspend Service — POST /api/v1/users/{user_id}/suspend
// ============================================================

// AggregatedSuspendUser 禁用用户
//
// @Summary  禁用用户（聚合 API）
// @Tags     聚合API-用户管理
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    user_id path int true "用户 ID"
// @Success  200 {object} map[string]interface{} "{status: success, status_code: 2000}"
// @Router   /users/{user_id}/suspend [post]
func AggregatedSuspendUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		aggregatedFail(c, "无效的 user_id")
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		aggregatedFail(c, "用户不存在")
		return
	}

	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		aggregatedFail(c, "无权操作同级或更高级用户")
		return
	}
	if user.Role == common.RoleRootUser {
		aggregatedFail(c, "不能禁用超级管理员")
		return
	}

	user.Status = common.UserStatusDisabled
	if err := user.Update(false); err != nil {
		aggregatedFail(c, fmt.Sprintf("禁用用户失败: %s", err.Error()))
		return
	}

	aggregatedSuccess(c, gin.H{
		"status_code": aggregatedStatusCodeOK,
	})
}

// ============================================================
// API 4: Reactivate Service — POST /api/v1/users/{user_id}/reactivate
// ============================================================

// AggregatedReactivateUser 启用用户
//
// @Summary  启用用户（聚合 API）
// @Tags     聚合API-用户管理
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    user_id path int true "用户 ID"
// @Success  200 {object} map[string]interface{} "{status: success, status_code: 2000}"
// @Router   /users/{user_id}/reactivate [post]
func AggregatedReactivateUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		aggregatedFail(c, "无效的 user_id")
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		aggregatedFail(c, "用户不存在")
		return
	}

	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		aggregatedFail(c, "无权操作同级或更高级用户")
		return
	}

	user.Status = common.UserStatusEnabled
	if err := user.Update(false); err != nil {
		aggregatedFail(c, fmt.Sprintf("启用用户失败: %s", err.Error()))
		return
	}

	aggregatedSuccess(c, gin.H{
		"status_code": aggregatedStatusCodeOK,
	})
}

// ============================================================
// API 5: Reset Password — POST /api/v1/users/{user_id}/reset-password
// ============================================================

// AggregatedResetUserPassword 发送重置密码邮件
//
// @Summary  发送重置密码邮件（聚合 API）
// @Tags     聚合API-用户管理
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    user_id path int true "用户 ID"
// @Success  200 {object} map[string]interface{} "{status: success, status_code: 2000}"
// @Router   /users/{user_id}/reset-password [post]
func AggregatedResetUserPassword(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		aggregatedFail(c, "无效的 user_id")
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		aggregatedFail(c, "用户不存在")
		return
	}

	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		aggregatedFail(c, "无权操作同级或更高级用户")
		return
	}

	// Issue #62 / 需求变更 #61: 不直接重置密码，而是发送重置密码邮件
	// 复用现有 SendPasswordResetEmail 的逻辑
	if user.Email == "" {
		aggregatedFail(c, "用户未设置邮箱，无法发送重置密码邮件")
		return
	}

	code := common.GenerateVerificationCode(0)
	common.RegisterVerificationCodeWithKey(user.Email, code, common.PasswordResetPurpose)
	link := fmt.Sprintf("%s/user/reset?email=%s&token=%s",
		system_setting.ServerAddress,
		url.QueryEscape(user.Email), url.QueryEscape(code))
	// 链接用于 href 属性与可见文本：HTML 转义防止 &、引号等破坏属性或注入标签。
	// 模板以 {{.Link}} 原样插入，故传入已转义链接。
	escapedLink := html.EscapeString(link)
	subject, content := common.RenderPasswordResetEmail(common.SystemName, escapedLink, common.VerificationValidMinutes)
	if err := common.SendEmail(subject, user.Email, content); err != nil {
		common.SysError(fmt.Sprintf("AggregatedResetUserPassword 发送重置密码邮件失败: %v", err))
		aggregatedFail(c, fmt.Sprintf("发送重置密码邮件失败: %s", err.Error()))
		return
	}

	aggregatedSuccess(c, gin.H{
		"status_code": aggregatedStatusCodeOK,
	})
}

// ============================================================
// API 6: Adjust User Quota — POST /api/v2/users/{user_id}/adjust-quota
// ============================================================

// AggregatedAdjustQuotaRequest 调整用户额度请求（增值套餐）
type AggregatedAdjustQuotaRequest struct {
	AddedQuota int `json:"added_quota"` // 额度变化量，正数为增加额度，负数为扣减额度
}

// AggregatedAdjustQuota 调整用户额度（与用户管理中的添加额度一致，支持正数和负数）
//
// @Summary  调整用户额度（聚合 API-额度调整）
// @Tags     聚合API-额度调整(增值套餐)
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    user_id path int true "用户 ID"
// @Param    body body AggregatedAdjustQuotaRequest true "额度调整请求（added_quota 为正数增加额度，负数扣减额度）"
// @Success  200 {object} map[string]interface{} "{status: success, status_code: 2000, current_quota: int, current_price: float64, total_quota: int, total_price: float64}"
// @Router   /users/{user_id}/adjust-quota [post]
func AggregatedAdjustQuota(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		aggregatedFail(c, "无效的 user_id")
		return
	}

	var req AggregatedAdjustQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		aggregatedFail(c, "参数格式错误")
		return
	}

	if req.AddedQuota == 0 {
		aggregatedFail(c, "added_quota 不能为 0")
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		aggregatedFail(c, "用户不存在")
		return
	}

	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		aggregatedFail(c, "无权操作同级或更高级用户")
		return
	}

	// 充值前金额（原生额度对应的 USD 金额）。
	// QuotaPerUnit 为可配置项（strconv.ParseFloat 丢弃错误），为 0/NaN/±Inf 时
	// decimal.Div/NewFromFloat 会 panic；退化为 1 仅影响展示金额，不影响计费。
	quotaPerUnit := common.QuotaPerUnit
	if quotaPerUnit <= 0 || math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) {
		quotaPerUnit = 1
	}
	dQuotaPerUnit := decimal.NewFromFloat(quotaPerUnit)
	currentPrice := decimal.NewFromInt(int64(user.Quota)).Div(dQuotaPerUnit).InexactFloat64()

	// 强制同步写库（db=true），跳过 BatchUpdate 异步入队。
	// 聚合 API 需在响应中立即返回充值后总额，若走批处理会延迟落库导致 total_* 不准。
	if req.AddedQuota > 0 {
		if err := model.IncreaseUserQuota(user.Id, req.AddedQuota, true); err != nil {
			common.SysError(fmt.Sprintf("AggregatedAdjustQuota IncreaseUserQuota error: %v", err))
			aggregatedFail(c, fmt.Sprintf("调整额度失败: %s", err.Error()))
			return
		}
	} else {
		if err := model.DecreaseUserQuota(user.Id, -req.AddedQuota, true); err != nil {
			common.SysError(fmt.Sprintf("AggregatedAdjustQuota DecreaseUserQuota error: %v", err))
			aggregatedFail(c, fmt.Sprintf("调整额度失败: %s", err.Error()))
			return
		}
	}

	// 重新查询最新额度
	updatedUser, err := model.GetUserById(userId, false)
	if err != nil {
		aggregatedFail(c, fmt.Sprintf("调整额度成功但查询最新额度失败: %s", err.Error()))
		return
	}

	// 充值后金额（充值后原生额度对应的 USD 金额）
	totalPrice := decimal.NewFromInt(int64(updatedUser.Quota)).Div(dQuotaPerUnit).InexactFloat64()

	// 记录日志（与 UpdateUser 中额度变更日志格式一致）
	model.RecordLog(user.Id, model.LogTypeManage,
		fmt.Sprintf("聚合API将用户额度从 %s修改为 %s", logger.LogQuota(user.Quota), logger.LogQuota(updatedUser.Quota)))

	aggregatedSuccess(c, gin.H{
		"status_code":   aggregatedStatusCodeOK,
		"current_quota": user.Quota,        // 充值前额度
		"current_price": currentPrice,      // 充值前金额
		"total_quota":   updatedUser.Quota, // 充值后总额度
		"total_price":   totalPrice,        // 充值后总金额
	})
}

// ============================================================
// API 7: Get User Status — GET /api/v2/users/{user_id}/status
// ============================================================

// userStatusRoleToString 将角色常量转换为可读字符串。
func userStatusRoleToString(role int) string {
	switch role {
	case common.RoleRootUser:
		return "root"
	case common.RoleAdminUser:
		return "admin"
	case common.RoleOpsUser:
		return "ops"
	case common.RoleCommonUser:
		return "common"
	case common.RoleGuestUser:
		return "guest"
	default:
		return "unknown"
	}
}

// userStatusToString 将用户状态常量转换为 enabled/disabled 字符串。
func userStatusToString(status int) string {
	if status == common.UserStatusDisabled {
		return "disabled"
	}
	return "enabled"
}

// AggregatedUserStatusPlanItem 订阅计划条目
type AggregatedUserStatusPlanItem struct {
	Id                 int    `json:"id" example:"8"`
	Status             string `json:"status" example:"active"`
	ValidityStartAt    string `json:"validity_start_at"`
	ValidityEndAt      string `json:"validity_end_at"`
	PlanId             int    `json:"plan_id" example:"3"`
	PlanTitle          string `json:"plan_title" example:"3-month Top-Up plan"`
	PlanRawQuota       int64  `json:"plan_raw_quota" example:"300000000"`
	PlanRemainingQuota int64  `json:"plan_remaining_quota" example:"300000000"`
}

// AggregatedGetUserStatus 查看用户状态（聚合 API）
//
// @Summary  查看用户状态（聚合 API）
// @Description 返回用户基本信息和全部订阅计划列表，plans 按 id 降序排列
// @Tags     聚合API-用户管理
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    user_id path int true "用户 ID"
// @Success  200 {object} map[string]interface{} "{status: success, user_id, user_role, user_group, user_status, invitor_id, invitor_username, plans: [...], total_price, total_quota}"
// @Router   /users/{user_id}/status [get]
func AggregatedGetUserStatus(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		aggregatedFail(c, "无效的 user_id")
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		aggregatedFail(c, "用户不存在")
		return
	}

	// 权限检查：不能查看同级或更高级用户的敏感信息（分组/额度/邀请人/订阅）。
	// 与其他聚合 API 保持一致，防止低权限调用方读取 root/admin 的详情。
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		aggregatedFail(c, "无权操作同级或更高级用户")
		return
	}

	// 构建响应数据
	resp := gin.H{
		"user_id":     user.Id,
		"user_role":   userStatusRoleToString(user.Role),
		"user_group":  user.Group,
		"user_status": userStatusToString(user.Status),
	}

	// 邀请人信息（InviterId 字段）
	if user.InviterId > 0 {
		resp["invitor_id"] = user.InviterId
		inviter, invErr := model.GetUserById(user.InviterId, false)
		if invErr == nil && inviter != nil {
			resp["invitor_username"] = inviter.Username
		} else {
			resp["invitor_username"] = ""
		}
	} else {
		resp["invitor_id"] = 0
		resp["invitor_username"] = ""
	}

	// 查询用户全部订阅记录（active + expired + cancelled），按 id 降序排列
	subs, subErr := model.GetAllUserSubscriptionsByIdDesc(user.Id)
	if subErr != nil {
		common.SysError(fmt.Sprintf("AggregatedGetUserStatus GetAllUserSubscriptionsByIdDesc error: %v", subErr))
		aggregatedFail(c, "查询订阅记录失败")
		return
	}
	plans := make([]AggregatedUserStatusPlanItem, 0)
	for _, summary := range subs {
		if summary.Subscription == nil {
			continue
		}
		sub := summary.Subscription

		// 查询订阅计划详情
		planTitle := ""
		if sub.PlanId > 0 {
			plan, planErr := model.GetSubscriptionPlanById(sub.PlanId)
			if planErr == nil && plan != nil {
				planTitle = plan.Title
			}
		}

		// 将内部状态映射为对外状态值：
		// active -> active, cancelled -> invalidated, 其他按原值返回
		externalStatus := sub.Status
		if externalStatus == "cancelled" {
			externalStatus = "invalidated"
		}

		plans = append(plans, AggregatedUserStatusPlanItem{
			Id:                 sub.Id,
			Status:             externalStatus,
			ValidityStartAt:    time.Unix(sub.StartTime, 0).Format("2006-01-02 15:04:05"),
			ValidityEndAt:      time.Unix(sub.EndTime, 0).Format("2006-01-02 15:04:05"),
			PlanId:             sub.PlanId,
			PlanTitle:          planTitle,
			PlanRawQuota:       sub.AmountTotal,
			PlanRemainingQuota: sub.AmountTotal - sub.AmountUsed,
		})
	}
	resp["plans"] = plans

	// 当前总额度和总额
	quotaPerUnit := common.QuotaPerUnit
	if quotaPerUnit <= 0 || math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) {
		quotaPerUnit = 1
	}
	dQuotaPerUnit := decimal.NewFromFloat(quotaPerUnit)
	resp["total_price"] = decimal.NewFromInt(int64(user.Quota)).Div(dQuotaPerUnit).InexactFloat64()
	resp["total_quota"] = user.Quota

	aggregatedSuccess(c, resp)
}

// ============================================================
// API 9: Delete User — POST /api/v2/users/{user_id}/delete
// ============================================================

// AggregatedDeleteUser 删除用户（仅管理员可调用，硬删除）
//
// @Summary  删除用户（聚合 API）
// @Tags     聚合API-用户管理
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    user_id path int true "用户 ID"
// @Success  200 {object} map[string]interface{} "{status: success, status_code: 2000}"
// @Router   /users/{user_id}/delete [post]
func AggregatedDeleteUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		aggregatedFail(c, "无效的 user_id")
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		aggregatedFail(c, "用户不存在")
		return
	}

	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		aggregatedFail(c, "无权操作同级或更高级用户")
		return
	}
	if user.Role == common.RoleRootUser {
		aggregatedFail(c, "不能删除超级管理员")
		return
	}

	if err := model.HardDeleteUserById(userId); err != nil {
		common.SysError(fmt.Sprintf("AggregatedDeleteUser HardDeleteUserById error: %v", err))
		aggregatedFail(c, fmt.Sprintf("删除用户失败: %s", err.Error()))
		return
	}

	aggregatedSuccess(c, gin.H{
		"status_code": aggregatedStatusCodeOK,
	})
}

// ============================================================
// API 8: Bind Subscription to User — POST /api/v2/users/{user_id}/bind-subscription
// ============================================================

// AggregatedBindSubscriptionRequest 给用户绑定订阅的请求参数
type AggregatedBindSubscriptionRequest struct {
	PlanId int `json:"plan_id"` // 订阅计划 ID
}

// AggregatedBindSubscription 给已有用户绑定订阅计划
//
// @Summary  给用户绑定订阅（聚合 API）
// @Tags     聚合API-订阅管理
// @Security ApiKeyAuth
// @Accept   json
// @Produce  json
// @Param    user_id path int true "用户 ID"
// @Param    body body AggregatedBindSubscriptionRequest true "绑定订阅请求"
// @Success  200 {object} map[string]interface{} "{status: success, status_code: 2000}"
// @Router   /users/{user_id}/bind-subscription [post]
func AggregatedBindSubscription(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		aggregatedFail(c, "无效的 user_id")
		return
	}

	var req AggregatedBindSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		aggregatedFail(c, "参数格式错误")
		return
	}

	if req.PlanId <= 0 {
		aggregatedFail(c, "plan_id 不能为空且必须大于 0")
		return
	}

	// 验证用户存在
	user, err := model.GetUserById(userId, false)
	if err != nil {
		aggregatedFail(c, "用户不存在")
		return
	}

	// 权限检查：不能操作同级或更高级用户
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		aggregatedFail(c, "无权操作同级或更高级用户")
		return
	}

	// 验证订阅计划存在
	if _, err := model.GetSubscriptionPlanById(req.PlanId); err != nil {
		aggregatedFail(c, fmt.Sprintf("订阅计划(plan_id=%d)不存在: %s", req.PlanId, err.Error()))
		return
	}

	// 执行绑定
	msg, err := model.AdminBindSubscription(userId, req.PlanId, "aggregated_api")
	if err != nil {
		common.SysError(fmt.Sprintf("AggregatedBindSubscription AdminBindSubscription error: %v", err))
		aggregatedFail(c, fmt.Sprintf("绑定订阅失败: %s", err.Error()))
		return
	}
	if msg != "" {
		common.SysLog(fmt.Sprintf("AggregatedBindSubscription 绑定订阅提示: %s", msg))
	}

	aggregatedSuccess(c, gin.H{
		"status_code": aggregatedStatusCodeOK,
	})
}
