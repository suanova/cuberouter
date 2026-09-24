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
import axios, { type AxiosRequestConfig } from 'axios'
import { t } from 'i18next'
import { toast } from 'sonner'

import {
  applyAuthRotation,
  clearAuthentication,
  refreshAuthentication,
} from '@/lib/auth-session'
import {
  getServerErrorCode,
  getServerErrorMessageKey,
} from '@/lib/server-error-message'
import {
  getAccountContextCacheKey,
  getAccountContextHeaders,
  useAccountContextStore,
} from '@/stores/account-context-store'
import { useAuthStore } from '@/stores/auth-store'

declare module 'axios' {
  export interface AxiosRequestConfig {
    skipBusinessError?: boolean
    skipErrorHandler?: boolean
    disableDuplicate?: boolean
    skipAuthRefresh?: boolean
    authRetry?: boolean
    acceptAuthRotation?: boolean
    skipAccountContext?: boolean
  }
}

export type ApiRequestConfig = AxiosRequestConfig

export const api = axios.create({
  baseURL: '',
  withCredentials: true,
  headers: {
    // no-store forbids storage; no-cache also revalidates any older cached response.
    'Cache-Control': 'no-cache, no-store',
  },
})

const inFlightGet = new Map<string, Promise<unknown>>()
const originalGet = api.get.bind(api)

api.get = ((url: string, config: ApiRequestConfig = {}) => {
  if (config.disableDuplicate) return originalGet(url, config)

  const params = config.params ? JSON.stringify(config.params) : '{}'
  const sessionSID = useAuthStore.getState().auth.session?.sid || 'anonymous'
  // 账号上下文走请求头，不在 url/params 里，所以必须并进去：否则切到组织后立刻
  // 发出的同 URL 请求会命中个人上下文那次还在途中的请求。
  const accountContextKey = getAccountContextCacheKey()
  const key = `${sessionSID}:${accountContextKey}:${url}?${params}`
  const existingRequest = inFlightGet.get(key)
  if (existingRequest) return existingRequest

  const request = originalGet(url, config).finally(() => {
    inFlightGet.delete(key)
  })
  inFlightGet.set(key, request)
  return request
}) as typeof api.get

/**
 * 平台管理接口是跨组织的，组织由路径里的 id 决定，不由上下文决定。带上上下文反而
 * 会被后端当成另一次权限解析，所以按前缀整体排除。
 */
function isPlatformAdminRequest(url: string | undefined): boolean {
  return typeof url === 'string' && url.startsWith('/api/admin')
}

function redirectToSignIn(): void {
  if (
    typeof window !== 'undefined' &&
    window.location.pathname !== '/sign-in'
  ) {
    window.location.replace('/sign-in')
  }
}

api.interceptors.response.use(
  (response) => {
    if (response.config.acceptAuthRotation && response.data?.success === true) {
      applyAuthRotation(response.data.data)
    }

    if (
      !response.config.skipBusinessError &&
      typeof response.data?.success === 'boolean' &&
      !response.data.success
    ) {
      const messageKey = getServerErrorMessageKey(response.data)
      toast.error(
        messageKey
          ? t(messageKey)
          : response.data.message || t('Request failed')
      )
    }
    return response
  },
  async (error) => {
    const config = error?.config as ApiRequestConfig | undefined
    const skipErrorHandler = config?.skipErrorHandler
    const status = error?.response?.status

    if (status === 401) {
      if (config && !config.skipAuthRefresh && !config.authRetry) {
        config.authRetry = true
        const outcome = await refreshAuthentication()
        if (outcome.kind === 'authenticated') {
          const token = useAuthStore.getState().auth.accessToken
          if (token) {
            config.headers = {
              ...config.headers,
              Authorization: `Bearer ${token}`,
            }
          }
          return api.request(config)
        }

        if (outcome.kind === 'anonymous' || outcome.kind === 'out_of_sync') {
          if (!skipErrorHandler) toast.error(t('Session expired!'))
          redirectToSignIn()
        }
      } else if (config?.authRetry) {
        clearAuthentication(false)
        if (!skipErrorHandler) toast.error(t('Session expired!'))
        redirectToSignIn()
      } else if (!skipErrorHandler) {
        toast.error(t('Session expired!'))
      }
    } else if (!skipErrorHandler) {
      if (getServerErrorCode(error) === 'organization_context_mismatch') {
        // 服务端说本地选的上下文已经不能用了（组织被解散、成员被移出、或者本地存的
        // 上下文和请求路径里的组织对不上）。这里只把本地选中项清回个人，怎么恢复交给
        // 发起请求的页面——组织页知道自己该提示什么、该退到哪一步，拦截器不知道。
        useAccountContextStore.getState().accountContext.setCurrent(null)
      }
      const messageKey = getServerErrorMessageKey(error)
      const message = messageKey
        ? t(messageKey)
        : error?.response?.data?.message ||
          error?.message ||
          t('Request failed')
      toast.error(message)
    }
    throw error
  }
)

api.interceptors.request.use((config) => {
  const accessToken = useAuthStore.getState().auth.accessToken
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`
  }

  // 个人上下文下 getAccountContextHeaders 返回空对象，等同于不声明，后端按个人处理。
  if (!config.skipAccountContext && !isPlatformAdminRequest(config.url)) {
    for (const [name, value] of Object.entries(getAccountContextHeaders())) {
      config.headers[name] = value
    }
  }
  return config
})
