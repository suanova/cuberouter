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
import type { TaskArtifact } from '@/features/usage-logs/types'

const MARKDOWN_LINK_PATTERN = /\]\(([^)\s]+)\)/g

function basenameOf(target: string): string {
  let decoded = target
  try {
    decoded = decodeURIComponent(target)
  } catch {
    // 非法转义时按原样比对。
  }
  const parts = decoded.split(/[\\/]/)
  return parts.at(-1) ?? ''
}

/**
 * 把回复里的文件链接改写到产物的签名下载地址。
 *
 * 智能体习惯用 `[报告.pptx](/data/workspace/报告.pptx)` 指代自己写出的文件，这个
 * 路径只存在于 dsh 容器里；页面按网关域名解析后必然 404。产物清单带有真实文件名
 * 和 capability URL，只要按文件名对得上就换成可下载的地址，其余链接保持原样。
 */
export function rewriteArtifactLinks(
  reply: string,
  artifacts: TaskArtifact[]
): string {
  if (!reply || artifacts.length === 0) {
    return reply
  }

  const byFilename = new Map<string, TaskArtifact>()
  for (const artifact of artifacts) {
    if (artifact.name) {
      byFilename.set(artifact.name, artifact)
    }
    byFilename.set(artifact.key, artifact)
  }

  return reply.replaceAll(
    MARKDOWN_LINK_PATTERN,
    (match: string, target: string) => {
      // 绝对地址（含 mailto: 等协议）和页内锚点不归产物管。
      if (/^[a-z][a-z0-9+.-]*:/i.test(target) || target.startsWith('#')) {
        return match
      }
      const artifact = byFilename.get(basenameOf(target))
      return artifact ? `](${artifact.content_url})` : match
    }
  )
}
