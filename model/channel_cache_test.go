package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/stretchr/testify/assert"
)

// TestFilterChannelsByRequestPathAndModel 锁定渠道选择契约：只有 Advanced
// Custom（type 58）渠道按请求路径 + 模型过滤；其余渠道（包括 AstraFlow
// type 59）无论请求路径如何都无条件保留。这是 AstraFlow 多模态"一个渠道
// 同时服务 chat / responses / embeddings / image / 视频任务"的前提——
// 若 type 59 被路径过滤，非视频模型将选不到渠道。
func TestFilterChannelsByRequestPathAndModel(t *testing.T) {
	channelSyncLock.Lock()
	oldChannelsIDM := channelsIDM
	oldAdvancedCustom := channel2advancedCustomConfig
	defer func() {
		channelsIDM = oldChannelsIDM
		channel2advancedCustomConfig = oldAdvancedCustom
		channelSyncLock.Unlock()
	}()

	const (
		astraFlowID        = 59
		advancedChatID     = 58
		advancedResponsesID = 57
	)
	channelsIDM = map[int]*Channel{
		astraFlowID:         {Id: astraFlowID, Type: constant.ChannelTypeAstraFlow},
		advancedChatID:      {Id: advancedChatID, Type: constant.ChannelTypeAdvancedCustom},
		advancedResponsesID: {Id: advancedResponsesID, Type: constant.ChannelTypeAdvancedCustom},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{
		advancedChatID: {
			Routes: []dto.AdvancedCustomRoute{
				{IncomingPath: "/v1/chat/completions", Models: []string{"deepseek-v3"}},
			},
		},
		advancedResponsesID: {
			Routes: []dto.AdvancedCustomRoute{
				{IncomingPath: "/v1/responses", Models: []string{"deepseek-v3"}},
			},
		},
	}

	candidates := []int{astraFlowID, advancedChatID, advancedResponsesID}

	// 空路径跳过过滤（既有基线行为）。
	assert.ElementsMatch(t, candidates,
		filterChannelsByRequestPathAndModel(candidates, "", "deepseek-v3"))

	// chat 路径：AstraFlow 与匹配路由的 Advanced Custom 保留，不匹配的被过滤。
	assert.ElementsMatch(t, []int{astraFlowID, advancedChatID},
		filterChannelsByRequestPathAndModel(candidates, "/v1/chat/completions", "deepseek-v3"))

	// responses 路径：AstraFlow 保留，Advanced Custom 按各自路由过滤。
	assert.ElementsMatch(t, []int{astraFlowID, advancedResponsesID},
		filterChannelsByRequestPathAndModel(candidates, "/v1/responses", "deepseek-v3"))

	// 其余模式路径（embeddings / image / 视频任务）：AstraFlow 一律保留。
	for _, path := range []string{"/v1/embeddings", "/v1/images/generations", "/v1/videos/generations/tasks"} {
		assert.ElementsMatch(t, []int{astraFlowID},
			filterChannelsByRequestPathAndModel(candidates, path, "deepseek-v3"))
	}
}
