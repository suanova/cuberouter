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

// Package agentkit 提供一个很小的服务端工具调用循环，供账户助手复用。
//
// 它刻意不包含任何业务逻辑：工具由调用方注册，模型补全由调用方注入。
// 最重要的是「工具即答案」门禁：如果本次问答没有任何工具成功返回结果，
// 循环会把结果标记为 Refused，调用方必须据此拒绝回答，而不是把模型
// 的自由文本原样透出。这样即使模型被诱导，也无法凭记忆编造答案。
package agentkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

const (
	// EventToolCall 在单个工具执行结束后发出。
	EventToolCall = "tool_call"
	// EventToolError 在工具未知或执行失败时发出。
	EventToolError = "tool_error"

	toolExecutionTimeout = 10 * time.Second
)

var (
	ErrEmptyCompletion = errors.New("agentkit: 模型未返回任何候选")
	ErrToolLoop        = errors.New("agentkit: 工具调用轮次异常")
)

// Principal 是工具执行的可信身份，只由服务端填充，绝不来自模型参数。
type Principal struct {
	UserID int
	// IsOrganizationAdmin 为 true 表示该用户至少是一个组织的管理员。
	// 组织类工具仍会逐个组织复核，不能只依赖这个标记。
	IsOrganizationAdmin bool
}

// Tool 是一个可被模型调用的只读工具。
type Tool struct {
	Name        string
	Description string
	Parameters  any
	// Run 返回给模型的文本结果。arguments 是模型生成的 JSON 字符串。
	Run func(ctx context.Context, principal Principal, arguments string) (string, error)
}

// Registry 保存一次问答可用的工具集合。
type Registry struct {
	tools map[string]Tool
	order []string
}

func NewRegistry(tools ...Tool) *Registry {
	registry := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, tool := range tools {
		if tool.Name == "" || tool.Run == nil {
			continue
		}
		registry.tools[tool.Name] = tool
		registry.order = append(registry.order, tool.Name)
	}
	return registry
}

// Definitions 返回传给模型的 tools 定义。
func (r *Registry) Definitions() []dto.ToolCallRequest {
	if r == nil || len(r.order) == 0 {
		return nil
	}
	definitions := make([]dto.ToolCallRequest, 0, len(r.order))
	for _, name := range r.order {
		tool := r.tools[name]
		definitions = append(definitions, dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return definitions
}

// Run 执行工具。found=false 表示工具不在白名单内。
func (r *Registry) Run(ctx context.Context, principal Principal, name, arguments string) (output string, found bool, err error) {
	if r == nil {
		return "", false, errors.New("没有可用工具")
	}
	tool, ok := r.tools[name]
	if !ok {
		return "", false, fmt.Errorf("工具 %q 不在允许范围内", name)
	}
	toolCtx, cancel := context.WithTimeout(ctx, toolExecutionTimeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("工具执行异常: %v", recovered)
		}
	}()
	output, err = tool.Run(toolCtx, principal, arguments)
	return output, true, err
}

// CompleteFunc 执行一轮模型补全。调用方负责选择模型、渠道与计费账号。
type CompleteFunc func(ctx context.Context, req *dto.GeneralOpenAIRequest) (*dto.OpenAITextResponse, error)

// Event 描述循环中的可观测事件，供 SSE 与审计使用。
type Event struct {
	Type       string
	Tool       string
	Success    bool
	DurationMs int64
	// Round 从 1 开始计数。
	Round int
}

// Options 是单次问答的循环参数。
type Options struct {
	Model        string
	SystemPrompt string
	History      []dto.Message
	UserMessage  string
	MaxTokens    int
	MaxRounds    int
	MaxToolCalls int
	OnEvent      func(Event)
}

// Result 是循环输出。
type Result struct {
	Content             string
	Rounds              int
	ToolCalls           int
	SuccessfulToolCalls int
	PromptTokens        int
	CompletionTokens    int
	// Refused 为 true 表示没有任何工具成功返回结果，调用方必须拒绝回答。
	Refused bool
}

// Run 执行「模型 → 工具 → 模型」循环。
//
// 门禁规则：
//   - 只有至少一次工具成功执行，最终文本才会被接受；
//   - 工具轮数达到 MaxRounds 后强制再补全一轮且不再提供工具；
//   - 工具调用次数达到 MaxToolCalls 后停止执行，注入强制收尾提示；
//   - 未知工具或工具错误不会计入成功次数。
func Run(ctx context.Context, complete CompleteFunc, registry *Registry, principal Principal, opts Options) (Result, error) {
	result := Result{}
	if complete == nil {
		return result, ErrToolLoop
	}
	if opts.MaxRounds <= 0 {
		opts.MaxRounds = 1
	}
	if opts.MaxToolCalls < 0 {
		opts.MaxToolCalls = 0
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 512
	}
	if strings.TrimSpace(opts.UserMessage) == "" {
		return result, errors.New("用户问题为空")
	}

	messages := make([]dto.Message, 0, len(opts.History)+3)
	if opts.SystemPrompt != "" {
		messages = append(messages, dto.Message{Role: "system", Content: opts.SystemPrompt})
	}
	for _, message := range opts.History {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		if strings.TrimSpace(message.StringContent()) == "" {
			continue
		}
		messages = append(messages, dto.Message{Role: message.Role, Content: message.StringContent()})
	}
	messages = append(messages, dto.Message{Role: "user", Content: opts.UserMessage})

	definitions := registry.Definitions()
	maxTokens := uint(opts.MaxTokens)
	executed := 0
	toolCalls := 0
	forceFinal := false

	completeRound := func(allowTools bool) (*dto.OpenAITextResponse, error) {
		request := &dto.GeneralOpenAIRequest{
			Model:     opts.Model,
			Messages:  messages,
			MaxTokens: &maxTokens,
		}
		if allowTools && len(definitions) > 0 {
			request.Tools = definitions
			request.ToolChoice = "auto"
		}
		response, err := complete(ctx, request)
		if err != nil {
			return nil, err
		}
		if response == nil {
			return nil, ErrEmptyCompletion
		}
		result.PromptTokens += response.Usage.PromptTokens
		result.CompletionTokens += response.Usage.CompletionTokens
		return response, nil
	}

	for round := 1; round <= opts.MaxRounds; round++ {
		result.Rounds = round
		lastRound := round == opts.MaxRounds
		allowTools := !lastRound && !forceFinal && len(definitions) > 0 && toolCalls < opts.MaxToolCalls

		response, err := completeRound(allowTools)
		if err != nil {
			return result, err
		}
		if len(response.Choices) == 0 {
			return result, ErrEmptyCompletion
		}
		message := response.Choices[0].Message

		calls := message.ParseToolCalls()
		if len(calls) == 0 {
			content := strings.TrimSpace(message.StringContent())
			result.ToolCalls = toolCalls
			result.SuccessfulToolCalls = executed
			if executed == 0 {
				result.Refused = true
				result.Content = ""
				return result, nil
			}
			result.Content = content
			return result, nil
		}

		// 最后一轮不再执行工具：模型若仍要求调用工具，直接进入强制收尾。
		if !allowTools {
			forceFinal = true
			break
		}

		assistantContent := message.Content
		if strings.TrimSpace(message.StringContent()) == "" {
			assistantContent = ""
		}
		assistantMessage := dto.Message{Role: "assistant", Content: assistantContent}
		assistantMessage.SetToolCalls(calls)
		messages = append(messages, assistantMessage)

		budgetExhausted := false
		for _, call := range calls {
			if toolCalls >= opts.MaxToolCalls {
				budgetExhausted = true
				break
			}
			toolCalls++
			name := call.Function.Name
			startedAt := time.Now()
			output, found, runErr := registry.Run(ctx, principal, name, call.Function.Arguments)
			success := found && runErr == nil
			if success {
				executed++
			} else {
				if runErr != nil {
					output = "工具执行失败，无法提供该部分数据：" + runErr.Error()
				} else {
					output = "工具不可用，无法提供该部分数据。"
				}
			}
			messages = append(messages, dto.Message{
				Role:       "tool",
				ToolCallId: call.ID,
				Content:    output,
			})
			if opts.OnEvent != nil {
				eventType := EventToolCall
				if !success {
					eventType = EventToolError
				}
				opts.OnEvent(Event{
					Type:       eventType,
					Tool:       name,
					Success:    success,
					DurationMs: time.Since(startedAt).Milliseconds(),
					Round:      round,
				})
			}
		}
		result.ToolCalls = toolCalls
		if budgetExhausted {
			forceFinal = true
			break
		}
	}
	result.ToolCalls = toolCalls
	result.SuccessfulToolCalls = executed

	// 强制收尾：不再提供工具，要求模型只依据已返回的工具结果作答。
	messages = append(messages, dto.Message{
		Role: "system",
		Content: "工具调用已结束。请只依据以上工具返回的数据回答用户；" +
			"如果数据不足，请明确说明无法回答，不要编造。",
	})
	response, err := completeRound(false)
	if err != nil {
		return result, err
	}
	if len(response.Choices) == 0 {
		return result, ErrEmptyCompletion
	}
	result.Rounds++
	if executed == 0 {
		result.Refused = true
		result.Content = ""
		return result, nil
	}
	result.Content = strings.TrimSpace(response.Choices[0].Message.StringContent())
	return result, nil
}

// MarshalToolResult 是工具返回值的统一序列化方式。
func MarshalToolResult(value any) string {
	data, err := common.Marshal(value)
	if err != nil {
		return `{"error":"结果序列化失败"}`
	}
	return string(data)
}
