package claude

import (
	"net/http/httptest"
	"regexp"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var toolUseIDInPayload = regexp.MustCompile(`"id":"([^"]+)"`)

// streamBlockStart builds the content_block_start event an upstream sends when
// it opens a content block.
func streamBlockStart(blockType, id, name string) string {
	return `{"type":"content_block_start","index":1,"content_block":{"type":"` + blockType +
		`","id":"` + id + `","name":"` + name + `","input":{}}}`
}

func handleStreamEvent(t *testing.T, claudeInfo *ClaudeResponseInfo, event string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	info := &relaycommon.RelayInfo{OriginModelName: "kimi-k3", RelayFormat: types.RelayFormatClaude}

	require.Nil(t, HandleStreamResponseData(c, info, claudeInfo, event))
	return w.Body.String()
}

func TestHandleStreamResponseDataRewritesNameDerivedToolUseIDs(t *testing.T) {
	body := handleStreamEvent(t, &ClaudeResponseInfo{Usage: &dto.Usage{}},
		streamBlockStart("tool_use", "toolu_Bash_0", "Bash"))

	assert.NotContains(t, body, "toolu_Bash_0", "name-derived upstream id must not reach the client")
	assert.Contains(t, body, `"name":"Bash"`, "the tool_use block itself must survive")
}

func TestHandleStreamResponseDataDistinguishesResponses(t *testing.T) {
	first := handleStreamEvent(t, &ClaudeResponseInfo{Usage: &dto.Usage{}},
		streamBlockStart("tool_use", "toolu_Bash_0", "Bash"))
	second := handleStreamEvent(t, &ClaudeResponseInfo{Usage: &dto.Usage{}},
		streamBlockStart("tool_use", "toolu_Bash_0", "Bash"))

	firstMatch := toolUseIDInPayload.FindStringSubmatch(first)
	secondMatch := toolUseIDInPayload.FindStringSubmatch(second)
	require.Len(t, firstMatch, 2)
	require.Len(t, secondMatch, 2)
	assert.NotEqual(t, firstMatch[1], secondMatch[1],
		"two responses deriving the same upstream id must not collide in one conversation")
}

func TestHandleStreamResponseDataKeepsProviderNativeToolUseIDs(t *testing.T) {
	body := handleStreamEvent(t, &ClaudeResponseInfo{Usage: &dto.Usage{}},
		streamBlockStart("tool_use", "toolu_01ABCdefGHIjklMNOpqrSTUv", "Bash"))

	assert.Contains(t, body, "toolu_01ABCdefGHIjklMNOpqrSTUv")
}

func TestHandleClaudeResponseDataRewritesNameDerivedToolUseIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	info := &relaycommon.RelayInfo{OriginModelName: "kimi-k3", RelayFormat: types.RelayFormatClaude}
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}

	payload := []byte(`{"type":"message","id":"msg_1","content":[` +
		`{"type":"text","text":"hi"},` +
		`{"type":"tool_use","id":"Bash_0","name":"Bash","input":{}}` +
		`],"usage":{"input_tokens":1,"output_tokens":1}}`)

	require.Nil(t, HandleClaudeResponseData(c, info, claudeInfo, nil, payload))

	body := w.Body.String()
	assert.NotContains(t, body, `"id":"Bash_0"`, "name-derived upstream id must not reach the client")
	assert.Contains(t, body, `"type":"tool_use"`, "the tool_use block itself must survive")
	assert.Contains(t, body, `"text":"hi"`, "sibling content must survive the rewrite")
}

func TestHandleStreamResponseDataKeepsServerToolUseIDs(t *testing.T) {
	// Server-side tools keep upstream-issued ids that the provider references
	// across turns; rewriting them would break server-tool continuation.
	// The id is deliberately name-derived so only the block type can save it.
	body := handleStreamEvent(t, &ClaudeResponseInfo{Usage: &dto.Usage{}},
		streamBlockStart("server_tool_use", "web_search_0", "web_search"))

	assert.Contains(t, body, `"id":"web_search_0"`)
}
