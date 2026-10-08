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
// 端口列表写进 fetch_setting.allowed_ports 时必须保持字符串形态：后端把它解码进
// Go 的 []string（FetchSetting.AllowedPorts），数字数组会被 json.Unmarshal 拒绝，
// 而配置加载器会静默保留默认值 —— 界面上显示的允许端口因此从不生效。
// 同时不能把端口段（"8000-9000"）解析成起始端口，它由 common/ssrf_protection.go
// 的 parsePortRanges 展开成整个区间。
export const splitAllowedPorts = (value: string): string[] =>
  value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
