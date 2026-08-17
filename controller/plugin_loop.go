package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	maxPluginsPerMessage = 3
	maxToolRounds        = 10
	toolNameSeparator    = "__"
	// toolRoundLimitNote is the spec'd system note appended to the working
	// message list when the loop cap is reached.
	toolRoundLimitNote = "tool round limit reached"
)

// PluginLoopEvent is a process hint streamed to the browser while the plugin
// loop runs: interim assistant text from a round, or a completed MCP tool
// call. The playground renders these as muted lines above the final answer.
type PluginLoopEvent struct {
	Type       string `json:"type"`             // "interim" | "tool_call"
	Plugin     string `json:"plugin,omitempty"` // plugin slug (tool_call only)
	Tool       string `json:"tool,omitempty"`   // tool name (tool_call only)
	Args       string `json:"args,omitempty"`   // raw tool arguments (tool_call only)
	DurationMs int64  `json:"durationMs"`       // tool execution duration (tool_call only); always present so the UI can render 0
	Text       string `json:"text,omitempty"`   // interim assistant text (interim only)
}

// relayRound performs one non-streaming relay round; a variable so tests can
// stub the full Relay pipeline.
var relayRound = invokeRelayRound

var pluginMentionPattern = regexp.MustCompile(`(?:^|[^a-zA-Z0-9_])@([a-z0-9][a-z0-9-]{1,63})`)

// ExtractPluginMentions returns deduped @slug mentions. A `@` preceded by a
// word character (emails, handles) is not a mention.
func ExtractPluginMentions(text string) []string {
	matches := pluginMentionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		slug := m[1]
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, slug)
	}
	return out
}

// ResolveMentionedPlugins maps slugs to enabled plugins, preserving mention
// order, dropping unknown slugs, capped at maxPluginsPerMessage.
func ResolveMentionedPlugins(slugs []string, enabled []*model.Plugin) []*model.Plugin {
	if len(slugs) == 0 {
		return nil
	}
	bySlug := make(map[string]*model.Plugin, len(enabled))
	for _, p := range enabled {
		bySlug[p.Slug] = p
	}
	out := make([]*model.Plugin, 0, len(slugs))
	for _, slug := range slugs {
		if p, ok := bySlug[slug]; ok {
			out = append(out, p)
			if len(out) >= maxPluginsPerMessage {
				break
			}
		}
	}
	return out
}

// InjectPluginSkill prepends the plugin's skill markdown to the system prompt.
func InjectPluginSkill(req *dto.GeneralOpenAIRequest, p *model.Plugin) {
	if p.SkillContent == "" {
		return
	}
	block := fmt.Sprintf("<plugin name=\"%s\">\n%s\n</plugin>", p.Slug, p.SkillContent)
	if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
		existing := req.Messages[0].StringContent()
		req.Messages[0].SetStringContent(block + "\n\n" + existing)
		return
	}
	req.Messages = append([]dto.Message{{Role: "system", Content: block}}, req.Messages...)
}

// InjectPluginTools appends the plugin's MCP tools as OpenAI function tools,
// namespaced as {slug}__{tool} to avoid cross-plugin collisions.
func InjectPluginTools(req *dto.GeneralOpenAIRequest, p *model.Plugin, tools []mcp.Tool) {
	for _, t := range tools {
		var params any
		if len(t.InputSchema) > 0 {
			_ = common.Unmarshal(t.InputSchema, &params)
		}
		req.Tools = append(req.Tools, dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        p.Slug + toolNameSeparator + t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
}

// SplitNamespacedToolName splits "{slug}__{tool}" back into its parts.
func SplitNamespacedToolName(name string) (slug, tool string, ok bool) {
	idx := strings.Index(name, toolNameSeparator)
	if idx <= 0 || idx+len(toolNameSeparator) >= len(name) {
		return "", "", false
	}
	return name[:idx], name[idx+len(toolNameSeparator):], true
}

func lastUserMessageText(req *dto.GeneralOpenAIRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].StringContent()
		}
	}
	return ""
}

// playgroundWithPlugins injects skills + tools, runs the loop, and streams
// plugin process events (interim text, tool calls) live as SSE chunks, then
// the final answer. SSE headers are only written on the first event or the
// final content, so failures before any event keep the JSON error response
// the playground already understands.
func playgroundWithPlugins(c *gin.Context, req *dto.GeneralOpenAIRequest, plugins []*model.Plugin) {
	for _, p := range plugins {
		InjectPluginSkill(req, p)
		tools, err := service.ListPluginTools(c.Request.Context(), p)
		if err != nil {
			common.SysLog("plugin " + p.Slug + " tools/list failed: " + err.Error())
			continue // skill still injected; tools skipped
		}
		InjectPluginTools(req, p, tools)
	}

	slugs := make([]string, 0, len(plugins))
	for _, p := range plugins {
		slugs = append(slugs, p.Slug)
	}
	common.SetContextKey(c, constant.ContextKeyPluginSlugs, strings.Join(slugs, ","))

	streamStarted := false
	streamChunk := func(delta map[string]any) {
		if !streamStarted {
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			streamStarted = true
		}
		chunk := map[string]any{
			"id":      newPluginCompletionID(c),
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []map[string]any{{
				"index":         0,
				"delta":         delta,
				"finish_reason": nil,
			}},
		}
		data, _ := common.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}

	finalResp, relayErr := runPluginLoop(c, req, plugins, func(ev PluginLoopEvent) {
		streamChunk(map[string]any{"plugin_event": ev})
	})
	if relayErr != nil {
		if streamStarted {
			streamErrorAsContent(c, streamChunk, relayErr.Error())
			return
		}
		c.JSON(relayErr.StatusCode, gin.H{"error": relayErr.ToOpenAIError()})
		return
	}
	if finalResp == nil || len(finalResp.Choices) == 0 {
		if streamStarted {
			streamErrorAsContent(c, streamChunk, "plugin loop produced no response")
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "plugin loop produced no response", "type": "server_error"}})
		return
	}

	content := finalResp.Choices[0].Message.StringContent()
	streamFinalAnswer(c, req.Model, content)
}

// streamErrorAsContent terminates a live plugin stream with the error text as
// the final assistant content, since the response status is already committed.
func streamErrorAsContent(c *gin.Context, streamChunk func(delta map[string]any), message string) {
	streamChunk(map[string]any{"content": message})
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

// streamFinalAnswer emits content as one OpenAI SSE content chunk followed by
// [DONE]. The playground stream parser (stream-utils.ts) reads
// choices[0].delta.content and terminates only on `data: [DONE]`; it does not
// require a leading role chunk, a finish_reason chunk, or a usage chunk.
func streamFinalAnswer(c *gin.Context, model string, content string) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	chunk := map[string]any{
		"id":      newPluginCompletionID(c),
		"object":  "chat.completion.chunk",
		"created": common.GetTimestamp(),
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{"role": "assistant", "content": content},
			"finish_reason": nil,
		}},
	}
	data, _ := common.Marshal(chunk)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

// newPluginCompletionID derives the completion id from the request id when one
// is available, so distinct plugin answers don't share one hardcoded id; it
// falls back to a random suffix otherwise.
func newPluginCompletionID(c *gin.Context) string {
	if requestId := c.GetString(common.RequestIdKey); requestId != "" {
		return "chatcmpl-plugin-" + requestId
	}
	return "chatcmpl-plugin-" + common.GetRandomString(16)
}

// runPluginLoop drives up to maxToolRounds non-streaming relay calls in
// recorder-backed sub-contexts (channel-test pattern). Returns the final
// response to stream to the browser. When onEvent is provided, it is invoked
// for each interim assistant text and completed MCP tool call so callers can
// stream plugin process hints live.
func runPluginLoop(c *gin.Context, req *dto.GeneralOpenAIRequest, plugins []*model.Plugin, onEvent ...func(PluginLoopEvent)) (*dto.OpenAITextResponse, *types.NewAPIError) {
	emit := func(PluginLoopEvent) {}
	if len(onEvent) > 0 && onEvent[0] != nil {
		emit = onEvent[0]
	}

	userId := c.GetInt("id")
	// Only ContextKeyUsingGroup is read here: the outer Playground request's
	// Distribute already validated this group against the user's usable groups
	// (and seeded it as "group"); the loop is only reachable from that path.
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)

	// Map slug -> mcp client; one client per plugin per request.
	clients := make(map[string]*mcp.Client, len(plugins))
	for _, p := range plugins {
		clients[p.Slug] = mcp.NewClientWithAuth(p.McpUrl, p.AuthHeader, p.AuthToken)
	}

	streamFalse := false
	req.Stream = &streamFalse
	req.StreamOptions = nil

	toolCallsExecuted := 0
	var lastResp *dto.OpenAITextResponse
	for round := 0; round < maxToolRounds; round++ {
		// The round's consume log records how many tool calls preceded it.
		common.SetContextKey(c, constant.ContextKeyPluginToolCalls, toolCallsExecuted)
		resp, relayErr := relayRound(c, req, userId, group)
		if relayErr != nil {
			if round > 0 && lastResp != nil {
				// Mid-loop failure (e.g. quota): the last response is a
				// tool-calls response with empty content, so stream the
				// relay error as the final assistant message instead.
				return errorFinalResponse(c, req.Model, lastResp, relayErr), nil
			}
			return nil, relayErr
		}
		lastResp = resp
		if len(resp.Choices) == 0 {
			return resp, nil
		}
		choice := resp.Choices[0]
		toolCalls := choice.Message.ParseToolCalls()
		if choice.FinishReason != constant.FinishReasonToolCalls || len(toolCalls) == 0 {
			return resp, nil
		}

		// Append the assistant message (with tool calls) then each tool result.
		assistantMsg := dto.Message{Role: "assistant"}
		if content := choice.Message.StringContent(); content != "" {
			assistantMsg.SetStringContent(content)
			emit(PluginLoopEvent{Type: "interim", Text: content})
		} else {
			assistantMsg.SetNullContent()
		}
		assistantMsg.SetToolCalls(toolCalls)
		req.Messages = append(req.Messages, assistantMsg)

		for _, tc := range toolCalls {
			start := time.Now()
			result := executePluginToolCall(c.Request.Context(), clients, tc)
			if slug, tool, ok := SplitNamespacedToolName(tc.Function.Name); ok {
				emit(PluginLoopEvent{
					Type:       "tool_call",
					Plugin:     slug,
					Tool:       tool,
					Args:       tc.Function.Arguments,
					DurationMs: time.Since(start).Milliseconds(),
				})
			}
			req.Messages = append(req.Messages, dto.Message{
				Role:       "tool",
				ToolCallId: tc.ID,
				Content:    result,
			})
		}
		toolCallsExecuted += len(toolCalls)
	}
	// Round cap reached: per spec, append the system note and force one final
	// text-only round (tools stripped). The last response necessarily asked for
	// tool calls, so it cannot serve as the final answer itself.
	req.Messages = append(req.Messages, dto.Message{Role: "system", Content: toolRoundLimitNote})
	finalTools := req.Tools
	req.Tools = nil
	common.SetContextKey(c, constant.ContextKeyPluginToolCalls, toolCallsExecuted)
	resp, relayErr := relayRound(c, req, userId, group)
	req.Tools = finalTools
	if relayErr == nil && len(resp.Choices) > 0 {
		return resp, nil
	}
	// Final round failed (or returned nothing): fall back to a synthesized
	// assistant message so the streamed answer is never empty.
	return noteFinalResponse(c, req.Model, lastResp), nil
}

// errorFinalResponse reshapes the last (tool-calls) response into the final
// answer streamed to the browser when a mid-loop relay round fails: the error
// text becomes the assistant message content.
func errorFinalResponse(c *gin.Context, model string, lastResp *dto.OpenAITextResponse, relayErr *types.NewAPIError) *dto.OpenAITextResponse {
	return withFinalContent(c, model, lastResp, relayErr.Error())
}

// noteFinalResponse is the loop-cap fallback when the forced final text round
// fails: the spec'd note plus any content the last response already carried.
func noteFinalResponse(c *gin.Context, model string, lastResp *dto.OpenAITextResponse) *dto.OpenAITextResponse {
	content := ""
	if lastResp != nil && len(lastResp.Choices) > 0 {
		content = lastResp.Choices[0].Message.StringContent()
	}
	if content != "" {
		content = toolRoundLimitNote + "\n\n" + content
	} else {
		content = toolRoundLimitNote
	}
	return withFinalContent(c, model, lastResp, content)
}

func withFinalContent(c *gin.Context, model string, lastResp *dto.OpenAITextResponse, content string) *dto.OpenAITextResponse {
	resp := &dto.OpenAITextResponse{
		Id:      newPluginCompletionID(c),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   model,
	}
	if lastResp != nil {
		resp.Usage = lastResp.Usage
	}
	msg := dto.Message{Role: "assistant"}
	msg.SetStringContent(content)
	resp.Choices = []dto.OpenAITextResponseChoice{{
		Message:      msg,
		FinishReason: constant.FinishReasonStop,
	}}
	return resp
}

// executePluginToolCall runs one namespaced tool call and returns the tool
// message content. Errors are returned as content so the model can recover.
func executePluginToolCall(ctx context.Context, clients map[string]*mcp.Client, tc dto.ToolCallRequest) string {
	slug, toolName, ok := SplitNamespacedToolName(tc.Function.Name)
	if !ok {
		return "error: malformed tool name"
	}
	client, ok := clients[slug]
	if !ok {
		return "error: unknown plugin " + slug
	}
	// No extra deadline here: mcp.Client.CallTool applies its own
	// callToolTimeout; a stricter wrapper would cancel slow tools early.
	var args []byte
	if tc.Function.Arguments != "" {
		args = []byte(tc.Function.Arguments)
	} else {
		args = []byte("{}")
	}
	result, err := client.CallTool(ctx, toolName, args)
	if err != nil {
		return "error: " + err.Error()
	}
	if result.IsError {
		return "error: " + result.Text
	}
	return result.Text
}

// selectPluginRoundChannel pre-selects the channel for a plugin round's
// sub-context, mirroring middleware.Distribute: Relay's internal getChannel
// only runs service.CacheGetRandomSatisfiedChannel on retries, when
// relayInfo.ChannelMeta is already set — the first attempt trusts the channel
// context keys seeded here. Without this the first (and usually only) attempt
// runs against an empty channel and fails with an empty upstream URL.
func selectPluginRoundChannel(subCtx *gin.Context, req *dto.GeneralOpenAIRequest, group string) *types.NewAPIError {
	channel, _, selErr := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:         subCtx,
		TokenGroup:  group,
		ModelName:   req.Model,
		RequestPath: "/pg/chat/completions",
		Retry:       common.GetPointer(0),
	})
	if selErr != nil {
		return types.NewError(selErr, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return types.NewError(fmt.Errorf("no available channel for model %s in group %s", req.Model, group), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	return middleware.SetupContextForSelectedChannel(subCtx, channel, req.Model)
}

// invokeRelayRound performs one full Relay pass in a throwaway gin context
// whose writer is an httptest recorder, then parses the non-stream response.
// The sub-context replicates the middleware chain the real /pg route runs:
// UserAuth (user cache + token context), then Distribute (model/group keys +
// the pre-selected channel via selectPluginRoundChannel).
func invokeRelayRound(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
	body, err := common.Marshal(req)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	w := httptest.NewRecorder()
	subCtx, _ := gin.CreateTestContext(w)
	subCtx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", io.NopCloser(bytes.NewReader(body)))
	subCtx.Request.Header.Set("Content-Type", "application/json")
	subCtx.Request.ContentLength = int64(len(body))

	bs, err := common.CreateBodyStorage(body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	subCtx.Set(common.KeyBodyStorage, bs)

	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	userCache.WriteContext(subCtx)
	subCtx.Set("id", userId)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-plugin-%s", group),
		Group:  group,
	}
	if err := middleware.SetupContextForToken(subCtx, tempToken); err != nil {
		return nil, types.NewError(err, types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
	}

	// Replicate the group key middleware.Distribute() resolves for /pg
	// requests, consumed by GenRelayInfo. ("original_model" is seeded by
	// SetupContextForSelectedChannel below, same as on the real route.)
	common.SetContextKey(subCtx, constant.ContextKeyUsingGroup, group)

	// Pre-select the channel for the first relay attempt, mirroring
	// middleware.Distribute (distributor.go). Selection errors surface with
	// ErrorCodeGetChannelFailed rather than Distribute's 503 +
	// ErrorCodeModelNotFound abort — the loop turns them into the streamed
	// assistant message instead.
	if newAPIError := selectPluginRoundChannel(subCtx, req, group); newAPIError != nil {
		return nil, newAPIError
	}

	// Propagate the plugin observability markers so each round's consume log
	// carries other.plugin_slugs / other.plugin_tool_calls (see
	// GenerateTextOtherInfo).
	if slugs := common.GetContextKeyString(c, constant.ContextKeyPluginSlugs); slugs != "" {
		common.SetContextKey(subCtx, constant.ContextKeyPluginSlugs, slugs)
	}
	if v, ok := common.GetContextKey(c, constant.ContextKeyPluginToolCalls); ok {
		common.SetContextKey(subCtx, constant.ContextKeyPluginToolCalls, v)
	}

	Relay(subCtx, types.RelayFormatOpenAI)

	respBody := w.Body.Bytes()
	var textResp dto.OpenAITextResponse
	if err := common.Unmarshal(respBody, &textResp); err != nil {
		return nil, types.NewError(fmt.Errorf("plugin loop: decode relay response: %w", err), types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	if w.Code >= 400 {
		return nil, types.NewError(fmt.Errorf("plugin loop: relay round failed with status %d: %s", w.Code, strings.TrimSpace(string(respBody))), types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	return &textResp, nil
}
