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
import { useTranslation } from 'react-i18next'

import { usePrivateImage } from '../hooks/use-private-image'

export function PrivateImage(props: {
  id: string
  alt: string
  className?: string
}) {
  const { t } = useTranslation()
  const image = usePrivateImage(props.id)
  if (!image.url) {
    return (
      <div
        role='status'
        className='bg-muted text-muted-foreground flex min-h-24 items-center justify-center rounded-xl p-4 text-xs'
      >
        {t(image.failed ? 'Image unavailable or expired' : 'Loading image…')}
      </div>
    )
  }
  return (
    <img
      src={image.url}
      alt={props.alt}
      className={props.className ?? 'w-full rounded-xl object-contain'}
      loading='lazy'
    />
  )
}
