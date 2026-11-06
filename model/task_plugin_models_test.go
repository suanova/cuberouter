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
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaskPluginModelsByGroups 锁定 Agent 页面的模型过滤：任务插件渠道里的
// 模型名（如 dsh）只在其所属渠道出现，普通聊天模型不会被列进来；plugin
// 参数再把结果收窄到单个插件 key。
func TestTaskPluginModelsByGroups(t *testing.T) {
	truncateTables(t)
	priority := int64(0)
	weight := uint(1)
	baseURL := "https://example.com"
	dshSetting := `{"task_plugin_key":"dsh"}`
	otherSetting := `{"task_plugin_key":"generic-alpha"}`
	channels := []Channel{
		{Id: 910001, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Name: "dsh", Models: "dsh", Group: "default", Priority: &priority, Weight: &weight, BaseURL: &baseURL, Setting: &dshSetting},
		{Id: 910002, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Name: "generic", Models: "generic-alpha", Group: "default", Priority: &priority, Weight: &weight, BaseURL: &baseURL, Setting: &otherSetting},
		{Id: 910003, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "chat", Models: "deepseek-flash", Group: "default", Priority: &priority, Weight: &weight},
	}
	for i := range channels {
		require.NoError(t, channels[i].Insert())
	}

	models, err := GetTaskPluginModelsByGroups([]string{"default"}, "")
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"dsh": {}, "generic-alpha": {}}, models)

	models, err = GetTaskPluginModelsByGroups([]string{"default"}, "dsh")
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"dsh": {}}, models)

	models, err = GetTaskPluginModelsByGroups([]string{"default"}, "missing")
	require.NoError(t, err)
	assert.Empty(t, models)

	models, err = GetTaskPluginModelsByGroups([]string{"vip"}, "")
	require.NoError(t, err)
	assert.Empty(t, models)

	models, err = GetTaskPluginModelsByGroups(nil, "dsh")
	require.NoError(t, err)
	assert.Empty(t, models)
}
