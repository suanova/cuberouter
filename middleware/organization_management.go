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
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func OrganizationManagementAuth(requiredCapabilities ...string) gin.HandlerFunc {
	return organizationPolicyAuth(service.OrganizationAccessModeManagement, false, requiredCapabilities...)
}

func OrganizationAccountContextAuth(requiredCapabilities ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		organizationId, ok := organizationPolicyPathId(c)
		if !ok {
			return
		}
		accountContext, ctxErr := resolveRequestAccountContext(c, userId)
		if ctxErr != nil {
			abortAccountContext(c, ctxErr.status, ctxErr.code, ctxErr.message)
			return
		}
		if ctxErr := ensureAccountContextPathMatches(c, accountContext); ctxErr != nil {
			abortAccountContext(c, ctxErr.status, ctxErr.code, ctxErr.message)
			return
		}
		writeAccountContextToGin(c, accountContext)
		decision, err := service.GetOrganizationPolicyDecisionForUser(userId, organizationId, service.OrganizationAccessModeWorkspace)
		if err != nil {
			abortOrganizationPolicy(c, http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, err.Error())
			return
		}
		if !decision.Allowed {
			abortOrganizationPolicyDecision(c, decision)
			return
		}
		for _, capability := range requiredCapabilities {
			if !decision.HasCapability(capability) {
				abortOrganizationPolicyCapabilityDenied(c, decision)
				return
			}
		}
		writeOrganizationPolicyContext(c, organizationId, decision)
		c.Next()
	}
}

func OrganizationReadOnlyAuth(requiredCapabilities ...string) gin.HandlerFunc {
	return organizationPolicyAuth(service.OrganizationAccessModeReadOnly, true, requiredCapabilities...)
}

func OrganizationAdminAuth(requiredCapabilities ...string) gin.HandlerFunc {
	return organizationPolicyAuth(service.OrganizationAccessModeAdmin, false, requiredCapabilities...)
}

func OrganizationReadAccessAuth(requiredCapabilities ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		organizationId, ok := organizationPolicyPathId(c)
		if !ok {
			return
		}
		var organization model.Organization
		if err := model.DB.Select("id", "status").Where("id = ?", organizationId).First(&organization).Error; err != nil {
			abortOrganizationPolicy(c, http.StatusNotFound, types.ErrorCodeOrganizationAccessDenied, err.Error())
			return
		}
		accessMode := service.OrganizationAccessModeReadOnly
		if organization.Status == model.OrganizationStatusActive {
			accountContext, ctxErr := resolveRequestAccountContext(c, userId)
			if ctxErr != nil {
				abortAccountContext(c, ctxErr.status, ctxErr.code, ctxErr.message)
				return
			}
			if ctxErr := ensureAccountContextPathMatches(c, accountContext); ctxErr != nil {
				abortAccountContext(c, ctxErr.status, ctxErr.code, ctxErr.message)
				return
			}
			writeAccountContextToGin(c, accountContext)
			accessMode = service.OrganizationAccessModeWorkspace
		}
		decision, err := service.GetOrganizationPolicyDecisionForUser(userId, organizationId, accessMode)
		if err != nil {
			abortOrganizationPolicy(c, http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, err.Error())
			return
		}
		if !decision.Allowed {
			abortOrganizationPolicyDecision(c, decision)
			return
		}
		for _, capability := range requiredCapabilities {
			if !decision.HasCapability(capability) {
				abortOrganizationPolicyCapabilityDenied(c, decision)
				return
			}
		}
		writeOrganizationPolicyContext(c, organizationId, decision)
		c.Next()
	}
}

func organizationPolicyAuth(accessMode string, readOnly bool, requiredCapabilities ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if readOnly && !organizationReadOnlyMethod(c.Request.Method) {
			abortOrganizationPolicy(c, http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
			return
		}
		userId := c.GetInt("id")
		organizationId, ok := organizationPolicyPathId(c)
		if !ok {
			return
		}
		decision, err := service.GetOrganizationPolicyDecisionForUser(userId, organizationId, accessMode)
		if err != nil {
			abortOrganizationPolicy(c, http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, err.Error())
			return
		}
		if !decision.Allowed {
			abortOrganizationPolicyDecision(c, decision)
			return
		}
		for _, capability := range requiredCapabilities {
			if !decision.HasCapability(capability) {
				abortOrganizationPolicyCapabilityDenied(c, decision)
				return
			}
		}
		writeOrganizationPolicyContext(c, organizationId, decision)
		c.Next()
	}
}

func organizationPolicyPathId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid organization id"})
		c.Abort()
		return 0, false
	}
	return id, true
}

func organizationReadOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func writeOrganizationPolicyContext(c *gin.Context, organizationId int, decision service.OrganizationPolicyDecision) {
	common.SetContextKey(c, constant.ContextKeyOrganizationId, organizationId)
	common.SetContextKey(c, constant.ContextKeyOrganizationRole, decision.Role)
	common.SetContextKey(c, constant.ContextKeyOrganizationAccessMode, decision.AccessMode)
	common.SetContextKey(c, constant.ContextKeyOrganizationCapabilities, decision.Capabilities)
}

func organizationPolicyDecisionCode(decision service.OrganizationPolicyDecision) types.ErrorCode {
	if decision.Code != "" {
		return decision.Code
	}
	return types.ErrorCodeOrganizationAccessDenied
}

func organizationPolicyDecisionMessage(decision service.OrganizationPolicyDecision) string {
	if decision.Message != "" {
		return decision.Message
	}
	return "organization access denied"
}

func abortOrganizationPolicyDecision(c *gin.Context, decision service.OrganizationPolicyDecision) {
	status := http.StatusForbidden
	code := organizationPolicyDecisionCode(decision)
	if code == types.ErrorCodeOrganizationDissolved {
		status = http.StatusGone
	}
	abortOrganizationPolicy(c, status, code, organizationPolicyDecisionMessage(decision))
}

func abortOrganizationPolicyCapabilityDenied(c *gin.Context, decision service.OrganizationPolicyDecision) {
	if decision.Code == types.ErrorCodeOrganizationDissolved {
		abortOrganizationPolicyDecision(c, decision)
		return
	}
	abortOrganizationPolicy(c, http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
}

func abortOrganizationPolicy(c *gin.Context, status int, code types.ErrorCode, message string) {
	c.JSON(status, gin.H{"success": false, "message": message, "code": code})
	c.Abort()
}
