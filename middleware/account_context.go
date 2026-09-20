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
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	accountContextTypeHeader = "X-Account-Context-Type"
	accountContextIdHeader   = "X-Account-Context-Id"
)

type requestAccountContext struct {
	Type              string
	Id                int
	OrganizationId    int
	ActorUserId       int
	CreatorUserId     int
	ResponsibleUserId int
	OrganizationQuota int
}

type accountContextError struct {
	status  int
	code    types.ErrorCode
	message string
}

func AccountContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId <= 0 {
			abortAccountContext(c, http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
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
		c.Next()
	}
}

func resolveRequestAccountContext(c *gin.Context, userId int) (*requestAccountContext, *accountContextError) {
	contextType := strings.ToLower(strings.TrimSpace(c.GetHeader(accountContextTypeHeader)))
	if contextType == "" {
		contextType = model.AccountContextTypePersonal
	}
	contextId, ctxErr := parseAccountContextIdHeader(c)
	if ctxErr != nil {
		return nil, ctxErr
	}
	switch contextType {
	case model.AccountContextTypePersonal:
		if contextId != 0 && contextId != userId {
			return nil, &accountContextError{status: http.StatusForbidden, code: types.ErrorCodeOrganizationAccessDenied, message: "personal account context mismatch"}
		}
		return &requestAccountContext{Type: model.AccountContextTypePersonal, Id: userId, ActorUserId: userId, CreatorUserId: userId, ResponsibleUserId: userId}, nil
	case model.AccountContextTypeOrganization:
		if contextId <= 0 {
			return nil, &accountContextError{status: http.StatusBadRequest, code: types.ErrorCodeOrganizationContextMismatch, message: "invalid organization account context"}
		}
		organization, member, err := service.GetOrganizationForMember(userId, contextId, false)
		if err != nil {
			return nil, accountContextErrorFromErr(err)
		}
		if organization.Status != model.OrganizationStatusActive || member.Status != model.OrganizationMemberStatusActive {
			return nil, &accountContextError{status: http.StatusForbidden, code: types.ErrorCodeOrganizationAccessDenied, message: "organization account context unavailable"}
		}
		return &requestAccountContext{Type: model.AccountContextTypeOrganization, Id: organization.Id, OrganizationId: organization.Id, ActorUserId: userId, CreatorUserId: userId, ResponsibleUserId: userId, OrganizationQuota: organization.Quota - organization.UsedQuota}, nil
	default:
		return nil, &accountContextError{status: http.StatusBadRequest, code: types.ErrorCodeOrganizationContextMismatch, message: "invalid account context type"}
	}
}

func parseAccountContextIdHeader(c *gin.Context) (int, *accountContextError) {
	rawId := strings.TrimSpace(c.GetHeader(accountContextIdHeader))
	if rawId == "" {
		return 0, nil
	}
	contextId, err := strconv.Atoi(rawId)
	if err != nil || contextId <= 0 {
		return 0, &accountContextError{status: http.StatusBadRequest, code: types.ErrorCodeOrganizationContextMismatch, message: "invalid account context id"}
	}
	return contextId, nil
}

func ensureAccountContextPathMatches(c *gin.Context, accountContext *requestAccountContext) *accountContextError {
	pathOrganizationId := accountContextPathOrganizationId(c)
	if pathOrganizationId == 0 {
		return nil
	}
	if accountContext == nil || accountContext.Type != model.AccountContextTypeOrganization || pathOrganizationId != accountContext.OrganizationId {
		return &accountContextError{status: http.StatusForbidden, code: types.ErrorCodeOrganizationContextMismatch, message: "organization context mismatch"}
	}
	return nil
}

func accountContextPathOrganizationId(c *gin.Context) int {
	fullPath := c.FullPath()
	if !strings.Contains(fullPath, "/organizations/:id") {
		return 0
	}
	pathOrganizationId, err := strconv.Atoi(c.Param("id"))
	if err != nil || pathOrganizationId <= 0 {
		return 0
	}
	return pathOrganizationId
}

func writeAccountContextToGin(c *gin.Context, accountContext *requestAccountContext) {
	common.SetContextKey(c, constant.ContextKeyAccountContextType, accountContext.Type)
	common.SetContextKey(c, constant.ContextKeyAccountContextId, accountContext.Id)
	common.SetContextKey(c, constant.ContextKeyScopeType, accountContext.Type)
	common.SetContextKey(c, constant.ContextKeyScopeId, accountContext.Id)
	common.SetContextKey(c, constant.ContextKeyBillingAccountType, accountContext.Type)
	common.SetContextKey(c, constant.ContextKeyBillingAccountId, accountContext.Id)
	common.SetContextKey(c, constant.ContextKeyOrganizationId, accountContext.OrganizationId)
	common.SetContextKey(c, constant.ContextKeyActorUserId, accountContext.ActorUserId)
	common.SetContextKey(c, constant.ContextKeyCreatorUserId, accountContext.CreatorUserId)
	common.SetContextKey(c, constant.ContextKeyResponsibleUserId, accountContext.ResponsibleUserId)
	common.SetContextKey(c, constant.ContextKeyOrganizationQuota, accountContext.OrganizationQuota)
}

func accountContextErrorFromErr(err error) *accountContextError {
	message := err.Error()
	switch {
	case strings.Contains(message, "organization disabled"):
		return &accountContextError{status: http.StatusForbidden, code: types.ErrorCodeOrganizationDisabled, message: message}
	case strings.Contains(message, "organization dissolved"):
		return &accountContextError{status: http.StatusGone, code: types.ErrorCodeOrganizationDissolved, message: message}
	default:
		return &accountContextError{status: http.StatusForbidden, code: types.ErrorCodeOrganizationAccessDenied, message: message}
	}
}

func abortAccountContext(c *gin.Context, status int, code types.ErrorCode, message string) {
	c.JSON(status, gin.H{"success": false, "message": message, "code": code})
	c.Abort()
}
