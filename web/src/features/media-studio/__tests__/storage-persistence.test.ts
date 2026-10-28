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
import { afterEach, expect, test, vi } from 'vitest'

import { requestStudioPersistence } from '../lib/storage-persistence'

const FLAG = 'cuberouter-studio-persistence-requested'
const original = Object.getOwnPropertyDescriptor(navigator, 'storage')

/** jsdom 没有 StorageManager，测试里按用例替换成浏览器可能暴露的几种形态。 */
function setStorage(value: unknown) {
  Object.defineProperty(navigator, 'storage', { configurable: true, value })
}
afterEach(() => {
  localStorage.clear()
  if (original) Object.defineProperty(navigator, 'storage', original)
  else Reflect.deleteProperty(navigator, 'storage')
})
test('an origin without the storage API resolves false instead of throwing', async () => {
  // crypto/storage 这类 API 只在安全上下文暴露，http://<ip>:<port> 上就是 undefined。
  setStorage(undefined)
  await expect(requestStudioPersistence()).resolves.toBe(false)
  setStorage({ persist: vi.fn() })
  await expect(requestStudioPersistence()).resolves.toBe(false)
})
test('already persisted storage is left alone', async () => {
  const persist = vi.fn().mockResolvedValue(true)
  setStorage({ persisted: vi.fn().mockResolvedValue(true), persist })
  await expect(requestStudioPersistence()).resolves.toBe(true)
  expect(persist).not.toHaveBeenCalled()
})
test('the first call asks the browser once and later calls reuse the marker', async () => {
  const persist = vi.fn().mockResolvedValue(true)
  setStorage({ persisted: vi.fn().mockResolvedValue(false), persist })
  await expect(requestStudioPersistence()).resolves.toBe(true)
  expect(persist).toHaveBeenCalledTimes(1)
  expect(localStorage.getItem(FLAG)).toBe('1')
  // 打开 IndexedDB 的每条路径都会触发一次，标记位保证不会反复打扰用户。
  await expect(requestStudioPersistence()).resolves.toBe(false)
  expect(persist).toHaveBeenCalledTimes(1)
})
test('a refusal or a throwing API is reported as false, never as an error', async () => {
  setStorage({
    persisted: vi.fn().mockResolvedValue(false),
    persist: vi.fn().mockResolvedValue(false),
  })
  await expect(requestStudioPersistence()).resolves.toBe(false)
  localStorage.clear()
  setStorage({
    persisted: vi.fn().mockRejectedValue(new Error('denied')),
    persist: vi.fn(),
  })
  await expect(requestStudioPersistence()).resolves.toBe(false)
})
