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
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFormFile 往 multipart writer 里写一个文件字段。
func writeFormFile(t *testing.T, writer *multipart.Writer, field, filename, content string) {
	t.Helper()

	part, err := writer.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
}

// readFormFile 读出一个 multipart 文件字段的内容。
func readFormFile(t *testing.T, header *multipart.FileHeader) string {
	t.Helper()

	file, err := header.Open()
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	return string(content)
}

// readConvertedMultipart 用重建后的 Content-Type 解析 ConvertImageRequest 的返回体:
// 解析成功即证明返回体与 c.Request.Header 上的新 boundary 一致，而不是沿用客户端的
// 旧 Content-Type。
func readConvertedMultipart(t *testing.T, context *gin.Context, converted any) *multipart.Form {
	t.Helper()

	convertedBody, ok := converted.(*bytes.Buffer)
	require.True(t, ok, "a multipart edit must be rebuilt into a raw body, got %T", converted)

	mediaType, params, err := mime.ParseMediaType(context.Request.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType, "the Content-Type must be switched to the rebuilt body")
	require.NotEmpty(t, params["boundary"])

	form, err := multipart.NewReader(convertedBody, params["boundary"]).ReadForm(32 << 20)
	require.NoError(t, err)
	return form
}

// TestConvertImageEditRequestRebuildsMultipart 锁定 multipart /v1/images/edits 的重建
// 契约：客户端表单必须被重建为 raw body——模型与非文件字段原样携带，图片与蒙版文件
// 保留原名与 MIME，Content-Type 换成重建后的 boundary。星图上游只认 multipart 表单；
// 若把 DTO 序列化成 JSON 发出，文件部分会全部丢失，上游也无法按表单解析请求体。
func TestConvertImageEditRequestRebuildsMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "make it blue"))
	require.NoError(t, writer.WriteField("size", "1024x1024"))
	require.NoError(t, writer.WriteField("n", "1"))
	writeFormFile(t, writer, "image", "input.png", "fake image bytes")
	writeFormFile(t, writer, "mask", "mask.png", "fake mask bytes")
	require.NoError(t, writer.Close())

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())
	// host 在转换前已按表单解析(relay/helper/valid_request.go)，这里复现同一顺序。
	require.NoError(t, context.Request.ParseMultipartForm(32<<20))

	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}
	request := dto.ImageRequest{Model: "gpt-image-2", Prompt: "make it blue"}

	converted, err := (&Adaptor{}).ConvertImageRequest(context, info, request)
	require.NoError(t, err)
	form := readConvertedMultipart(t, context, converted)

	require.Equal(t, []string{"gpt-image-2"}, form.Value["model"], "the model field must be carried into the rebuilt form")
	require.Equal(t, []string{"make it blue"}, form.Value["prompt"])
	require.Equal(t, []string{"1024x1024"}, form.Value["size"])
	require.Equal(t, []string{"1"}, form.Value["n"])

	require.Len(t, form.File["image"], 1)
	require.Equal(t, "input.png", form.File["image"][0].Filename)
	require.Equal(t, "image/png", form.File["image"][0].Header.Get("Content-Type"))
	require.Equal(t, "fake image bytes", readFormFile(t, form.File["image"][0]))

	require.Len(t, form.File["mask"], 1)
	require.Equal(t, "mask.png", form.File["mask"][0].Filename)
	require.Equal(t, "image/png", form.File["mask"][0].Header.Get("Content-Type"))
	require.Equal(t, "fake mask bytes", readFormFile(t, form.File["mask"][0]))
}

// TestConvertImageEditRequestMultipleImagesUseArrayField 锁定多图编辑的字段契约：
// 多张图片必须按 image[] 重复字段发出(单张用 image)，与上游 OpenAI 兼容约定一致。
func TestConvertImageEditRequestMultipleImagesUseArrayField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	writeFormFile(t, writer, "image", "first.png", "first image")
	writeFormFile(t, writer, "image", "second.png", "second image")
	require.NoError(t, writer.Close())

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, context.Request.ParseMultipartForm(32<<20))

	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}

	converted, err := (&Adaptor{}).ConvertImageRequest(context, info, dto.ImageRequest{Model: "gpt-image-2"})
	require.NoError(t, err)
	form := readConvertedMultipart(t, context, converted)

	require.Empty(t, form.File["image"], "multiple images must not stay under the singular field")
	require.Len(t, form.File["image[]"], 2)
	require.Equal(t, "first.png", form.File["image[]"][0].Filename)
	require.Equal(t, "second.png", form.File["image[]"][1].Filename)
	require.Equal(t, "first image", readFormFile(t, form.File["image[]"][0]))
	require.Equal(t, "second image", readFormFile(t, form.File["image[]"][1]))
}

// TestConvertImageEditRequestKeepsJSONPassthrough 锁定 JSON 图片编辑的直传契约：
// 以 JSON 内联参考图的编辑请求保持 DTO 形状(由 host 序列化并叠加参数覆盖)，
// Content-Type 也不得被改写。
func TestConvertImageEditRequestKeepsJSONPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewBufferString(`{"model":"gpt-image-2","prompt":"make it blue"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}
	request := dto.ImageRequest{Model: "gpt-image-2", Prompt: "make it blue"}

	converted, err := (&Adaptor{}).ConvertImageRequest(context, info, request)
	require.NoError(t, err)
	got, ok := converted.(dto.ImageRequest)
	require.True(t, ok, "a JSON edit must stay a DTO, got %T", converted)
	assert.Equal(t, request, got)
	assert.Equal(t, "application/json", context.Request.Header.Get("Content-Type"))
}

// TestRelayImageEditMultipartSendsFormRequest 锁定 multipart 图片编辑的发送契约：
// 重建后的表单体连同换好的 Content-Type 必须原样到达上游 /v1/images/edits，
// 方法、路径与 Bearer 鉴权与其它模式一致。
func TestRelayImageEditMultipartSendsFormRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotMethod, gotPath, gotAuth, gotContentType string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1700000000,"data":[{"url":"https://cdn.example.com/blue.png"}],"usage":{"input_tokens":30,"output_tokens":229,"total_tokens":259}}`))
	}))
	defer server.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "make it blue"))
	writeFormFile(t, writer, "image", "input.png", "fake image bytes")
	require.NoError(t, writer.Close())

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, context.Request.ParseMultipartForm(32<<20))

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL,
			ChannelType:       constant.ChannelTypeAstraFlow,
			ApiKey:            "sk-test",
			UpstreamModelName: "gpt-image-2",
		},
		RequestURLPath:  "/v1/images/edits",
		RelayFormat:     types.RelayFormatOpenAIImage,
		RelayMode:       relayconstant.RelayModeImagesEdits,
		OriginModelName: "gpt-image-2",
	}

	converted, err := adaptor.ConvertImageRequest(context, info, dto.ImageRequest{Model: "gpt-image-2", Prompt: "make it blue"})
	require.NoError(t, err)
	requestBody, ok := converted.(*bytes.Buffer)
	require.True(t, ok, "a multipart edit must be rebuilt into a raw body, got %T", converted)

	respAny, err := adaptor.DoRequest(context, info, requestBody)
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/v1/images/edits", gotPath)
	assert.Equal(t, "Bearer sk-test", gotAuth)
	assert.Equal(t, context.Request.Header.Get("Content-Type"), gotContentType, "the rebuilt boundary must be the one sent upstream")

	mediaType, params, err := mime.ParseMediaType(gotContentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	form, err := multipart.NewReader(bytes.NewReader(gotBody), params["boundary"]).ReadForm(32 << 20)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-image-2"}, form.Value["model"])
	require.Len(t, form.File["image"], 1)
	require.Equal(t, "input.png", form.File["image"][0].Filename)
	assert.Equal(t, "fake image bytes", readFormFile(t, form.File["image"][0]))
}
