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
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetOrganizationGroups 获取组织可用分组
//
// @Summary      获取组织可用分组
// @Description  返回组织当前可用的模型分组列表
// @Tags         组织
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/groups [get]
func GetOrganizationGroups(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	groups, err := service.GetOrganizationGroups(c.GetInt("id"), organizationId, organizationAccessMode(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}
