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
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// ImageStudioGroupAuth must follow UserAuth. Read current account membership:
// session group snapshots and request headers cannot grant Studio access.
func ImageStudioGroupAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		id := c.GetInt("id")
		if id <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "login_required"})
			return
		}
		user, err := model.GetUserCache(id)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "image_studio_access_unavailable"})
			return
		}
		if user.Status != common.UserStatusEnabled || !model.ImageStudioGroupAllowed(user.Group) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "image_studio_access_denied"})
			return
		}
		c.Next()
	}
}
