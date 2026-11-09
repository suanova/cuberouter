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
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"gorm.io/gorm"
)

const (
	organizationAuditActionDshUpdate = "organization.dsh.update"

	// dshOrganizationDefaultContainerPort 与 docker-compose.dsh-org.yml 中
	// dsh 容器暴露的端口保持一致，仅用于给管理员提示渠道 Base URL。
	dshOrganizationDefaultContainerPort = 3080
)

// dshOrganizationAdminMutex 串行化"读改写入 OptionMap"的绑定/密钥更新。
// OptionMap 没有 compare-and-swap，两个管理员并发保存会互相覆盖。
var dshOrganizationAdminMutex sync.Mutex

var (
	ErrDshOrganizationNotFound     = errors.New("organization not found")
	ErrDshChannelNotFound          = errors.New("channel not found")
	ErrDshChannelInvalid           = errors.New("channel is not an enabled DSH task-plugin channel")
	ErrDshChannelBaseURLMissing    = errors.New("channel base url is required")
	ErrDshChannelGroupMismatch     = errors.New("channel groups must include the organization group")
	ErrDshChannelAlreadyBound      = errors.New("channel is already bound to another organization")
	ErrDshSecretUpdateConflict     = errors.New("cannot set and clear the organization secret at the same time")
	ErrDshSecretInvalid            = errors.New("invalid organization secret")
	ErrDshSecretNotConfigured      = errors.New("organization secret is not configured")
	errDshOrganizationNotBindable  = errors.New("organization cannot be bound to DSH")
	dshOrganizationMaskedPreviewSz = 4
)

// DshChannelOption 是管理端渠道下拉的一项。绑定关系一并返回，避免前端
// 自己拼两次请求。
type DshChannelOption struct {
	Id                  int      `json:"id"`
	Name                string   `json:"name"`
	Status              int      `json:"status"`
	BaseURL             string   `json:"base_url"`
	Groups              []string `json:"groups"`
	BoundOrganizationId int      `json:"bound_organization_id"`
}

// DshOrganizationConfig 是管理端一个组织的 DSH 配置视图。密钥只暴露
// "是否已配置"和脱敏预览，明文只在显式 Reveal 时返回。
type DshOrganizationConfig struct {
	OrganizationId        int      `json:"organization_id"`
	OrganizationName      string   `json:"organization_name"`
	OrganizationGroup     string   `json:"organization_group"`
	OrganizationStatus    string   `json:"organization_status"`
	Allowed               bool     `json:"allowed"`
	ChannelId             int      `json:"channel_id"`
	ChannelAvailable      bool     `json:"channel_available"`
	ChannelBaseURL        string   `json:"channel_base_url"`
	ChannelGroups         []string `json:"channel_groups"`
	SuggestedBaseURL      string   `json:"suggested_base_url"`
	DeepseekKeyConfigured bool     `json:"deepseek_key_configured"`
	DeepseekKeyPreview    string   `json:"deepseek_key_preview"`
}

// ListDshChannels 列出全部 DSH 任务插件渠道（含停用渠道，便于发现失效绑定）。
func ListDshChannels() ([]DshChannelOption, error) {
	var channels []model.Channel
	if err := model.DB.Where("type = ?", constant.ChannelTypeTaskPlugin).Order("id asc").Find(&channels).Error; err != nil {
		return nil, err
	}
	bindings := setting.GetDshOrganizationChannels()
	boundOrgs := make(map[int]int, len(bindings))
	for organizationId, channelId := range bindings {
		boundOrgs[channelId] = organizationId
	}
	options := make([]DshChannelOption, 0, len(channels))
	for i := range channels {
		channel := &channels[i]
		if strings.TrimSpace(channel.GetSetting().TaskPluginKey) != setting.DshTaskPluginKey {
			continue
		}
		options = append(options, DshChannelOption{
			Id:                  channel.Id,
			Name:                channel.Name,
			Status:              channel.Status,
			BaseURL:             rawChannelBaseURL(channel),
			Groups:              channel.GetGroups(),
			BoundOrganizationId: boundOrgs[channel.Id],
		})
	}
	return options, nil
}

// GetDshOrganizationConfig 读取单个组织的 DSH 配置。
func GetDshOrganizationConfig(organizationId int) (*DshOrganizationConfig, error) {
	organization, err := loadDshOrganization(organizationId)
	if err != nil {
		return nil, err
	}
	config := &DshOrganizationConfig{
		OrganizationId:     organization.Id,
		OrganizationName:   organization.Name,
		OrganizationGroup:  NormalizeOrganizationGroup(organization.Group),
		OrganizationStatus: organization.Status,
		Allowed:            slices.Contains(setting.GetDshAllowedOrganizationIds(), organization.Id),
		SuggestedBaseURL:   fmt.Sprintf("http://dsh-org-%d:%d", organization.Id, dshOrganizationDefaultContainerPort),
	}

	bindings := setting.GetDshOrganizationChannels()
	if channelId, ok := bindings[organization.Id]; ok {
		config.ChannelId = channelId
		if channel, err := model.GetChannelById(channelId, false); err == nil {
			config.ChannelBaseURL = rawChannelBaseURL(channel)
			config.ChannelGroups = channel.GetGroups()
			config.ChannelAvailable = dshChannelUsable(channel)
		}
	}

	if key := setting.GetDshOrganizationDeepseekKey(organization.Id); key != "" {
		config.DeepseekKeyConfigured = true
		config.DeepseekKeyPreview = maskDshOrganizationSecret(key)
	}
	return config, nil
}

// DshOrganizationUpdate 是管理端一次保存的内容。ChannelId 为 0 表示解绑
// （组织保留在白名单里）。DeepseekKey 为 nil 表示不修改；ClearDeepseekKey
// 与 DeepseekKey 互斥。
type DshOrganizationUpdate struct {
	ChannelId        int
	DeepseekKey      *string
	ClearDeepseekKey bool
}

// UpdateDshOrganizationConfig 保存组织绑定与（可选）组织的 DeepSeek Key。
//
// 绑定成功后自动把组织加入白名单：两个选项共同定义"已开通"，管理端不该
// 要求管理员在两处分别点一次保存还保持顺序正确。两项在同一次
// UpdateOptionsBulk 中落库。
func UpdateDshOrganizationConfig(organizationId, operatorUserId int, update DshOrganizationUpdate, auditMetadata ...OrganizationAuditRequestMetadata) (*DshOrganizationConfig, error) {
	if organizationId <= 0 {
		return nil, ErrDshOrganizationNotFound
	}
	if update.DeepseekKey != nil && update.ClearDeepseekKey {
		return nil, ErrDshSecretUpdateConflict
	}

	dshOrganizationAdminMutex.Lock()
	defer dshOrganizationAdminMutex.Unlock()

	organization, err := loadDshOrganization(organizationId)
	if err != nil {
		return nil, err
	}
	if organization.Status == model.OrganizationStatusDissolved {
		return nil, errDshOrganizationNotBindable
	}

	bindings := setting.GetDshOrganizationChannels()
	allowed := setting.GetDshAllowedOrganizationIds()
	keys := setting.GetDshOrganizationDeepseekKeys()

	previousChannelId := bindings[organizationId]
	previousAllowed := slices.Contains(allowed, organizationId)
	previousKeyConfigured := strings.TrimSpace(keys[organizationId]) != ""

	if update.ChannelId > 0 {
		channel, err := model.GetChannelById(update.ChannelId, false)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrDshChannelNotFound
			}
			return nil, err
		}
		if !dshChannelUsable(channel) {
			return nil, ErrDshChannelInvalid
		}
		if rawChannelBaseURL(channel) == "" {
			return nil, ErrDshChannelBaseURLMissing
		}
		// abilities 按 (channel, group, model) 生成；渠道分组里没有组织分组时
		// 模型下拉会是空的，这里提前拒绝并让管理员先改渠道。
		if !slices.Contains(channel.GetGroups(), NormalizeOrganizationGroup(organization.Group)) {
			return nil, fmt.Errorf("%w: add %q to the channel groups first", ErrDshChannelGroupMismatch, NormalizeOrganizationGroup(organization.Group))
		}
		for boundOrganizationId, boundChannelId := range bindings {
			if boundChannelId == update.ChannelId && boundOrganizationId != organizationId {
				return nil, fmt.Errorf("%w: organization %d", ErrDshChannelAlreadyBound, boundOrganizationId)
			}
		}
		bindings[organizationId] = update.ChannelId
		if !slices.Contains(allowed, organizationId) {
			allowed = append(allowed, organizationId)
			sort.Ints(allowed)
		}
	} else {
		delete(bindings, organizationId)
	}

	keyChanged := false
	if update.DeepseekKey != nil {
		key := strings.TrimSpace(*update.DeepseekKey)
		if err := setting.ValidateDshOrganizationSecretKey(key); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrDshSecretInvalid, err.Error())
		}
		keys[organizationId] = key
		keyChanged = true
	}
	if update.ClearDeepseekKey {
		delete(keys, organizationId)
		keyChanged = true
	}

	values, err := dshOrganizationOptionValues(bindings, allowed)
	if err != nil {
		return nil, err
	}
	if keyChanged {
		encodedKeys, err := encodeDshOrganizationSecrets(keys)
		if err != nil {
			return nil, err
		}
		values[setting.DshOrganizationDeepseekSecretKey] = encodedKeys
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		return nil, err
	}

	// 审计只写变更摘要，绝不写明文密钥。审计失败不阻断已经落库的配置，
	// 但必须留痕到系统日志。
	before := map[string]any{
		"channel_id":              previousChannelId,
		"deepseek_key_configured": previousKeyConfigured,
		"allowed":                 previousAllowed,
	}
	after := map[string]any{
		"channel_id":              update.ChannelId,
		"deepseek_key_configured": strings.TrimSpace(keys[organizationId]) != "",
		"allowed":                 slices.Contains(allowed, organizationId),
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		return recordOrganizationAudit(tx, organization, operatorUserId, organizationAuditOperatorRolePlatformRoot, organizationAuditActionDshUpdate, "organization", organization.Id, before, after, "dsh organization configuration update", auditMetadata...)
	}); err != nil {
		common.SysError(fmt.Sprintf("dsh organization: audit for organization %d failed: %s", organizationId, err.Error()))
	}

	return GetDshOrganizationConfig(organizationId)
}

// RevealDshOrganizationSecret 返回组织 DeepSeek Key 明文，仅在管理员显式
// 复制时调用。
func RevealDshOrganizationSecret(organizationId int) (string, error) {
	if organizationId <= 0 {
		return "", ErrDshOrganizationNotFound
	}
	if _, err := loadDshOrganization(organizationId); err != nil {
		return "", err
	}
	key := setting.GetDshOrganizationDeepseekKey(organizationId)
	if key == "" {
		return "", ErrDshSecretNotConfigured
	}
	return key, nil
}

func loadDshOrganization(organizationId int) (*model.Organization, error) {
	var organization model.Organization
	if err := model.DB.Where("id = ?", organizationId).First(&organization).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDshOrganizationNotFound
		}
		return nil, err
	}
	return &organization, nil
}

// dshChannelUsable reports whether a channel can serve DSH traffic. It mirrors
// resolveDshChannel's runtime predicate so the admin view does not disagree
// with the resolver.
func dshChannelUsable(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	if channel.Status != common.ChannelStatusEnabled {
		return false
	}
	if channel.Type != constant.ChannelTypeTaskPlugin {
		return false
	}
	return strings.TrimSpace(channel.GetSetting().TaskPluginKey) == setting.DshTaskPluginKey
}

func rawChannelBaseURL(channel *model.Channel) string {
	if channel == nil || channel.BaseURL == nil {
		return ""
	}
	return strings.TrimSpace(*channel.BaseURL)
}

func dshOrganizationOptionValues(bindings map[int]int, allowed []int) (map[string]string, error) {
	encodedBindings, err := common.Marshal(bindings)
	if err != nil {
		return nil, err
	}
	encodedAllowed, err := common.Marshal(allowed)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		setting.DshOrganizationChannelsKey:   string(encodedBindings),
		setting.DshAllowedOrganizationIdsKey: string(encodedAllowed),
	}, nil
}

func encodeDshOrganizationSecrets(keys map[int]string) (string, error) {
	encoded, err := common.Marshal(keys)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func maskDshOrganizationSecret(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= dshOrganizationMaskedPreviewSz {
		return "****"
	}
	return "****" + key[len(key)-dshOrganizationMaskedPreviewSz:]
}
