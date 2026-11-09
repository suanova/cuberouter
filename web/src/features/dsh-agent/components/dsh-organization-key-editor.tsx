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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import { revealDshOrganizationSecret, updateDshOrganizationConfig } from '../admin-api'

type DshOrganizationKeyEditorProps = {
  organizationId: number
  // 服务端当前的渠道绑定：保存密钥时原样回传，避免覆盖未保存的渠道改动。
  channelId: number
  configured: boolean
  preview: string
  onSaved: () => Promise<unknown>
}

/**
 * 组织 DeepSeek Key 的编辑行。密钥只以脱敏预览展示；明文仅在显式点击"复制"
 * 时通过一次单独的请求取回，用于拼进 docker compose 命令。
 */
export function DshOrganizationKeyEditor(props: DshOrganizationKeyEditorProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState('')

  const saveMutation = useMutation({
    mutationFn: (input: { deepseekKey?: string; clearDeepseekKey?: boolean }) =>
      updateDshOrganizationConfig(props.organizationId, {
        channelId: props.channelId,
        ...input,
      }),
    onSuccess: async () => {
      setDraft('')
      await props.onSaved()
      toast.success(t('Organization DeepSeek key saved'))
    },
    onError: (error: Error) => {
      toast.error(t('Failed to save the organization DeepSeek key'), {
        description: error.message,
      })
    },
  })

  const revealMutation = useMutation({
    mutationFn: () => revealDshOrganizationSecret(props.organizationId),
    onSuccess: async (secret) => {
      const copied = await copyToClipboard(secret)
      if (copied) {
        toast.success(t('Organization DeepSeek key copied'))
      } else {
        toast.error(t('Failed to copy'))
      }
    },
    onError: (error: Error) => {
      toast.error(t('Failed to read the organization DeepSeek key'), {
        description: error.message,
      })
    },
  })

  const saving = saveMutation.isPending
  const trimmed = draft.trim()

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <Input
        aria-label={t('Organization DeepSeek key')}
        className='w-72'
        disabled={saving}
        onChange={(event) => setDraft(event.target.value)}
        placeholder={t('Paste the organization DeepSeek key')}
        type='password'
        value={draft}
      />
      <Button
        disabled={trimmed === '' || saving}
        onClick={() => saveMutation.mutate({ deepseekKey: trimmed })}
        size='sm'
        variant='outline'
      >
        {t('Save key')}
      </Button>
      <Button
        disabled={!props.configured || saving}
        onClick={() => saveMutation.mutate({ clearDeepseekKey: true })}
        size='sm'
        variant='outline'
      >
        {t('Clear key')}
      </Button>
      <Button
        disabled={!props.configured || revealMutation.isPending}
        onClick={() => revealMutation.mutate()}
        size='sm'
        variant='ghost'
      >
        {t('Copy key')}
      </Button>
      <span className='text-muted-foreground text-xs'>
        {props.configured
          ? t('Configured: {{preview}}', { preview: props.preview || '****' })
          : t('Not configured')}
      </span>
    </div>
  )
}
