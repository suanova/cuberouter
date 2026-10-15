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
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// Private aggregate only. The dispatcher gets capacity, not provider credentials.
func ImageStudioChannelCapacity(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	secret := os.Getenv("IMAGE_STUDIO_RELAY_TOKEN")
	if len(secret) < 32 || strings.TrimSpace(os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL")) == "" {
		c.AbortWithStatusJSON(503, gin.H{"error": "image_studio_channel_disabled"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.GetHeader("Authorization")), []byte("Bearer "+secret)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "dispatcher_auth_required"})
		return
	}
	count, err := middleware.ImageChannelPoolCapacity()
	if err != nil {
		c.AbortWithStatusJSON(503, gin.H{"error": "image_channel_pool_unavailable"})
		return
	}
	c.JSON(200, gin.H{"protocol": "image-studio-capacity-v1", "model": "2.1", "capacity": count})
}
