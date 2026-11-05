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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { markPath, type ReferenceMarkup } from '../lib/reference-markup'
import type { StudioAsset } from '../workflow-types'

export function ReferenceMarkPaths({
  marks,
  kind,
  ratio,
  maskOnly = false,
}: Pick<ReferenceMarkup, 'marks' | 'kind'> & {
  ratio: number
  maskOnly?: boolean
}) {
  return marks.map((mark, index) => (
    <path
      key={mark.id ?? index}
      d={markPath(mark, ratio)}
      stroke={kind === 'region' ? 'white' : mark.color}
      strokeWidth={mark.width * Math.min(ratio, 1)}
      strokeLinecap='round'
      strokeLinejoin='round'
      fill={kind === 'region' && mark.tool !== 'brush' ? 'white' : 'none'}
      opacity={maskOnly ? 1 : 0.65}
    />
  ))
}

export function ReferenceThumbnail({
  image,
  drawing,
}: {
  image: StudioAsset
  drawing?: ReferenceMarkup
}) {
  const { t } = useTranslation()
  const [ratio, setRatio] = useState(1)
  return (
    <figure className='bg-muted relative aspect-square overflow-hidden rounded-lg'>
      <img
        src={image.url}
        alt={t('Reference image')}
        className={`size-full ${drawing ? 'object-contain' : 'object-cover'}`}
        onLoad={({ currentTarget }) =>
          setRatio(currentTarget.naturalWidth / currentTarget.naturalHeight)
        }
      />
      {drawing && (
        <svg
          viewBox={`0 0 ${ratio} 1`}
          preserveAspectRatio='xMidYMid meet'
          className='pointer-events-none absolute inset-0 size-full'
          role='img'
          aria-label={t('Applied drawing')}
        >
          <ReferenceMarkPaths
            marks={drawing.marks}
            kind={drawing.kind}
            ratio={ratio}
          />
        </svg>
      )}
    </figure>
  )
}
