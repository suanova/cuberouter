/*
Copyright (C) 2023-2026 QuantumNous

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
import { describe, expect, test } from 'vitest'

import {
  buildSample,
  type SampleContext,
} from '../components/model-details-api'

const ctx: SampleContext = {
  baseUrl: 'https://api.example.com',
  apiKeyEnv: 'CR_API_KEY',
  modelName: 'minimax-h3-a',
  endpointType: 'video-generation',
  endpointPath: '/v1/video/generations',
}

describe('buildSample for the task-plugin video endpoint', () => {
  test('renders the generic task submit against the endpoint path', () => {
    // 任务插件渠道（62）的模型（如 minimax-h3-a）走 /v1/video/generations。
    // 此前端点类型落到 openai，示例渲染成聊天请求；现在必须给出任务提交示例。
    const code = buildSample('curl', 'video-generation', ctx)

    expect(code).toContain('curl https://api.example.com/v1/video/generations')
    expect(code).toContain('"model": "minimax-h3-a"')
    expect(code).toContain('"duration": 5')
    expect(code).not.toContain('/v1/videos/generations/tasks')
    expect(code).not.toContain('chat/completions')
  })

  test('curl sample polls the generic task endpoint', () => {
    const code = buildSample('curl', 'video-generation', ctx)

    expect(code).toContain(
      'curl https://api.example.com/v1/video/generations/<task_id>'
    )
  })

  test('python sample reads task_id from the submit response', () => {
    const code = buildSample('python', 'video-generation', ctx)

    expect(code).toContain(
      'requests.post("https://api.example.com/v1/video/generations"'
    )
    expect(code).toContain('task_id = resp.json()["task_id"]')
  })

  test('python sample polls on data.status with the gateway terminal values', () => {
    // 网关轮询响应是 {code, message, data:{status, fail_reason, result_url}}，
    // 终态是 SUCCESS / FAILURE。示例此前按 status=="completed"/"failed" 判断，
    // 永远不会命中；undefined 提示词还会被拼进 print 行。
    const code = buildSample('python', 'video-generation', ctx)

    expect(code).toContain('data = task["data"]')
    expect(code).toContain('data["status"] == "SUCCESS"')
    expect(code).toContain('data["status"] == "FAILURE"')
    expect(code).toContain('data["result_url"]')
    expect(code).not.toContain('"completed"')
    expect(code).not.toContain('undefined')
  })

  test('typescript sample polls on data.status with the gateway terminal values', () => {
    const code = buildSample('typescript', 'video-generation', ctx)

    expect(code).toContain('const { data } = await poll.json()')
    expect(code).toContain("data.status === 'SUCCESS'")
    expect(code).toContain("data.status === 'FAILURE'")
    expect(code).toContain('data.result_url')
    expect(code).not.toContain("'completed'")
    expect(code).not.toContain('undefined')
  })
})

describe('buildSample keeps other endpoint styles intact', () => {
  test('chat endpoints still render the chat sample', () => {
    const code = buildSample('curl', 'openai', {
      ...ctx,
      modelName: 'gpt-5.6',
      endpointType: 'openai',
      endpointPath: '/v1/chat/completions',
    })

    expect(code).toContain('/v1/chat/completions')
    expect(code).toContain('"messages"')
  })

  test('ark-style video endpoints keep the Ark task sample', () => {
    const code = buildSample('curl', 'ark-video', {
      ...ctx,
      modelName: 'doubao-seedance-2-0',
      endpointType: 'ark-video',
      endpointPath: '/v1/videos/generations/tasks',
    })

    expect(code).toContain('/v1/videos/generations/tasks')
  })
})
