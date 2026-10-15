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

import "github.com/gin-gonic/gin"

// Polling budgets are separate from login and shared-office settings requests.
func ImageStudioIPRateLimit() func(c *gin.Context)   { return rateLimitFactory(2400, 60, "ISIP") }
func ImageStudioUserRateLimit() func(c *gin.Context) { return userRateLimitFactory(120, 60, "ISU") }
