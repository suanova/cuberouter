package oairesponses

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// streamToolUseIDs converts one independent streamed response and returns the
// tool_use ids the client would receive.
func streamToolUseIDs(t *testing.T, callID string) []string {
	t.Helper()
	state := NewResponsesToClaudeStreamState("resp_1", "kimi-k3")
	events := []*dto.ResponsesStreamResponse{
		{Type: responsesEventCreated, Response: &dto.OpenAIResponsesResponse{ID: "resp_1", Model: "kimi-k3"}},
		{
			Type:        responsesEventOutputItemAdded,
			OutputIndex: kitutil.GetPointer(0),
			ItemID:      "fc_1",
			Item: &dto.ResponsesOutput{
				Type:   responsesOutputTypeFunctionCall,
				ID:     "fc_1",
				CallId: callID,
				Name:   "Bash",
			},
		},
	}

	var ids []string
	for _, event := range events {
		responses, _, err := state.ConvertChunk(event, 0)
		require.NoError(t, err)
		for _, response := range responses {
			if response.Type == "content_block_start" && response.ContentBlock != nil && response.ContentBlock.Type == "tool_use" {
				ids = append(ids, response.ContentBlock.Id)
			}
		}
	}
	return ids
}

func conversionFromResponses(t *testing.T, callID string) *dto.ClaudeResponse {
	t.Helper()
	response, _, err := ResponsesResponseToClaudeMessagesResponse(&dto.OpenAIResponsesResponse{
		ID:    "resp_1",
		Model: "kimi-k3",
		Output: []dto.ResponsesOutput{{
			Type:   responsesOutputTypeFunctionCall,
			ID:     "fc_1",
			CallId: callID,
			Name:   "Bash",
		}},
	})
	require.NoError(t, err)
	return response
}

func TestResponsesToClaudeStreamRewritesNameDerivedToolUseIDs(t *testing.T) {
	first := streamToolUseIDs(t, "Bash_0")
	second := streamToolUseIDs(t, "Bash_0")

	require.Len(t, first, 1)
	require.Len(t, second, 1)
	assert.NotEqual(t, "Bash_0", first[0], "name-derived upstream id must not reach the client")
	assert.NotEqual(t, first[0], second[0],
		"two responses deriving the same upstream id must not collide in one conversation")
}

func TestResponsesToClaudeStreamKeepsProviderNativeToolUseIDs(t *testing.T) {
	ids := streamToolUseIDs(t, "call_00_CUm0TepZnqOuU6ftGV4U7855")

	require.Len(t, ids, 1)
	assert.Equal(t, "call_00_CUm0TepZnqOuU6ftGV4U7855", ids[0])
}

func TestResponsesResponseToClaudeRewritesNameDerivedToolUseIDs(t *testing.T) {
	first := conversionFromResponses(t, "Bash_0")
	second := conversionFromResponses(t, "Bash_0")

	require.Len(t, first.Content, 1)
	require.Len(t, second.Content, 1)
	assert.NotEqual(t, "Bash_0", first.Content[0].Id, "name-derived upstream id must not reach the client")
	assert.NotEqual(t, first.Content[0].Id, second.Content[0].Id,
		"two responses deriving the same upstream id must not collide in one conversation")
}

func TestResponsesResponseToClaudeKeepsProviderNativeToolUseIDs(t *testing.T) {
	response := conversionFromResponses(t, "call_00_CUm0TepZnqOuU6ftGV4U7855")

	require.Len(t, response.Content, 1)
	assert.Equal(t, "call_00_CUm0TepZnqOuU6ftGV4U7855", response.Content[0].Id)
}
