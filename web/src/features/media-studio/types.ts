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
export type AspectRatio = '1:1' | '16:9' | '9:16' | '4:3' | '3:4' | '3:2' | '2:3'

export interface StudioParams {
  prompt: string
  ratio: AspectRatio
  count: number
  steps: number
  seed: number
  cfg: number
}

export interface GeneratedImage {
  url: string
  seed?: number
}

export interface GenerationResult {
  created: number
  images: GeneratedImage[]
  raw: unknown
}

export type GenerationStatus = 'idle' | 'generating' | 'success' | 'error'
