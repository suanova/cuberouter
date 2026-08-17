package controller

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// ============================================================
// Issue #61 — 5 个聚合 API（对外第三方收敛接口）
//
// 对外暴露的简化接口，封装内部用户管理、订阅计划、用户启停和
// 密码重置能力。鉴权通过 AdminAuth（access token 或 session）。
// 响应格式与内部 API 不同，遵循 Issue 中定义的 {status, ...} 格式。
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
}

// AggregatedCreateUser 创建用户（默认禁用状态）并绑定订阅计划
//
// @Summary  创建用户（聚合 API，默认禁用状态）
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
	// Issue #62 / 需求变更 #61: 注册后用户默认为禁用状态，需管理员启用
	myRole := c.GetInt("role")
	cleanUser := model.User{
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.Username,
		Email:       req.Email,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusDisabled,
	}
	if myRole <= common.RoleCommonUser {
		// 安全：确保不会创建比自己角色更高的用户（RoleCommonUser 已是最低）
		cleanUser.Role = common.RoleCommonUser
	}
	if err := cleanUser.Insert(0); err != nil {
		aggregatedFail(c, fmt.Sprintf("创建用户失败: %s", err.Error()))
		return
	}

	// Insert 后重新设置状态为禁用（Insert 可能不保留我们设置的 Status）
	if err := model.DB.Model(&model.User{}).Where("id = ?", cleanUser.Id).Update("status", common.UserStatusDisabled).Error; err != nil {
		common.SysError(fmt.Sprintf("AggregatedCreateUser 设置用户禁用状态失败: %v", err))
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
	subject := common.WrapBilingualSubject(
		fmt.Sprintf("%s Password Reset", common.SystemName),
		fmt.Sprintf("%s密码重置", common.SystemName),
	)
	// 链接用于 href 属性与可见文本：HTML 转义防止 &、引号等破坏属性或注入标签
	escapedLink := html.EscapeString(link)
	zhContent := fmt.Sprintf("<p>您好，你正在进行%s密码重置。</p>"+
		"<p>点击 <a href='%s'>此处</a> 进行密码重置。</p>"+
		"<p>如果链接无法点击，请尝试点击下面的链接或将其复制到浏览器中打开：<br> %s </p>"+
		"<p>重置链接 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, escapedLink, escapedLink, common.VerificationValidMinutes)
	enContent := fmt.Sprintf("<p>Hello, you are resetting your password for %s.</p>"+
		"<p>Click <a href='%s'>here</a> to reset your password.</p>"+
		"<p>If the link does not work, please copy and paste the following URL into your browser:<br> %s </p>"+
		"<p>This reset link is valid for %d minutes. If you did not request this, please ignore this email.</p>", common.SystemName, escapedLink, escapedLink, common.VerificationValidMinutes)
	content := common.WrapBilingualContent(enContent, zhContent)
	if err := common.SendEmail(subject, user.Email, content); err != nil {
		common.SysError(fmt.Sprintf("AggregatedResetUserPassword 发送重置密码邮件失败: %v", err))
		aggregatedFail(c, fmt.Sprintf("发送重置密码邮件失败: %s", err.Error()))
		return
	}

	aggregatedSuccess(c, gin.H{
		"status_code": aggregatedStatusCodeOK,
	})
}
