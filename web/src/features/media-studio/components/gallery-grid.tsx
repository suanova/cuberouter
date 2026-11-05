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
import type { ReactNode } from 'react'

export function GalleryGrid(props: { label: string; children: ReactNode }) {
  return (
    <section
      aria-label={props.label}
      tabIndex={0}
      className='focus-visible:outline-ring @container/gallery h-[min(55dvh,26rem)] [scrollbar-gutter:stable] overflow-y-auto p-0.5 focus-visible:outline-2 focus-visible:outline-offset-2 lg:h-[clamp(15rem,calc(100dvh-25rem),30rem)] lg:overscroll-y-contain'
    >
      <div className='grid auto-rows-fr grid-cols-2 gap-3 @md/gallery:grid-cols-3 @xl/gallery:grid-cols-4 @3xl/gallery:grid-cols-5'>
        {props.children}
      </div>
    </section>
  )
}
