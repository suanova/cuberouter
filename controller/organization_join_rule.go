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
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type createOrganizationJoinRulesRequest struct {
	Patterns []string `json:"patterns"`
	Reason   string   `json:"reason"`
}

// ListOrganizationJoinRules 列出某组织的自动加入规则（仅 root）
func ListOrganizationJoinRules(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	rules, err := service.ListOrganizationJoinRules(c.GetInt("id"), organizationId, organizationAccessMode(c))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, rules)
}

// CreateOrganizationJoinRules 批量新增自动加入规则（仅 root）
func CreateOrganizationJoinRules(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	var req createOrganizationJoinRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	result, err := service.CreateOrganizationJoinRules(c.GetInt("id"), organizationId, organizationAccessMode(c), service.CreateJoinRulesRequest{Patterns: req.Patterns, Reason: req.Reason}, organizationAuditRequestMetadata(c))
	if err != nil {
		if errors.Is(err, service.ErrOrganizationJoinRuleEmailVerificationDisabled) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "email verification is disabled, join rules will never take effect",
				"code":    types.ErrorCodeOrganizationJoinRuleEmailVerificationDisabled,
			})
			return
		}
		writeOrganizationError(c, err)
		return
	}
	if len(result.LineErrors) > 0 {
		// 逐行返回原因，前端按 kind 出本地化文案；不允许部分写入。
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"message":     "organization join rule validation failed",
			"code":        types.ErrorCodeOrganizationJoinRuleInvalid,
			"line_errors": result.LineErrors,
			"notices":     result.Notices,
		})
		return
	}
	common.ApiSuccess(c, gin.H{"rules": result.Rules, "notices": result.Notices})
}

// DeleteOrganizationJoinRule 删除一条自动加入规则（仅 root）
func DeleteOrganizationJoinRule(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	ruleId, err := strconv.Atoi(c.Param("ruleId"))
	if err != nil || ruleId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid join rule id"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = strings.TrimSpace(c.Query("reason"))
	}
	if err := service.DeleteOrganizationJoinRule(c.GetInt("id"), organizationId, ruleId, organizationAccessMode(c), reason, organizationAuditRequestMetadata(c)); err != nil {
		writeOrganizationError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
