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
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetSelfOrganizationTokens returns the organization keys the caller can use.
// Unlike the organization key page this is not scoped to one organization, and
// unlike the personal key list it includes the organization keys the caller
// holds or may use publicly.
func GetSelfOrganizationTokens(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的身份信息")
		return
	}
	pageInfo := common.GetPageQuery(c)
	status, _ := strconv.Atoi(c.Query("status"))
	organizationId, _ := strconv.Atoi(c.Query("organization_id"))
	tokens, total, err := service.ListSelfOrganizationTokens(userId, service.SelfOrganizationTokenListRequest{
		Offset:         pageInfo.GetStartIdx(),
		Limit:          pageInfo.GetPageSize(),
		Keyword:        c.Query("keyword"),
		Token:          c.Query("token"),
		Status:         status,
		Visibility:     c.Query("visibility"),
		OrganizationID: organizationId,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tokens)
	common.ApiSuccess(c, pageInfo)
}
