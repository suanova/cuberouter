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
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestModelTypeResolver(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	origDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = origDB })
	require.NoError(t, db.AutoMigrate(&model.Model{}))
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: "gpt-4o", Tags: "推理,工具"},
		{ModelName: "gpt-image", Tags: "图像生成"},
		{ModelName: "wan3.0-video", Tags: "视频生成"},
		{ModelName: "tts-1", Tags: "音频处理"},
		{ModelName: "embed", Tags: "向量"},
		{ModelName: "deleted-image", Tags: "图像生成", DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}).Error)

	resolver, err := LoadModelTypeResolver()
	require.NoError(t, err)
	assert.Equal(t, ModelUsageTypeText, resolver.Resolve("gpt-4o"))
	assert.Equal(t, ModelUsageTypeImage, resolver.Resolve("gpt-image"))
	assert.Equal(t, ModelUsageTypeVideo, resolver.Resolve("wan3.0-video"))
	assert.Equal(t, ModelUsageTypeAudio, resolver.Resolve("tts-1"))
	assert.Equal(t, ModelUsageTypeVector, resolver.Resolve("embed"))
	assert.Equal(t, ModelUsageTypeImage, resolver.Resolve("deleted-image"))
	assert.Equal(t, ModelUsageTypeImage, resolver.Resolve("mj_imagine"))
	assert.Equal(t, ModelUsageTypeImage, resolver.Resolve("swap_face"))
	assert.Equal(t, ModelUsageTypeVideo, resolver.Resolve("mj_video"))
	assert.Equal(t, ModelUsageTypeOther, resolver.Resolve("unknown-model"))
}
