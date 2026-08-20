package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAstraFlowChannelRegistration 锁定 AstraFlow 渠道（59）在三层注册表中
// 的映射契约：渠道类型 → API 类型、端点类型（OpenAI video 任务端点），
// 保证渠道创建后可被正确路由到视频任务链路。
func TestAstraFlowChannelRegistration(t *testing.T) {
	apiType, ok := ChannelType2APIType(constant.ChannelTypeAstraFlow)
	require.True(t, ok)
	assert.Equal(t, constant.APITypeAstraFlow, apiType)

	endpointTypes := GetEndpointTypesByChannelType(constant.ChannelTypeAstraFlow, "doubao-seedance-2-0-260128")
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, endpointTypes)
}
