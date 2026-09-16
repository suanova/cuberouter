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
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Sparkles } from 'lucide-react'

import { DEFAULT_PARAMS, STUDIO_MODEL } from './constants'
import { useGeneration } from './hooks/use-generation'
import { clearHistory, loadHistory, saveHistoryEntry } from './lib/history'
import type { GenerationResult, HistoryEntry, StudioParams } from './types'
import { DebugPanel } from './components/debug-panel'
import { HistoryList } from './components/history-list'
import { PreviewPanel } from './components/preview-panel'
import { StudioForm } from './components/studio-form'

function createId(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

export function MediaStudio() {
  const { t } = useTranslation()
  const [params, setParams] = useState<StudioParams>({ ...DEFAULT_PARAMS })
  const [history, setHistory] = useState<HistoryEntry[]>(() => loadHistory())

  const { status, elapsedMs, result, error, requestBody, rawResponse, start, restore } =
    useGeneration({
      onSuccess: (
        generated: GenerationResult,
        elapsed: number,
        usedParams: StudioParams,
      ) => {
        const entry: HistoryEntry = {
          id: createId(),
          prompt: usedParams.prompt.trim(),
          params: { ...usedParams },
          imageUrls: generated.images.map((image) => image.url),
          elapsedMs: elapsed,
          createdAt: Date.now(),
        }
        setHistory(saveHistoryEntry(entry))
      },
    })

  const handleGenerate = useCallback(() => {
    void start(params)
  }, [params, start])

  let errorText: string | null = null
  if (status === 'error' && error) {
    if (error.kind === 'server') {
      errorText = error.message
    } else {
      errorText = t(error.message)
    }
  }

  return (
    <div className='mx-auto flex w-full max-w-6xl flex-col gap-6 p-4 sm:p-6'>
      <header className='flex items-center gap-3'>
        <Sparkles aria-hidden='true' className='size-6 text-primary' />
        <div>
          <h1 className='text-lg font-semibold'>{t('Media Studio')}</h1>
          <p className='text-xs text-muted-foreground'>
            {t('Turn your ideas into images with {{model}}.', {
              model: STUDIO_MODEL,
            })}
          </p>
        </div>
      </header>

      <div className='grid grid-cols-1 gap-6 lg:grid-cols-[370px_minmax(0,1fr)]'>
        <div className='h-fit rounded-2xl border border-border bg-card p-4 lg:sticky lg:top-4'>
          <StudioForm
            params={params}
            generating={status === 'generating'}
            errorText={errorText}
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
            />
          </div>

          <div className='rounded-2xl border border-border bg-card p-4'>
            <DebugPanel requestBody={requestBody} rawResponse={rawResponse} />
          </div>

          <div className='rounded-2xl border border-border bg-card p-4'>
            <HistoryList
              entries={history}
              disabled={status === 'generating'}
              onSelect={(entry) => {
                setParams({ ...entry.params })
                restore(entry)
              }}
              onClear={() => {
                clearHistory()
                setHistory([])
              }}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
