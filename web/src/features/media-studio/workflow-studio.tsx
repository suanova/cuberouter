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
import { useMutation } from '@tanstack/react-query'
import { Sparkles, LayoutGrid, History, Image } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { ImageEditor } from './components/image-editor'
import { TemplateGallery } from './components/template-gallery'
import { WorkflowComposer } from './components/workflow-composer'
import { WorkflowHistory } from './components/workflow-history'
import { WorkflowResults } from './components/workflow-results'
import { useWorkflow } from './hooks/use-workflow'
import { initialDraft, templateDraft, workflowError } from './lib/workflow'
import { workflowAPI } from './workflow-api'
import type {
  StudioAsset,
  WorkflowConfig,
  WorkflowDraft,
  WorkflowJob,
} from './workflow-types'

export function WorkflowStudio(props: { config: WorkflowConfig }) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<WorkflowDraft>({ ...initialDraft })
  const [references, setReferences] = useState<StudioAsset[]>([])
  const [view, setView] = useState<'templates' | 'result' | 'history'>(
    'templates'
  )
  const [templateUndo, setTemplateUndo] = useState<WorkflowDraft>()
  const [editor, setEditor] = useState<{
    asset: StudioAsset
    parent?: WorkflowJob
  }>()
  const workflow = useWorkflow()
  const upload = useMutation({
    mutationFn: async (files: File[]) => {
      if (files.length + draft.references.length > 3) {
        throw new Error('Choose at most three reference images.')
      }
      if (
        files.some(
          (file) =>
            file.size > 10 * 1024 * 1024 ||
            !['image/png', 'image/jpeg', 'image/webp'].includes(file.type)
        )
      ) {
        throw new Error('Upload PNG, JPEG or WebP files of at most 10 MB each.')
      }
      for (const file of files) {
        const asset = await workflowAPI.upload(file)
        setReferences((current) => [...current, asset])
        setDraft((current) => ({
          ...current,
          references: [...current.references, asset.id],
        }))
      }
    },
    retry: false,
  })
  const errors = [
    workflow.generation.error,
    workflow.deletion.error,
    workflow.history.error,
    upload.error,
  ].filter(Boolean)
  const locked = workflow.busy || upload.isPending
  return (
    <div className='min-h-0 flex-1 overflow-y-auto'>
      <div className='mx-auto flex w-full max-w-[1500px] flex-col gap-6 p-4 sm:p-6'>
        <header className='flex flex-wrap items-start justify-between gap-4'>
          <div className='flex items-center gap-3'>
            <div className='bg-primary/10 rounded-2xl p-3'>
              <Sparkles className='text-primary size-5' aria-hidden='true' />
            </div>
            <div>
              <h1 className='text-xl font-semibold tracking-tight'>
                {t('Media Studio')}
              </h1>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t('Create, refine and keep every version.')}
              </p>
            </div>
          </div>
          <div className='flex flex-wrap gap-2 text-[10px]'>
            {Object.entries(props.config.health).map(([name, state]) => (
              <span
                key={name}
                className='bg-card rounded-full border px-3 py-1.5'
              >
                {t(name)}{' '}
                <span className='text-muted-foreground'>· {t(state)}</span>
              </span>
            ))}
          </div>
        </header>
        <div className='grid min-w-0 grid-cols-1 items-start gap-6 lg:grid-cols-[350px_minmax(0,1fr)] xl:grid-cols-[370px_minmax(0,1fr)]'>
          <aside className='bg-card rounded-2xl border p-4 xl:p-5'>
            <WorkflowComposer
              draft={draft}
              config={props.config}
              references={references}
              busy={workflow.busy}
              uploading={upload.isPending}
              onChange={setDraft}
              onUpload={(files) => upload.mutate(files)}
              onTemplates={() => setView('templates')}
              onTools={(asset) => setEditor({ asset })}
              onReset={() => {
                setDraft({ ...initialDraft })
                setTemplateUndo(undefined)
              }}
              onGenerate={() => {
                setView('result')
                workflow.generation.mutate({ ...draft })
              }}
            />
            {templateUndo && (
              <Button
                variant='ghost'
                size='sm'
                className='mt-3 w-full'
                disabled={locked}
                onClick={() => {
                  setDraft(templateUndo)
                  setTemplateUndo(undefined)
                }}
              >
                {t('Undo template')}
              </Button>
            )}
          </aside>
          <main className='min-w-0 space-y-4'>
            <div
              className='flex flex-wrap items-center gap-1 border-b pb-3'
              aria-label={t('Studio views')}
            >
              {(
                [
                  {
                    id: 'templates',
                    label: 'Template gallery',
                    icon: LayoutGrid,
                  },
                  { id: 'result', label: 'Result', icon: Image },
                  { id: 'history', label: 'Creation history', icon: History },
                ] as const
              ).map((item) => (
                <Button
                  key={item.id}
                  variant={view === item.id ? 'secondary' : 'ghost'}
                  size='sm'
                  aria-pressed={view === item.id}
                  onClick={() => setView(item.id)}
                >
                  <item.icon className='size-4' />
                  {t(item.label)}
                  {item.id === 'history' && (
                    <span className='text-muted-foreground'>
                      {workflow.jobs.length}
                    </span>
                  )}
                </Button>
              ))}
            </div>
            {errors.map((error) => (
              <p
                key={workflowError(error)}
                role='alert'
                className='bg-destructive/10 text-destructive rounded-xl p-3 text-sm'
              >
                {t(workflowError(error))}
              </p>
            ))}
            {view === 'templates' && (
              <TemplateGallery
                disabled={locked}
                onApply={(template, fields) => {
                  setTemplateUndo(draft)
                  setDraft(templateDraft(template, fields, draft))
                }}
              />
            )}
            {view === 'result' && (
              <WorkflowResults
                job={workflow.selected}
                busy={locked}
                onEdit={(asset, job) => {
                  setReferences([asset])
                  setDraft({
                    ...initialDraft,
                    mode: 'edit',
                    size: 'auto',
                    references: [asset.id],
                    parent_id: job.id,
                    expected_text: job.request.expected_text ?? '',
                  })
                }}
                onTools={(asset, job) => setEditor({ asset, parent: job })}
              />
            )}
            {view === 'history' && (
              <WorkflowHistory
                jobs={workflow.jobs}
                selected={workflow.selected?.id}
                busy={workflow.busy}
                onSelect={(id) => {
                  workflow.select(id)
                  setView('result')
                }}
                onDelete={(id) => workflow.deletion.mutate(id)}
              />
            )}
            <p className='text-muted-foreground border-t pt-4 text-[11px] leading-relaxed'>
              {t(
                'Your uploads and saved versions belong to your account. Studio copies expire after {{days}} days.',
                { days: props.config.retention_days }
              )}
            </p>
          </main>
        </div>
      </div>
      {editor && (
        <ImageEditor
          key={editor.asset.id}
          asset={editor.asset}
          parent={editor.parent}
          onClose={() => setEditor(undefined)}
          onSaved={(job) => {
            workflow.select(job.id)
            setView('result')
            void workflow.refresh()
          }}
          onRegional={async (next) => {
            setView('result')
            await workflow.generation.mutateAsync(next)
          }}
        />
      )}
    </div>
  )
}
