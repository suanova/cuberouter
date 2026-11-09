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

// DshUploadSettings 汇报附件上传是否可用。未配置共享卷时前端不显示回形针，
// 避免用户选完文件才看到「未配置」。
func DshUploadSettings(c *gin.Context) {
	_, err := service.LoadDshUploadConfig()
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"enabled":   err == nil && c.GetInt("id") > 0,
		"max_bytes": service.DshUploadMaxBytes,
		"max_files": service.DshUploadMaxFiles,
		"accept":    service.DshUploadAccept(),
	})
}

// DshUpload 接收单个附件并落进所属组织的共享卷。每个请求只收一个文件：前端
// 按文件逐个上传，失败时可以只重试那一个。组织由 DshUploadOrganization
// 中间件解析后写进上下文。
func DshUpload(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	userId := c.GetInt("id")
	if userId <= 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	organizationId := common.GetContextKeyInt(c, constant.ContextKeyOrganizationId)
	if organizationId <= 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"message": "DSH uploads require an organization"}})
		return
	}
	// 多留 1 MB 给 multipart 边界与表单头；真正的文件大小在服务层再校验一次。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.DshUploadMaxBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(service.DshUploadMaxBytes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Upload a single file of at most 25 MB."}})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Upload a single file in the file field."}})
		return
	}
	attachment, err := service.SaveDshAttachment(organizationId, userId, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, attachment)
}
