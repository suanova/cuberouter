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
package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// setCubyAppApiRoutes registers which Cuby apps (plug-ins) an account may use.
//
// /account/apps is read by the Cuby desktop client with the signed-in account's
// access token; its path and answer format are a contract with that client.
// The /admin routes are where a platform administrator registers apps and gives
// them to users and organizations.
func setCubyAppApiRoutes(apiRouter *gin.RouterGroup) {
	apiRouter.GET("/account/apps", middleware.UserAuth(), middleware.DisableCache(), controller.GetAccountCubyApps)

	adminCubyAppRoute := apiRouter.Group("/admin/cuby-apps")
	adminCubyAppRoute.Use(middleware.AdminAuth())
	{
		adminCubyAppRoute.GET("", controller.ListCubyApps)
		adminCubyAppRoute.POST("", controller.CreateCubyApp)
		adminCubyAppRoute.PATCH("/:id", controller.UpdateCubyApp)
		adminCubyAppRoute.DELETE("/:id", controller.DeleteCubyApp)
		adminCubyAppRoute.GET("/:id/grants", controller.ListCubyAppGrants)
		adminCubyAppRoute.POST("/:id/grants", controller.CreateCubyAppGrant)
		adminCubyAppRoute.DELETE("/:id/grants/:grantId", controller.DeleteCubyAppGrant)
	}
	apiRouter.PUT("/admin/cuby-app-setting", middleware.AdminAuth(), controller.UpdateCubyAppSetting)
}
