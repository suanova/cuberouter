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
package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// DshOrganizationInfo 返回当前请求解析到的组织，供前端给新会话固定组织。
// 解析本身由 middleware.DshOrganization 完成（它与提交路径使用同一套规则），
// 这里只把结果按 JSON 返回：自定义响应头在跨域开发环境下前端读不到。
func DshOrganizationInfo(c *gin.Context) {
	organizationId := common.GetContextKeyInt(c, constant.ContextKeyOrganizationId)
	userId := common.GetContextKeyInt(c, constant.ContextKeyActorUserId)
	if organizationId <= 0 || userId <= 0 {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "DSH organization is not available"})
		return
	}
	organization, _, err := service.GetOrganizationForMember(userId, organizationId, false)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "DSH organization is not available"})
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, gin.H{
		"organization_id":   organization.Id,
		"organization_name": organization.Name,
	})
}
