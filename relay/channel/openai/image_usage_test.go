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
package openai

import (
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestOpenaiImageHandlerMapsOutputTokenDetails 锁定 OpenAI Images 响应中
// output_tokens_details 的解析与映射契约：上游把生成图像的 token 数放在
// output_tokens_details.image_tokens（星图 gpt-image-2.5 实测如此），
// 计费的 img_o 变量取自 CompletionTokenDetails.ImageTokens
// （service/tiered_settle.go BuildTieredTokenParams）。若这一层不映射，
// 图像输出 token 会残留为文本输出（c）：表达式计费里 c 的系数通常是 0，
// 生成图的输出侧等于不计费。
func TestOpenaiImageHandlerMapsOutputTokenDetails(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"created":1700000000,"data":[{"b64_json":"aW1n"}],"usage":{"input_tokens":14,"input_tokens_details":{"image_tokens":0,"text_tokens":14},"output_tokens":196,"output_tokens_details":{"image_tokens":196,"text_tokens":0},"total_tokens":210}}`

	c, _, resp, info := newImageTestContext(t, body, "application/json", false)
	info.RelayMode = relayconstant.RelayModeImagesGenerations

	usage, err := OpenaiImageHandler(c, info, resp)
	require.Nil(t, err)
	require.Equal(t, 14, usage.PromptTokens)
	require.Equal(t, 196, usage.CompletionTokens)
	require.Equal(t, 196, usage.CompletionTokenDetails.ImageTokens,
		"image output tokens must land in CompletionTokenDetails.ImageTokens")
	require.Equal(t, 0, usage.CompletionTokenDetails.TextTokens)

	// 整链到计费参数：表达式引用 img_o 时，图像输出从 c 中扣除并落到 ImgO。
	params := service.BuildTieredTokenParams(usage, false, map[string]bool{"img_o": true})
	require.Equal(t, 196.0, params.ImgO)
	require.Equal(t, 0.0, params.C, "image output tokens must be excluded from the text-output remainder")
	require.Equal(t, 14.0, params.P)
}
