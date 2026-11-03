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
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// ModelUsageType is the stable enum the usage page exposes.
type ModelUsageType string

const (
	ModelUsageTypeText   ModelUsageType = "text"
	ModelUsageTypeImage  ModelUsageType = "image"
	ModelUsageTypeVideo  ModelUsageType = "video"
	ModelUsageTypeAudio  ModelUsageType = "audio"
	ModelUsageTypeVector ModelUsageType = "vector"
	ModelUsageTypeOther  ModelUsageType = "other"
)

const (
	modelUsageTagImage  = "图像生成"
	modelUsageTagVideo  = "视频生成"
	modelUsageTagAudio  = "音频处理"
	modelUsageTagVector = "向量"
)

// ModelTypeResolver maps a model name to its usage type using the model tag
// vocabulary in docs/import-models/model-tags.md. Soft-deleted model rows are
// included so historical usage keeps its type; an active row wins over a
// deleted row with the same name.
type ModelTypeResolver struct {
	byName map[string]ModelUsageType
}

func LoadModelTypeResolver() (*ModelTypeResolver, error) {
	var models []model.Model
	if err := model.DB.Unscoped().
		Select("id, model_name, tags, deleted_at").
		Find(&models).Error; err != nil {
		return nil, err
	}
	byName := make(map[string]ModelUsageType, len(models))
	for _, item := range models {
		kind := modelTagsUsageType(item.Tags)
		if !item.DeletedAt.Valid {
			byName[item.ModelName] = kind
		} else if _, ok := byName[item.ModelName]; !ok {
			byName[item.ModelName] = kind
		}
	}
	return &ModelTypeResolver{byName: byName}, nil
}

func (r *ModelTypeResolver) Resolve(modelName string) ModelUsageType {
	if r == nil {
		return ModelUsageTypeOther
	}
	// Midjourney models are not in the models table; their prefix is the type.
	if modelName == "mj_video" {
		return ModelUsageTypeVideo
	}
	if strings.HasPrefix(modelName, "mj_") || modelName == "swap_face" {
		return ModelUsageTypeImage
	}
	if kind, ok := r.byName[modelName]; ok {
		return kind
	}
	return ModelUsageTypeOther
}

func modelTagsUsageType(tags string) ModelUsageType {
	for _, raw := range strings.Split(tags, ",") {
		switch strings.TrimSpace(raw) {
		case modelUsageTagImage:
			return ModelUsageTypeImage
		case modelUsageTagVideo:
			return ModelUsageTypeVideo
		case modelUsageTagAudio:
			return ModelUsageTypeAudio
		case modelUsageTagVector:
			return ModelUsageTypeVector
		}
	}
	return ModelUsageTypeText
}
