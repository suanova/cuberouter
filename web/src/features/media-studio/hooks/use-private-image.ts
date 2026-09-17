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
import { useEffect, useState } from 'react'

import { useAuthStore } from '@/stores/auth-store'

import { workflowAPI } from '../workflow-api'

export function usePrivateImage(id?: string) {
  const owner = useAuthStore((state) => state.auth.user?.id)
  const [url, setURL] = useState('')
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    const controller = new AbortController()
    let objectURL = ''
    setURL('')
    setFailed(false)
    if (id) {
      workflowAPI
        .asset(id, controller.signal)
        .then((blob) => {
          if (controller.signal.aborted) return
          objectURL = URL.createObjectURL(blob)
          setURL(objectURL)
        })
        .catch(() => {
          if (!controller.signal.aborted) setFailed(true)
        })
    }
    return () => {
      controller.abort()
      if (objectURL) URL.revokeObjectURL(objectURL)
    }
  }, [id, owner])
  return { url, failed }
}
