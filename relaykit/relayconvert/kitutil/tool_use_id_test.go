package kitutil

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Claude clients validate tool_use ids against this shape.
var claudeToolUseIDPattern = regexp.MustCompile(`^toolu_[A-Za-z0-9_-]{1,64}$`)

func TestNormalizeToolUseIDRewritesNameDerivedIDs(t *testing.T) {
	upstreamIDs := []string{
		"toolu_Bash_0",     // Aliyun native Anthropic endpoint
		"Bash_0",           // Aliyun OpenAI-compatible endpoint
		"Bash_12_deadbeef", // 星图-style name_index_hash form
	}
	for _, upstreamID := range upstreamIDs {
		got := NormalizeToolUseID(upstreamID, "Bash", "a1b2c3d4", 0)

		assert.NotEqual(t, upstreamID, got, "name-derived id %q must be replaced", upstreamID)
		assert.Regexp(t, claudeToolUseIDPattern, got, "replacement for %q must be a valid Claude tool_use id", upstreamID)
	}
}

func TestNormalizeToolUseIDDistinguishesResponsesAndOrdinals(t *testing.T) {
	first := NormalizeToolUseID("toolu_Bash_0", "Bash", "aaaa1111", 0)
	otherResponse := NormalizeToolUseID("toolu_Bash_0", "Bash", "bbbb2222", 0)
	otherOrdinal := NormalizeToolUseID("toolu_Bash_0", "Bash", "aaaa1111", 1)

	assert.NotEqual(t, first, otherResponse, "same upstream id in two responses must not collide")
	assert.NotEqual(t, first, otherOrdinal, "two tool calls in one response must not collide")
}

func TestNormalizeToolUseIDKeepsProviderNativeIDs(t *testing.T) {
	nativeIDs := []string{
		"toolu_01ABCdefGHIjklMNOpqrSTUv",   // Anthropic
		"call_00_CUm0TepZnqOuU6ftGV4U7855", // OpenAI-compatible upstreams
		"toolu_bdrk_01ABCdefGHIjkl",        // AWS Bedrock
		"toolu_vrtx_01ABCdefGHIjkl",        // Google Vertex
	}
	for _, nativeID := range nativeIDs {
		assert.Equal(t, nativeID, NormalizeToolUseID(nativeID, "Bash", "a1b2c3d4", 0),
			"provider-native id %q must pass through untouched", nativeID)
	}
}

func TestToolUseIDTokenIsStableWithinResponseAndDistinctBetween(t *testing.T) {
	var slot string

	first := ToolUseIDToken(&slot)
	assert.NotEmpty(t, first)
	assert.Equal(t, first, ToolUseIDToken(&slot), "must stay stable for the whole response")

	var otherSlot string
	assert.NotEqual(t, first, ToolUseIDToken(&otherSlot), "must differ between responses")
}

func TestNormalizeToolUseIDKeepsProviderNamespacesWhenTheToolNameCollides(t *testing.T) {
	tests := []struct {
		name       string
		toolName   string
		upstreamID string
	}{
		{name: "openai call namespace", toolName: "call", upstreamID: "call_00_CUm0TepZnqOuU6ftGV4U7855"},
		{name: "bedrock namespace", toolName: "bdrk", upstreamID: "toolu_bdrk_01ABCdefGHIjkl"},
		{name: "vertex namespace", toolName: "vrtx", upstreamID: "toolu_vrtx_01ABCdefGHIjkl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.upstreamID, NormalizeToolUseID(tt.upstreamID, tt.toolName, "a1b2c3d4", 0),
				"a tool name that collides with a provider namespace must not rewrite that provider's ids")
		})
	}
}

func TestNormalizeToolUseIDRequiresNamePrefixMatch(t *testing.T) {
	// The tool name appearing mid-id is not the name-derived shape.
	assert.Equal(t, "toolu_MyBash_0", NormalizeToolUseID("toolu_MyBash_0", "Bash", "a1b2c3d4", 0))
	// Without a tool name there is nothing to compare against.
	assert.Equal(t, "toolu_Bash_0", NormalizeToolUseID("toolu_Bash_0", "", "a1b2c3d4", 0))
}
