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
import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import { afterEach, beforeAll } from 'vitest'

beforeAll(async () => {
  await i18next.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: {
      en: {
        translation: {},
      },
    },
  })
})

afterEach(() => {
  cleanup()
})

Object.defineProperty(window, 'matchMedia', {
  configurable: true,
  value: (query: string): MediaQueryList => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
})

// Node >= 25 在 global 上定义了被禁用的 localStorage/sessionStorage getter
// (未传 --localstorage-file 时读值为 undefined)。vitest 的 jsdom 环境对已
// 存在于 Node global 的 key 不再挂 window 侧实现,zustand persist 因此拿到
// undefined storage 直接崩溃。这里显式用 jsdom 的实现覆盖;旧版 Node 上
// global 已是 jsdom 的(非 undefined),此块为无操作。
type JSDOMWindowLike = {
  localStorage?: Storage
  sessionStorage?: Storage
}
const jsdomWindowLike = (
  globalThis as { jsdom?: { window?: JSDOMWindowLike } }
).jsdom?.window
for (const storageKey of ['localStorage', 'sessionStorage'] as const) {
  const domStorage = jsdomWindowLike?.[storageKey]
  if (
    domStorage &&
    typeof (globalThis as Record<string, unknown>)[storageKey] === 'undefined'
  ) {
    Object.defineProperty(globalThis, storageKey, {
      configurable: true,
      value: domStorage,
    })
  }
}

window.requestAnimationFrame = (callback: FrameRequestCallback) =>
  window.setTimeout(() => callback(performance.now()), 0)
window.cancelAnimationFrame = (handle: number) => window.clearTimeout(handle)

class ResizeObserverMock {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

Object.defineProperty(globalThis, 'ResizeObserver', {
  configurable: true,
  value: ResizeObserverMock,
})

Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
  configurable: true,
  value: () => undefined,
})
