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
package billingexpr

import "sync"

// UsageFieldSchema 描述一个任务计费事实（表达式里的 u("key")）的取值域。
// 与 pkg/jsplugin.UsageFieldSchema 同形，但不依赖插件系统：Go 任务适配器
// （如 relay/channel/task/astraflow 的 kling 家族）用它在 init 阶段注册自己
// 提供的计费事实，供计费表达式保存时校验 u() 键、生成冒烟测试向量。
type UsageFieldSchema struct {
	Type string   // "number" | "boolean" | "string"
	Unit string   // "second" | "count" | "token" | "credit"
	Enum []string // 字符串枚举事实的合法取值
}

type taskUsageSchemaEntry struct {
	match  func(model string) bool
	schema map[string]UsageFieldSchema
}

var (
	taskUsageSchemaMu       sync.RWMutex
	taskUsageSchemaRegistry []taskUsageSchemaEntry
)

// RegisterTaskUsageSchema 注册一组任务计费事实：match 命中模型名时，该模型的
// 计费表达式按此 schema 校验 u() 键（保存时）。由 Go 任务适配器在 init 中调用。
// 同名模型同时有插件 schema 时插件优先——校验路径先查插件注册表再查本注册表。
func RegisterTaskUsageSchema(match func(model string) bool, schema map[string]UsageFieldSchema) {
	if match == nil || len(schema) == 0 {
		return
	}
	taskUsageSchemaMu.Lock()
	defer taskUsageSchemaMu.Unlock()
	taskUsageSchemaRegistry = append(taskUsageSchemaRegistry, taskUsageSchemaEntry{match: match, schema: schema})
}

// LookupTaskUsageSchema 返回首个匹配该模型名的注册 schema。
func LookupTaskUsageSchema(model string) (map[string]UsageFieldSchema, bool) {
	taskUsageSchemaMu.RLock()
	defer taskUsageSchemaMu.RUnlock()
	for _, entry := range taskUsageSchemaRegistry {
		if entry.match(model) {
			return entry.schema, true
		}
	}
	return nil, false
}
