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
	"net"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
)

// SwaggerAPIIPWhitelist 只允许白名单内的客户端 IP 访问 Swagger 文档与 /api/v2
// 对外接口。名单为空表示不启用限制，保持未配置时的既有行为。
func SwaggerAPIIPWhitelist() gin.HandlerFunc {
	return func(c *gin.Context) {
		whitelist := security_setting.GetSecuritySetting().SwaggerAPIIPWhitelist
		if len(whitelist) == 0 {
			c.Next()
			return
		}
		if ip := net.ParseIP(c.ClientIP()); ip != nil && common.IsIpInCIDRList(ip, whitelist) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"code":    "API_IP_WHITELIST_DENIED",
			"message": "Access denied: client IP is not in the API whitelist",
		})
	}
}
