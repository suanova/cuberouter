package kitutil

import (
	"fmt"
	"strings"
)

// ToolUseIDToken returns a per-response token, generating one into slot on
// first use. Response state types back a stable slot with it so that
// replacements for name-derived tool ids stay stable within one response and
// unique across responses.
func ToolUseIDToken(slot *string) string {
	if slot == nil {
		return ""
	}
	if *slot == "" {
		*slot = GetUUID()[:8]
	}
	return *slot
}

// NormalizeToolUseID returns an id that is safe to hand a Claude client.
//
// Some upstreams derive tool_use ids from the tool name plus a per-response
// ordinal instead of issuing an opaque handle: Aliyun's native Anthropic
// endpoint returns "toolu_Bash_0" on every turn, and 星图 returns "Bash_0".
// Claude clients require tool_use ids to be unique across the whole
// conversation, so a repeat makes the client drop the block -- a message left
// with no content surfaces to the user as "[Tool use interrupted]".
//
// Only name-derived ids are replaced. Provider-native ids never embed the tool
// name, so Anthropic ("toolu_01ABC..."), Bedrock ("toolu_bdrk_..."), Vertex
// ("toolu_vrtx_...") and OpenAI-compatible upstreams ("call_00_...") pass
// through untouched. Those namespaces are also excluded explicitly, so a tool
// named after one of them cannot make us rewrite that provider's ids.
//
// responseToken must be stable for the whole response and distinct between
// responses; ordinal distinguishes sibling tool calls within one response.
func NormalizeToolUseID(id, name, responseToken string, ordinal int) string {
	if name == "" {
		return id
	}
	// A tool may be named after a provider's id namespace ("call", "bdrk",
	// "vrtx"); that must not make us rewrite that provider's own ids.
	if strings.HasPrefix(id, "call_") ||
		strings.HasPrefix(id, "toolu_bdrk_") ||
		strings.HasPrefix(id, "toolu_vrtx_") {
		return id
	}
	if !strings.HasPrefix(id, name+"_") && !strings.HasPrefix(id, "toolu_"+name+"_") {
		return id
	}
	return fmt.Sprintf("toolu_%s_%d", responseToken, ordinal)
}
