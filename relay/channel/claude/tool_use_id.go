package claude

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
)

// normalizeToolUseIDsPayload replaces tool_use ids that an upstream derived
// from the tool name and repeats on every turn: Aliyun's native Anthropic
// endpoint returns "toolu_Bash_0" each turn and 星图 returns "Bash_0". Claude
// clients require tool_use ids to be unique across the whole conversation; on a
// repeat they drop the block, and a message left with no content surfaces to the
// user as "[Tool use interrupted]".
//
// The payload is decoded into a generic map so upstream fields this host does
// not model survive the round trip, and the parsed response is updated in step
// so both views of the same event agree. Payloads needing no rewrite are
// returned byte-for-byte unchanged. Server-side tool blocks keep their
// upstream ids, which the provider references across turns.
func normalizeToolUseIDsPayload(data string, response *dto.ClaudeResponse, token string) string {
	var payload map[string]any
	if err := common.UnmarshalJsonStr(data, &payload); err != nil {
		return data
	}

	changed := false
	normalizeBlock := func(block map[string]any, ordinal int) (string, bool) {
		if block["type"] != "tool_use" {
			return "", false
		}
		id, _ := block["id"].(string)
		name, _ := block["name"].(string)
		normalized := kitutil.NormalizeToolUseID(id, name, token, ordinal)
		if normalized == id {
			return "", false
		}
		block["id"] = normalized
		changed = true
		return normalized, true
	}

	if block, ok := payload["content_block"].(map[string]any); ok {
		ordinal := 0
		if index, ok := payload["index"].(float64); ok {
			ordinal = int(index)
		}
		if normalized, ok := normalizeBlock(block, ordinal); ok && response != nil && response.ContentBlock != nil {
			response.ContentBlock.Id = normalized
		}
	}
	if content, ok := payload["content"].([]any); ok {
		for i, item := range content {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if normalized, ok := normalizeBlock(block, i); ok && response != nil && i < len(response.Content) {
				response.Content[i].Id = normalized
			}
		}
	}
	if !changed {
		return data
	}

	encoded, err := common.Marshal(payload)
	if err != nil {
		return data
	}
	return string(encoded)
}
