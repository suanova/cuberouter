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
import { useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useRef } from 'react'

import { getUserModels } from './api'
import { ConversationList } from './components/conversation-list'
import { DshComposer } from './components/dsh-composer'
import { DshHeader } from './components/dsh-header'
import { DshThread } from './components/dsh-thread'
import { useDshConversations } from './hooks/use-dsh-conversations'
import { useDshTask } from './hooks/use-dsh-task'
import { isSessionExpiredError } from './lib/payload'
import type { DshModelOption } from './types'

// 页面默认选带 dsh 字样的模型，没有匹配时退回列表第一项。这里不按
// endpoint 过滤模型：/api/user/models 只返回用户可用的模型，筛选需要额外
// 的数据库查询，而管理端尚未提供任务插件专用模型的分组。
function pickDefaultModel(models: DshModelOption[]): string {
  const preferred = models.find((model) => /dsh/i.test(model.value))
  return preferred?.value ?? models[0]?.value ?? ''
}

export function DshAgent() {
  const {
    conversations,
    activeConversation,
    updateConversation,
    createNew,
    selectConversation,
    removeConversation,
  } = useDshConversations()
  const { sendPrompt, resumeTask, isRunning } = useDshTask(updateConversation)

  // 刷新前仍在运行的一轮：网关还在跟踪任务，导入时带着 taskId 重新挂上轮询；
  // 连 taskId 都没有说明提交没完成，直接标记失败，避免永远转圈。
  const initialConversationsRef = useRef(conversations)
  useEffect(() => {
    for (const conversation of initialConversationsRef.current) {
      for (const turn of conversation.turns) {
        if (turn.status !== 'running') {
          continue
        }
        if (turn.taskId) {
          void resumeTask(conversation.id, turn)
          continue
        }
        updateConversation(conversation.id, (current) => ({
          ...current,
          turns: current.turns.map((item) =>
            item.id === turn.id
              ? { ...item, status: 'failed' as const, finishedAt: Date.now() }
              : item
          ),
        }))
      }
    }
  }, [resumeTask, updateConversation])

  // 独立缓存键：'user-models' 下缓存的是 @/lib/api 的原始响应，和这里映射后的
  // DshModelOption[] 形状不同，共用会互相污染。
  const { data: models = [] } = useQuery({
    queryKey: ['dsh-agent', 'user-models'],
    queryFn: getUserModels,
    staleTime: 5 * 60 * 1000,
  })

  const activeId = activeConversation?.id ?? null
  const activeModel = activeConversation?.model ?? ''

  useEffect(() => {
    if (
      !activeConversation ||
      activeConversation.model ||
      models.length === 0
    ) {
      return
    }
    const fallback = pickDefaultModel(models)
    updateConversation(activeConversation.id, (conversation) => ({
      ...conversation,
      model: fallback,
    }))
  }, [activeConversation, models, updateConversation])

  const handleModelChange = useCallback(
    (model: string) => {
      if (!activeId) {
        return
      }
      updateConversation(activeId, (conversation) => ({
        ...conversation,
        model,
      }))
    },
    [activeId, updateConversation]
  )

  const handlePresetChange = useCallback(
    (preset: string) => {
      if (!activeId) {
        return
      }
      updateConversation(activeId, (conversation) => ({
        ...conversation,
        preset,
      }))
    },
    [activeId, updateConversation]
  )

  const handleCreate = useCallback(() => {
    createNew(activeModel || pickDefaultModel(models))
  }, [activeModel, createNew, models])

  const handleSubmit = useCallback(
    (prompt: string) => {
      if (!activeConversation) {
        return
      }
      void sendPrompt(activeConversation, prompt)
    },
    [activeConversation, sendPrompt]
  )

  const sessionExpired = Boolean(
    activeConversation?.turns.some(
      (turn) => turn.status === 'failed' && isSessionExpiredError(turn.error)
    )
  )

  return (
    <div className='flex size-full min-h-0 overflow-hidden'>
      <ConversationList
        activeId={activeId}
        conversations={conversations}
        onCreate={handleCreate}
        onDelete={removeConversation}
        onSelect={selectConversation}
      />
      <div className='flex min-w-0 flex-1 flex-col overflow-hidden'>
        <DshHeader
          model={activeModel}
          models={models}
          onModelChange={handleModelChange}
          onNewConversation={handleCreate}
          onPresetChange={handlePresetChange}
          preset={activeConversation?.preset ?? ''}
        />
        {/* 滚动容器必须是受限的 flex 列：StickToBottom 只在父级高度确定时
            才能把内部内容区变成可滚动区域，否则消息区会被外层裁掉。 */}
        <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
          <DshThread
            sessionExpired={sessionExpired}
            turns={activeConversation?.turns ?? []}
          />
        </div>
        <DshComposer
          disabled={!activeConversation || !activeModel}
          isRunning={isRunning(activeId)}
          onSubmit={handleSubmit}
        />
      </div>
    </div>
  )
}
