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
package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// DshAccess gates the DSH Agent dashboard surface (/dsh uploads and /pg/agent
// tasks). It must run after an auth middleware populated the caller identity.
//
// The /pg/agent group currently serves only the DSH agent page; when another
// agent task plugin joins it, this gate must move to plugin-key detection.
func DshAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !service.CanAccessDsh(dshRequesterUserId(c)) {
			abortWithOpenAiMessage(c, http.StatusForbidden, common.TranslateMessage(c, i18n.MsgDshAccessDenied))
			return
		}
		c.Next()
	}
}

// DshSubmitAccess gates task-plugin submits whose resolved plugin is DSH. It
// must run after PrepareTaskPluginSubmit, which stores the resolved key.
//
// It no longer consults CanAccessDsh: for /v1 tasks the token scope is the
// authority. A personal token carries no organization, so DSH submits are
// rejected strictly — administrators and root included.
func DshSubmitAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("expected_task_plugin_key") != setting.DshTaskPluginKey {
			c.Next()
			return
		}
		userId := dshRequesterUserId(c)
		organizationId := common.GetContextKeyInt(c, constant.ContextKeyOrganizationId)
		if userId <= 0 || organizationId <= 0 {
			abortDshOrganizationError(c, service.ErrDshOrganizationRequired)
			return
		}
		_, channelId, err := service.ResolveDshOrganizationForToken(userId, organizationId)
		if err != nil {
			abortDshOrganizationError(c, err)
			return
		}
		AddDshOrganizationPin(c, channelId)
		// TokenAuth 会在管理员令牌 key 带 "-<channelId>" 后缀时加 rank-0 的
		// PinSourceToken；那种 pin 必须在这里暴露出来，否则任务会被引到别的实例。
		pin, found, _ := service.GetChannelConstraints(c).ResolvedPin()
		if !found || pin.Source != dto.PinSourceOrganization || pin.ChannelId != channelId {
			abortDshOrganizationError(c, service.ErrDshChannelUnavailable)
			return
		}
		c.Next()
	}
}

func dshRequesterUserId(c *gin.Context) int {
	userId := common.GetContextKeyInt(c, constant.ContextKeyActorUserId)
	if userId <= 0 {
		userId = c.GetInt("id")
	}
	return userId
}
