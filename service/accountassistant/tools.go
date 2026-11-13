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

package accountassistant

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/agentkit"
	"github.com/QuantumNous/new-api/setting/account_assistant_setting"

	"gorm.io/gorm"
)

const (
	maxUsageDays      = 90
	maxTaskRows       = 10
	maxKeyRows        = 20
	maxFaqResults     = 3
	maxChargeOtherLen = 3000
)

type usageBucket struct {
	Key              string  `json:"key"`
	RequestCount     int     `json:"request_count"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CacheTokens      int     `json:"cache_tokens"`
	Quota            int     `json:"quota"`
	AmountUSD        float64 `json:"amount_usd"`
}

// buildRegistry 按当前用户身份注册白名单工具。普通用户永远拿不到组织工具，
// 组织工具自身还会对每个组织复核 owner/admin 身份。
func buildRegistry(principal agentkit.Principal, setting account_assistant_setting.AccountAssistantSetting) *agentkit.Registry {
	tools := []agentkit.Tool{
		newGetMyAccountTool(),
		newQueryMyUsageTool(),
		newExplainMyChargeTool(),
		newListMyKeysTool(),
		newGetMyTasksTool(),
		newSearchHelpDocsTool(setting),
	}
	if principal.IsOrganizationAdmin {
		tools = append(tools, organizationTools()...)
	}
	return agentkit.NewRegistry(tools...)
}

func newGetMyAccountTool() agentkit.Tool {
	return agentkit.Tool{
		Name:        "get_my_account",
		Description: "查询当前用户的账户信息：状态、分组、余额、累计用量与请求数。用户询问「我的余额/额度/账户状态」时使用。",
		Parameters:  objectSchema(map[string]any{}),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			user, err := model.GetUserById(principal.UserID, true)
			if err != nil {
				return "", toolError("get_my_account", err)
			}
			if user == nil {
				return "", errors.New("账户不存在")
			}
			return agentkit.MarshalToolResult(map[string]any{
				"user_id":           user.Id,
				"username":          user.Username,
				"display_name":      user.DisplayName,
				"status":            userStatusText(user.Status),
				"group":             user.Group,
				"balance_quota":     user.Quota,
				"balance_usd":       quotaToUSD(user.Quota),
				"used_quota":        user.UsedQuota,
				"used_usd":          quotaToUSD(user.UsedQuota),
				"request_count":     user.RequestCount,
				"prompt_tokens":     user.TotalPromptTokens,
				"completion_tokens": user.TotalCompletionTokens,
				"cache_tokens":      user.TotalCacheTokens,
			}), nil
		},
	}
}

func newQueryMyUsageTool() agentkit.Tool {
	return agentkit.Tool{
		Name:        "query_my_usage",
		Description: "查询当前用户最近 N 天（1-90，默认 7）的个人用量：请求数、token、消费金额，按天和按模型汇总。只含个人额度，不含组织额度。",
		Parameters: objectSchema(map[string]any{
			"days": integerProperty("查询天数，1-90，默认 7", 1, maxUsageDays),
		}),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			var args struct {
				Days int `json:"days"`
			}
			if err := parseArguments(arguments, &args); err != nil {
				return "", err
			}
			days := clampInt(args.Days, 1, maxUsageDays, 7)
			endTs := common.GetTimestamp()
			startTs := endTs - int64(days)*model.SelfUsageDaySeconds
			rows, err := model.GetSelfUsageLogRows(principal.UserID, startTs, endTs)
			if err != nil {
				return "", toolError("query_my_usage", err)
			}

			total := usageBucket{Key: "total"}
			byDay := make(map[string]*usageBucket)
			byModel := make(map[string]*usageBucket)
			for _, row := range rows {
				addUsageRow(&total, row)

				dayKey := time.Unix(row.BucketKey*model.SelfUsageDaySeconds, 0).UTC().Format("2006-01-02")
				addUsageRow(getUsageBucket(byDay, dayKey), row)

				modelName := row.ModelName
				if modelName == "" {
					modelName = "unknown"
				}
				addUsageRow(getUsageBucket(byModel, modelName), row)
			}
			total.AmountUSD = quotaToUSD(total.Quota)

			daysList := make([]*usageBucket, 0, len(byDay))
			for _, item := range byDay {
				item.AmountUSD = quotaToUSD(item.Quota)
				daysList = append(daysList, item)
			}
			sort.Slice(daysList, func(i, j int) bool { return daysList[i].Key < daysList[j].Key })

			modelsList := make([]*usageBucket, 0, len(byModel))
			for _, item := range byModel {
				item.AmountUSD = quotaToUSD(item.Quota)
				modelsList = append(modelsList, item)
			}
			sort.Slice(modelsList, func(i, j int) bool {
				if modelsList[i].Quota == modelsList[j].Quota {
					return modelsList[i].Key < modelsList[j].Key
				}
				return modelsList[i].Quota > modelsList[j].Quota
			})
			if len(modelsList) > 10 {
				modelsList = modelsList[:10]
			}

			return agentkit.MarshalToolResult(map[string]any{
				"days":       days,
				"start_date": time.Unix(startTs, 0).Format("2006-01-02"),
				"end_date":   time.Unix(endTs, 0).Format("2006-01-02"),
				"total":      total,
				"by_day":     daysList,
				"top_models": modelsList,
				"note":       "金额按平台内部额度折算为美元，仅供参考；退款与罚金会以负数或独立行体现。",
			}), nil
		},
	}
}

func newExplainMyChargeTool() agentkit.Tool {
	return agentkit.Tool{
		Name:        "explain_my_charge",
		Description: "解释当前用户的一笔具体消费。需要提供消费记录 id 或请求 request_id；返回该笔消费的模型、token、额度、计费明细（other）与日志内容。",
		Parameters: objectSchema(map[string]any{
			"log_id":     integerProperty("消费日志 id（与 request_id 二选一）", 1, math.MaxInt32),
			"request_id": stringProperty("请求 request_id（与 log_id 二选一）"),
		}),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			var args struct {
				LogID     int    `json:"log_id"`
				RequestID string `json:"request_id"`
			}
			if err := parseArguments(arguments, &args); err != nil {
				return "", err
			}
			args.RequestID = strings.TrimSpace(args.RequestID)
			if args.LogID <= 0 && args.RequestID == "" {
				return "", errors.New("请提供 log_id 或 request_id")
			}
			if model.LOG_DB == nil {
				return "", errors.New("日志库不可用")
			}

			query := model.LOG_DB.Where("user_id = ?", principal.UserID)
			if args.LogID > 0 {
				query = query.Where("id = ?", args.LogID)
			} else {
				query = query.Where("request_id = ?", args.RequestID)
			}
			var logEntry model.Log
			if err := query.Order("id desc").First(&logEntry).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return agentkit.MarshalToolResult(map[string]any{
						"found":   false,
						"message": "未找到属于当前用户的这笔消费记录，请确认 id 或 request_id。",
					}), nil
				}
				return "", toolError("explain_my_charge", err)
			}

			return agentkit.MarshalToolResult(map[string]any{
				"found":             true,
				"log_id":            logEntry.Id,
				"request_id":        logEntry.RequestId,
				"created_at":        formatUnix(logEntry.CreatedAt),
				"type":              logEntry.Type,
				"model_name":        logEntry.ModelName,
				"group":             logEntry.Group,
				"token_name":        logEntry.TokenName,
				"quota":             logEntry.Quota,
				"amount_usd":        quotaToUSD(logEntry.Quota),
				"prompt_tokens":     logEntry.PromptTokens,
				"completion_tokens": logEntry.CompletionTokens,
				"cache_tokens":      logEntry.CacheTokens,
				"use_time_seconds":  logEntry.UseTime,
				"content":           truncateRunes(logEntry.Content, 1000),
				"billing_detail":    truncateRunes(logEntry.Other, maxChargeOtherLen),
				"note":              "billing_detail 是计费时的明细快照；退款/罚金记录会以独立日志出现。",
			}), nil
		},
	}
}

func newListMyKeysTool() agentkit.Tool {
	return agentkit.Tool{
		Name:        "list_my_keys",
		Description: "列出当前用户最近的个人 API 密钥（最多 20 个），只返回掩码、状态、剩余额度、过期时间与模型限制，不返回完整 key。",
		Parameters:  objectSchema(map[string]any{}),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			tokens, err := model.GetAllUserTokens(principal.UserID, 0, maxKeyRows)
			if err != nil {
				return "", toolError("list_my_keys", err)
			}
			items := make([]map[string]any, 0, len(tokens))
			for _, token := range tokens {
				items = append(items, map[string]any{
					"name":            token.Name,
					"key_masked":      token.GetMaskedKey(),
					"status":          tokenStatusText(token.Status),
					"remain_quota":    token.RemainQuota,
					"remain_usd":      quotaToUSD(token.RemainQuota),
					"unlimited_quota": token.UnlimitedQuota,
					"used_quota":      token.UsedQuota,
					"used_usd":        quotaToUSD(token.UsedQuota),
					"expired_at":      formatUnix(token.ExpiredTime),
					"group":           token.Group,
					"model_limits":    token.ModelLimits,
				})
			}
			return agentkit.MarshalToolResult(map[string]any{
				"count": len(items),
				"items": items,
				"note":  "完整密钥无法通过助手查看；如需更换请到密钥页面重置。",
			}), nil
		},
	}
}

func newGetMyTasksTool() agentkit.Tool {
	return agentkit.Tool{
		Name:        "get_my_tasks",
		Description: "查询当前用户的图片/视频等异步任务。可按 task_id 精确查询，或按状态列出最近 10 个任务；返回状态、进度、失败原因、计费额度与提交/完成时间。",
		Parameters: objectSchema(map[string]any{
			"task_id": stringProperty("任务 id，提供时精确查询"),
			"status":  stringProperty("任务状态筛选，如 SUCCESS、FAILURE、IN_PROGRESS", "SUCCESS", "FAILURE", "IN_PROGRESS"),
		}),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			var args struct {
				TaskID string `json:"task_id"`
				Status string `json:"status"`
			}
			if err := parseArguments(arguments, &args); err != nil {
				return "", err
			}
			args.TaskID = strings.TrimSpace(args.TaskID)
			args.Status = strings.TrimSpace(args.Status)

			if args.TaskID != "" {
				task, exist, err := model.GetByTaskIdAndUser(args.TaskID, principal.UserID)
				if err != nil {
					return "", toolError("get_my_tasks", err)
				}
				if !exist || task == nil {
					return agentkit.MarshalToolResult(map[string]any{
						"found":   false,
						"message": "未找到属于当前用户的任务。",
					}), nil
				}
				return agentkit.MarshalToolResult(map[string]any{
					"found": true,
					"task":  taskSummary(task),
				}), nil
			}

			tasks := model.TaskGetAllUserTask(principal.UserID, 0, maxTaskRows, model.SyncTaskQueryParams{
				Status: args.Status,
			})
			items := make([]map[string]any, 0, len(tasks))
			for _, task := range tasks {
				items = append(items, taskSummary(task))
			}
			return agentkit.MarshalToolResult(map[string]any{
				"count": len(items),
				"items": items,
			}), nil
		},
	}
}

func newSearchHelpDocsTool(setting account_assistant_setting.AccountAssistantSetting) agentkit.Tool {
	type faqEntry struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	return agentkit.Tool{
		Name:        "search_help_docs",
		Description: "在运营方维护的帮助文档/FAQ 中检索。用户询问平台规则、充值退款、发票、限额、使用流程等非账户数据问题时使用。",
		Parameters: objectSchema(map[string]any{
			"query": stringProperty("检索关键词或用户原问题"),
		}, "query"),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			var args struct {
				Query string `json:"query"`
			}
			if err := parseArguments(arguments, &args); err != nil {
				return "", err
			}
			query := strings.TrimSpace(args.Query)
			if query == "" {
				return "", errors.New("请提供检索关键词")
			}
			var entries []faqEntry
			if err := common.UnmarshalJsonStr(setting.FAQJSON, &entries); err != nil {
				return "", errors.New("帮助文档配置格式错误，请联系管理员")
			}
			terms := strings.Fields(strings.ToLower(query))
			type scoredEntry struct {
				entry faqEntry
				score int
			}
			scored := make([]scoredEntry, 0, len(entries))
			for _, entry := range entries {
				if strings.TrimSpace(entry.Title) == "" && strings.TrimSpace(entry.Content) == "" {
					continue
				}
				title := strings.ToLower(entry.Title)
				content := strings.ToLower(entry.Content)
				score := 0
				if strings.Contains(title, strings.ToLower(query)) || strings.Contains(content, strings.ToLower(query)) {
					score += 5
				}
				for _, term := range terms {
					if strings.Contains(title, term) {
						score += 2
					}
					if strings.Contains(content, term) {
						score++
					}
				}
				if score > 0 {
					scored = append(scored, scoredEntry{entry: entry, score: score})
				}
			}
			sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
			if len(scored) > maxFaqResults {
				scored = scored[:maxFaqResults]
			}
			results := make([]map[string]any, 0, len(scored))
			for _, item := range scored {
				results = append(results, map[string]any{
					"title":   item.entry.Title,
					"content": truncateRunes(item.entry.Content, 1200),
				})
			}
			return agentkit.MarshalToolResult(map[string]any{
				"count":   len(results),
				"results": results,
				"note":    "帮助文档由运营方维护；未命中时请建议用户联系人工，不要自行推测平台规则。",
			}), nil
		},
	}
}

func taskSummary(task *model.Task) map[string]any {
	if task == nil {
		return map[string]any{}
	}
	return map[string]any{
		"task_id":     task.TaskID,
		"platform":    string(task.Platform),
		"action":      task.Action,
		"status":      string(task.Status),
		"progress":    task.Progress,
		"fail_reason": task.FailReason,
		"quota":       task.Quota,
		"amount_usd":  quotaToUSD(task.Quota),
		"model":       task.Properties.OriginModelName,
		"submit_time": formatUnix(task.SubmitTime),
		"finish_time": formatUnix(task.FinishTime),
	}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringProperty(description string, enum ...string) map[string]any {
	property := map[string]any{
		"type":        "string",
		"description": description,
	}
	if len(enum) > 0 {
		property["enum"] = enum
	}
	return property
}

func integerProperty(description string, minValue, maxValue int) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
		"minimum":     minValue,
		"maximum":     maxValue,
	}
}

func parseArguments(arguments string, target any) error {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" || arguments == "null" {
		return nil
	}
	if err := common.UnmarshalJsonStr(arguments, target); err != nil {
		return errors.New("参数格式不正确")
	}
	return nil
}

func getUsageBucket(buckets map[string]*usageBucket, key string) *usageBucket {
	if existing, ok := buckets[key]; ok {
		return existing
	}
	created := &usageBucket{Key: key}
	buckets[key] = created
	return created
}

func addUsageRow(bucket *usageBucket, row model.SelfUsageLogRow) {
	bucket.RequestCount += row.RequestCount
	bucket.PromptTokens += row.PromptTokens
	bucket.CompletionTokens += row.CompletionTokens
	bucket.CacheTokens += row.CacheTokens
	bucket.Quota += row.Quota
}

func toolError(tool string, err error) error {
	common.SysError("account assistant tool " + tool + " failed: " + err.Error())
	return errors.New("数据查询失败，请稍后重试")
}

func quotaToUSD(quota int) float64 {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	return math.Round(float64(quota)/common.QuotaPerUnit*1e6) / 1e6
}

func clampInt(value, minValue, maxValue, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func userStatusText(status int) string {
	if status == common.UserStatusEnabled {
		return "正常"
	}
	return "已禁用"
}

func tokenStatusText(status int) string {
	switch status {
	case common.TokenStatusEnabled:
		return "启用"
	case common.TokenStatusDisabled:
		return "已禁用"
	case common.TokenStatusExpired:
		return "已过期"
	case common.TokenStatusExhausted:
		return "额度耗尽"
	default:
		return fmt.Sprintf("未知(%d)", status)
	}
}

func formatUnix(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}
