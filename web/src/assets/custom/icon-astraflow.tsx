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
/**
 * AstraFlow channel icon.
 * Vectorized from the official favicon at https://astraflow.ucloud.cn/
 * (a solid five-pointed star in indigo). "Astra" is Latin for "star", hence
 * the star mark. Do not fall back to the OpenAI icon for channel type 59.
 */
import { useId, type SVGProps } from 'react'

type IconAstraflowProps = SVGProps<SVGSVGElement> & {
  size?: number
}

export function IconAstraflow({ size = 20, ...props }: IconAstraflowProps) {
  const gradientId = useId()

  return (
    <svg
      xmlns='http://www.w3.org/2000/svg'
      viewBox='0 0 24 24'
      width={size}
      height={size}
      aria-hidden='true'
      {...props}
    >
      <defs>
        <linearGradient
          id={gradientId}
          x1='12'
          y1='1'
          x2='12'
          y2='21'
          gradientUnits='userSpaceOnUse'
        >
          <stop stopColor='#8B9BEA' />
          <stop offset='1' stopColor='#4244F7' />
        </linearGradient>
      </defs>
      <path
        fill={`url(#${gradientId})`}
        d='M12 1 14.47 8.6 22.46 8.6 16 13.3 18.47 20.9 12 16.2 5.53 20.9 8 13.3 1.54 8.6 9.53 8.6 Z'
      />
    </svg>
  )
}
