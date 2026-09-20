package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// GetAccountUsableGroups 返回某个账户（个人用户或组织）可用的分组集合。
// accountGroup 是该账户自身所属的分组，特殊可用分组设置以它为键。
func GetAccountUsableGroups(accountGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if accountGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(accountGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果accountGroup不在UserUsableGroups中，返回UserUsableGroups + accountGroup
		if _, ok := groupsCopy[accountGroup]; !ok {
			groupsCopy[accountGroup] = "用户分组"
		}
	}
	return groupsCopy
}

func GetUserUsableGroups(userGroup string) map[string]string {
	return GetAccountUsableGroups(userGroup)
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

func IsAccountSelectableGroup(accountGroup, groupName string) bool {
	if groupName == "" || groupName == "auto" {
		return false
	}
	return GroupInUserUsableGroups(accountGroup, groupName) && ratio_setting.ContainsGroupRatio(groupName)
}

func IsUserSelectableGroup(userGroup, groupName string) bool {
	return IsAccountSelectableGroup(userGroup, groupName)
}

// GetAccountAutoGroup 根据账户基础分组获取自动分组设置
func GetAccountAutoGroup(accountGroup string) []string {
	autoGroups := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range setting.GetAutoGroups() {
		if !IsAccountSelectableGroup(accountGroup, group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		autoGroups = append(autoGroups, group)
	}
	return autoGroups
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	return GetAccountAutoGroup(userGroup)
}

// FilterAccountTokenAutoGroups applies current permissions before the current
// per-token limit. It intentionally does not fall back to the global Auto list.
func FilterAccountTokenAutoGroups(accountGroup string, groups []string) []string {
	maxCount := setting.GetMaxTokenAutoGroups()
	filtered := make([]string, 0, min(len(groups), maxCount))
	seen := make(map[string]struct{})
	for _, group := range groups {
		if !IsAccountSelectableGroup(accountGroup, group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		filtered = append(filtered, group)
		if len(filtered) == maxCount {
			break
		}
	}
	return filtered
}

func FilterUserTokenAutoGroups(userGroup string, groups []string) []string {
	return FilterAccountTokenAutoGroups(userGroup, groups)
}

// GetRequestAutoGroups resolves the ordered Auto groups for the current token.
// The absence of the context value means that the token inherits the complete
// global Auto list; a present (even empty) value is an explicit token snapshot.
func GetRequestAutoGroups(c *gin.Context, userGroup string) []string {
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenAutoGroups)
	if !ok {
		return GetUserAutoGroup(userGroup)
	}
	groups, ok := value.([]string)
	if !ok {
		return []string{}
	}
	return FilterUserTokenAutoGroups(userGroup, groups)
}

// GetGroupsEnabledModels 按 groups 顺序获取各分组启用的模型并去重
func GetGroupsEnabledModels(groups []string) []string {
	seen := make(map[string]struct{})
	models := make([]string, 0)
	for _, group := range groups {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			if _, ok := seen[modelName]; !ok {
				seen[modelName] = struct{}{}
				models = append(models, modelName)
			}
		}
	}
	return models
}

// GetAccountGroupRatio 获取账户使用某个分组的倍率
// accountGroup 账户基础分组
// group 需要获取倍率的分组
func GetAccountGroupRatio(accountGroup, group string) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(accountGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	return GetAccountGroupRatio(userGroup, group)
}
