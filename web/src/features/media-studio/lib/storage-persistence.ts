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
const REQUESTED_KEY = 'cuberouter-studio-persistence-requested'

/**
 * 请求浏览器把本站存储标记为持久化，降低图片/视频历史在存储压力下被整块回收的概率。
 * 该 API 只在安全上下文（HTTPS 或 localhost）暴露，浏览器也可以直接拒绝，所以全部
 * 调用都走可选链；失败只影响历史留存，不影响生成，调用方不应把它当作错误。
 * 标记位保证每个浏览器只尝试一次，用户清空站点数据后会重新询问。
 */
export async function requestStudioPersistence(): Promise<boolean> {
  const storage = navigator.storage
  if (!storage?.persisted || !storage.persist) {
    return false
  }
  try {
    if (await storage.persisted()) {
      return true
    }
    if (localStorage.getItem(REQUESTED_KEY)) {
      return false
    }
    localStorage.setItem(REQUESTED_KEY, '1')
    return await storage.persist()
  } catch {
    return false
  }
}
