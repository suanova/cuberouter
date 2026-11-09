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

	"github.com/gin-gonic/gin"
)

// ListDshAdminChannels 列出可绑定的 DSH 渠道（含停用渠道与被占用的绑定）。
func ListDshAdminChannels(c *gin.Context) {
	channels, err := service.ListDshChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, channels)
}

// GetDshOrganizationConfig 读取单个组织的 DSH 配置（密钥已脱敏）。
func GetDshOrganizationConfig(c *gin.Context) {
	organizationId, ok := parseDshOrganizationId(c)
	if !ok {
		return
	}
	config, err := service.GetDshOrganizationConfig(organizationId)
	if err != nil {
		writeDshAdminError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, config)
}

type updateDshOrganizationRequest struct {
	ChannelId        int     `json:"channel_id"`
	DeepseekKey      *string `json:"deepseek_key"`
	ClearDeepseekKey bool    `json:"clear_deepseek_key"`
}

// UpdateDshOrganizationConfig 保存组织绑定渠道与（可选）组织 DeepSeek Key。
// 审计只记录 organization_id、channel_id 与是否变更密钥，绝不记录明文。
func UpdateDshOrganizationConfig(c *gin.Context) {
	organizationId, ok := parseDshOrganizationId(c)
	if !ok {
		return
	}
	var req updateDshOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	config, err := service.UpdateDshOrganizationConfig(organizationId, c.GetInt("id"), service.DshOrganizationUpdate{
		ChannelId:        req.ChannelId,
		DeepseekKey:      req.DeepseekKey,
		ClearDeepseekKey: req.ClearDeepseekKey,
	}, organizationAuditRequestMetadata(c))
	if err != nil {
		writeDshAdminError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, config)
}

// RevealDshOrganizationSecret 返回组织 DeepSeek Key 明文，供管理员复制进
// compose 命令。只在显式点击复制时调用。
func RevealDshOrganizationSecret(c *gin.Context) {
	organizationId, ok := parseDshOrganizationId(c)
	if !ok {
		return
	}
	secret, err := service.RevealDshOrganizationSecret(organizationId)
	if err != nil {
		writeDshAdminError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, gin.H{"deepseek_key": secret})
}

func parseDshOrganizationId(c *gin.Context) (int, bool) {
	organizationId, err := strconv.Atoi(c.Param("id"))
	if err != nil || organizationId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid organization id"})
		return 0, false
	}
	return organizationId, true
}

func writeDshAdminError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, service.ErrDshOrganizationNotFound), errors.Is(err, service.ErrDshChannelNotFound):
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{"success": false, "message": err.Error()})
}
