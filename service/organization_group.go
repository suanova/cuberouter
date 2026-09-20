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
package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const DefaultOrganizationGroup = "default"

func NormalizeOrganizationGroup(group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		return DefaultOrganizationGroup
	}
	return group
}

func normalizeOrganizationGroup(group string) string {
	return NormalizeOrganizationGroup(group)
}

func GetOrganizationUsableGroups(organizationGroup string) map[string]string {
	return GetAccountUsableGroups(normalizeOrganizationGroup(organizationGroup))
}

func GetOrganizationAutoGroup(organizationGroup string) []string {
	return GetAccountAutoGroup(normalizeOrganizationGroup(organizationGroup))
}

func GetOrganizationGroupRatio(organizationGroup, group string) float64 {
	return GetAccountGroupRatio(normalizeOrganizationGroup(organizationGroup), group)
}

func GetOrganizationGroups(operatorUserId, organizationId int, accessMode string) (map[string]map[string]interface{}, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, errors.New("invalid organization request")
	}
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}
	if !actor.Capabilities.CanViewOrganization {
		return nil, errors.New("permission denied")
	}
	return buildOrganizationGroupsResponse(actor.Organization.Group), nil
}

func buildOrganizationGroupsResponse(organizationGroup string) map[string]map[string]interface{} {
	usableGroups := make(map[string]map[string]interface{})
	organizationUsableGroups := GetOrganizationUsableGroups(organizationGroup)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		if desc, ok := organizationUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": GetOrganizationGroupRatio(organizationGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := organizationUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": 1,
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	return usableGroups
}

func validateOrganizationTokenGroup(organizationGroup, tokenGroup string) error {
	tokenGroup = strings.TrimSpace(tokenGroup)
	if tokenGroup == "" {
		return nil
	}
	if _, ok := GetOrganizationUsableGroups(organizationGroup)[tokenGroup]; !ok {
		return errors.New("token group is not usable by organization")
	}
	if tokenGroup != "auto" && !ratio_setting.ContainsGroupRatio(tokenGroup) {
		return errors.New("token group is deprecated")
	}
	return nil
}

func getOrganizationGroupById(organizationId int) (string, error) {
	var organization model.Organization
	if err := model.DB.Select("id", "group").Where("id = ?", organizationId).First(&organization).Error; err != nil {
		return "", err
	}
	return normalizeOrganizationGroup(organization.Group), nil
}
