package controller

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

// pricingAccountGroup 解析当前生效的分组：组织上下文用组织的分组，否则用用户自己的。
//
// 组织密钥按组织的分组倍率计费，价目表也必须按组织的可用分组过滤，
// 否则用户在组织上下文里会看到自己根本用不了（或者价钱不对）的模型。
// 解析不出组织时退回个人分组——切换上下文失败不该让人看不到价目表。
func pricingAccountGroup(userId int) string {
	context, err := service.ResolveCurrentAccountContext(userId)
	if err == nil && context != nil && context.Type == model.AccountContextTypeOrganization {
		var organization model.Organization
		dbErr := model.DB.Select("id", "group").Where("id = ?", context.Id).First(&organization).Error
		if dbErr == nil {
			return service.NormalizeOrganizationGroup(organization.Group)
		}
		if !errors.Is(dbErr, gorm.ErrRecordNotFound) {
			common.SysError("failed to load organization pricing group: " + dbErr.Error())
		}
	}
	user, err := model.GetUserCache(userId)
	if err == nil {
		return user.Group
	}
	return ""
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if exists {
		if id, ok := userId.(int); ok {
			group = pricingAccountGroup(id)
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}
	// 当前用户的模型折扣随价目表一起下发，前端据此展示折后价。折扣只能进这个
	// 独立响应字段：pricing 切片是共享缓存对象，改写元素会串到其他用户。
	userModelDiscounts := map[string]float64{}
	if exists {
		if id, ok := userId.(int); ok {
			// 读不出折扣就按无折扣处理，价目表不该因为折扣读取失败而 500；但失败
			// 必须留痕——前端据此展示折后价与徽标，静默降级会让折扣「看起来没配」。
			loaded, err := model.GetUserModelDiscounts(id, false)
			if err != nil {
				common.SysError(fmt.Sprintf("用户模型折扣读取失败，价目表按原价下发: userId=%d, err=%v", id, err))
			} else if loaded != nil {
				userModelDiscounts = loaded
			}
		}
	}

	usableGroup = service.GetAccountUsableGroups(group)
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	// 组织成员模型白名单：受限成员只看到白名单内的模型价目（平台管理员与无成员
	// 限制的用户 allow 为 nil，整段跳过）。只按本请求重建切片，绝不改写共享缓存
	// 元素——否则会串到其他用户的价目表。
	if viewerId, ok := userId.(int); exists && ok {
		if allow := model.GetUserModelAllowList(viewerId); allow != nil {
			// 一次请求内只为整张价目表构建一次规范化集合。
			memberAccess := model.BuildMemberModelAccessSet(allow)
			allowedPricing := make([]model.Pricing, 0, len(pricing))
			for _, item := range pricing {
				if memberAccess.Allows(item.ModelName) {
					allowedPricing = append(allowedPricing, item)
				}
			}
			pricing = allowedPricing
		}
	}
	// check groupRatio contains usableGroup
	for group := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}

	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"pricings":        pricing,
			"off_peak_window": ratio_setting.GetOffPeakWindow(),
		},
		"vendors":              model.GetVendors(),
		"group_ratio":          groupRatio,
		"user_model_discounts": userModelDiscounts,
		"usable_group":         usableGroup,
		"supported_endpoint":   model.GetSupportedEndpointMap(),
		"auto_groups":          service.GetAccountAutoGroup(group),
		"pricing_version":      "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
