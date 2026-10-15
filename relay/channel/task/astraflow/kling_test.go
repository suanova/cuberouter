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
package astraflow

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- kling-v3 成形 ----------

// TestKlingV3I2VFromImages 锁定图生视频契约：统一字段 images 落到 kling 方言的
// parameters.image / image_tail（首帧/尾帧），kling_v3_type 自动推导为 i2v，
// 缺省 mode/sound 按上游默认补齐（std / off）。
func TestKlingV3I2VFromImages(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "kling-v3",
		Prompt:   "gentle motion",
		Images:   []string{"https://cdn.example.com/first.jpg", "https://cdn.example.com/last.jpg"},
		Duration: 5,
	}

	body, err := adaptor.convertKlingRequestPayload(&req, "kling-v3")
	require.NoError(t, err)

	require.Equal(t, "kling-v3", body.Model)
	require.Equal(t, "gentle motion", body.Input.Prompt)
	require.Equal(t, "i2v", body.Parameters.KlingV3Type)
	require.Equal(t, "https://cdn.example.com/first.jpg", body.Parameters.Image)
	require.Equal(t, "https://cdn.example.com/last.jpg", body.Parameters.ImageTail)
	require.Equal(t, "std", body.Parameters.Mode)
	require.Equal(t, "off", body.Parameters.Sound)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 5, int(*body.Parameters.Duration))
	// kling 不走 Ark content：input 里不得出现 content 字段（结构体无该字段即为编译期保证）。
	assert.Empty(t, body.Input.ImgURL)
	assert.Empty(t, body.Input.VideoURL)
}

// TestKlingV3T2VWhenNoImage 锁定模式推导：无图无视频 → t2v，且不携带 image。
func TestKlingV3T2VWhenNoImage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{Model: "kling-v3", Prompt: "a sunset", Mode: "pro", Seconds: "3"}

	body, err := adaptor.convertKlingRequestPayload(&req, "kling-v3")
	require.NoError(t, err)

	require.Equal(t, "t2v", body.Parameters.KlingV3Type)
	require.Empty(t, body.Parameters.Image)
	require.Equal(t, "pro", body.Parameters.Mode)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 3, int(*body.Parameters.Duration))
}

// TestKlingV3MetadataOverridesAndMotionControl 锁定 metadata 透传与运动控制形态：
// kling_v3_type/mode/sound/aspect_ratio 显式值覆盖推导；img_url+video_url 走
// motion_control（input 承载，不在 parameters）。
func TestKlingV3MetadataOverridesAndMotionControl(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "kling-v3",
		Prompt: "dance",
		Images: []string{"https://cdn.example.com/would-be-first.jpg"},
		Metadata: map[string]interface{}{
			"kling_v3_type":         "motion_control",
			"img_url":               "https://cdn.example.com/character.jpg",
			"video_url":             "https://cdn.example.com/motion.mp4",
			"character_orientation": "image",
			"keep_original_sound":   "no",
			"sound":                 "off",
			"aspect_ratio":          "9:16",
		},
	}

	body, err := adaptor.convertKlingRequestPayload(&req, "kling-v3")
	require.NoError(t, err)

	require.Equal(t, "motion_control", body.Parameters.KlingV3Type)
	require.Equal(t, "https://cdn.example.com/character.jpg", body.Input.ImgURL)
	require.Equal(t, "https://cdn.example.com/motion.mp4", body.Input.VideoURL)
	require.Equal(t, "image", body.Parameters.CharacterOrientation)
	require.Equal(t, "no", body.Parameters.KeepOriginalSound)
	require.Equal(t, "9:16", body.Parameters.AspectRatio)
	// motion_control 的参考图在 input.img_url：显式指定模式时不得再落 parameters.image。
	assert.Empty(t, body.Parameters.Image)
}

// TestKlingV3ExplicitTypeWinsOverDerivation 锁定显式模式优先：带图但显式 t2v 时
// 保持 t2v（生产环境建议显式指定，避免上游隐式路由歧义）。
func TestKlingV3ExplicitTypeWinsOverDerivation(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "kling-v3",
		Prompt:   "text only",
		Images:   []string{"https://cdn.example.com/ignored.jpg"},
		Metadata: map[string]interface{}{"kling_v3_type": "t2v"},
	}

	body, err := adaptor.convertKlingRequestPayload(&req, "kling-v3")
	require.NoError(t, err)
	require.Equal(t, "t2v", body.Parameters.KlingV3Type)
}

// ---------- kling-v3-omni 成形 ----------

// TestKlingOmniImageListTypes 锁定 omni 首尾帧/参考图的类型标注：单图 = 首帧；
// 两张 = 首帧+尾帧；三张及以上不带类型（参考图），并沿用上游默认 mode=pro、
// 缺省宽高比 16:9。
func TestKlingOmniImageListTypes(t *testing.T) {
	adaptor := &TaskAdaptor{}

	single := relaycommon.TaskSubmitReq{Model: "kling-v3-omni", Prompt: "x", Images: []string{"https://cdn.example.com/a.jpg"}}
	body, err := adaptor.convertKlingRequestPayload(&single, "kling-v3-omni")
	require.NoError(t, err)
	require.Equal(t, "pro", body.Parameters.Mode)
	require.Equal(t, "16:9", body.Parameters.AspectRatio)
	require.Len(t, body.Parameters.ImageList, 1)
	assert.Equal(t, "first_frame", body.Parameters.ImageList[0].Type)

	two := relaycommon.TaskSubmitReq{Model: "kling-v3-omni", Prompt: "x", Images: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"}}
	body, err = adaptor.convertKlingRequestPayload(&two, "kling-v3-omni")
	require.NoError(t, err)
	require.Len(t, body.Parameters.ImageList, 2)
	assert.Equal(t, "first_frame", body.Parameters.ImageList[0].Type)
	assert.Equal(t, "end_frame", body.Parameters.ImageList[1].Type)

	three := relaycommon.TaskSubmitReq{Model: "kling-v3-omni", Prompt: "x", Images: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg", "https://cdn.example.com/c.jpg"}}
	body, err = adaptor.convertKlingRequestPayload(&three, "kling-v3-omni")
	require.NoError(t, err)
	require.Len(t, body.Parameters.ImageList, 3)
	assert.Empty(t, body.Parameters.ImageList[0].Type, "3+ images are untyped reference images")
	assert.Empty(t, body.Parameters.ImageList[1].Type)
	assert.Empty(t, body.Parameters.ImageList[2].Type)
}

// TestKlingOmniVideoListPassthrough 锁定视频编辑/参考透传：metadata.video_list 原样
// 落到 parameters.video_list；显式 video_list 时不再回填 image_list。
func TestKlingOmniVideoListPassthrough(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "kling-v3-omni",
		Prompt: "color grade it",
		Metadata: map[string]interface{}{
			"video_list": []interface{}{
				map[string]interface{}{"video_url": "https://cdn.example.com/in.mp4", "refer_type": "base", "keep_original_sound": "yes"},
			},
		},
	}

	body, err := adaptor.convertKlingRequestPayload(&req, "kling-v3-omni")
	require.NoError(t, err)
	require.Len(t, body.Parameters.VideoList, 1)
	assert.Equal(t, "https://cdn.example.com/in.mp4", body.Parameters.VideoList[0].VideoURL)
	assert.Equal(t, "base", body.Parameters.VideoList[0].ReferType)
	assert.Equal(t, "yes", body.Parameters.VideoList[0].KeepOriginalSound)
	assert.Empty(t, body.Parameters.ImageList)
}

// ---------- 计费事实 ----------

// TestKlingUsageFacts 锁定计费事实契约：表达式 u() 变量读取的
// seconds/mode/sound/variant 必须与成形逻辑同源；时长优先级 metadata > duration >
// seconds > 缺省 5；omni 的参考维度按 image_list 是否含无类型参考图推导，
// metadata.reference 可显式覆盖。
func TestKlingUsageFacts(t *testing.T) {
	facts, err := klingUsageFacts(&relaycommon.TaskSubmitReq{
		Model:    "kling-v3",
		Prompt:   "x",
		Images:   []string{"https://cdn.example.com/a.jpg"},
		Duration: 3,
	}, "kling-v3")
	require.NoError(t, err)
	assert.Equal(t, 3.0, facts["seconds"])
	assert.Equal(t, "std", facts["mode"])
	assert.Equal(t, "off", facts["sound"])
	assert.Equal(t, "i2v", facts["variant"])

	facts, err = klingUsageFacts(&relaycommon.TaskSubmitReq{
		Model:    "kling-v3",
		Prompt:   "x",
		Metadata: map[string]interface{}{"duration": 10, "mode": "pro", "sound": "on", "kling_v3_type": "t2v"},
	}, "kling-v3")
	require.NoError(t, err)
	assert.Equal(t, 10.0, facts["seconds"])
	assert.Equal(t, "pro", facts["mode"])
	assert.Equal(t, "on", facts["sound"])
	assert.Equal(t, "t2v", facts["variant"])

	// omni：无参考 → noref；带无类型参考图 → ref；metadata.reference 显式覆盖。
	facts, err = klingUsageFacts(&relaycommon.TaskSubmitReq{
		Model: "kling-v3-omni", Prompt: "x", Images: []string{"https://cdn.example.com/a.jpg"},
	}, "kling-v3-omni")
	require.NoError(t, err)
	assert.Equal(t, "noref", facts["reference"], "a single typed first frame is not a reference")

	facts, err = klingUsageFacts(&relaycommon.TaskSubmitReq{
		Model: "kling-v3-omni", Prompt: "x",
		Images: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg", "https://cdn.example.com/c.jpg"},
	}, "kling-v3-omni")
	require.NoError(t, err)
	assert.Equal(t, "ref", facts["reference"])

	facts, err = klingUsageFacts(&relaycommon.TaskSubmitReq{
		Model: "kling-v3-omni", Prompt: "x", Images: []string{"https://cdn.example.com/a.jpg"},
		Metadata: map[string]interface{}{"reference": "ref"},
	}, "kling-v3-omni")
	require.NoError(t, err)
	assert.Equal(t, "ref", facts["reference"], "metadata.reference overrides the heuristic")

	// 非法枚举：模式/声音/时长越界直接报错（计费事实必须可校验）。
	_, err = klingUsageFacts(&relaycommon.TaskSubmitReq{Model: "kling-v3", Prompt: "x", Mode: "ultra"}, "kling-v3")
	require.Error(t, err)
	_, err = klingUsageFacts(&relaycommon.TaskSubmitReq{Model: "kling-v3", Prompt: "x", Metadata: map[string]interface{}{"sound": "loud"}}, "kling-v3")
	require.Error(t, err)
}

// ---------- 轮询与终态 ----------

// TestParseTaskResultUnknownStatusMapsToUnknown 锁定未知状态契约：上游返回
// 无法识别的 task_status 时映射为 UNKNOWN（轮询层按失败预算处理 → 超限失败并退款），
// 而不是 InProgress 永久静默等待。
func TestParseTaskResultUnknownStatusMapsToUnknown(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{"output":{"task_id":"t1","task_status":"Weird"},"usage":{}}`)

	info, err := adaptor.ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, "UNKNOWN", info.Status)
}

// TestParseTaskResultKlingSuccessCarriesSecondsFact 锁定终态事实：成功响应中的
// usage.duration 作为 seconds 事实并入 UsageFacts，结算时用终态事实重跑表达式。
func TestParseTaskResultKlingSuccessCarriesSecondsFact(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{"output":{"task_id":"t1","task_status":"Success","urls":["https://cdn.example.com/v.mp4"]},"usage":{"duration":8}}`)

	info, err := adaptor.ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, "SUCCESS", info.Status)
	require.NotNil(t, info.UsageFacts)
	assert.Equal(t, 8.0, info.UsageFacts["seconds"])
}

// ---------- 兼容 ----------

// TestSeedanceKeepsArkShape 锁定回归：非 kling 模型仍按既有 Ark 形态成形
// （content 直传 + duration 进 parameters），不受 kling 分支影响。
func TestSeedanceKeepsArkShape(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:      "doubao-seedance-2-5-260628",
		Prompt:     "run",
		Resolution: "720p",
		Duration:   5,
		Content: []relaycommon.TaskContentItem{
			{Type: "image_url", ImageURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://cdn.example.com/f.jpg")}, Role: lo.ToPtr("first_frame")},
		},
	}

	body, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)
	require.Len(t, body.Input.Content, 2)
	assert.Equal(t, "720p", body.Parameters.Resolution)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 5, int(*body.Parameters.Duration))
}

// 保证测试引用的 dto 类型可用（IntValue/BoolValue 在成形结构中被使用）。
var _ = dto.IntValue(0)

// TestKlingUsageSchemaRegistered 锁定 init 注册契约：kling 家族模型拥有注册的
// 计费事实 schema（表达式保存时可声明 u() 键），非 kling 模型不受影响。
func TestKlingUsageSchemaRegistered(t *testing.T) {
	schema, ok := billingexpr.LookupTaskUsageSchema("kling-v3")
	require.True(t, ok)
	for _, key := range []string{"seconds", "mode", "sound", "variant", "reference"} {
		require.Contains(t, schema, key)
	}

	_, ok = billingexpr.LookupTaskUsageSchema("doubao-seedance-2-5-260628")
	require.False(t, ok)
}
