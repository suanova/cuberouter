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
} as const

export const TASK_STORAGE_KEY = 'dsh-agent:conversations:v1'

// 网关侧每 15s 才刷新一次上游任务，页面 2s 轮询只是尽快看到刷新后的快照。
export const TASK_POLL_INTERVAL_MS = 2000

// 网关任务状态（model.Task.Status）
export const GATEWAY_SETTLED_STATUSES = ['SUCCESS', 'FAILURE'] as const

export const PRESET_OPTIONS = [
  { value: '', label: 'Default preset' },
  { value: 'standard', label: 'Standard' },
  { value: 'minimal', label: 'Minimal' },
  { value: 'code', label: 'Code' },
  { value: 'cordis', label: 'Cordis' },
] as const
