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
export const API_ENDPOINTS = {
  USER_MODELS: '/api/user/models',
  AGENT_TASKS: '/pg/agent/tasks',
  TASK_ARTIFACTS: '/api/task',
  DSH_ORGANIZATION: '/api/dsh/organization',
  DSH_UPLOADS: '/api/dsh/uploads',
  DSH_UPLOAD_SETTINGS: '/api/dsh/uploads/config',
} as const

// 每个组织一个 dsh 容器：会话一旦建立就把组织固定下来，提交与上传都带这个头，
// 同一会话始终落在同一个实例里。
export const DSH_ORGANIZATION_HEADER = 'X-DSH-Organization-Id'

// 模型下拉只列任务插件渠道托管的模型：渠道里的 dsh 只是插件入口，混在普通
// 聊天模型里会被误选。plugin 对应渠道 setting 里的 task_plugin_key。
export const AGENT_MODEL_FILTER = {
  endpoint: 'agent',
  plugin: 'dsh',
} as const

export const TASK_STORAGE_KEY = 'dsh-agent:conversations:v1'

// 网关侧每 15s 才刷新一次上游任务，页面 2s 轮询只是尽快看到刷新后的快照。
export const TASK_POLL_INTERVAL_MS = 2000

// 网关任务状态（model.Task.Status）
export const GATEWAY_SETTLED_STATUSES = ['SUCCESS', 'FAILURE'] as const

// 新会话默认挂 standard 预设：不挂预设时 dsh 只保留读写工作目录的工具，
// 模型会回答自己不会写文件或导出 PPT。服务端 overlay.yml 的 defaultPreset
// 同步为 standard，两侧一致。
export const DEFAULT_PRESET = 'standard'

export const PRESET_OPTIONS = [
  { value: '', label: 'Default preset' },
  { value: 'standard', label: 'Standard' },
  { value: 'minimal', label: 'Minimal' },
  { value: 'ptc', label: 'PTC' },
  { value: 'cordis', label: 'Cordis' },
] as const
