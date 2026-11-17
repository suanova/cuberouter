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
package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// CubyAppSetting decides whether this instance tells the Cuby desktop client
// which of its apps (plug-ins) an account may use.
//
// Off is the default on purpose. A Cuby client treats "this instance does not
// manage apps" (HTTP 404 on /api/account/apps) as "no limit", but an empty list
// as "this account has no apps". Shipping the route switched on would therefore
// lock every app of every signed-in Cuby user until grants exist. An
// administrator registers the apps and the grants first, then switches it on.
type CubyAppSetting struct {
	Enabled bool `json:"enabled"`
}

var cubyAppSetting = CubyAppSetting{
	Enabled: false,
}

func init() {
	config.GlobalConfig.Register("cuby_app_setting", &cubyAppSetting)
}

// CubyAppSettingEnabledOptionKey is the option key that persists the switch.
const CubyAppSettingEnabledOptionKey = "cuby_app_setting.enabled"

func IsCubyAppManagementEnabled() bool {
	return cubyAppSetting.Enabled
}
