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
package cubestack

// ChannelName 与 Web 端渠道类型名称保持一致。
const ChannelName = "CubeStack"

// ModelList 是渠道测试等流程使用的参考模型列表；实际可用模型由渠道
// 模型列表决定（SGLang 服务以 --served-model-name 暴露的任意模型名）。
var ModelList = []string{
	"MiniMax-H3",
}
