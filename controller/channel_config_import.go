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
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/channelconfig"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

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

// ChannelConfigApplyResult is the full outcome of one import.
type ChannelConfigApplyResult struct {
	Channels []ChannelConfigChannelResult `json:"channels"`
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
	// initial setup and watched for ConfigMap updates. When it is empty both are
	// disabled.
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
func ApplyChannelConfigFromEnv() (*ChannelConfigApplyResult, error) {
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
	return applyChannelConfigBytes(path, data)
}

func channelConfigPath() string {
	return strings.TrimSpace(os.Getenv(ChannelConfigEnvFile))
}

// applyChannelConfigBytes parses and applies one snapshot of the file. The
// applied hash is recorded only after a successful apply so a failed attempt is
// retried on the next watcher tick.
func applyChannelConfigBytes(path string, data []byte) (*ChannelConfigApplyResult, error) {
	config, err := channelconfig.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse channel config %s: %w", path, err)
	}
	result, err := applyChannelConfig(config)
	if err != nil {
		return result, err
	}
	hash := sha256.Sum256(data)
	channelConfigStateMu.Lock()
	channelConfigApplied = true
	channelConfigAppliedHash = hash
	channelConfigStateMu.Unlock()
	return result, nil
}

// StartChannelConfigSync applies CHANNEL_CONFIG_FILE once and then watches it
// for changes. A ConfigMap volume is updated by atomically swapping a symlink,
// so polling the file content is reliable and needs no restart or fsnotify.
//
// Only the master node applies the file; other replicas pick the result up
// through the regular channel/option sync loops. This avoids every replica
// writing the same channels when the ConfigMap changes.
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

	result, err := applyChannelConfigBytes(path, data)
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
	common.SysLog(message)
}

// applyChannelConfig creates or updates the channels described by the config.
//
// Model discovery is a preflight step: if any channel cannot be built or its
// upstream model list cannot be fetched, nothing is written. Failures while
// writing individual channels are collected so the remaining channels can
// still be applied; the import is idempotent and can be rerun.
func applyChannelConfig(config *channelconfig.Config) (*ChannelConfigApplyResult, error) {
	channelConfigApplyMu.Lock()
	defer channelConfigApplyMu.Unlock()

	result := &ChannelConfigApplyResult{
		Channels: make([]ChannelConfigChannelResult, 0, len(config.Channels)),
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

	var applyErrors []error
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
			applyChannelConfigToExisting(item.existing, item.channel, item.source)
			if err := item.existing.Save(); err != nil {
				channelResult.Action = "failed"
				channelResult.Error = fmt.Sprintf("save channel: %v", err)
			} else if err := item.existing.UpdateAbilities(nil); err != nil {
				channelResult.Action = "failed"
				channelResult.Error = fmt.Sprintf("rebuild abilities: %v", err)
			}
		} else {
			channelResult.Action = "created"
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
		if channelResult.Error != "" {
			applyErrors = append(applyErrors, fmt.Errorf("channel %q: %s", item.source.Name, channelResult.Error))
		}
		result.Channels = append(result.Channels, channelResult)
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
