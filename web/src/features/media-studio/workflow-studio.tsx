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
import { Sparkles } from 'lucide-react'

import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { getStudioModels } from './api'
import { TemplateGallery } from './components/template-gallery'
import { WorkflowComposer } from './components/workflow-composer'
import { WorkflowHistory } from './components/workflow-history'
import { WorkflowResults } from './components/workflow-results'
import { useWorkflow } from './hooks/use-workflow'
import { initialDraft, templateDraft, workflowError } from './lib/workflow'
import { localImage, referenceAsset, workflowAPI } from './workflow-api'
import type { WorkflowDraft } from './workflow-types'

export function WorkflowStudio(props: { owner: number }) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<WorkflowDraft>({ ...initialDraft })
  const [view, setView] = useState<'templates' | 'result'>('templates')
  const models = useQuery({
    queryKey: ['studio-models', props.owner],
    queryFn: getStudioModels,
    retry: false,
  })
  const configuration = useQuery({
    queryKey: ['studio-config', props.owner],
    queryFn: workflowAPI.config,
    retry: false,
  })
  const config = configuration.data ?? {
    upload_enabled: false,
    edit_models: [],
  }
  const eligible = (models.data ?? []).filter(
    (name) => draft.mode === 'create' || config.edit_models.includes(name)
  )
  const current = {
    ...draft,
    model: eligible.includes(draft.model) ? draft.model : (eligible[0] ?? ''),
  }
  const workflow = useWorkflow(props.owner)
  const references = useMutation({
    mutationFn: async (files: Blob[]) => {
      if (files.length + draft.references.length > 3) {
        throw new Error('Choose at most three reference images.')
      }
      return Promise.all(files.map(referenceAsset))
    },
    onSuccess: (assets) =>
      setDraft((value) => ({
        ...value,
        references: [...value.references, ...assets],
      })),
    retry: false,
  })
  const continuation = useMutation({
    mutationFn: async (args: { url: string; parent: string }) => {
      const asset = await localImage(args.url)
      if (
        !['image/png', 'image/jpeg', 'image/webp'].includes(asset.mime) ||
        asset.url.length > 14 * 1024 * 1024
      ) {
        throw new Error('Upload PNG, JPEG or WebP files of at most 10 MB each.')
      }
      return { ...args, asset }
    },
    onSuccess: ({ asset, parent }) =>
      setDraft({
        ...initialDraft,
        mode: 'edit',
        references: [asset],
        parent_id: parent,
      }),
    retry: false,
  })
  const busy =
    workflow.generation.isPending ||
    references.isPending ||
    continuation.isPending
  const errors = [
    models.error,
    configuration.error,
    references.error,
    continuation.error,
    workflow.generation.error,
    workflow.deletion.error,
  ].filter(Boolean)
  return (
    <div className='min-h-0 flex-1 overflow-y-auto'>
      <div className='mx-auto flex w-full max-w-[1500px] flex-col gap-6 p-4 sm:p-6'>
        <header className='flex items-center gap-3'>
          <Sparkles className='text-primary size-6' aria-hidden='true' />
          <div>
            <h1 className='text-xl font-semibold'>{t('Media Studio')}</h1>
            <p className='text-muted-foreground text-xs'>
              {t('Create, refine and keep every version.')}
            </p>
          </div>
        </header>
        <div className='grid items-start gap-6 lg:grid-cols-[350px_minmax(0,1fr)]'>
          <aside className='bg-card rounded-2xl border p-4'>
            <WorkflowComposer
              draft={current}
              config={config}
              models={models.data ?? []}
              busy={busy}
              loading={models.isPending}
              onChange={setDraft}
              onUpload={(files) => references.mutate(files)}
              onGenerate={() => {
                setView('result')
                workflow.generation.mutate(structuredClone(current))
              }}
              onReset={() => setDraft({ ...initialDraft })}
            />
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
              <TemplateGallery
                disabled={busy}
                onApply={(template, fields) =>
                  setDraft(templateDraft(template, fields, current))
                }
              />
            )}
            {view === 'result' && (
              // 结果与本地历史合并在同一视图：结果在上，历史列表在下。
              <div className='space-y-4'>
                <div className='bg-card rounded-2xl border p-4'>
                  <WorkflowResults
                    job={workflow.selected}
                    busy={workflow.generation.isPending}
                    submitted={workflow.generation.variables}
                    elapsed={workflow.elapsed}
                    onEdit={(asset, job) =>
                      continuation.mutate({ url: asset.url, parent: job.id })
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
      </div>
    </div>
  )
}
