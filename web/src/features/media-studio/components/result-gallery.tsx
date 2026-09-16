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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { GenerationResult } from '../types'

interface ResultGalleryProps {
  result: GenerationResult
}

export function ResultGallery({ result }: ResultGalleryProps) {
  const { t } = useTranslation()
  const [imageSize, setImageSize] = useState<[number, number] | null>(null)

  const handleLoaded = (event: React.SyntheticEvent<HTMLImageElement>) => {
    const img = event.currentTarget
    if (imageSize === null && img.naturalWidth > 0) {
      setImageSize([img.naturalWidth, img.naturalHeight])
    }
  }

  const isSingle = result.images.length === 1

  if (isSingle) {
    return (
      <figure className='flex flex-col gap-3'>
        <img
          src={result.images[0].url}
          alt={t('Generated image')}
          onLoad={handleLoaded}
          className='max-h-[60vh] w-auto max-w-full rounded-lg border border-border bg-muted object-contain'
        />
        <figcaption className='flex items-center justify-between gap-2'>
          {sizeText(imageSize)}
          <a
            href={result.images[0].url}
            target='_blank'
            rel='noopener noreferrer'
            className='text-sm text-primary underline-offset-4 hover:underline'
            download
          >
            {t('Download')}
          </a>
        </figcaption>
      </figure>
    )
  }

  return (
    <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
      {result.images.map((image, index) => (
        <figure
          key={image.url}
          className='flex flex-col gap-1.5'
        >
          <img
            src={image.url}
            alt={t('Generated image')}
            onLoad={handleLoaded}
            className='aspect-auto w-full rounded-lg border border-border bg-muted object-contain'
          />
          <figcaption className='flex items-center justify-between gap-2 text-[11px] text-muted-foreground'>
            <span>
              {index + 1} / {result.images.length}
              {image.seed !== undefined ? ` · Seed ${image.seed}` : ''}
            </span>
            <a
              href={image.url}
              target='_blank'
              rel='noopener noreferrer'
              className='text-primary underline-offset-4 hover:underline'
              download
            >
              {t('Download')}
            </a>
          </figcaption>
        </figure>
      ))}
    </div>
  )
}

function sizeText(size: [number, number] | null): string {
  if (size === null) {
    return ''
  }
  return `${size[0]} × ${size[1]}`
}
