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
import { Globe } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { useSystemConfig } from '@/hooks/use-system-config'
import { isValidCounterpartUrl } from '@/lib/counterpart-url'

/**
 * 站点区域切换入口：跳转到对端站点（如 cuberouter.cn <-> cuberouter.com）。
 * 开关、对端地址与文案来自后台设置（site.counterpart_enabled / site.counterpart_url /
 * site.counterpart_label），开关关闭、地址留空或不是绝对 HTTP(S) 地址时整块不渲染。
 * 跨域跳转在新标签页打开，并保留当前路径与 hash。
 */
export function SiteRegionSwitcher() {
  const { t } = useTranslation()
  const { siteCounterpartEnabled, siteCounterpartUrl, siteCounterpartLabel } =
    useSystemConfig()

  if (!siteCounterpartEnabled || !isValidCounterpartUrl(siteCounterpartUrl)) {
    return null
  }

  // 刻意不带查询串：重置口令、OAuth code 一类的凭据都走 query，
  // 而它们在对端站点（独立部署、独立数据库）本就无效，没有理由带到另一个域上去。
  const url = new URL(siteCounterpartUrl)
  url.pathname = window.location.pathname
  url.hash = window.location.hash
  const href = url.toString()

  const label = siteCounterpartLabel?.trim() || t('Another site')

  return (
    <a
      href={href}
      target='_blank'
      rel='noopener noreferrer'
      aria-label={t('Switch to another site')}
      className='text-muted-foreground hover:text-foreground inline-flex h-9 items-center gap-1.5 rounded-lg px-2 text-sm font-medium transition-colors duration-200 sm:px-3'
    >
      <Globe className='size-[1.2rem]' />
      <span className='hidden sm:inline'>{label}</span>
    </a>
  )
}
