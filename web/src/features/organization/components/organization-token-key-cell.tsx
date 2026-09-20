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
import { Check, Copy, Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { cn } from '@/lib/utils'

import { organizationTokenKeyPreview } from '../lib'
import type { OrganizationTokenRow } from '../types'

/**
 * A key, masked until asked for.
 *
 * The list already carries the secret, so hiding it here is about not putting a
 * credential on screen by default rather than about withholding it — which is
 * also why revealing needs no permission of its own. Copying does: a member who
 * may not take a key away gets a disabled button rather than a silent one.
 */
export function OrganizationTokenKeyCell(props: {
  token: OrganizationTokenRow
  canCopy: boolean
}) {
  const { t } = useTranslation()
  const [revealed, setRevealed] = useState(false)
  const [copied, setCopied] = useState(false)

  const fullKey = (props.token.key ?? '').trim()
  const display = revealed && fullKey
    ? fullKey
    : organizationTokenKeyPreview(props.token)

  if (!display) {
    return <span className='text-muted-foreground'>-</span>
  }

  const handleCopy = async () => {
    if (!props.canCopy || !fullKey) return
    const ok = await copyToClipboard(fullKey)
    if (!ok) {
      toast.error(t('Failed to copy to clipboard'))
      return
    }
    setCopied(true)
    toast.success(t('Copied to clipboard!'))
    window.setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className='flex items-center gap-1'>
      <span
        className={cn(
          'max-w-[160px] truncate font-mono text-xs',
          revealed && 'max-w-[260px]'
        )}
        title={display}
      >
        {display}
      </span>
      <Button
        type='button'
        variant='ghost'
        size='icon-xs'
        aria-label={t('Show or hide key')}
        disabled={!fullKey}
        onClick={() => setRevealed((previous) => !previous)}
      >
        {revealed ? <EyeOff /> : <Eye />}
      </Button>
      <Button
        type='button'
        variant='ghost'
        size='icon-xs'
        aria-label={t('Copy key')}
        disabled={!props.canCopy}
        onClick={() => void handleCopy()}
      >
        {copied ? <Check className='text-success' /> : <Copy />}
      </Button>
    </div>
  )
}
