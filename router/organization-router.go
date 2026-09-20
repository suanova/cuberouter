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
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func setOrganizationApiRoutes(apiRouter *gin.RouterGroup) {
	organizationRoute := apiRouter.Group("/organizations")
	organizationRoute.Use(middleware.UserAuth())
	{
		organizationRoute.GET("", controller.ListSelfOrganizations)
		organizationRoute.GET("/self", controller.ListSelfOrganizations)
		organizationRoute.POST("", controller.CreateOrganization)
		organizationRoute.GET("/:id", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganization), controller.GetOrganization)
		organizationRoute.PATCH("/:id", middleware.OrganizationManagementAuth(service.OrganizationCapabilityUpdateOrganization), controller.UpdateOrganization)
		organizationRoute.PATCH("/:id/status", middleware.OrganizationManagementAuth(), controller.UpdateOrganizationStatus)
		organizationRoute.DELETE("/:id", middleware.OrganizationManagementAuth(service.OrganizationCapabilityDissolveOrganization), controller.DissolveOrganization)
		organizationRoute.PUT("/:id/owner", middleware.OrganizationManagementAuth(), controller.TransferOrganizationOwner)
		organizationRoute.GET("/:id/groups", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganization), controller.GetOrganizationGroups)
		organizationRoute.GET("/:id/members", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganization), controller.ListOrganizationMembers)
		organizationRoute.PATCH("/:id/members/:userId", middleware.OrganizationManagementAuth(service.OrganizationCapabilityManageMembers), controller.UpdateOrganizationMember)
		organizationRoute.DELETE("/:id/members/me", middleware.OrganizationAccountContextAuth(service.OrganizationCapabilityExitOrganization), controller.ExitOrganization)
		organizationRoute.DELETE("/:id/members/:userId", middleware.OrganizationManagementAuth(service.OrganizationCapabilityManageMembers), controller.RemoveOrganizationMember)
		organizationRoute.GET("/:id/invitations", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewInvitations), controller.ListOrganizationInvites)
		organizationRoute.GET("/:id/tokens", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationTokens), controller.ListOrganizationTokens)
		organizationRoute.GET("/:id/tokens/:tokenId", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationTokens), controller.GetOrganizationToken)
		organizationRoute.GET("/:id/quota-data", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.GetOrganizationQuotaData)
		organizationRoute.GET("/:id/logs", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationLogs), controller.ListOrganizationLogs)
		organizationRoute.GET("/:id/logs/stats", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationLogs), controller.GetOrganizationLogStats)
		organizationRoute.GET("/:id/tasks", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationLogs), controller.ListOrganizationTasks)
		organizationRoute.GET("/:id/midjourney-tasks", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationLogs), controller.ListOrganizationMidjourneyTasks)
		organizationRoute.GET("/:id/billing/summary", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewMembersFull, service.OrganizationCapabilityViewOrganizationUsage), controller.GetOrganizationBillingSummary)
		organizationRoute.GET("/:id/billing/members/me", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.GetMyOrganizationMemberBilling)
		organizationRoute.GET("/:id/billing/user-summaries", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.ListOrganizationBillingUserSummaries)
		organizationRoute.GET("/:id/billing/monthly-summaries", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.ListOrganizationBillingMonthlySummaries)
		organizationRoute.GET("/:id/billing/records", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.ListOrganizationBillingDetails)
		organizationRoute.GET("/:id/audit-logs", middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganizationAuditLogs), controller.ListOrganizationAuditLogs)
	}

	organizationAccountContextRoute := apiRouter.Group("/organizations")
	organizationAccountContextRoute.Use(middleware.UserAuth(), middleware.AccountContext(), middleware.OrganizationAccountContextAuth())
	{
		organizationAccountContextRoute.POST("/:id/invitations", controller.CreateOrganizationInvite)
		organizationAccountContextRoute.DELETE("/:id/invitations/:invitationId", controller.RevokeOrganizationInvite)
		organizationAccountContextRoute.POST("/:id/tokens", controller.CreateOrganizationToken)
		organizationAccountContextRoute.POST("/:id/token-batches", controller.BatchCreateOrganizationTokens)
		organizationAccountContextRoute.POST("/:id/token-deletions", controller.BatchDeleteOrganizationTokens)
		organizationAccountContextRoute.PATCH("/:id/tokens/:tokenId", controller.UpdateOrganizationToken)
		organizationAccountContextRoute.DELETE("/:id/tokens/:tokenId", controller.DeleteOrganizationToken)
		organizationAccountContextRoute.PATCH("/:id/tokens/:tokenId/responsible-user", controller.UpdateOrganizationTokenResponsibility)
	}

	apiRouter.GET("/organization-invitations/:token", middleware.TryUserAuth(), controller.GetOrganizationInvite)
	apiRouter.PATCH("/organization-invitations/:token", middleware.UserAuth(), controller.AcceptOrganizationInvite)

	adminOrganizationRoute := apiRouter.Group("/admin/organizations")
	adminOrganizationRoute.Use(middleware.AdminAuth())
	{
		adminOrganizationRoute.GET("", controller.ListOrganizations)
		adminOrganizationRoute.GET("/:id", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganization), controller.GetOrganization)
		adminOrganizationRoute.PATCH("/:id", middleware.OrganizationAdminAuth(service.OrganizationCapabilityUpdateOrganization), controller.UpdateOrganization)
		adminOrganizationRoute.PATCH("/:id/status", middleware.OrganizationAdminAuth(), controller.UpdateOrganizationPlatformStatus)
		adminOrganizationRoute.DELETE("/:id", middleware.OrganizationAdminAuth(service.OrganizationCapabilityDissolveOrganization), controller.DissolveOrganization)
		adminOrganizationRoute.PUT("/:id/owner", middleware.OrganizationAdminAuth(), controller.TransferOrganizationOwner)
		adminOrganizationRoute.POST("/:id/quota-adjustments", middleware.OrganizationAdminAuth(service.OrganizationCapabilityAdjustOrganizationQuota), controller.AdjustOrganizationQuota)
		adminOrganizationRoute.GET("/:id/members", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganization), controller.ListOrganizationMembers)
		adminOrganizationRoute.POST("/:id/members", middleware.OrganizationAdminAuth(service.OrganizationCapabilityManageMembers), controller.AddOrganizationMember)
		adminOrganizationRoute.PATCH("/:id/members/:userId", middleware.OrganizationAdminAuth(service.OrganizationCapabilityManageMembers), controller.UpdateOrganizationMember)
		adminOrganizationRoute.DELETE("/:id/members/:userId", middleware.OrganizationAdminAuth(service.OrganizationCapabilityManageMembers), controller.RemoveOrganizationMember)
		adminOrganizationRoute.GET("/:id/tokens", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationTokens), controller.ListOrganizationTokens)
		adminOrganizationRoute.POST("/:id/token-deletions", middleware.OrganizationAdminAuth(service.OrganizationCapabilityManageOrganizationTokens), controller.BatchDeleteOrganizationTokens)
		adminOrganizationRoute.GET("/:id/tokens/:tokenId", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationTokens), controller.GetOrganizationToken)
		adminOrganizationRoute.PATCH("/:id/tokens/:tokenId", middleware.OrganizationAdminAuth(service.OrganizationCapabilityManageOrganizationTokens), controller.UpdateOrganizationToken)
		adminOrganizationRoute.DELETE("/:id/tokens/:tokenId", middleware.OrganizationAdminAuth(service.OrganizationCapabilityManageOrganizationTokens), controller.DeleteOrganizationToken)
		adminOrganizationRoute.PATCH("/:id/tokens/:tokenId/responsible-user", middleware.OrganizationAdminAuth(service.OrganizationCapabilityManageOrganizationTokens), controller.UpdateOrganizationTokenResponsibility)
		adminOrganizationRoute.GET("/:id/logs", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationLogs), controller.ListOrganizationLogs)
		adminOrganizationRoute.GET("/:id/logs/stats", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationLogs), controller.GetOrganizationLogStats)
		adminOrganizationRoute.GET("/:id/tasks", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationLogs), controller.ListOrganizationTasks)
		adminOrganizationRoute.GET("/:id/midjourney-tasks", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationLogs), controller.ListOrganizationMidjourneyTasks)
		adminOrganizationRoute.GET("/:id/quota-data", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.GetOrganizationQuotaData)
		adminOrganizationRoute.GET("/:id/billing/summary", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.GetOrganizationBillingSummary)
		adminOrganizationRoute.GET("/:id/billing/user-summaries", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.ListOrganizationBillingUserSummaries)
		adminOrganizationRoute.GET("/:id/billing/monthly-summaries", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.ListOrganizationBillingMonthlySummaries)
		adminOrganizationRoute.GET("/:id/billing/records", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationUsage), controller.ListOrganizationBillingDetails)
		adminOrganizationRoute.GET("/:id/audit-logs", middleware.OrganizationAdminAuth(service.OrganizationCapabilityViewOrganizationAuditLogs), controller.ListOrganizationAuditLogs)
	}

	adminOrganizationAuditRoute := apiRouter.Group("/admin/organization-audit-logs")
	adminOrganizationAuditRoute.Use(middleware.AdminAuth())
	{
		adminOrganizationAuditRoute.GET("/", controller.ListAllOrganizationAuditLogs)
	}
}
