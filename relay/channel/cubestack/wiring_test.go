/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package cubestack_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/cubestack"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCubeStackRegistration 锁定渠道注册链: 渠道类型 → API 类型 → 适配器。
// 三处接线漏任何一处都不会编译失败,但后果都只有到运行时才看得见——渠道类型没映射
// 会静默落回 OpenAI 适配器(只剩 chat 一条路,原生 responses/messages 全丢),
// GetAdaptor 少了分支则会让 controller 启动时的 GetAdaptor(i) 循环拿到 nil 适配器
// panic(controller/model.go 无 nil 检查)。故这里直接断言注册结果。
func TestCubeStackRegistration(t *testing.T) {
	apiType, ok := common.ChannelType2APIType(constant.ChannelTypeCubeStack)
	require.True(t, ok, "channel type 64 is missing from ChannelType2APIType")
	require.Equal(t, constant.APITypeCubeStack, apiType)

	adaptor := relay.GetAdaptor(apiType)
	require.NotNil(t, adaptor, "GetAdaptor has no case for APITypeCubeStack")
	require.IsType(t, &cubestack.Adaptor{}, adaptor)
	assert.Equal(t, "CubeStack", adaptor.GetChannelName())

	// 视频侧仍由任务适配器承载(relay/channel/task/cubestack),文本适配器的模型列表
	// 必须留空,免得把视频模型也挂到文本端点下。
	assert.Empty(t, adaptor.GetModelList())
}
