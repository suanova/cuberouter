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
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// These are protocol capabilities confirmed by the operator, not guessed from model names.
func MediaStudioConfig(c *gin.Context) {
	_, err := service.LoadStudioUploadConfig()
	models := []string{}
	for _, name := range strings.Split(os.Getenv("MEDIA_STUDIO_EDIT_MODELS"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			models = append(models, name)
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"upload_enabled": err == nil, "edit_models": models})
}

func MediaStudioUpload(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if c.GetInt("id") <= 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var request struct {
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid upload request"}})
		return
	}
	if request.Size <= 0 || request.Size > service.StudioUploadMaxBytes || (request.ContentType != "image/png" && request.ContentType != "image/jpeg" && request.ContentType != "image/webp") {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Upload PNG, JPEG or WebP files of at most 10 MB each."}})
		return
	}
	cfg, err := service.LoadStudioUploadConfig()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "Reference uploads are not configured."}})
		return
	}
	ticket, err := service.PresignStudioUpload(c.Request.Context(), cfg, c.GetInt("id"), request.ContentType, request.Size, time.Now())
	if err != nil {
		common.SysError("Media Studio upload signing failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "Could not prepare upload."}})
		return
	}
	c.JSON(http.StatusOK, ticket)
}
