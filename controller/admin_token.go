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
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func adminTokenOperator(c *gin.Context) service.AdminTokenOperator {
	return service.AdminTokenOperator{
		UserId: c.GetInt("id"),
		Role:   c.GetInt("role"),
	}
}

func writeAdminTokenError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAdminTokenForbidden):
		common.ApiErrorI18n(c, i18n.MsgAuthInsufficientPrivilege)
	case errors.Is(err, service.ErrAdminTokenNotFound):
		common.ApiErrorI18n(c, i18n.MsgTokenInvalid)
	default:
		common.ApiError(c, err)
	}
}

func recordAdminTokenAudit(c *gin.Context, action string, token *model.Token, owner *model.User) {
	recordManageAudit(c, action, map[string]interface{}{
		"token_id":        token.Id,
		"token_name":      token.Name,
		"target_user_id":  owner.Id,
		"target_username": owner.Username,
	})
}

// AdminListTokens returns the all-users personal key list for the admin API
// Keys page. Keys are masked; the full value is only available through
// AdminGetTokenKey.
func AdminListTokens(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status, _ := strconv.Atoi(c.Query("status"))
	tokens, total, err := service.ListAdminPersonalTokens(adminTokenOperator(c), service.AdminPersonalTokenListRequest{
		Offset:   pageInfo.GetStartIdx(),
		Limit:    pageInfo.GetPageSize(),
		Keyword:  c.Query("keyword"),
		Token:    c.Query("token"),
		Username: c.Query("username"),
		Status:   status,
	})
	if err != nil {
		writeAdminTokenError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tokens)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetTokenKey reveals a personal key for an administrator. Viewing another
// user's secret is itself audited.
func AdminGetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	token, owner, err := service.GetAdminManagedPersonalToken(adminTokenOperator(c), id)
	if err != nil {
		writeAdminTokenError(c, err)
		return
	}
	recordAdminTokenAudit(c, "token.key_view", token, owner)
	common.ApiSuccess(c, gin.H{"key": token.GetFullKey()})
}

type adminTokenStatusRequest struct {
	Status int `json:"status"`
}

// AdminUpdateTokenStatus enables or disables another user's personal key. The
// same expired/exhausted guards as the owner path apply.
func AdminUpdateTokenStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	var request adminTokenStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if request.Status != common.TokenStatusEnabled && request.Status != common.TokenStatusDisabled {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	token, owner, err := service.GetAdminManagedPersonalToken(adminTokenOperator(c), id)
	if err != nil {
		writeAdminTokenError(c, err)
		return
	}
	if request.Status == common.TokenStatusEnabled {
		if token.Status == common.TokenStatusExpired && token.ExpiredTime <= common.GetTimestamp() && token.ExpiredTime != -1 {
			common.ApiErrorI18n(c, i18n.MsgTokenExpiredCannotEnable)
			return
		}
		if token.Status == common.TokenStatusExhausted && token.RemainQuota <= 0 && !token.UnlimitedQuota {
			common.ApiErrorI18n(c, i18n.MsgTokenExhaustedCannotEable)
			return
		}
	}
	token.Status = request.Status
	// Status-only write: never rewrite the rest of the row from this snapshot.
	// A concurrent owner edit or quota decrement must not be reverted.
	if err := token.SelectUpdate(); err != nil {
		common.ApiError(c, err)
		return
	}
	action := "token.status_disable"
	if request.Status == common.TokenStatusEnabled {
		action = "token.status_enable"
	}
	recordAdminTokenAudit(c, action, token, owner)
	common.ApiSuccess(c, nil)
}

// AdminDeleteToken deletes another user's personal key and records the action.
func AdminDeleteToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	token, owner, err := service.GetAdminManagedPersonalToken(adminTokenOperator(c), id)
	if err != nil {
		writeAdminTokenError(c, err)
		return
	}
	if err := token.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordAdminTokenAudit(c, "token.delete", token, owner)
	common.ApiSuccess(c, nil)
}
