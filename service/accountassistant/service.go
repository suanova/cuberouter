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
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/agentkit"
	"github.com/QuantumNous/new-api/setting/account_assistant_setting"
)

const (
	maxMessageRunes     = 2000
	maxHistoryMessages  = 8
	maxHistoryRunes     = 2000
	redisOperationLimit = 2 * time.Second
)

var (
	// ErrDisabled 表示运营方尚未启用或尚未配置模型。调用方应拒绝服务。
	ErrDisabled = errors.New("账户助手未启用")
	// ErrEmptyMessage 表示用户问题为空。
	ErrEmptyMessage = errors.New("请输入问题")
	// ErrRateLimited 表示小时限额已用完。
	ErrRateLimited = errors.New("提问过于频繁，请稍后再试")
	// ErrDailyLimit 表示当日提问次数已用完。
	ErrDailyLimit = errors.New("今日提问次数已达上限，请联系人工")
	// ErrTokenBudget 表示当日模型 token 预算已用完。
	ErrTokenBudget = errors.New("今日助手额度已用完，请联系人工")
)

// Service 把用户问题交给模型，但只允许模型通过白名单工具回答。
type Service struct {
	complete agentkit.CompleteFunc
}

func NewService(complete agentkit.CompleteFunc) *Service {
	return &Service{complete: complete}
}

// Chat 执行一次账户助手问答。
//
// 调用方必须在完成所有校验并通过 SSE 发送开始事件后再调用，因为一旦
// 模型补全开始就会消耗运营方 token。
func (s *Service) Chat(
	ctx context.Context,
	principal agentkit.Principal,
	message string,
	history []dto.Message,
	onEvent func(agentkit.Event),
) (agentkit.Result, error) {
	setting := account_assistant_setting.GetAccountAssistantSetting()
	if !setting.Enabled || strings.TrimSpace(setting.Model) == "" {
		return agentkit.Result{}, ErrDisabled
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return agentkit.Result{}, ErrEmptyMessage
	}
	message = truncateRunes(message, maxMessageRunes)
	if principal.UserID <= 0 {
		return agentkit.Result{}, errors.New("用户身份无效")
	}

	if err := consumeQuestionQuota(principal.UserID, setting); err != nil {
		return agentkit.Result{}, err
	}

	// 预留本次问答可能消耗的最大 token 之后再启动模型补全，使并发提问的
	// 预算检查是原子的；运行结束后再按实际用量冲正差额。预留失败则直接
	// 拒绝，避免并发请求都读到同一个旧值而超额。
	var estimate int64
	if setting.DailyTokenBudget > 0 {
		estimate = int64(setting.MaxTokens) * (int64(setting.MaxRounds) + 1)
		if !reserveTokenUsage(principal.UserID, estimate, int64(setting.DailyTokenBudget)) {
			return agentkit.Result{}, ErrTokenBudget
		}
	}

	result, err := agentkit.Run(ctx, s.complete, buildRegistry(principal, setting), principal, agentkit.Options{
		Model:        setting.Model,
		SystemPrompt: systemPrompt,
		History:      sanitizeHistory(history),
		UserMessage:  message,
		MaxTokens:    setting.MaxTokens,
		MaxRounds:    setting.MaxRounds,
		MaxToolCalls: setting.MaxToolCalls,
		OnEvent:      onEvent,
	})
	// 即使补全失败，已产生的 token 也要计入预算，避免失败重试绕过限额；
	// 同时对预留做冲正（actual - estimate，负数表示多退）。
	actual := int64(result.PromptTokens + result.CompletionTokens)
	adjustTokenUsage(principal.UserID, actual-estimate)
	if err != nil {
		common.SysLog(fmt.Sprintf("account assistant chat failed (user=%d): %s", principal.UserID, err.Error()))
	}
	return result, err
}

func sanitizeHistory(history []dto.Message) []dto.Message {
	if len(history) == 0 {
		return nil
	}
	if len(history) > maxHistoryMessages {
		history = history[len(history)-maxHistoryMessages:]
	}
	sanitized := make([]dto.Message, 0, len(history))
	for _, message := range history {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(message.StringContent())
		if content == "" {
			continue
		}
		sanitized = append(sanitized, dto.Message{
			Role:    message.Role,
			Content: truncateRunes(content, maxHistoryRunes),
		})
	}
	return sanitized
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

var (
	hourlyLimiter common.InMemoryRateLimiter
	dailyLimiter  common.InMemoryRateLimiter

	tokenBudgetMu    sync.Mutex
	tokenBudgetStore = make(map[string]int64)
)

func init() {
	hourlyLimiter.Init(2 * time.Hour)
	dailyLimiter.Init(25 * time.Hour)
}

func consumeQuestionQuota(userID int, setting account_assistant_setting.AccountAssistantSetting) error {
	if !consumeRateSlot("hourly", userID, setting.HourlyLimit, time.Hour, &hourlyLimiter) {
		return ErrRateLimited
	}
	if !consumeRateSlot("daily", userID, setting.DailyLimit, 24*time.Hour, &dailyLimiter) {
		return ErrDailyLimit
	}
	return nil
}

// consumeRateSlot 优先使用 Redis 做跨实例计数；Redis 不可用时退化为
// 单实例内存滑窗，保证功能可用。
func consumeRateSlot(scope string, userID, max int, window time.Duration, limiter *common.InMemoryRateLimiter) bool {
	if max <= 0 {
		return true
	}
	if common.RedisEnabled && common.RDB != nil {
		slot := time.Now().Unix() / int64(window.Seconds())
		key := fmt.Sprintf("account_assistant:rate:%s:%d:%d", scope, userID, slot)
		ctx, cancel := context.WithTimeout(context.Background(), redisOperationLimit)
		defer cancel()
		count, err := common.RDB.Incr(ctx, key).Result()
		if err == nil {
			if count == 1 {
				common.RDB.Expire(ctx, key, window)
			}
			return count <= int64(max)
		}
		common.SysLog("account assistant redis rate limit failed: " + err.Error())
	}
	return limiter.Request(strconv.Itoa(userID), max, int64(window.Seconds()))
}

func tokenBudgetKey(userID int) string {
	return fmt.Sprintf("account_assistant:tokens:%d:%s", userID, time.Now().Format("2006-01-02"))
}

func tokenUsage(userID int) int64 {
	key := tokenBudgetKey(userID)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), redisOperationLimit)
		defer cancel()
		value, err := common.RDB.Get(ctx, key).Int64()
		if err == nil {
			return value
		}
	}
	tokenBudgetMu.Lock()
	defer tokenBudgetMu.Unlock()
	return tokenBudgetStore[key]
}

// reserveTokenUsage atomically sets aside an estimated token amount for a
// question before the model run starts. Each concurrent request checks the
// increment against the daily budget, so parallel questions cannot all pass
// against the same pre-run balance. A failed reservation rolls the increment
// back and reports the budget as exhausted.
func reserveTokenUsage(userID int, estimate int64, budget int64) bool {
	if userID <= 0 || estimate <= 0 {
		return true
	}
	key := tokenBudgetKey(userID)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), redisOperationLimit)
		defer cancel()
		current, err := common.RDB.IncrBy(ctx, key, estimate).Result()
		if err == nil {
			if current > budget {
				common.RDB.IncrBy(ctx, key, -estimate)
				return false
			}
			common.RDB.Expire(ctx, key, 25*time.Hour)
			return true
		}
		common.SysLog("account assistant redis token reserve failed: " + err.Error())
	}
	tokenBudgetMu.Lock()
	defer tokenBudgetMu.Unlock()
	projected := tokenBudgetStore[key] + estimate
	if projected > budget {
		return false
	}
	tokenBudgetStore[key] = projected
	return true
}

// adjustTokenUsage adds a signed delta (positive usage, or a negative refund
// of the part of a reservation that was not consumed) to the user's daily
// token counter. It stores the Delta of 0 safely without resetting the key.
func adjustTokenUsage(userID int, delta int64) {
	key := tokenBudgetKey(userID)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), redisOperationLimit)
		defer cancel()
		if err := common.RDB.IncrBy(ctx, key, delta).Err(); err == nil {
			common.RDB.Expire(ctx, key, 25*time.Hour)
			return
		} else {
			common.SysLog("account assistant redis token budget failed: " + err.Error())
		}
	}
	tokenBudgetMu.Lock()
	tokenBudgetStore[key] += delta
	tokenBudgetMu.Unlock()
}

func addTokenUsage(userID, tokens int) {
	if userID <= 0 || tokens <= 0 {
		return
	}
	adjustTokenUsage(userID, int64(tokens))
}
