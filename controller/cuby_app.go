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
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetAccountCubyApps answers the Cuby desktop client: which apps may the
// signed-in account use. The answer format is a contract with that client.
//
// While the switch is off it answers HTTP 404, the same as an instance that
// does not have this route at all, so Cuby limits nothing.
func GetAccountCubyApps(c *gin.Context) {
	apps, err := service.ListAccountCubyApps(c.GetInt("id"))
	if err != nil {
		writeCubyAppError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"apps": apps})
}

func ListCubyApps(c *gin.Context) {
	apps, err := service.ListCubyAppsForManagement()
	if err != nil {
		writeCubyAppError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"enabled": operation_setting.IsCubyAppManagementEnabled(),
		"apps":    apps,
	})
}

func UpdateCubyAppSetting(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "enabled is required"})
		return
	}
	if err := service.SetCubyAppManagementEnabled(*req.Enabled); err != nil {
		writeCubyAppError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"enabled": operation_setting.IsCubyAppManagementEnabled()})
}

func CreateCubyApp(c *gin.Context) {
	var req struct {
		AppId       string `json:"app_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	app, err := service.CreateCubyApp(c.GetInt("id"), service.CreateCubyAppRequest{AppId: req.AppId, Name: req.Name, Description: req.Description})
	if err != nil {
		writeCubyAppError(c, err)
		return
	}
	common.ApiSuccess(c, app)
}

func UpdateCubyApp(c *gin.Context) {
	id, ok := parseCubyAppId(c)
	if !ok {
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	app, err := service.UpdateCubyApp(id, service.UpdateCubyAppRequest{Name: req.Name, Description: req.Description, Status: req.Status})
	if err != nil {
		writeCubyAppError(c, err)
		return
	}
	common.ApiSuccess(c, app)
}

func DeleteCubyApp(c *gin.Context) {
	id, ok := parseCubyAppId(c)
	if !ok {
		return
	}
	if err := service.DeleteCubyApp(id); err != nil {
		writeCubyAppError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ListCubyAppGrants(c *gin.Context) {
	id, ok := parseCubyAppId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	grants, total, err := service.ListCubyAppGrants(id, pageInfo)
	if err != nil {
		writeCubyAppError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(grants)
	common.ApiSuccess(c, pageInfo)
}

func CreateCubyAppGrant(c *gin.Context) {
	id, ok := parseCubyAppId(c)
	if !ok {
		return
	}
	var req struct {
		SubjectType string `json:"subject_type"`
		SubjectId   int    `json:"subject_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	grant, err := service.CreateCubyAppGrant(c.GetInt("id"), id, req.SubjectType, req.SubjectId)
	if err != nil {
		writeCubyAppError(c, err)
		return
	}
	common.ApiSuccess(c, grant)
}

func DeleteCubyAppGrant(c *gin.Context) {
	id, ok := parseCubyAppId(c)
	if !ok {
		return
	}
	grantId, err := strconv.Atoi(c.Param("grantId"))
	if err != nil || grantId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid cuby app grant id"})
		return
	}
	if err := service.DeleteCubyAppGrant(id, grantId); err != nil {
		writeCubyAppError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func parseCubyAppId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid cuby app id"})
		return 0, false
	}
	return id, true
}

func writeCubyAppError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, service.ErrCubyAppsNotManaged),
		errors.Is(err, service.ErrCubyAppNotFound),
		errors.Is(err, service.ErrCubyAppGrantNotFound),
		errors.Is(err, service.ErrCubyAppHolderNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrCubyAppIdTaken),
		errors.Is(err, service.ErrCubyAppGrantExists):
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{"success": false, "message": err.Error()})
}
