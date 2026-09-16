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
import { useCallback, useEffect, useRef, useState } from 'react'
import { isAxiosError } from 'axios'

import { generateImages } from '../api'
import {
  buildGenerationRequest,
  type GenerationRequestBody,
} from '../lib/request-builder'
import {
  extractGenerationError,
  type GenerationErrorInfo,
} from '../lib/errors'
import type { GenerationResult, GenerationStatus, StudioParams } from '../types'

const ELAPSED_TICK_MS = 1000

interface UseGenerationReturn {
  status: GenerationStatus
  elapsedMs: number
  result: GenerationResult | null
  error: GenerationErrorInfo | null
  requestBody: GenerationRequestBody | null
  rawResponse: unknown
  start: (params: StudioParams, model: string) => Promise<void>
  reset: () => void
}

/**
 * 同步生成的状态机：idle → generating → success | error。
 * 生成期间每秒刷新一次已用时长；成功/失败后停止计时。
 */
export function useGeneration(): UseGenerationReturn {
  const [status, setStatus] = useState<GenerationStatus>('idle')
  const [elapsedMs, setElapsedMs] = useState(0)
  const [result, setResult] = useState<GenerationResult | null>(null)
  const [error, setError] = useState<GenerationErrorInfo | null>(null)
  const [requestBody, setRequestBody] = useState<GenerationRequestBody | null>(null)
  const [rawResponse, setRawResponse] = useState<unknown>(null)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const generatingRef = useRef(false)

  const stopTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
  }, [])

  useEffect(() => {
    return () => {
      stopTimer()
    }
  }, [stopTimer])

  const start = useCallback(
    async (params: StudioParams, model: string) => {
      if (generatingRef.current) {
        return
      }
      const body = buildGenerationRequest(params, model)
      generatingRef.current = true
      setRequestBody(body)
      setRawResponse(null)
      setResult(null)
      setError(null)
      setElapsedMs(0)
      setStatus('generating')

      const startedAt = Date.now()
      timerRef.current = setInterval(() => {
        setElapsedMs(Date.now() - startedAt)
      }, ELAPSED_TICK_MS)

      try {
        const apiResult = await generateImages(body)
        setRawResponse(apiResult.raw)
        setResult({
          created: apiResult.created,
          images: apiResult.images,
          raw: apiResult.raw,
        })
        setStatus('success')
      } catch (err) {
        setRawResponse(
          isAxiosError(err) ? err.response?.data : (err as Error | undefined)?.message,
        )
        setError(extractGenerationError(err))
        setStatus('error')
      } finally {
        stopTimer()
        setElapsedMs(Date.now() - startedAt)
        generatingRef.current = false
      }
    },
    [stopTimer],
  )

  const reset = useCallback(() => {
    stopTimer()
    setStatus('idle')
    setElapsedMs(0)
    setResult(null)
    setError(null)
    setRequestBody(null)
    setRawResponse(null)
  }, [stopTimer])

  return {
    status,
    elapsedMs,
    result,
    error,
    requestBody,
    rawResponse,
    start,
    reset,
  }
}
