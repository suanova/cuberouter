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

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- wan3 成形 ----------

// TestWanT2VDefaults 锁定文生视频契约：prompt 落 input.prompt；参数缺省
// duration=5、resolution 显式 720p（上游缺省 1080p，与计费缺省档不一致，
// 必须显式下发）；无媒体时不下发 ratio（走上游缺省 16:9）、不下发 media。
func TestWanT2VDefaults(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "a sunset over the sea"}

	body, err := deriveWanRequest(&req, "wan3.0-video")
	require.NoError(t, err)

	require.Equal(t, "wan3.0-video", body.Model)
	require.Equal(t, "a sunset over the sea", body.Input.Prompt)
	assert.Empty(t, body.Input.Media)
	assert.Equal(t, "720p", body.Parameters.Resolution)
	assert.Empty(t, body.Parameters.Ratio)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 5, int(*body.Parameters.Duration))
}

// TestWanI2VFromSingleImage 锁定图生视频契约：单图（OpenAI 风格 images）=
// first_frame 媒体；含首帧时 ratio 缺省 adaptive（输出比例跟随输入图）。
func TestWanI2VFromSingleImage(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "girl runs",
		Images: []string{"https://cdn.example.com/first.jpg"},
	}

	body, err := deriveWanRequest(&req, "wan3.0-video")
	require.NoError(t, err)

	require.Len(t, body.Input.Media, 1)
	assert.Equal(t, "first_frame", body.Input.Media[0].Type)
	assert.Equal(t, "https://cdn.example.com/first.jpg", body.Input.Media[0].URL)
	assert.Equal(t, "adaptive", body.Parameters.Ratio)
}

// TestWanFirstLastFromTwoImages 锁定首尾帧契约：两张图 = first_frame +
// last_frame（模型自动生成中间过渡）。
func TestWanFirstLastFromTwoImages(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video-prime",
		Prompt: "cat stands up",
		Images: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"},
	}

	body, err := deriveWanRequest(&req, "wan3.0-video-prime")
	require.NoError(t, err)

	require.Len(t, body.Input.Media, 2)
	assert.Equal(t, "first_frame", body.Input.Media[0].Type)
	assert.Equal(t, "last_frame", body.Input.Media[1].Type)
	assert.Equal(t, "adaptive", body.Parameters.Ratio)
}

// TestWanReferencesFromThreeImages 锁定参考生视频契约：三张及以上无角色图 =
// reference_image（全模态参考）；无首尾帧时不下发 ratio。
func TestWanReferencesFromThreeImages(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "the character in image 1 dances",
		Images: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg", "https://cdn.example.com/c.jpg"},
	}

	body, err := deriveWanRequest(&req, "wan3.0-video")
	require.NoError(t, err)

	require.Len(t, body.Input.Media, 3)
	for i, media := range body.Input.Media {
		assert.Equal(t, "reference_image", media.Type, "media[%d]", i)
	}
	assert.Empty(t, body.Parameters.Ratio)
}

// TestWanContentRolesAndVideoAudio 锁定 content 数组归类：image_url 的角色
// 标注优先于数量启发式；video_url/audio_url 项按 reference_video /
// reference_audio 归类（与 seedance 的 Ark content 请求形态兼容）。
func TestWanContentRolesAndVideoAudio(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "dance to the music",
		Content: []relaycommon.TaskContentItem{
			{Type: "image_url", ImageURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://cdn.example.com/ref.jpg")}, Role: lo.ToPtr("reference_image")},
			{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://cdn.example.com/motion.mp4")}},
			{Type: "audio_url", AudioURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://cdn.example.com/music.mp3")}},
		},
	}

	body, err := deriveWanRequest(&req, "wan3.0-video")
	require.NoError(t, err)

	require.Len(t, body.Input.Media, 3)
	assert.Equal(t, "reference_image", body.Input.Media[0].Type)
	assert.Equal(t, "https://cdn.example.com/ref.jpg", body.Input.Media[0].URL)
	assert.Equal(t, "reference_video", body.Input.Media[1].Type)
	assert.Equal(t, "reference_audio", body.Input.Media[2].Type)
}

// TestWanUntypedImagePositionalUnderMixedRoles 锁定混用规则：显式角色优先，
// 无角色图片按"全部图片"的数量与序号参与启发式（两张图中的无角色项 = 尾帧）。
func TestWanUntypedImagePositionalUnderMixedRoles(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "x",
		Content: []relaycommon.TaskContentItem{
			{Type: "image_url", ImageURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://cdn.example.com/first.jpg")}, Role: lo.ToPtr("first_frame")},
			{Type: "image_url", ImageURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://cdn.example.com/last.jpg")}},
		},
	}

	body, err := deriveWanRequest(&req, "wan3.0-video")
	require.NoError(t, err)

	require.Len(t, body.Input.Media, 2)
	assert.Equal(t, "first_frame", body.Input.Media[0].Type)
	assert.Equal(t, "last_frame", body.Input.Media[1].Type)
}

// TestWanMetadataOverrides 锁定 metadata 显式通道：media 整体替换推导结果
// （reference_video 等只能由此下发）；resolution/ratio/duration 显式值优先。
func TestWanMetadataOverrides(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "x",
		Images: []string{"https://cdn.example.com/ignored.jpg"},
		Metadata: map[string]interface{}{
			"media": []interface{}{
				map[string]interface{}{"type": "reference_video", "url": "https://cdn.example.com/motion.mp4"},
			},
			"resolution": "1080p",
			"ratio":      "9:16",
			"duration":   8,
		},
	}

	body, err := deriveWanRequest(&req, "wan3.0-video")
	require.NoError(t, err)

	require.Len(t, body.Input.Media, 1)
	assert.Equal(t, "reference_video", body.Input.Media[0].Type)
	assert.Equal(t, "1080p", body.Parameters.Resolution)
	assert.Equal(t, "9:16", body.Parameters.Ratio)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 8, int(*body.Parameters.Duration))
}

// TestWanDurationPriorityAndBounds 锁定时长契约：metadata > duration > seconds >
// 缺省 5；计费上不可表达的取值（≤0 或超过 MaxTaskDurationSeconds）直接报错。
func TestWanDurationPriorityAndBounds(t *testing.T) {
	body, err := deriveWanRequest(&relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "x", Seconds: "3"}, "wan3.0-video")
	require.NoError(t, err)
	assert.Equal(t, 3, int(*body.Parameters.Duration))

	body, err = deriveWanRequest(&relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "x", Duration: 7, Seconds: "3"}, "wan3.0-video")
	require.NoError(t, err)
	assert.Equal(t, 7, int(*body.Parameters.Duration))

	body, err = deriveWanRequest(&relaycommon.TaskSubmitReq{
		Model: "wan3.0-video", Prompt: "x", Duration: 7,
		Metadata: map[string]interface{}{"duration": 9},
	}, "wan3.0-video")
	require.NoError(t, err)
	assert.Equal(t, 9, int(*body.Parameters.Duration))

	// 上游 -1（自动时长）在计费上不可表达：拒绝而不是按缺省 5 秒静默计费。
	_, err = deriveWanRequest(&relaycommon.TaskSubmitReq{
		Model: "wan3.0-video", Prompt: "x",
		Metadata: map[string]interface{}{"duration": -1},
	}, "wan3.0-video")
	require.Error(t, err)

	_, err = deriveWanRequest(&relaycommon.TaskSubmitReq{
		Model: "wan3.0-video", Prompt: "x",
		Metadata: map[string]interface{}{"duration": relaycommon.MaxTaskDurationSeconds + 1},
	}, "wan3.0-video")
	require.Error(t, err)
}

// TestWanResolutionFromSize 锁定分辨率归一：size 尺寸写法归一到档位；
// 无法识别的非空取值原样下发（交给上游拒绝，不静默改写）。
func TestWanResolutionFromSize(t *testing.T) {
	body, err := deriveWanRequest(&relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "x", Size: "1280x720"}, "wan3.0-video")
	require.NoError(t, err)
	assert.Equal(t, "720p", body.Parameters.Resolution)

	body, err = deriveWanRequest(&relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "x", Size: "1920x1080"}, "wan3.0-video")
	require.NoError(t, err)
	assert.Equal(t, "1080p", body.Parameters.Resolution)

	body, err = deriveWanRequest(&relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "x", Resolution: "720P"}, "wan3.0-video")
	require.NoError(t, err)
	assert.Equal(t, "720p", body.Parameters.Resolution)

	body, err = deriveWanRequest(&relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "x", Resolution: "hd"}, "wan3.0-video")
	require.NoError(t, err)
	assert.Equal(t, "hd", body.Parameters.Resolution)
}

// TestWanModelFamilyBoundary 锁定家族边界：仅 wan3. 前缀进入本方言；wan 老代
// （wanx2.x）与其它 wan* 命名不受影响，导入前必须另行探测。
func TestWanModelFamilyBoundary(t *testing.T) {
	assert.True(t, isWanModel("wan3.0-video"))
	assert.True(t, isWanModel("wan3.0-video-prime"))
	assert.False(t, isWanModel("wanx2.1-t2v"))
	assert.False(t, isWanModel("Wan-AI/Wan2.6-T2V"))
	assert.False(t, isWanModel("kling-v3"))
	assert.False(t, isWanModel("doubao-seedance-2-5-260628"))
}
