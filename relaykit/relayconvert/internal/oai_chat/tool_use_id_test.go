package oaichat

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func streamToolCallChunk(upstreamID string) *dto.ChatCompletionsStreamResponse {
	index := 0
	finishReason := "tool_calls"
	return &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "kimi-k3",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index:    &index,
					ID:       upstreamID,
					Function: dto.FunctionResponse{Name: "Bash", Arguments: "{}"},
				}},
			},
			FinishReason: &finishReason,
		}},
	}
}

// streamToolUseIDs converts one independent response and returns the tool_use
// ids the client would receive.
func streamToolUseIDs(upstreamID string) []string {
	var ids []string
	for _, response := range StreamResponseOpenAI2Claude(streamToolCallChunk(upstreamID), &convmeta.Values{}) {
		if response.Type == "content_block_start" && response.ContentBlock != nil && response.ContentBlock.Type == "tool_use" {
			ids = append(ids, response.ContentBlock.Id)
		}
	}
	return ids
}

func toolCallResponse(upstreamID, toolName string) *dto.OpenAITextResponse {
	msg := dto.Message{Role: "assistant"}
	msg.SetToolCalls([]dto.ToolCallRequest{
		{ID: upstreamID, Type: "function", Function: dto.FunctionRequest{Name: toolName, Arguments: "{}"}},
	})
	return &dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Model:   "kimi-k3",
		Choices: []dto.OpenAITextResponseChoice{{Message: msg, FinishReason: "tool_calls"}},
	}
}

func TestResponseOpenAI2ClaudeRewritesNameDerivedToolUseIDs(t *testing.T) {
	first := ResponseOpenAI2Claude(toolCallResponse("Bash_0", "Bash"), nil)
	second := ResponseOpenAI2Claude(toolCallResponse("Bash_0", "Bash"), nil)

	require.Len(t, first.Content, 1)
	require.Len(t, second.Content, 1)
	assert.NotEqual(t, "Bash_0", first.Content[0].Id, "name-derived upstream id must not reach the client")
	assert.NotEqual(t, first.Content[0].Id, second.Content[0].Id,
		"two responses deriving the same upstream id must not collide in one conversation")
}

func TestStreamResponseOpenAI2ClaudeRewritesNameDerivedToolUseIDs(t *testing.T) {
	first := streamToolUseIDs("toolu_Bash_0")
	second := streamToolUseIDs("toolu_Bash_0")

	require.Len(t, first, 1)
	require.Len(t, second, 1)
	assert.NotEqual(t, "toolu_Bash_0", first[0], "name-derived upstream id must not reach the client")
	assert.NotEqual(t, first[0], second[0],
		"two responses deriving the same upstream id must not collide in one conversation")
}

func TestStreamResponseOpenAI2ClaudeKeepsProviderNativeToolUseIDs(t *testing.T) {
	ids := streamToolUseIDs("call_00_CUm0TepZnqOuU6ftGV4U7855")

	require.Len(t, ids, 1)
	assert.Equal(t, "call_00_CUm0TepZnqOuU6ftGV4U7855", ids[0])
}

func TestResponseOpenAI2ClaudeKeepsProviderNativeToolUseIDs(t *testing.T) {
	resp := ResponseOpenAI2Claude(toolCallResponse("call_00_CUm0TepZnqOuU6ftGV4U7855", "Bash"), nil)

	require.Len(t, resp.Content, 1)
	assert.Equal(t, "call_00_CUm0TepZnqOuU6ftGV4U7855", resp.Content[0].Id)
}

func TestResponseOpenAI2ClaudeDistinguishesSiblingToolUseIDs(t *testing.T) {
	msg := dto.Message{Role: "assistant"}
	msg.SetToolCalls([]dto.ToolCallRequest{
		{ID: "Bash_0", Type: "function", Function: dto.FunctionRequest{Name: "Bash", Arguments: "{}"}},
		{ID: "Bash_1", Type: "function", Function: dto.FunctionRequest{Name: "Bash", Arguments: "{}"}},
	})
	resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Model:   "kimi-k3",
		Choices: []dto.OpenAITextResponseChoice{{Message: msg, FinishReason: "tool_calls"}},
	}, nil)

	require.Len(t, resp.Content, 2)
	assert.NotEqual(t, resp.Content[0].Id, resp.Content[1].Id,
		"two tool calls in one response must not share an id")
}
