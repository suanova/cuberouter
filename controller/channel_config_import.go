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
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/channelconfig"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ChannelConfigApplyOptions controls one declarative channel import.
type ChannelConfigApplyOptions struct {
	// DryRun builds the full plan, including upstream model discovery, but
	// writes nothing to the database.
	DryRun bool
}

// ChannelConfigChannelResult reports one channel of the plan.
type ChannelConfigChannelResult struct {
	Name     string `json:"name"`
	ID       int    `json:"id"`
	Type     int    `json:"type"`
	TypeName string `json:"type_name"`
	Action   string `json:"action"`
	// Models is the resolved list written to the channel (and abilities).
	Models []string `json:"models"`
	// Fetched is the raw upstream /v1/models result before include/exclude.
	Fetched []string `json:"fetched,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// ChannelConfigPricingResult is the pricing part of the plan.
type ChannelConfigPricingResult struct {
	Models   int            `json:"models"`
	Written  map[string]int `json:"written,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
	// Detail is the price plan in USD per 1M tokens, keyed by model name.
	Detail map[string]channelconfig.Price `json:"detail,omitempty"`
}

// ChannelConfigApplyResult is the full outcome of one import.
type ChannelConfigApplyResult struct {
	DryRun   bool                         `json:"dry_run"`
	Channels []ChannelConfigChannelResult `json:"channels"`
	Pricing  ChannelConfigPricingResult   `json:"pricing"`
	Warnings []string                     `json:"warnings"`
}

type preparedChannelConfig struct {
	source   *channelconfig.Channel
	channel  *model.Channel
	existing *model.Channel
	models   []string
	fetched  []string
}

const (
	// ChannelConfigEnvFile names the bootstrap file that is applied after the
	// initial setup, watched for ConfigMap updates, and used to refresh prices
	// after upstream model updates. When it is empty all three are disabled.
	ChannelConfigEnvFile = "CHANNEL_CONFIG_FILE"
)

const channelConfigSyncInterval = time.Minute

var (
	channelConfigStateMu     sync.Mutex
	channelConfigApplied     bool
	channelConfigAppliedHash [sha256.Size]byte
	channelConfigLogged      bool
	channelConfigLoggedHash  [sha256.Size]byte
	channelConfigReadError   string
	// channelConfigApplyMu serializes in-process applies so the setup hook and
	// the ConfigMap watcher cannot insert the same channel twice.
	channelConfigApplyMu sync.Mutex
)

// ApplyChannelConfigFromEnv applies the file named by CHANNEL_CONFIG_FILE. It
// returns (nil, nil) when the variable is not set.
func ApplyChannelConfigFromEnv(options ChannelConfigApplyOptions) (*ChannelConfigApplyResult, error) {
	path := channelConfigPath()
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// The ConfigMap may be written by another component after startup. A
		// missing file is not an error: the watcher applies it once it appears.
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read channel config %s: %w", path, err)
	}
	return applyChannelConfigBytes(path, data, options)
}

func channelConfigPath() string {
	return strings.TrimSpace(os.Getenv(ChannelConfigEnvFile))
}

// applyChannelConfigBytes parses and applies one snapshot of the file. The
// applied hash is recorded only after a successful non-dry-run so a failed
// attempt is retried on the next watcher tick.
func applyChannelConfigBytes(path string, data []byte, options ChannelConfigApplyOptions) (*ChannelConfigApplyResult, error) {
	config, err := channelconfig.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse channel config %s: %w", path, err)
	}
	result, err := ApplyChannelConfig(config, options)
	if err != nil {
		return result, err
	}
	if !options.DryRun {
		hash := sha256.Sum256(data)
		channelConfigStateMu.Lock()
		channelConfigApplied = true
		channelConfigAppliedHash = hash
		channelConfigStateMu.Unlock()
	}
	return result, nil
}

// StartChannelConfigSync applies CHANNEL_CONFIG_FILE once and then watches it
// for changes. A ConfigMap volume is updated by atomically swapping a symlink,
// so polling the file content is reliable and needs no restart or fsnotify.
//
// Only the master node applies the file; other replicas pick the result up
// through the regular channel/option sync loops. This avoids every replica
// writing the same channels and prices when the ConfigMap changes.
func StartChannelConfigSync() {
	path := channelConfigPath()
	if path == "" || !common.IsMasterNode {
		return
	}
	common.SysLog(fmt.Sprintf("channel config sync enabled: file=%s interval=%s", path, channelConfigSyncInterval))
	go func() {
		syncChannelConfigFile()
		ticker := time.NewTicker(channelConfigSyncInterval)
		defer ticker.Stop()
		for range ticker.C {
			syncChannelConfigFile()
		}
	}()
}

func syncChannelConfigFile() {
	path := channelConfigPath()
	if path == "" || !constant.Setup {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		logChannelConfigReadError(path, err)
		return
	}
	channelConfigStateMu.Lock()
	channelConfigReadError = ""
	channelConfigStateMu.Unlock()
	hash := sha256.Sum256(data)

	channelConfigStateMu.Lock()
	unchanged := channelConfigApplied && channelConfigAppliedHash == hash
	alreadyLogged := channelConfigLogged && channelConfigLoggedHash == hash
	channelConfigStateMu.Unlock()
	if unchanged {
		return
	}

	result, err := applyChannelConfigBytes(path, data, ChannelConfigApplyOptions{})
	if err != nil {
		if !alreadyLogged {
			common.SysError(fmt.Sprintf("channel config sync failed: %v", err))
			channelConfigStateMu.Lock()
			channelConfigLogged = true
			channelConfigLoggedHash = hash
			channelConfigStateMu.Unlock()
		}
		return
	}
	channelConfigStateMu.Lock()
	channelConfigLogged = false
	channelConfigStateMu.Unlock()
	logChannelConfigApplied(result)
}

func logChannelConfigReadError(path string, err error) {
	message := fmt.Sprintf("read channel config %s: %v", path, err)
	channelConfigStateMu.Lock()
	unchanged := channelConfigReadError == message
	channelConfigReadError = message
	channelConfigStateMu.Unlock()
	if !unchanged {
		common.SysLog(message)
	}
}

func logChannelConfigApplied(result *ChannelConfigApplyResult) {
	if result == nil {
		return
	}
	message := fmt.Sprintf("channel config applied from %s: channels=%d", channelConfigPath(), len(result.Channels))
	if result.Pricing.Models > 0 {
		message += fmt.Sprintf(" priced_models=%d", result.Pricing.Models)
	}
	common.SysLog(message)
	for _, warning := range result.Warnings {
		common.SysLog("channel config warning: " + warning)
	}
	for _, warning := range result.Pricing.Warnings {
		common.SysLog("channel config warning: " + warning)
	}
}

// ApplyChannelConfigFile reads and applies a channel bootstrap file.
func ApplyChannelConfigFile(path string, options ChannelConfigApplyOptions) (*ChannelConfigApplyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read channel config %s: %w", path, err)
	}
	config, err := channelconfig.Parse(data)
	if err != nil {
		return nil, err
	}
	return ApplyChannelConfig(config, options)
}

// ApplyChannelConfig creates or updates the channels described by the config.
//
// Model discovery is a preflight step: if any channel cannot be built or its
// upstream model list cannot be fetched, nothing is written. Failures while
// writing individual channels are collected so the remaining channels can
// still be applied; the import is idempotent and can be rerun.
func ApplyChannelConfig(config *channelconfig.Config, options ChannelConfigApplyOptions) (*ChannelConfigApplyResult, error) {
	channelConfigApplyMu.Lock()
	defer channelConfigApplyMu.Unlock()

	result := &ChannelConfigApplyResult{
		DryRun:   options.DryRun,
		Channels: make([]ChannelConfigChannelResult, 0, len(config.Channels)),
		Warnings: []string{},
	}

	prepared := make([]*preparedChannelConfig, 0, len(config.Channels))
	for index := range config.Channels {
		source := &config.Channels[index]
		channel, err := buildChannelFromConfig(source)
		if err != nil {
			return nil, fmt.Errorf("channel %q: %w", source.Name, err)
		}
		if err := validateChannel(channel, true); err != nil {
			return nil, fmt.Errorf("channel %q: %w", source.Name, err)
		}
		models, fetched, err := resolveChannelModels(source, channel)
		if err != nil {
			return nil, fmt.Errorf("channel %q: %w", source.Name, err)
		}
		channel.Models = strings.Join(models, ",")
		if err := validateChannel(channel, true); err != nil {
			return nil, fmt.Errorf("channel %q: %w", source.Name, err)
		}
		existing, err := findChannelForConfig(source.Name)
		if err != nil {
			return nil, fmt.Errorf("channel %q: %w", source.Name, err)
		}
		prepared = append(prepared, &preparedChannelConfig{
			source:   source,
			channel:  channel,
			existing: existing,
			models:   models,
			fetched:  fetched,
		})
	}

	prices, priceWarnings, err := buildPricePlan(config, prepared)
	if err != nil {
		return nil, err
	}
	result.Warnings = append(result.Warnings, priceWarnings...)
	result.Pricing = ChannelConfigPricingResult{Models: len(prices)}
	plannedPrices := prices

	var applyErrors []error
	applied := make([]*preparedChannelConfig, 0, len(prepared))
	for _, item := range prepared {
		channelResult := ChannelConfigChannelResult{
			Name:     item.source.Name,
			Type:     item.channel.Type,
			TypeName: constant.GetChannelTypeName(item.channel.Type),
			Models:   item.models,
			Fetched:  item.fetched,
		}
		if item.existing != nil {
			channelResult.ID = item.existing.Id
			channelResult.Action = "updated"
			if !options.DryRun {
				applyChannelConfigToExisting(item.existing, item.channel, item.source)
				if err := item.existing.Save(); err != nil {
					channelResult.Action = "failed"
					channelResult.Error = fmt.Sprintf("save channel: %v", err)
				} else if err := item.existing.UpdateAbilities(nil); err != nil {
					channelResult.Action = "failed"
					channelResult.Error = fmt.Sprintf("rebuild abilities: %v", err)
				}
			}
		} else {
			channelResult.Action = "created"
			if !options.DryRun {
				if item.channel.Priority == nil {
					zero := int64(0)
					item.channel.Priority = &zero
				}
				if item.channel.Weight == nil {
					one := uint(1)
					item.channel.Weight = &one
				}
				if err := item.channel.Insert(); err != nil {
					channelResult.Action = "failed"
					channelResult.Error = fmt.Sprintf("create channel: %v", err)
				} else {
					channelResult.ID = item.channel.Id
				}
			}
		}
		if channelResult.Error != "" {
			applyErrors = append(applyErrors, fmt.Errorf("channel %q: %s", item.source.Name, channelResult.Error))
		} else {
			applied = append(applied, item)
		}
		result.Channels = append(result.Channels, channelResult)
	}

	if !options.DryRun {
		appliedModels := make(map[string]struct{})
		for _, item := range applied {
			for _, name := range item.models {
				appliedModels[name] = struct{}{}
			}
		}
		appliedPrices := make(map[string]channelconfig.ModelPrice, len(prices))
		for name, price := range prices {
			if _, ok := appliedModels[name]; ok {
				appliedPrices[name] = price
			}
		}
		plannedPrices = appliedPrices
		priceResult, err := service.ApplyModelPrices(appliedPrices)
		if priceResult != nil {
			result.Pricing.Models = priceResult.Models
			result.Pricing.Written = priceResult.Written
			result.Pricing.Warnings = priceResult.Warnings
		}
		if err != nil {
			applyErrors = append(applyErrors, fmt.Errorf("apply model prices: %w", err))
		}
	}

	result.Pricing.Detail = make(map[string]channelconfig.Price, len(plannedPrices))
	for name, price := range plannedPrices {
		result.Pricing.Detail[name] = price.ToPrice()
	}

	return result, errors.Join(applyErrors...)
}

func buildChannelFromConfig(source *channelconfig.Channel) (*model.Channel, error) {
	channelType, err := channelconfig.ResolveChannelType(source.Type)
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimSpace(source.BaseURL)
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(channelType)
	}
	keys := source.ChannelKeys()
	if len(keys) == 0 {
		return nil, errors.New("channel key is empty")
	}
	group := source.Group
	if group == "" {
		group = "default"
	}

	channel := &model.Channel{
		Type:        channelType,
		Name:        source.Name,
		Key:         strings.Join(keys, "\n"),
		BaseURL:     &baseURL,
		Group:       group,
		Status:      1,
		CreatedTime: common.GetTimestamp(),
		Priority:    source.Priority,
		Weight:      source.Weight,
		Remark:      source.Remark,
		Tag:         source.Tag,
		TestModel:   source.TestModel,
		AutoBan:     source.AutoBan,
		Other:       source.Other,
	}
	if source.Status != nil {
		channel.Status = *source.Status
	}
	if len(keys) > 1 {
		multiKeyMode := constant.MultiKeyMode(source.MultiKeyMode)
		if multiKeyMode == "" {
			multiKeyMode = constant.MultiKeyModeRandom
		}
		channel.ChannelInfo = model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: len(keys),
			MultiKeyMode: multiKeyMode,
		}
	}

	if len(source.ModelMapping) > 0 {
		encoded, err := common.Marshal(source.ModelMapping)
		if err != nil {
			return nil, fmt.Errorf("model_mapping: %w", err)
		}
		mapping := string(encoded)
		channel.ModelMapping = &mapping
	}

	paramOverride, err := marshalOverrideJSON(source.Overrides)
	if err != nil {
		return nil, fmt.Errorf("param_override: %w", err)
	}
	channel.ParamOverride = paramOverride

	headerOverride, err := marshalOverrideJSON(source.HeaderOverride)
	if err != nil {
		return nil, fmt.Errorf("header_override: %w", err)
	}
	channel.HeaderOverride = headerOverride

	setting, err := buildChannelSetting(source.Proxy, source.Settings)
	if err != nil {
		return nil, err
	}
	channel.Setting = setting

	otherSettings, err := buildChannelOtherSettings(source.OtherSettings)
	if err != nil {
		return nil, err
	}
	channel.OtherSettings = otherSettings

	return channel, nil
}

func resolveChannelModels(source *channelconfig.Channel, channel *model.Channel) ([]string, []string, error) {
	modelsConfig := source.Models
	if modelsConfig == nil {
		modelsConfig = &channelconfig.Models{}
	}
	var fetched []string
	if modelsConfig.Fetch == nil || *modelsConfig.Fetch {
		ids, err := fetchChannelUpstreamModelIDs(channel)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch upstream models from %s: %w (set models.fetch=false and models.list to use a fixed list)", channel.GetBaseURL(), err)
		}
		fetched = ids
	}
	models, err := channelconfig.FilterModels(fetched, modelsConfig.List, modelsConfig.Include, modelsConfig.Exclude)
	if err != nil {
		return nil, nil, err
	}
	if len(models) == 0 {
		return nil, nil, errors.New("no models resolved: enable models.fetch, add models.list, or relax the include/exclude filters")
	}
	return models, fetched, nil
}

func findChannelForConfig(name string) (*model.Channel, error) {
	var channels []model.Channel
	if err := model.DB.Where("name = ?", name).Order("id asc").Limit(2).Find(&channels).Error; err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, nil
	}
	if len(channels) > 1 {
		common.SysLog(fmt.Sprintf("channel config: %d channels are named %q, updating #%d", len(channels), name, channels[0].Id))
	}
	return &channels[0], nil
}

// applyChannelConfigToExisting copies the declarative fields onto a loaded
// channel. Fields omitted by the config keep their current value; fields the
// config owns are always replaced so rerunning converges to the file.
func applyChannelConfigToExisting(existing *model.Channel, prepared *model.Channel, source *channelconfig.Channel) {
	existing.Type = prepared.Type
	existing.Name = prepared.Name
	existing.Key = prepared.Key
	existing.BaseURL = prepared.BaseURL
	existing.Models = prepared.Models
	existing.Group = prepared.Group
	existing.ParamOverride = prepared.ParamOverride
	existing.HeaderOverride = prepared.HeaderOverride
	existing.ChannelInfo = prepared.ChannelInfo

	if prepared.Priority != nil {
		existing.Priority = prepared.Priority
	}
	if prepared.Weight != nil {
		existing.Weight = prepared.Weight
	}
	if source.Status != nil {
		existing.Status = prepared.Status
	}
	if source.Tag != nil {
		existing.Tag = prepared.Tag
	}
	if source.Remark != nil {
		existing.Remark = prepared.Remark
	}
	if source.TestModel != nil {
		existing.TestModel = prepared.TestModel
	}
	if source.AutoBan != nil {
		existing.AutoBan = prepared.AutoBan
	}
	if source.Other != "" {
		existing.Other = prepared.Other
	}
	if source.ModelMapping != nil {
		existing.ModelMapping = prepared.ModelMapping
	}
	if prepared.Setting != nil {
		existing.Setting = prepared.Setting
	}
	if prepared.OtherSettings != "" {
		existing.OtherSettings = prepared.OtherSettings
	}
}

func marshalOverrideJSON(value map[string]any) (*string, error) {
	if len(value) == 0 {
		empty := "{}"
		return &empty, nil
	}
	encoded, err := common.Marshal(value)
	if err != nil {
		return nil, err
	}
	text := string(encoded)
	return &text, nil
}

func buildChannelSetting(proxy string, settings map[string]any) (*string, error) {
	if proxy == "" && len(settings) == 0 {
		return nil, nil
	}
	merged := make(map[string]any, len(settings)+1)
	for key, value := range settings {
		merged[key] = value
	}
	if proxy != "" {
		merged["proxy"] = proxy
	}
	encoded, err := common.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	var parsed dto.ChannelSettings
	if err := common.Unmarshal(encoded, &parsed); err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	text := string(encoded)
	return &text, nil
}

func buildChannelOtherSettings(settings map[string]any) (string, error) {
	if len(settings) == 0 {
		return "", nil
	}
	encoded, err := common.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("other_settings: %w", err)
	}
	var parsed dto.ChannelOtherSettings
	if err := common.Unmarshal(encoded, &parsed); err != nil {
		return "", fmt.Errorf("other_settings: %w", err)
	}
	normalized, err := common.Marshal(parsed)
	if err != nil {
		return "", fmt.Errorf("other_settings: %w", err)
	}
	return string(normalized), nil
}

// priceLayersForChannel returns the price layers that belong to one channel,
// from the global defaults up to that channel's per-model overrides. The global
// pricing.models layer is applied separately because it spans all channels.
func priceLayersForChannel(config *channelconfig.Config, source *channelconfig.Channel) []channelconfig.PriceLayer {
	layers := make([]channelconfig.PriceLayer, 0, 3)
	if config.Pricing != nil && config.Pricing.Defaults != nil {
		layers = append(layers, channelconfig.PriceLayer{Defaults: config.Pricing.Defaults})
	}
	if source.Pricing != nil {
		if source.Pricing.Defaults != nil {
			layers = append(layers, channelconfig.PriceLayer{Defaults: source.Pricing.Defaults})
		}
		if len(source.Pricing.Models) > 0 {
			layers = append(layers, channelconfig.PriceLayer{Models: source.Pricing.Models})
		}
	}
	return layers
}

func buildPricePlan(config *channelconfig.Config, prepared []*preparedChannelConfig) (map[string]channelconfig.ModelPrice, []string, error) {
	overall := make(map[string]channelconfig.ModelPrice)
	owner := make(map[string]string)
	warnings := []string{}
	allModels := make([]string, 0)
	unknownModels := make(map[string]struct{})

	for _, item := range prepared {
		plan, unknown, err := channelconfig.BuildPricePlan(item.models, priceLayersForChannel(config, item.source))
		if err != nil {
			return nil, nil, fmt.Errorf("channel %q: %w", item.source.Name, err)
		}
		for _, name := range unknown {
			unknownModels[name] = struct{}{}
		}
		for name, price := range plan {
			if previous, exists := overall[name]; exists && !previous.Equal(price) {
				warnings = append(warnings, fmt.Sprintf(
					"model %q is priced differently by channels %q and %q; the later channel wins",
					name, owner[name], item.source.Name,
				))
			}
			overall[name] = price
			owner[name] = item.source.Name
		}
		allModels = append(allModels, item.models...)
	}

	allModels = uniqueSortedStrings(allModels)
	scope := make(map[string]struct{}, len(allModels))
	for _, name := range allModels {
		scope[name] = struct{}{}
	}
	for _, name := range sortedStringSet(unknownModels) {
		if _, ok := scope[name]; !ok {
			warnings = append(warnings, fmt.Sprintf("pricing.model %q is not served by any configured channel", name))
		}
	}

	if config.Pricing != nil && len(config.Pricing.Models) > 0 {
		plan, unknown, err := channelconfig.BuildPricePlan(allModels, []channelconfig.PriceLayer{{Models: config.Pricing.Models}})
		if err != nil {
			return nil, nil, fmt.Errorf("pricing: %w", err)
		}
		for name, price := range plan {
			overall[name] = price
		}
		for _, name := range unknown {
			warnings = append(warnings, fmt.Sprintf("pricing.model %q is not served by any configured channel", name))
		}
	}

	if configHasPricing(config) {
		for _, name := range allModels {
			if _, ok := overall[name]; ok {
				continue
			}
			if ratio_setting.IsRegisteredModel(name) {
				continue
			}
			if _, ok := ratio_setting.GetImagePrice(name); ok {
				continue
			}
			if _, ok := ratio_setting.GetVideoPrice(name); ok {
				continue
			}
			warnings = append(warnings, fmt.Sprintf(
				"model %q has no configured price and no built-in price; requests for it fail with \"model price is not configured\"",
				name,
			))
		}
	}
	return overall, warnings, nil
}

// configHasPricing reports whether the file configures any price at all. When
// it does not, model prices are maintained elsewhere and import must not warn
// about every unpriced model.
func configHasPricing(config *channelconfig.Config) bool {
	has := func(pricing *channelconfig.Pricing) bool {
		return pricing != nil && (pricing.Defaults != nil || len(pricing.Models) > 0)
	}
	if has(config.Pricing) {
		return true
	}
	for index := range config.Channels {
		if has(config.Channels[index].Pricing) {
			return true
		}
	}
	return false
}

// refreshChannelConfigPrices re-applies the price rules of CHANNEL_CONFIG_FILE
// to the channel's current model list. It runs after upstream discovery changes
// a channel's models, so newly added models are priced immediately without
// rerunning the whole import. Only prices are touched; keys, addresses and
// other admin edits are left alone.
func refreshChannelConfigPrices(channel *model.Channel) (int, []string, error) {
	path := channelConfigPath()
	if path == "" {
		return 0, nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, fmt.Errorf("read channel config %s: %w", path, err)
	}
	config, err := channelconfig.Parse(data)
	if err != nil {
		return 0, nil, fmt.Errorf("parse channel config %s: %w", path, err)
	}

	var source *channelconfig.Channel
	for index := range config.Channels {
		if config.Channels[index].Name == channel.Name {
			source = &config.Channels[index]
			break
		}
	}
	if source == nil {
		return 0, []string{fmt.Sprintf("channel %q is not defined in %s; prices were not refreshed", channel.Name, path)}, nil
	}

	models, err := channelconfig.FilterModels(channel.GetModels(), nil, nil, nil)
	if err != nil {
		return 0, nil, err
	}
	if len(models) == 0 {
		return 0, nil, nil
	}

	plan, unknown, err := channelconfig.BuildPricePlan(models, priceLayersForChannel(config, source))
	if err != nil {
		return 0, nil, fmt.Errorf("build price plan for channel %q: %w", channel.Name, err)
	}
	warnings := make([]string, 0, len(unknown))
	for _, name := range unknown {
		warnings = append(warnings, fmt.Sprintf("channel %q: pricing.model %q is not served by the channel", channel.Name, name))
	}
	if config.Pricing != nil && len(config.Pricing.Models) > 0 {
		globalPlan, _, err := channelconfig.BuildPricePlan(models, []channelconfig.PriceLayer{{Models: config.Pricing.Models}})
		if err != nil {
			return 0, warnings, fmt.Errorf("build global price plan for channel %q: %w", channel.Name, err)
		}
		for name, price := range globalPlan {
			plan[name] = price
		}
	}
	if len(plan) == 0 {
		return 0, warnings, nil
	}
	for _, name := range models {
		if _, ok := plan[name]; ok {
			continue
		}
		if ratio_setting.IsRegisteredModel(name) {
			continue
		}
		if _, ok := ratio_setting.GetImagePrice(name); ok {
			continue
		}
		if _, ok := ratio_setting.GetVideoPrice(name); ok {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"channel %q: model %q has no configured price and no built-in price; requests for it fail with \"model price is not configured\"",
			channel.Name, name,
		))
	}

	result, err := service.ApplyModelPrices(plan)
	if err != nil {
		return 0, warnings, err
	}
	if result == nil {
		return 0, warnings, nil
	}
	warnings = append(warnings, result.Warnings...)
	return result.Models, warnings, nil
}

// refreshChannelConfigPricesAfterUpstreamChange is the non-fatal hook used by
// the upstream model update flows: a broken bootstrap file must not block the
// model list update the admin explicitly asked for.
func refreshChannelConfigPricesAfterUpstreamChange(channel *model.Channel) {
	priced, warnings, err := refreshChannelConfigPrices(channel)
	for _, warning := range warnings {
		common.SysLog(fmt.Sprintf("channel config price refresh: channel_id=%d channel_name=%s %s", channel.Id, channel.Name, warning))
	}
	if err != nil {
		common.SysError(fmt.Sprintf("channel config price refresh failed: channel_id=%d channel_name=%s err=%v", channel.Id, channel.Name, err))
		return
	}
	if priced > 0 {
		common.SysLog(fmt.Sprintf("channel config price refresh applied: channel_id=%d channel_name=%s models=%d", channel.Id, channel.Name, priced))
	}
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
