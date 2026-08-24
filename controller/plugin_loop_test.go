package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPluginMentions(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"hello @web-search please find X", []string{"web-search"}},
		{"@a1 and @b2", []string{"a1", "b2"}},
		{"@dup @dup", []string{"dup"}},
		{"email me at user@example.com", nil}, // mid-word @ not a mention
		{"no mentions", nil},
		{"@CAPS not valid", nil},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, ExtractPluginMentions(tc.text), tc.text)
	}
}

func TestResolveMentionedPluginsCap(t *testing.T) {
	enabled := []*model.Plugin{
		{Slug: "p1", Enabled: true},
		{Slug: "p2", Enabled: true},
		{Slug: "p3", Enabled: true},
		{Slug: "p4", Enabled: true},
	}
	got := ResolveMentionedPlugins([]string{"p1", "p2", "p3", "p4", "nope"}, enabled)
	require.Len(t, got, 3) // capped at 3, unknown slug dropped
	assert.Equal(t, "p1", got[0].Slug)
}

func TestInjectPluginSkill(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "hi @search"},
		},
	}
	InjectPluginSkill(req, &model.Plugin{Slug: "search", SkillContent: "Always cite sources."})
	require.Equal(t, "system", req.Messages[0].Role)
	sys := req.Messages[0].StringContent()
	assert.Contains(t, sys, `<plugin name="search">`)
	assert.Contains(t, sys, "Always cite sources.")
	assert.Contains(t, sys, "You are helpful.")

	// No system message → one is created.
	req2 := &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	InjectPluginSkill(req2, &model.Plugin{Slug: "s", SkillContent: "S"})
	require.Len(t, req2.Messages, 2)
	assert.Equal(t, "system", req2.Messages[0].Role)
}

func TestInjectPluginToolsNamespaced(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{}
	InjectPluginTools(req, &model.Plugin{Slug: "search"}, []mcp.Tool{
		{Name: "web", Description: "web search", InputSchema: json.RawMessage(`{"type":"object"}`)},
	})
	require.Len(t, req.Tools, 1)
	assert.Equal(t, "function", req.Tools[0].Type)
	assert.Equal(t, "search__web", req.Tools[0].Function.Name)
	assert.Equal(t, "web search", req.Tools[0].Function.Description)

	slug, tool, ok := SplitNamespacedToolName("search__web")
	assert.True(t, ok)
	assert.Equal(t, "search", slug)
	assert.Equal(t, "web", tool)
	_, _, ok = SplitNamespacedToolName("web")
	assert.False(t, ok)
}

// toolCallsResponse builds the response shape the loop sees when the model
// asks for tools: finish_reason=tool_calls with empty content.
func toolCallsResponse(promptTokens int) *dto.OpenAITextResponse {
	msg := dto.Message{Role: "assistant"}
	msg.SetNullContent()
	msg.SetToolCalls([]dto.ToolCallRequest{{ID: "call_1", Function: dto.FunctionRequest{Name: "search__web", Arguments: `{"query":"example"}`}}})
	resp := &dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{{
			Message:      msg,
			FinishReason: constant.FinishReasonToolCalls,
		}},
	}
	resp.Usage.PromptTokens = promptTokens
	return resp
}

// Mid-loop relay failure: the streamed final answer must carry the relay
// error text, not the stale empty tool-calls content.
func TestErrorFinalResponseContainsRelayError(t *testing.T) {
	c := newLoopTestContext()
	last := toolCallsResponse(42)
	relayErr := types.NewError(errors.New("quota exhausted mid-loop"), types.ErrorCodeBadResponse)

	final := errorFinalResponse(c, "gpt-test", last, relayErr)
	require.NotNil(t, final)
	require.Len(t, final.Choices, 1)
	assert.Equal(t, "assistant", final.Choices[0].Message.Role)
	assert.Contains(t, final.Choices[0].Message.StringContent(), "quota exhausted mid-loop")
	assert.Equal(t, constant.FinishReasonStop, final.Choices[0].FinishReason)
	assert.Equal(t, 42, final.Usage.PromptTokens)
	assert.NotEmpty(t, final.Id)
}

// Round-cap fallback: when the forced final text round fails, the streamed
// answer must still carry the spec'd note instead of empty content.
func TestNoteFinalResponseContainsNote(t *testing.T) {
	c := newLoopTestContext()
	// Typical case: last response is a tool-calls response with empty content.
	final := noteFinalResponse(c, "gpt-test", toolCallsResponse(7))
	require.Len(t, final.Choices, 1)
	assert.Contains(t, final.Choices[0].Message.StringContent(), toolRoundLimitNote)
	assert.Equal(t, 7, final.Usage.PromptTokens)

	// Any content the last response did have is preserved after the note.
	withContent := toolCallsResponse(0)
	withContent.Choices[0].Message.SetStringContent("partial answer")
	final2 := noteFinalResponse(c, "gpt-test", withContent)
	content := final2.Choices[0].Message.StringContent()
	assert.Contains(t, content, toolRoundLimitNote)
	assert.Contains(t, content, "partial answer")

	// No last response at all still yields the note.
	final3 := noteFinalResponse(c, "gpt-test", nil)
	require.Len(t, final3.Choices, 1)
	assert.Contains(t, final3.Choices[0].Message.StringContent(), toolRoundLimitNote)
}

func newLoopTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	c.Set("id", 1)
	return c
}

func textResponse(content string) *dto.OpenAITextResponse {
	msg := dto.Message{Role: "assistant"}
	msg.SetStringContent(content)
	return &dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{{
			Message:      msg,
			FinishReason: constant.FinishReasonStop,
		}},
	}
}

// stubRelayRound swaps the relay round function for the duration of a test.
func stubRelayRound(t *testing.T, fn func(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError)) {
	t.Helper()
	orig := relayRound
	relayRound = fn
	t.Cleanup(func() { relayRound = orig })
}

func loopRequest() *dto.GeneralOpenAIRequest {
	return &dto.GeneralOpenAIRequest{
		Model:    "gpt-test",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
		Tools: []dto.ToolCallRequest{{
			Type:     "function",
			Function: dto.FunctionRequest{Name: "search__web"},
		}},
	}
}

// loopPlugins returns a plugin whose MCP endpoint refuses connections
// instantly, so tool execution fails fast and feeds an error tool message
// back into the loop (the model-recovery path).
func loopPlugins() []*model.Plugin {
	return []*model.Plugin{{Slug: "search", McpUrl: "http://127.0.0.1:1/mcp"}}
}

// Mid-loop relay failure (round > 0): runPluginLoop must return a final
// response whose assistant content carries the relay error text.
func TestRunPluginLoopMidLoopFailureCarriesRelayError(t *testing.T) {
	c := newLoopTestContext()
	rounds := 0
	stubRelayRound(t, func(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
		rounds++
		if rounds == 1 {
			return toolCallsResponse(5), nil
		}
		return nil, types.NewError(errors.New("quota exhausted mid-loop"), types.ErrorCodeBadResponse)
	})

	final, relayErr := runPluginLoop(c, loopRequest(), loopPlugins())
	require.Nil(t, relayErr)
	require.NotNil(t, final)
	require.Len(t, final.Choices, 1)
	assert.Equal(t, 2, rounds)
	assert.Contains(t, final.Choices[0].Message.StringContent(), "quota exhausted mid-loop")
	assert.Equal(t, constant.FinishReasonStop, final.Choices[0].FinishReason)
}

// Round 0 failure still propagates the error (request fails) — unchanged
// behavior pinned by the spec.
func TestRunPluginLoopFirstRoundFailurePropagates(t *testing.T) {
	c := newLoopTestContext()
	stubRelayRound(t, func(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
		return nil, types.NewError(errors.New("channel down"), types.ErrorCodeBadResponse)
	})

	final, relayErr := runPluginLoop(c, loopRequest(), loopPlugins())
	require.Nil(t, final)
	require.NotNil(t, relayErr)
	assert.Contains(t, relayErr.Error(), "channel down")
}

// Loop cap: the loop must append the spec'd system note and force one final
// relay round with tools stripped; that round's text answer is the final one.
func TestRunPluginLoopCapForcesFinalToolLessRound(t *testing.T) {
	c := newLoopTestContext()
	var roundTools [][]dto.ToolCallRequest
	var finalRoundMessages []dto.Message
	stubRelayRound(t, func(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
		roundTools = append(roundTools, req.Tools)
		if req.Tools == nil {
			finalRoundMessages = append([]dto.Message(nil), req.Messages...)
			return textResponse("final answer"), nil
		}
		return toolCallsResponse(1), nil
	})

	final, relayErr := runPluginLoop(c, loopRequest(), loopPlugins())
	require.Nil(t, relayErr)
	require.NotNil(t, final)
	require.Len(t, roundTools, maxToolRounds+1)
	for i := 0; i < maxToolRounds; i++ {
		assert.NotEmpty(t, roundTools[i], "round %d should carry the injected tools", i)
	}
	assert.Nil(t, roundTools[maxToolRounds], "forced final round must strip tools")

	require.NotEmpty(t, finalRoundMessages)
	note := finalRoundMessages[len(finalRoundMessages)-1]
	assert.Equal(t, "system", note.Role)
	assert.Equal(t, toolRoundLimitNote, note.StringContent())

	require.Len(t, final.Choices, 1)
	assert.Equal(t, "final answer", final.Choices[0].Message.StringContent())
}

// Loop cap + the forced final round errors: fall back to a synthesized
// assistant message so the streamed answer is never empty.
func TestRunPluginLoopCapFinalRoundErrorFallsBack(t *testing.T) {
	c := newLoopTestContext()
	stubRelayRound(t, func(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
		if req.Tools == nil {
			return nil, types.NewError(errors.New("final round boom"), types.ErrorCodeBadResponse)
		}
		return toolCallsResponse(1), nil
	})

	final, relayErr := runPluginLoop(c, loopRequest(), loopPlugins())
	require.Nil(t, relayErr)
	require.NotNil(t, final)
	require.Len(t, final.Choices, 1)
	assert.Contains(t, final.Choices[0].Message.StringContent(), toolRoundLimitNote)
}

// Observability: the tool-call count context key must reflect the number of
// tool calls executed before each round, so that round's consume log records
// the current value.
func TestRunPluginLoopUpdatesToolCallCountPerRound(t *testing.T) {
	c := newLoopTestContext()
	var counts []int
	rounds := 0
	stubRelayRound(t, func(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
		counts = append(counts, common.GetContextKeyInt(c, constant.ContextKeyPluginToolCalls))
		rounds++
		if rounds < 3 {
			return toolCallsResponse(1), nil
		}
		return textResponse("done"), nil
	})

	final, relayErr := runPluginLoop(c, loopRequest(), loopPlugins())
	require.Nil(t, relayErr)
	require.NotNil(t, final)
	assert.Equal(t, []int{0, 1, 2}, counts)
}

func TestNewPluginCompletionID(t *testing.T) {
	c := newLoopTestContext()
	c.Set(common.RequestIdKey, "req-123")
	assert.Equal(t, "chatcmpl-plugin-req-123", newPluginCompletionID(c))

	c2 := newLoopTestContext()
	fallback := newPluginCompletionID(c2)
	assert.True(t, strings.HasPrefix(fallback, "chatcmpl-plugin-"))
	assert.NotEqual(t, "chatcmpl-plugin-", fallback)
}

// Process hints: the loop must emit an interim event for the round's visible
// assistant text and a tool_call event (slug, tool, args, duration) for each
// executed MCP tool, so the playground can stream them live.
func TestRunPluginLoopEmitsProcessEvents(t *testing.T) {
	c := newLoopTestContext()
	rounds := 0
	stubRelayRound(t, func(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
		rounds++
		if rounds == 1 {
			resp := toolCallsResponse(5)
			resp.Choices[0].Message.SetStringContent("Let me search for that.")
			return resp, nil
		}
		return textResponse("found it"), nil
	})

	var events []PluginLoopEvent
	final, relayErr := runPluginLoop(c, loopRequest(), loopPlugins(), func(ev PluginLoopEvent) {
		events = append(events, ev)
	})
	require.Nil(t, relayErr)
	require.NotNil(t, final)
	assert.Equal(t, "found it", final.Choices[0].Message.StringContent())

	require.Len(t, events, 2)
	assert.Equal(t, "interim", events[0].Type)
	assert.Equal(t, "Let me search for that.", events[0].Text)

	assert.Equal(t, "tool_call", events[1].Type)
	assert.Equal(t, "search", events[1].Plugin)
	assert.Equal(t, "web", events[1].Tool)
	assert.Equal(t, `{"query":"example"}`, events[1].Args)
	assert.GreaterOrEqual(t, events[1].DurationMs, int64(0))
}

// playgroundWithPlugins must stream plugin process events live as SSE chunks
// (with text/event-stream headers) followed by the final answer and [DONE].
func TestPlaygroundWithPluginsStreamsProcessEventsAndAnswer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	c.Set("id", 1)
	rounds := 0
	stubRelayRound(t, func(c *gin.Context, req *dto.GeneralOpenAIRequest, userId int, group string) (*dto.OpenAITextResponse, *types.NewAPIError) {
		rounds++
		if rounds == 1 {
			resp := toolCallsResponse(5)
			resp.Choices[0].Message.SetStringContent("Let me search for that.")
			return resp, nil
		}
		return textResponse("answer text"), nil
	})

	playgroundWithPlugins(c, loopRequest(), loopPlugins())

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	body := w.Body.String()
	assert.Contains(t, body, `"plugin_event"`)
	assert.Contains(t, body, `"type":"interim"`)
	assert.Contains(t, body, `"type":"tool_call"`)
	assert.Contains(t, body, `"content":"answer text"`)
	assert.True(t, strings.HasSuffix(body, "data: [DONE]\n\n"))

	// The tool_call event must always carry durationMs on the wire, including
	// when the tool finished in under a millisecond (omitempty would drop it).
	var toolEvent map[string]any
	found := false
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					PluginEvent json.RawMessage `json:"plugin_event"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 || len(chunk.Choices[0].Delta.PluginEvent) == 0 {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal(chunk.Choices[0].Delta.PluginEvent, &ev); err != nil {
			continue
		}
		if ev["type"] != "tool_call" {
			continue
		}
		toolEvent = ev
		found = true
	}
	require.True(t, found, "stream must contain a tool_call plugin event")
	durationMs, ok := toolEvent["durationMs"].(float64)
	require.True(t, ok, "tool_call event must carry a numeric durationMs")
	assert.GreaterOrEqual(t, durationMs, float64(0))
}
