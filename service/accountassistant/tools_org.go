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

package accountassistant

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/agentkit"
)

const maxOrgMemberRows = 100

// UserIsOrganizationAdmin 判断用户是否至少管理一个组织。它只用于决定是否
// 向模型暴露组织工具；每个工具执行时仍会逐组织复核权限。
func UserIsOrganizationAdmin(userID int) bool {
	if userID <= 0 || model.DB == nil {
		return false
	}
	var count int64
	err := model.DB.Model(&model.OrganizationMember{}).
		Where("user_id = ? AND status = ? AND role IN ?",
			userID,
			model.OrganizationMemberStatusActive,
			[]string{model.OrganizationRoleOwner, model.OrganizationRoleAdmin},
		).
		Count(&count).Error
	if err != nil {
		common.SysLog("account assistant org admin check failed: " + err.Error())
		return false
	}
	return count > 0
}

func organizationTools() []agentkit.Tool {
	return []agentkit.Tool{
		newGetOrgSummaryTool(),
		newQueryOrgUsageTool(),
		newListOrgMembersTool(),
	}
}

func newGetOrgSummaryTool() agentkit.Tool {
	return agentkit.Tool{
		Name:        "get_org_summary",
		Description: "查询当前用户作为 owner/admin 管理的组织概况：组织状态、额度、已用额度、成员数和请求数。不传 organization_id 时返回全部可管理组织。",
		Parameters: objectSchema(map[string]any{
			"organization_id": integerProperty("组织 id；不传则返回全部可管理组织", 1, 2147483647),
		}),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			var args struct {
				OrganizationID int `json:"organization_id"`
			}
			if err := parseArguments(arguments, &args); err != nil {
				return "", err
			}

			var memberships []model.OrganizationMember
			query := model.DB.Where("user_id = ? AND status = ? AND role IN ?",
				principal.UserID,
				model.OrganizationMemberStatusActive,
				[]string{model.OrganizationRoleOwner, model.OrganizationRoleAdmin},
			)
			if args.OrganizationID > 0 {
				query = query.Where("organization_id = ?", args.OrganizationID)
			}
			if err := query.Order("organization_id").Find(&memberships).Error; err != nil {
				return "", toolError("get_org_summary", err)
			}
			if len(memberships) == 0 {
				return agentkit.MarshalToolResult(map[string]any{
					"count":   0,
					"items":   []map[string]any{},
					"message": "当前用户没有可管理的组织。",
				}), nil
			}

			items := make([]map[string]any, 0, len(memberships))
			for _, membership := range memberships {
				actor, err := organizationAdminContext(principal, membership.OrganizationId)
				if err != nil {
					continue
				}
				organization := actor.Organization
				var memberCount int64
				if err := model.DB.Model(&model.OrganizationMember{}).
					Where("organization_id = ? AND status = ?", organization.Id, model.OrganizationMemberStatusActive).
					Count(&memberCount).Error; err != nil {
					common.SysLog("account assistant org member count failed: " + err.Error())
				}
				items = append(items, map[string]any{
					"organization_id": organization.Id,
					"name":            organization.Name,
					"slug":            organization.Slug,
					"status":          organization.Status,
					"role":            actor.OrganizationRole,
					"quota":           organization.Quota,
					"quota_usd":       quotaToUSD(organization.Quota),
					"used_quota":      organization.UsedQuota,
					"used_usd":        quotaToUSD(organization.UsedQuota),
					"request_count":   organization.RequestCount,
					"active_members":  memberCount,
					"group":           organization.Group,
					"created_at":      formatUnix(organization.CreatedAt),
				})
			}
			sort.Slice(items, func(i, j int) bool {
				return items[i]["organization_id"].(int) < items[j]["organization_id"].(int)
			})
			return agentkit.MarshalToolResult(map[string]any{
				"count": len(items),
				"items": items,
			}), nil
		},
	}
}

func newQueryOrgUsageTool() agentkit.Tool {
	return agentkit.Tool{
		Name:        "query_org_usage",
		Description: "查询指定组织最近 N 天（1-90，默认 7）的用量汇总：请求数、token、额度与按成员/日期的明细行。需要组织 owner/admin 权限。",
		Parameters: objectSchema(map[string]any{
			"organization_id": integerProperty("组织 id", 1, 2147483647),
			"days":            integerProperty("查询天数，1-90，默认 7", 1, maxUsageDays),
		}, "organization_id"),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			var args struct {
				OrganizationID int `json:"organization_id"`
				Days           int `json:"days"`
			}
			if err := parseArguments(arguments, &args); err != nil {
				return "", err
			}
			actor, err := organizationAdminContext(principal, args.OrganizationID)
			if err != nil {
				return "", err
			}
			days := clampInt(args.Days, 1, maxUsageDays, 7)
			endTs := common.GetTimestamp()
			startTs := endTs - int64(days)*model.SelfUsageDaySeconds

			data, err := service.GetOrganizationUsage(service.OrganizationUsageQuery{
				OperatorUserId: principal.UserID,
				OrganizationId: args.OrganizationID,
				AccessMode:     service.OrganizationAccessModeManagement,
				StartTs:        startTs,
				EndTs:          endTs,
				Page:           1,
				PageSize:       20,
			})
			if err != nil {
				return "", toolError("query_org_usage", err)
			}
			return agentkit.MarshalToolResult(map[string]any{
				"organization_id":   args.OrganizationID,
				"organization_name": actor.Organization.Name,
				"days":              days,
				"start_date":        data.Range.Start,
				"end_date":          data.Range.End,
				"totals":            data.Totals,
				"total_quota_usd":   quotaToUSD(data.Totals.Quota),
				"rows":              data.Rows.Items,
				"row_total":         data.Rows.Total,
				"truncated":         data.Rows.Total > len(data.Rows.Items),
				"note":              "明细仅返回前 20 行；金额单位是平台内部额度，quota_usd 可按总量换算。",
			}), nil
		},
	}
}

func newListOrgMembersTool() agentkit.Tool {
	type memberRow struct {
		UserId      int    `gorm:"column:user_id"`
		Username    string `gorm:"column:username"`
		DisplayName string `gorm:"column:display_name"`
		Email       string `gorm:"column:email"`
		Role        string `gorm:"column:role"`
		Status      string `gorm:"column:status"`
		QuotaLimit  int    `gorm:"column:quota_limit"`
		UsedQuota   int    `gorm:"column:used_quota"`
		JoinedAt    int64  `gorm:"column:joined_at"`
	}
	return agentkit.Tool{
		Name:        "list_org_members",
		Description: "列出指定组织的成员（最多 100 人）：角色、状态、邮箱掩码、成员额度与使用情况。需要组织 owner/admin 权限。",
		Parameters: objectSchema(map[string]any{
			"organization_id": integerProperty("组织 id", 1, 2147483647),
		}, "organization_id"),
		Run: func(ctx context.Context, principal agentkit.Principal, arguments string) (string, error) {
			var args struct {
				OrganizationID int `json:"organization_id"`
			}
			if err := parseArguments(arguments, &args); err != nil {
				return "", err
			}
			actor, err := organizationAdminContext(principal, args.OrganizationID)
			if err != nil {
				return "", err
			}

			var rows []memberRow
			err = model.DB.Table("organization_members AS m").
				Select("m.user_id, u.username, u.display_name, u.email, m.role, m.status, m.quota_limit, m.used_quota, m.joined_at").
				Joins("LEFT JOIN users AS u ON u.id = m.user_id").
				Where("m.organization_id = ?", args.OrganizationID).
				Order("m.role, m.user_id").
				Limit(maxOrgMemberRows).
				Scan(&rows).Error
			if err != nil {
				return "", toolError("list_org_members", err)
			}
			items := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				items = append(items, map[string]any{
					"user_id":      row.UserId,
					"username":     row.Username,
					"display_name": row.DisplayName,
					"email":        maskEmail(row.Email),
					"role":         row.Role,
					"status":       row.Status,
					"quota_limit":  row.QuotaLimit,
					"quota_usd":    quotaToUSD(row.QuotaLimit),
					"used_quota":   row.UsedQuota,
					"used_usd":     quotaToUSD(row.UsedQuota),
					"joined_at":    formatUnix(row.JoinedAt),
				})
			}
			return agentkit.MarshalToolResult(map[string]any{
				"organization_id":   args.OrganizationID,
				"organization_name": actor.Organization.Name,
				"count":             len(items),
				"items":             items,
				"note":              "邮箱已脱敏；完整成员名单请到组织管理页面查看。",
			}), nil
		},
	}
}

// organizationAdminContext 对每个组织独立复核 owner/admin 身份。Principal 上的
// IsOrganizationAdmin 只用于决定是否注册工具，不能作为授权依据。
func organizationAdminContext(principal agentkit.Principal, organizationID int) (*service.OrganizationActorContext, error) {
	if organizationID <= 0 {
		return nil, errors.New("请提供 organization_id")
	}
	actor, err := service.GetOrganizationActorContextForAccessMode(
		principal.UserID,
		organizationID,
		service.OrganizationAccessModeManagement,
		true,
	)
	if err != nil {
		common.SysLog("account assistant org access denied: " + err.Error())
		return nil, errors.New("无权查看该组织，或组织不存在")
	}
	if actor == nil || !actor.IsOrganizationAdmin {
		return nil, errors.New("需要组织 owner 或 admin 权限")
	}
	return actor, nil
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		if len(email) <= 2 {
			return "***"
		}
		return email[:1] + "***" + email[len(email)-1:]
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 2 {
		return local[:1] + "***" + domain
	}
	return local[:2] + "***" + domain
}
