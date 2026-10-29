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
package imageedit

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pngDataURLPrefix = "data:image/png;base64,"

func multipartRelayInfo(t *testing.T) *relaycommon.RelayInfo {
	t.Helper()
	info := newRelayInfo(relayconstant.RelayModeImagesEdits)
	info.ChannelSetting = dto.ChannelSettings{ImageEditUpstreamFormat: dto.ImageEditUpstreamMultipart}
	return info
}

type parsedFile struct {
	filename    string
	contentType string
	data        []byte
}

type parsedForm struct {
	values map[string][]string
	files  map[string][]parsedFile
}

func (f parsedForm) value(key string) string {
	values := f.values[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (f parsedForm) has(key string) bool {
	_, ok := f.values[key]
	return ok
}

// convertMultipartUpstream runs the conversion and parses what would go on the
// wire. The Content-Type header is asserted here because
// channel.SetupApiRequestHeader copies the client's Content-Type onto the
// outbound request: a body without its boundary is unusable upstream.
func convertMultipartUpstream(t *testing.T, body string, info *relaycommon.RelayInfo) parsedForm {
	t.Helper()
	var request dto.ImageRequest
	require.NoError(t, common.UnmarshalJsonStr(body, &request))

	c := newJSONContext()
	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
	require.NoError(t, err)

	buffer, ok := converted.(*bytes.Buffer)
	require.Truef(t, ok, "a multipart upstream must get a *bytes.Buffer, got %T", converted)

	contentType := c.Request.Header.Get("Content-Type")
	require.Truef(t, strings.HasPrefix(contentType, "multipart/form-data; boundary="),
		"outbound Content-Type must carry the boundary, got %q", contentType)
	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	require.NotEmpty(t, params["boundary"])

	reader := multipart.NewReader(bytes.NewReader(buffer.Bytes()), params["boundary"])
	form := parsedForm{values: map[string][]string{}, files: map[string][]parsedFile{}}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(part)
		require.NoError(t, err)
		name := part.FormName()
		if part.FileName() == "" {
			form.values[name] = append(form.values[name], string(data))
			continue
		}
		form.files[name] = append(form.files[name], parsedFile{
			filename:    part.FileName(),
			contentType: part.Header.Get("Content-Type"),
			data:        data,
		})
	}
	return form
}

func TestConvertImageRequestMultipartUpstreamEncodesFieldsAndFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"model":"qwen-image21","prompt":"钓鱼不应该吃掉","n":1,"size":"1024x1024",` +
		`"quality":"standard","num_inference_steps":30,"seed":42,"true_cfg_scale":4,` +
		`"image":"` + pngDataURLPrefix + base64Reference + `",` +
		`"mask":"` + pngDataURLPrefix + base64Reference + `"}`

	form := convertMultipartUpstream(t, body, multipartRelayInfo(t))

	assert.Equal(t, "qwen-image21", form.value("model"))
	assert.Equal(t, "钓鱼不应该吃掉", form.value("prompt"))
	assert.Equal(t, "1", form.value("n"))
	assert.Equal(t, "1024x1024", form.value("size"))
	assert.Equal(t, "standard", form.value("quality"))
	assert.Equal(t, "30", form.value("num_inference_steps"))
	assert.Equal(t, "42", form.value("seed"))
	assert.Equal(t, "4", form.value("true_cfg_scale"))

	// image and mask must travel as file parts, never as text fields.
	assert.False(t, form.has("image"), "image must not be sent as a text field")
	assert.False(t, form.has("mask"), "mask must not be sent as a text field")

	want, err := base64.StdEncoding.DecodeString(base64Reference)
	require.NoError(t, err)

	require.Len(t, form.files["image"], 1)
	assert.Equal(t, want, form.files["image"][0].data)
	assert.Equal(t, "image/png", form.files["image"][0].contentType)
	assert.Equal(t, "image-1.png", form.files["image"][0].filename)

	require.Len(t, form.files["mask"], 1)
	assert.Equal(t, want, form.files["mask"][0].data)
	assert.Equal(t, "image/png", form.files["mask"][0].contentType)
	assert.Equal(t, "mask.png", form.files["mask"][0].filename)
}

func TestConvertImageRequestMultipartUpstreamSendsEveryReferenceImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"model":"qwen-image21","prompt":"merge","image":["` + base64Reference + `","` + base64Reference + `"]}`
	form := convertMultipartUpstream(t, body, multipartRelayInfo(t))

	require.Len(t, form.files["image"], 2)
	assert.Equal(t, "image-1.png", form.files["image"][0].filename)
	assert.Equal(t, "image-2.png", form.files["image"][1].filename)
	assert.False(t, form.has("mask"), "an absent mask must not be sent at all")
}

func TestConvertImageRequestMultipartUpstreamAcceptsBareBase64AndSniffsType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"model":"qwen-image21","prompt":"invert","image":"` + base64Reference + `"}`
	form := convertMultipartUpstream(t, body, multipartRelayInfo(t))

	require.Len(t, form.files["image"], 1)
	// A 1x1 PNG data URL sniffed from bare base64 must not advertise
	// application/octet-stream.
	assert.Equal(t, "image/png", form.files["image"][0].contentType)
	assert.Equal(t, "image-1.png", form.files["image"][0].filename)
}

func TestConvertImageRequestMultipartUpstreamAppliesChannelParamOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// The JSON path applies param_override in relay.ImageHelper, which skips
	// *bytes.Buffer bodies. A channel that injects output_format must still see
	// the field upstream, otherwise sglang re-encodes RGBA output as JPEG and
	// fails on transparency.
	info := multipartRelayInfo(t)
	info.ChannelMeta.ParamOverride = map[string]interface{}{
		"operations": []interface{}{
			map[string]interface{}{"path": "output_format", "mode": "set", "value": "png"},
		},
	}

	body := `{"model":"qwen-image21","prompt":"invert","image":"` + base64Reference + `"}`
	form := convertMultipartUpstream(t, body, info)

	assert.Equal(t, "png", form.value("output_format"))
	assert.Equal(t, "invert", form.value("prompt"))
	assert.Equal(t, "qwen-image21", form.value("model"))
	require.Len(t, form.files["image"], 1)
}

func TestConvertImageRequestDefaultsToJSONWhenFormatUnset(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, setting := range []dto.ChannelSettings{{}, {ImageEditUpstreamFormat: dto.ImageEditUpstreamJSON}} {
		info := newRelayInfo(relayconstant.RelayModeImagesEdits)
		info.ChannelSetting = setting

		var request dto.ImageRequest
		require.NoError(t, common.UnmarshalJsonStr(`{"model":"qwen-image-edit-2511","prompt":"invert","image":"`+base64Reference+`"}`, &request))
		converted, err := (&Adaptor{}).ConvertImageRequest(newJSONContext(), info, request)
		require.NoError(t, err)
		_, isBuffer := converted.(*bytes.Buffer)
		assert.False(t, isBuffer, "JSON is still the default upstream format")
	}
}

func TestConvertImageRequestMultipartUpstreamRejectsInvalidBase64(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var request dto.ImageRequest
	require.NoError(t, common.UnmarshalJsonStr(`{"model":"qwen-image21","prompt":"invert","image":"not base64 !!"}`, &request))
	_, err := (&Adaptor{}).ConvertImageRequest(newJSONContext(), multipartRelayInfo(t), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base64")
}

func TestConvertImageRequestMultipartUpstreamRejectsNonBase64DataURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var request dto.ImageRequest
	require.NoError(t, common.UnmarshalJsonStr(`{"model":"qwen-image21","prompt":"invert","image":"data:image/png,rawbytes"}`, &request))
	_, err := (&Adaptor{}).ConvertImageRequest(newJSONContext(), multipartRelayInfo(t), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base64")
}
