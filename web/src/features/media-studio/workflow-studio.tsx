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
import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'

import { getStudioModels } from './api'
import { QwenEditor } from './components/qwen-editor'
import { QwenExamples } from './components/qwen-examples'
import { TemplateGallery } from './components/template-gallery'
import { WorkflowComposer } from './components/workflow-composer'
import { WorkflowHistory } from './components/workflow-history'
import { WorkflowResults } from './components/workflow-results'
import { useWorkflow } from './hooks/use-workflow'
import { editDraft, newEditSeed, referenceDimensions } from './lib/qwen-edit'
import {
  draftSchema,
  initialDraft,
  templateDraft,
  workflowError,
} from './lib/workflow'
import { localImage, referenceAsset } from './workflow-api'
import type { WorkflowConfig, WorkflowDraft } from './workflow-types'

export function WorkflowStudio(props: { owner: number }) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<WorkflowDraft>({ ...initialDraft })
  const [view, setView] = useState<'templates' | 'result'>('templates')
  const [editor, setEditor] = useState<{
    draft: WorkflowDraft
    draw: boolean
  }>()
  const models = useQuery({
    queryKey: ['studio-models', props.owner],
    queryFn: getStudioModels,
    retry: false,
  })
  const configuration = useQuery({
    queryKey: ['studio-config', props.owner],
    queryFn: async () =>
      (
        await api.get<WorkflowConfig>('/api/media-studio/config', {
          skipErrorHandler: true,
        })
      ).data,
    retry: false,
  })
  const config = configuration.data ?? {}
  const catalog = models.data ?? {
    textToImage: [],
    imageToImage: [],
    textToVideo: [],
    imageToVideo: [],
  }
  const eligible =
    draft.mode === 'create' ? catalog.textToImage : catalog.imageToImage
  const selectedModel = eligible.includes(draft.model)
    ? draft.model
    : (eligible[0] ?? '')
  const queued =
    !!config.qwen_queue_enabled && selectedModel === config.qwen_model
  const current: WorkflowDraft = {
    ...draft,
    model: selectedModel,
    queued,
    count: queued ? 1 : draft.count,
  }
  const workflow = useWorkflow(props.owner)
  const references = useMutation({
    mutationFn: async (files: Blob[]) => {
      if (files.length + draft.references.length > (queued ? 10 : 3)) {
        throw new Error(
          queued
            ? 'Choose at most ten reference images.'
            : 'Choose at most three reference images.'
        )
      }
      let size: string | undefined
      if (queued && !draft.references.length && files.length === 1) {
        const bitmap = await createImageBitmap(files[0])
        try {
          size = referenceDimensions(bitmap.width, bitmap.height)
        } finally {
          bitmap.close()
        }
      }
      return { assets: await Promise.all(files.map(referenceAsset)), size }
    },
    onSuccess: ({ assets, size }) =>
      setDraft((value) => ({
        ...value,
        size: size ?? value.size,
        seed:
          queued && !value.references.length && assets.length
            ? newEditSeed(value.seed)
            : value.seed,
        references: [...value.references, ...assets],
      })),
    retry: false,
  })
  const continuation = useMutation({
    mutationFn: async (args: {
      url: string
      parent: string
      request: WorkflowDraft
      draw?: boolean
    }) => {
      const asset = await localImage(args.url)
      if (
        !['image/png', 'image/jpeg', 'image/webp'].includes(asset.mime) ||
        asset.url.length > 14 * 1024 * 1024
      ) {
        throw new Error('Upload PNG, JPEG or WebP files of at most 10 MB each.')
      }
      return { ...args, asset }
    },
    onSuccess: ({ asset, parent, request, draw }) => {
      const next = editDraft(request, asset, parent)
      if (next.queued) setEditor({ draft: next, draw: !!draw })
      else setDraft(next)
    },
    retry: false,
  })
  const busy =
    workflow.generation.isPending ||
    workflow.recovering ||
    references.isPending ||
    continuation.isPending
  const errors = [
    models.error,
    references.error,
    continuation.error,
    workflow.generation.error,
    workflow.deletion.error,
  ].filter(Boolean)
  return (
    <div className='flex flex-col gap-6'>
      <div className='grid items-start gap-6 lg:grid-cols-[350px_minmax(0,1fr)]'>
        <aside className='bg-card rounded-2xl border p-4'>
          <WorkflowComposer
            draft={current}
            config={config}
            textToImageModels={catalog.textToImage}
            imageToImageModels={catalog.imageToImage}
            busy={busy}
            loading={models.isPending}
            onChange={setDraft}
            onUpload={(files) => references.mutate(files)}
            onGenerate={() => {
              setView('result')
              workflow.generation.mutate(structuredClone(current))
            }}
            onReset={() => setDraft({ ...initialDraft })}
            onDraw={() =>
              setEditor({
                draft: { ...current, seed: newEditSeed(current.seed) },
                draw: true,
              })
            }
          />
          {queued && !config.qwen_access && (
            <p role='alert'>
              {t(
                'Your account is not enrolled for Image Studio. Contact your administrator.'
              )}
            </p>
          )}
          {!!workflow.queueStatus && (
            <p role='status' className='mt-3 text-sm'>
              {workflow.queueStatus.state === 'queued'
                ? t('Queued · {{count}} waiting ahead', {
                    count: workflow.queueStatus.ahead ?? 0,
                  })
                : t(`Image job: ${workflow.queueStatus.state}`)}
            </p>
          )}
          {workflow.pending && !workflow.generation.isPending && (
            <div className='mt-3 space-y-2'>
              <Button variant='outline' onClick={workflow.reconnect}>
                {t('Reconnect to image job')}
              </Button>
              {!!workflow.queueStatus &&
                ['failed', 'cancelled', 'expired', 'released'].includes(
                  workflow.queueStatus.state
                ) && (
                  <Button
                    variant='outline'
                    onClick={() => {
                      if (workflow.pending) setDraft(workflow.pending.draft)
                      void workflow.clearEndedRequest()
                    }}
                  >
                    {t('Edit failed draft')}
                  </Button>
                )}
            </div>
          )}
        </aside>
        <main className='min-w-0 space-y-4'>
          <nav
            className='flex flex-wrap gap-1 border-b pb-3'
            aria-label={t('Studio views')}
          >
            {(
              [
                { id: 'templates', label: 'Template gallery' },
                { id: 'result', label: 'Result' },
              ] as const
            ).map((item) => (
              <Button
                key={item.id}
                size='sm'
                variant={view === item.id ? 'secondary' : 'ghost'}
                aria-pressed={view === item.id}
                onClick={() => setView(item.id)}
              >
                {t(item.label)}
              </Button>
            ))}
          </nav>
          {errors.map((error) => (
            <p
              key={workflowError(error)}
              role='alert'
              className='text-destructive text-sm'
            >
              {t(workflowError(error))}
            </p>
          ))}
          {!!workflow.warning && (
            <p role='status' className='text-muted-foreground text-sm'>
              {t(workflow.warning)}
            </p>
          )}
          {workflow.history.isError && (
            <p role='status' className='text-muted-foreground text-sm'>
              {t(
                'Local history storage is unavailable. Download images to keep them.'
              )}
            </p>
          )}
          {view === 'templates' && (
            <div className='space-y-6'>
              {queued && (
                <QwenExamples
                  draft={current}
                  disabled={busy || !config.qwen_access}
                  onApply={setDraft}
                />
              )}
              <TemplateGallery
                disabled={busy}
                onApply={(template, fields) =>
                  setDraft(templateDraft(template, fields, current))
                }
              />
            </div>
          )}
          {view === 'result' && (
            // 结果与本地历史合并在同一视图：结果在上，历史列表在下。
            <div className='space-y-4'>
              <div className='bg-card rounded-2xl border p-4'>
                <WorkflowResults
                  job={workflow.selected}
                  busy={workflow.generation.isPending}
                  count={workflow.generation.variables?.count ?? 1}
                  elapsed={workflow.elapsed}
                  queued={!!workflow.generation.variables?.queued}
                  onEdit={(asset, job) =>
                    continuation.mutate({
                      url: asset.url,
                      parent: job.id,
                      request: job.request,
                    })
                  }
                  onDraw={(asset, job) =>
                    continuation.mutate({
                      url: asset.url,
                      parent: job.id,
                      request: job.request,
                      draw: true,
                    })
                  }
                />
              </div>
              <div className='bg-card rounded-2xl border p-4'>
                <WorkflowHistory
                  jobs={workflow.history.data ?? []}
                  selected={workflow.selected?.id}
                  busy={busy}
                  onSelect={workflow.select}
                  onDelete={(id) => workflow.deletion.mutate(id)}
                />
              </div>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'History is stored in this browser for this account, up to 50 creations or 100 MB. It does not sync across devices and may be cleared by your browser. Download important images.'
                )}
              </p>
            </div>
          )}
        </main>
      </div>
      {!!config.audit_retention_days && queued && (
        <p className='text-muted-foreground text-xs'>
          {t(
            'The platform keeps account details, prompts, parameters and image hashes for 30 days. Generated images are kept temporarily for delivery, then released or expired after 10 minutes.'
          )}
        </p>
      )}
      {editor && (
        <QwenEditor
          draft={editor.draft}
          draw={editor.draw}
          busy={busy}
          onClose={() => setEditor(undefined)}
          onSubmit={(next) => {
            const normalized = { ...next, model: selectedModel, queued }
            setDraft(normalized)
            setEditor(undefined)
            if (
              !draftSchema.safeParse(normalized).success ||
              !queued ||
              !config.qwen_access
            ) {
              return
            }
            setView('result')
            workflow.generation.mutate(normalized)
          }}
        />
      )}
    </div>
  )
}
