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
import { BotIcon, PaperclipIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Conversation,
  ConversationContent,
  ConversationEmptyState,
  ConversationScrollButton,
} from '@/components/ai-elements/conversation'
import { Loader } from '@/components/ai-elements/loader'
import { Message, MessageContent } from '@/components/ai-elements/message'
import { Response } from '@/components/ai-elements/response'
import { Shimmer } from '@/components/ai-elements/shimmer'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

import { useTaskArtifacts } from '../hooks/use-task-artifacts'
import { rewriteArtifactLinks } from '../lib/artifact-links'
import {
  approximateMinutes,
  type TurnDurationEstimate,
} from '../lib/turn-duration'
import type { DshTurn } from '../types'
import { ToolCallsPanel } from './tool-calls-panel'
import { TurnArtifacts } from './turn-artifacts'

type DshThreadProps = {
  turns: DshTurn[]
  sessionExpired: boolean
  expectedTurnDuration: TurnDurationEstimate
}

function RunningIndicator(props: {
  startedAt: number
  expectedTurnDuration: TurnDurationEstimate
}) {
  const { t } = useTranslation()
  const [seconds, setSeconds] = useState(0)

  useEffect(() => {
    const update = () =>
      setSeconds(Math.max(0, Math.floor((Date.now() - props.startedAt) / 1000)))
    update()
    const timer = window.setInterval(update, 1000)
    return () => window.clearInterval(timer)
  }, [props.startedAt])

  // 超出预期一半还没结束就不再给具体数字，改口说明还在跑，免得计时器一直违背承诺。
  const overdue = seconds * 1000 > props.expectedTurnDuration.milliseconds * 1.5
  let expectation = t('Usually takes a few minutes.')
  if (overdue) {
    expectation = t('Taking longer than usual, the agent is still working.')
  } else if (props.expectedTurnDuration.fromHistory) {
    expectation = t('Usually takes about {{duration}}.', {
      duration: t('{{minutes}} min', {
        minutes: approximateMinutes(props.expectedTurnDuration.milliseconds),
      }),
    })
  }

  return (
    <div className='space-y-1 py-1'>
      <div className='flex items-center gap-2'>
        <Loader />
        <Shimmer className='text-sm' duration={1}>
          {t('Agent is working…')}
        </Shimmer>
        <span className='text-muted-foreground text-xs'>
          {t('{{seconds}}s', { seconds })}
        </span>
      </div>
      <p className='text-muted-foreground text-xs'>{expectation}</p>
      <p className='text-muted-foreground text-xs'>
        {t(
          'DSH returns the reply after the whole turn finishes, so output may arrive at once.'
        )}
      </p>
    </div>
  )
}

function TurnResult(props: {
  turn: DshTurn
  expectedTurnDuration: TurnDurationEstimate
}) {
  const { t } = useTranslation()
  // 产物清单也用来修回复里的文件链接：模型写的 /data/workspace/... 路径在浏览器里
  // 打不开，能对上产物文件名的链接换成签名下载地址。与产物卡片共用缓存，只发一次请求。
  const artifactsQuery = useTaskArtifacts(
    props.turn.taskId,
    props.turn.status !== 'running' && props.turn.status !== 'failed'
  )
  const artifacts = artifactsQuery.data?.artifacts ?? []

  if (props.turn.status === 'running') {
    return (
      <RunningIndicator
        expectedTurnDuration={props.expectedTurnDuration}
        startedAt={props.turn.createdAt}
      />
    )
  }

  if (props.turn.status === 'failed') {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Task failed')}</AlertTitle>
        <AlertDescription>
          {props.turn.error || t('The agent task failed.')}
        </AlertDescription>
      </Alert>
    )
  }

  const truncated = props.turn.status === 'max-tokens'
  const canceled = props.turn.status === 'canceled'

  return (
    <div className='w-full space-y-3'>
      <ToolCallsPanel toolCalls={props.turn.toolCalls} />
      {truncated && (
        <Alert>
          <AlertTitle>{t('Reply truncated')}</AlertTitle>
          <AlertDescription>
            {t('The agent reached its output token limit.')}
          </AlertDescription>
        </Alert>
      )}
      {canceled && (
        <Alert>
          <AlertTitle>{t('Task canceled')}</AlertTitle>
          <AlertDescription>
            {t('The agent turn was canceled before it finished.')}
          </AlertDescription>
        </Alert>
      )}
      {props.turn.reply ? (
        <MessageContent variant='flat'>
          <Response final>
            {rewriteArtifactLinks(props.turn.reply, artifacts)}
          </Response>
        </MessageContent>
      ) : (
        <p className='text-muted-foreground text-sm'>
          {t('The agent finished without a text reply.')}
        </p>
      )}
      {props.turn.taskId && <TurnArtifacts taskId={props.turn.taskId} />}
    </div>
  )
}

export function DshThread(props: DshThreadProps) {
  const { t } = useTranslation()

  return (
    <Conversation>
      {/* 与 playground 一致：滚动容器不带内边距，居中内容单独限宽，
          否则左右留白会比同一站点的对话页明显宽。 */}
      <ConversationContent className='p-0'>
        <div className='mx-auto w-full max-w-4xl space-y-5 px-4 py-4'>
          {props.turns.length === 0 ? (
            <ConversationEmptyState
              description={t(
                'Ask the agent to inspect code, run commands, or walk through a problem.'
              )}
              icon={<BotIcon className='size-8' />}
              title={t('Start a DSH conversation')}
            />
          ) : (
            props.turns.map((turn) => (
              <div className='space-y-3' key={turn.id}>
                <Message from='user'>
                  <div className='flex flex-col items-end gap-1.5'>
                    <MessageContent variant='flat'>
                      {turn.prompt}
                    </MessageContent>
                    {(turn.attachments?.length ?? 0) > 0 && (
                      <ul
                        aria-label={t('Attached files')}
                        className='flex flex-wrap justify-end gap-1.5'
                      >
                        {turn.attachments?.map((attachment) => (
                          <li
                            className='bg-muted/50 text-muted-foreground flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs'
                            key={attachment.path}
                            title={attachment.path}
                          >
                            <PaperclipIcon
                              aria-hidden='true'
                              className='size-3 shrink-0'
                            />
                            <span className='max-w-56 truncate'>
                              {attachment.name}
                            </span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                </Message>
                <Message from='assistant'>
                  <TurnResult
                    expectedTurnDuration={props.expectedTurnDuration}
                    turn={turn}
                  />
                </Message>
              </div>
            ))
          )}
          {props.sessionExpired && (
            <Alert>
              <AlertTitle>{t('Session expired')}</AlertTitle>
              <AlertDescription>
                {t(
                  'This DSH session can no longer be continued because the harness restarted or the session expired. Start a new conversation to keep going.'
                )}
              </AlertDescription>
            </Alert>
          )}
        </div>
      </ConversationContent>
      <ConversationScrollButton />
    </Conversation>
  )
}
