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
	// toolResultRunesUpperBound 是单个工具轮次可能reply到对话里的最大文本量。
	// 工具结果在 tools.go 里都被截断（账单明细 3000 字符、日志内容 1000 字符等），
	// 用它作为预留 prompt token 时的单轮工具上界。
	toolResultRunesUpperBound = 3000
	// defaultAccountAssistantMaxTokens/MaxRounds 在估算预留额时兜底：即便运营方
	// 把这两个值配成了 0，也按默认值估算，避免预留额过低。
	defaultAccountAssistantMaxTokens = 800
	defaultAccountAssistantMaxRounds = 2
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
	// 拒绝，避免并发请求都读到同一个旧值而超额。预留覆盖输入与输出的两侧：
	// 模型每轮会重读整段对话（system prompt + 用户问题 + 历史 + 工具结果），
	// 输出每轮至多 MaxTokens，因此按轮数估算一个保守上界。
	sanitizedHistory := sanitizeHistory(history)

	var reservation tokenBudgetReservation
	var estimate int64
	if setting.DailyTokenBudget > 0 {
		estimate = estimateTokenReservation(setting, message, sanitizedHistory)
		var ok bool
		reservation, ok = reserveTokenUsage(principal.UserID, estimate, int64(setting.DailyTokenBudget))
		if !ok {
			return agentkit.Result{}, ErrTokenBudget
		}
	}

	result, err := agentkit.Run(ctx, s.complete, buildRegistry(principal, setting), principal, agentkit.Options{
		Model:        setting.Model,
		SystemPrompt: systemPrompt,
		History:      sanitizedHistory,
		UserMessage:  message,
		MaxTokens:    setting.MaxTokens,
		MaxRounds:    setting.MaxRounds,
		MaxToolCalls: setting.MaxToolCalls,
		OnEvent:      onEvent,
	})

	// 结算预留与实际用量的差额。必须把确实用掉的 token 记到与预留相同的日期
	// key 与存储后端上，因此把本次预留的对象交给结算函数；即使补全失败也要冲正
	// （actual - estimate，负数表示多退），避免失败重试绕过限额。预留只有在开启
	// 每日预算时才发生，这里只在该前提下结算。
	if reservation.key != "" {
		actual := int64(result.PromptTokens + result.CompletionTokens)
		adjustTokenUsage(reservation, actual-estimate)
	}

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

// tokenBudgetBackend 标识一次预留落在了哪个存储后端上，结算必须回到同一
// 后端：若预留走内存而结算时 Redis 恰好恢复，把差额写到 Redis 会让内存里的
// 预留悬空，反之亦然。
type tokenBudgetBackend int

const (
	tokenBudgetBackendMemory tokenBudgetBackend = iota
	tokenBudgetBackendRedis
)

// tokenBudgetReservation 记录一次预留真正落到的日期 key 与后端。结算时用它
// 对齐回同一个计数器，跨零点、一端不可用等情况都不会把差额拆到另一个桶里。
type tokenBudgetReservation struct {
	key     string
	backend tokenBudgetBackend
}

// estimateTokenReservation 估算一次问答可能消耗的模型 token 上界，覆盖输入与输出：
// 每轮模型会重读 system prompt + 用户问题 + 历史 + 截至当下的全部工具结果，并
// 输出至多 MaxTokens。用「输入内容 rune 数」作为 tokens 的保守上界（1 个 CJK
// char ≈ 0.5~1 token，英文更少），再按轮数放大，得到一个不会低估的预留值。
func estimateTokenReservation(
	setting account_assistant_setting.AccountAssistantSetting,
	message string,
	history []dto.Message,
) int64 {
	maxTokens := setting.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultAccountAssistantMaxTokens
	}
	maxRounds := setting.MaxRounds
	if maxRounds <= 0 {
		maxRounds = defaultAccountAssistantMaxRounds
	}
	// 每轮都会重发的输入：system prompt + 用户问题 + 历史(已截断)。
	baseInputRunes := utf8.RuneCountInString(systemPrompt) + utf8.RuneCountInString(message)
	for _, h := range history {
		baseInputRunes += utf8.RuneCountInString(h.StringContent())
	}
	// 每个工具轮次还会把上一次的工具结果追加进对话，因此补一份工具结果上界。
	perRoundInputRunes := int64(baseInputRunes) + toolResultRunesUpperBound
	// 输出每轮至多 maxTokens，轮数 = MaxRounds + 1（最后一次强制收尾补全）。
	outputTokens := int64(maxTokens) * (int64(maxRounds) + 1)
	inputUpperBound := int64(maxRounds+1) * perRoundInputRunes
	return outputTokens + inputUpperBound
}

// reserveTokenUsage atomically sets aside an estimated token amount for a
// question before the model run starts. Each concurrent request checks the
// increment against the daily budget, so parallel questions cannot all pass
// against the same pre-run balance. On success it returns the reservation
// (date key + storage backend) so settlement can write to the same counter; a
// failed reservation rolls the increment back and reports the budget as
// exhausted.
func reserveTokenUsage(userID int, estimate int64, budget int64) (tokenBudgetReservation, bool) {
	if userID <= 0 || estimate <= 0 {
		return tokenBudgetReservation{}, true
	}
	key := tokenBudgetKey(userID)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), redisOperationLimit)
		defer cancel()
		current, err := common.RDB.IncrBy(ctx, key, estimate).Result()
		if err == nil {
			if current > budget {
				common.RDB.IncrBy(ctx, key, -estimate)
				return tokenBudgetReservation{}, false
			}
			common.RDB.Expire(ctx, key, 25*time.Hour)
			return tokenBudgetReservation{key: key, backend: tokenBudgetBackendRedis}, true
		}
		common.SysLog("account assistant redis token reserve failed: " + err.Error())
	}
	tokenBudgetMu.Lock()
	defer tokenBudgetMu.Unlock()
	projected := tokenBudgetStore[key] + estimate
	if projected > budget {
		return tokenBudgetReservation{}, false
	}
	tokenBudgetStore[key] = projected
	return tokenBudgetReservation{key: key, backend: tokenBudgetBackendMemory}, true
}

// adjustTokenUsage settles a reservation against its own counter: it adds a
// signed delta (positive = extra usage beyond the estimate, negative = refund)
// to the exact same date key and backend that received the reservation, so a
// chat crossing midnight or a Redis availability change cannot split a
// reservation across two counters.
func adjustTokenUsage(reservation tokenBudgetReservation, delta int64) {
	if reservation.key == "" {
		return
	}
	switch reservation.backend {
	case tokenBudgetBackendRedis:
		if common.RedisEnabled && common.RDB != nil {
			ctx, cancel := context.WithTimeout(context.Background(), redisOperationLimit)
			defer cancel()
			if err := common.RDB.IncrBy(ctx, reservation.key, delta).Err(); err == nil {
				common.RDB.Expire(ctx, reservation.key, 25*time.Hour)
			} else {
				common.SysLog("account assistant redis token budget failed: " + err.Error())
			}
		}
	case tokenBudgetBackendMemory:
		tokenBudgetMu.Lock()
		tokenBudgetStore[reservation.key] += delta
		tokenBudgetMu.Unlock()
	}
}

// addTokenUsage records real token usage directly against the user's daily
// counter on whichever backend is currently active. It does not check a budget —
// it is a passive metering helper (retained for tests and diagnostics).
func addTokenUsage(userID, tokens int) {
	if userID <= 0 || tokens <= 0 {
		return
	}
	key := tokenBudgetKey(userID)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), redisOperationLimit)
		defer cancel()
		if err := common.RDB.IncrBy(ctx, key, int64(tokens)).Err(); err == nil {
			common.RDB.Expire(ctx, key, 25*time.Hour)
		}
		return
	}
	tokenBudgetMu.Lock()
	tokenBudgetStore[key] += int64(tokens)
	tokenBudgetMu.Unlock()
}
