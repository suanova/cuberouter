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
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type setCurrentAccountContextRequest struct {
	Type string `json:"type"`
	Id   int    `json:"id"`
}

// ListAccountContexts 获取当前登录用户可切换的账号上下文列表(个人/组织)
func ListAccountContexts(c *gin.Context) {
	resp, err := service.ListAccountContexts(c.GetInt("id"))
	if err != nil {
		writeAccountContextError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

// SetCurrentAccountContext 切换当前会话的账号上下文(个人/组织)
func SetCurrentAccountContext(c *gin.Context) {
	var req setCurrentAccountContextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	context, err := service.SetCurrentAccountContext(c.GetInt("id"), req.Type, req.Id)
	if err != nil {
		writeAccountContextError(c, err)
		return
	}
	common.ApiSuccess(c, context)
}

func writeAccountContextError(c *gin.Context, err error) {
	var contextErr *service.AccountContextError
	if errors.As(err, &contextErr) {
		c.JSON(contextErr.Status, gin.H{"success": false, "message": contextErr.Message, "code": contextErr.Code})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error(), "code": types.ErrorCodeInvalidRequest})
}
