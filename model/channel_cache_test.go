package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/stretchr/testify/assert"
)

// TestFilterChannelsByRequestPathAndModel 锁定渠道选择契约：只有 Advanced
// Custom（type 58）渠道按请求路径 + 模型过滤；其余渠道（包括 AstraFlow
// type 59）无论请求路径如何都无条件保留。这是 AstraFlow 多模态"一个渠道
// 同时服务 chat / responses / embeddings / image / 视频任务"的前提——
// 若 type 59 被路径过滤，非视频模型将选不到渠道。
//
// 同步上游后该行为由 filterCandidateIDs + FilterRequestPath 约束实现
// （channelMatchesFilter 对非 Advanced Custom 渠道直接放行），本测试在
// 新 API 上继续锁定同一契约。
func TestFilterChannelsByRequestPathAndModel(t *testing.T) {
	channelSyncLock.Lock()
	oldChannelsIDM := channelsIDM
	defer func() {
		channelsIDM = oldChannelsIDM
		channelSyncLock.Unlock()
	}()

	const (
		astraFlowID        = 59
		advancedChatID     = 58
		advancedResponsesID = 57
	)
	chatChannel := &Channel{Id: advancedChatID, Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled}
	chatChannel.SetOtherSettings(kitdto.ChannelOtherSettings{
		AdvancedCustom: &kitdto.AdvancedCustomConfig{
			Routes: []kitdto.AdvancedCustomRoute{
				{IncomingPath: "/v1/chat/completions", Models: []string{"deepseek-v3"}},
			},
		},
	})
	responsesChannel := &Channel{Id: advancedResponsesID, Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled}
	responsesChannel.SetOtherSettings(kitdto.ChannelOtherSettings{
		AdvancedCustom: &kitdto.AdvancedCustomConfig{
			Routes: []kitdto.AdvancedCustomRoute{
				{IncomingPath: "/v1/responses", Models: []string{"deepseek-v3"}},
			},
		},
	})
	channelsIDM = map[int]*Channel{
		astraFlowID:         {Id: astraFlowID, Type: constant.ChannelTypeAstraFlow, Status: common.ChannelStatusEnabled},
		advancedChatID:      chatChannel,
		advancedResponsesID: responsesChannel,
	}

	candidates := []int{astraFlowID, advancedChatID, advancedResponsesID}
	pathFilter := func(path string) []dto.ChannelFilter {
		return []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: path}}
	}

	// 空路径跳过过滤（既有基线行为）。
	kept, _ := filterCandidateIDs(candidates, "deepseek-v3", pathFilter(""))
	assert.ElementsMatch(t, candidates, kept)

	// chat 路径：AstraFlow 与匹配路由的 Advanced Custom 保留，不匹配的被过滤。
	kept, _ = filterCandidateIDs(candidates, "deepseek-v3", pathFilter("/v1/chat/completions"))
	assert.ElementsMatch(t, []int{astraFlowID, advancedChatID}, kept)

	// responses 路径：AstraFlow 保留，Advanced Custom 按各自路由过滤。
	kept, _ = filterCandidateIDs(candidates, "deepseek-v3", pathFilter("/v1/responses"))
	assert.ElementsMatch(t, []int{astraFlowID, advancedResponsesID}, kept)

	// 其余模式路径（embeddings / image / 视频任务）：AstraFlow 一律保留。
	for _, path := range []string{"/v1/embeddings", "/v1/images/generations", "/v1/videos/generations/tasks"} {
		kept, _ = filterCandidateIDs(candidates, "deepseek-v3", pathFilter(path))
		assert.ElementsMatch(t, []int{astraFlowID}, kept)
	}
}
