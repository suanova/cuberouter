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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Sparkles } from 'lucide-react'

import { getStudioModels } from './api'
import { DEFAULT_PARAMS } from './constants'
import { useHistory } from './hooks/use-history'
import { useGeneration } from './hooks/use-generation'
import type { GenerationResult, HistoryEntry, StudioParams } from './types'
import { HistoryList } from './components/history-list'
import { PreviewPanel } from './components/preview-panel'
import { StudioForm } from './components/studio-form'

function createId(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

export function MediaStudio() {
  const { t } = useTranslation()
  const [params, setParams] = useState<StudioParams>({ ...DEFAULT_PARAMS })
  const [models, setModels] = useState<string[]>([])
  const [modelsLoading, setModelsLoading] = useState(true)
  const [model, setModel] = useState('')

  const { status, elapsedMs, result, error, start } = useGeneration()

  const {
    entries: historyEntries,
    loading: historyLoading,
    storageAvailable: historyStorageAvailable,
    saveEntry,
    removeEntry,
    clearEntries,
  } = useHistory()
  const savedResultRef = useRef<GenerationResult | null>(null)

  // 生成成功后写入本地历史（图片为自包含 data URL，可跨会话还原）。
  // 以 result 对象身份去重，避免状态刷新导致重复保存。
  useEffect(() => {
    if (status !== 'success' || result === null) {
      return
    }
    if (savedResultRef.current === result) {
      return
    }
    savedResultRef.current = result
    const entry: HistoryEntry = {
      id: createId(),
      prompt: params.prompt.trim(),
      model,
      params: { ...params },
      imageUrls: result.images.map((image) => image.url),
      elapsedMs,
      createdAt: Date.now(),
    }
    saveEntry(entry)
  }, [status, result, params, model, elapsedMs, saveEntry])

  useEffect(() => {
    let cancelled = false
    getStudioModels()
      .then((list) => {
        if (cancelled) {
          return
        }
        setModels(list)
        setModel((current) =>
          current !== '' && list.includes(current) ? current : (list[0] ?? ''),
        )
      })
      .finally(() => {
        if (!cancelled) {
          setModelsLoading(false)
        }
      })
      .catch(() => {
        // 拉取失败时保持空列表，页面会提示无可用模型
      })
    return () => {
      cancelled = true
    }
  }, [])

  const handleGenerate = useCallback(() => {
    void start(params, model)
  }, [params, model, start])

  let errorText: string | null = null
  if (status === 'error' && error) {
    if (error.kind === 'server') {
      errorText = error.message
    } else {
      errorText = t(error.message)
    }
  }

  return (
    <div className='min-h-0 flex-1 overflow-y-auto'>
      <div className='mx-auto flex w-full max-w-6xl flex-col gap-6 p-4 sm:p-6'>
        <header className='flex items-center gap-3'>
          <Sparkles aria-hidden='true' className='size-6 text-primary' />
          <div>
            <h1 className='text-lg font-semibold'>{t('Media Studio')}</h1>
            <p className='text-xs text-muted-foreground'>
              {t('Turn your ideas into images.')}
            </p>
          </div>
        </header>

        <div className='grid grid-cols-1 gap-6 lg:grid-cols-[370px_minmax(0,1fr)]'>
          <div className='h-fit rounded-2xl border border-border bg-card p-4 lg:sticky lg:top-4'>
            <StudioForm
              params={params}
              generating={status === 'generating'}
              errorText={errorText}
              models={models}
              modelsLoading={modelsLoading}
              model={model}
              onModelChange={setModel}
              onChange={setParams}
              onGenerate={handleGenerate}
            />
          </div>

          <div className='flex min-w-0 flex-col gap-4'>
            <div className='rounded-2xl border border-border bg-card p-4'>
              <PreviewPanel
                status={status}
                params={params}
                elapsedMs={elapsedMs}
                result={result}
                error={error}
                model={model}
              />
            </div>

            <div className='rounded-2xl border border-border bg-card p-4'>
              <HistoryList
                entries={historyEntries}
                loading={historyLoading}
                storageAvailable={historyStorageAvailable}
                onDelete={removeEntry}
                onClear={clearEntries}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
