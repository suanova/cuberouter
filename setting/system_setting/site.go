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
package system_setting

import "github.com/QuantumNous/new-api/setting/config"

// SiteSettings 描述站点之间的互跳配置。
// 中国大陆（.cn）与香港（.com）为独立部署，各自配置对端站点的地址与文案，
// 用于在页头渲染"站点切换"入口。
type SiteSettings struct {
	// CounterpartEnabled 站点切换入口的总开关。关闭时不展示入口，保留已填写的对端地址。
	CounterpartEnabled bool `json:"counterpart_enabled"`
	// CounterpartURL 对端站点的根地址，例如 https://cuberouter.com。为空时不展示切换入口。
	CounterpartURL string `json:"counterpart_url"`
	// CounterpartLabel 切换入口上展示的对端站点文案，例如"香港 · 国际"。
	CounterpartLabel string `json:"counterpart_label"`
}

var defaultSiteSettings = SiteSettings{
	// 默认关闭：需要站点互跳时由管理员在后台显式打开。
	CounterpartEnabled: false,
	CounterpartURL:     "",
	CounterpartLabel:   "",
}

func init() {
	config.GlobalConfig.Register("site", &defaultSiteSettings)
}

func GetSiteSettings() *SiteSettings {
	return &defaultSiteSettings
}
