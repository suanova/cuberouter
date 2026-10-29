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
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

// usesMultipartUpstream reports whether the channel asks to be driven with
// form-data. Only an explicit `"image_edit_upstream_format": "multipart"` in the
// channel settings switches the encoding: the default stays JSON so existing
// base64 channels keep their wire format.
func usesMultipartUpstream(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(info.ChannelSetting.ImageEditUpstreamFormat), dto.ImageEditUpstreamMultipart)
}

// buildMultipartImageEdit re-encodes the JSON edit request as multipart
// form-data. Upstreams such as sglang declare /v1/images/edits with Form/File
// fields only (their handler signature is `prompt: str = Form(...)`,
// `image: UploadFile = File(...)`, …), so a JSON body carries no form field at
// all and FastAPI rejects it with 422 "prompt: Field required" even though the
// prompt is right there in the body. Everything the JSON wire format can express
// as a scalar becomes a form field; image and mask become file parts.
func buildMultipartImageEdit(c *gin.Context, info *relaycommon.RelayInfo, request imageEditRequest) (*bytes.Buffer, error) {
	raw, err := common.Marshal(request)
	if err != nil {
		return nil, badRequest(fmt.Errorf("marshal image edit request: %w", err))
	}

	// On the JSON path relay.ImageHelper applies the channel's param_override
	// after conversion. Multipart conversions return a *bytes.Buffer, and that
	// branch skips the override (see relay/image_handler.go), so the same
	// configuration has to be applied here: otherwise channel-level fields the
	// operator injected on purpose (output_format, negative_prompt, …) vanish.
	// The call is self-guarding: with no override configured the body is
	// returned unchanged.
	raw, err = relaycommon.ApplyParamOverrideWithRelayInfo(raw, info)
	if err != nil {
		return nil, badRequest(fmt.Errorf("apply param override to image edit request: %w", err))
	}

	var fields map[string]json.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil {
		return nil, badRequest(fmt.Errorf("decode image edit request: %w", err))
	}

	images, err := imagePayloads(fields["image"])
	if err != nil {
		return nil, badRequest(err)
	}
	mask, hasMask, err := maskPayload(fields["mask"])
	if err != nil {
		return nil, badRequest(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Text fields first, in a stable order so a captured request is diffable.
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key == "image" || key == "mask" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, ok := scalarFormValue(fields[key])
		if !ok {
			continue
		}
		if err := writer.WriteField(key, value); err != nil {
			return nil, badRequest(fmt.Errorf("write form field %q: %w", key, err))
		}
	}

	for index, image := range images {
		if err := writeFilePart(writer, "image", fmt.Sprintf("image-%d", index+1), image); err != nil {
			return nil, badRequest(err)
		}
	}
	if hasMask {
		if err := writeFilePart(writer, "mask", "mask", mask); err != nil {
			return nil, badRequest(err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, badRequest(fmt.Errorf("close multipart body: %w", err))
	}
	// channel.SetupApiRequestHeader copies the client's Content-Type onto the
	// outbound request, so the boundary has to be published there.
	if c != nil && c.Request != nil {
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	}
	return &body, nil
}

// imagePayload is one decoded reference image or mask.
type imagePayload struct {
	data []byte
	mime string
}

// imagePayloads accepts the same shapes normalizeImage allows: a single base64
// string (data URL or bare) or an array of them.
func imagePayloads(raw json.RawMessage) ([]imagePayload, error) {
	if isEmptyJSON(raw) {
		return nil, errors.New("image is required: send the reference image as base64 in the \"image\" field")
	}
	var single string
	if err := common.Unmarshal(raw, &single); err == nil {
		payload, err := decodeImagePayload(single)
		if err != nil {
			return nil, err
		}
		return []imagePayload{payload}, nil
	}
	var multiple []string
	if err := common.Unmarshal(raw, &multiple); err == nil {
		if len(multiple) == 0 {
			return nil, errors.New("image is required: send the reference image as base64 in the \"image\" field")
		}
		payloads := make([]imagePayload, 0, len(multiple))
		for index, value := range multiple {
			payload, err := decodeImagePayload(value)
			if err != nil {
				return nil, fmt.Errorf("image[%d]: %w", index, err)
			}
			payloads = append(payloads, payload)
		}
		return payloads, nil
	}
	return nil, errors.New("image must be a base64 string or an array of base64 strings")
}

// maskPayload decodes the optional mask. An absent mask stays absent so a
// whole-image edit does not turn into a masked one.
func maskPayload(raw json.RawMessage) (imagePayload, bool, error) {
	if isEmptyJSON(raw) {
		return imagePayload{}, false, nil
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return imagePayload{}, false, errors.New("mask must be a base64 PNG string or a PNG data URL")
	}
	payload, err := decodeImagePayload(value)
	if err != nil {
		return imagePayload{}, false, fmt.Errorf("mask: %w", err)
	}
	return payload, true, nil
}

// decodeImagePayload turns a data URL or bare base64 string into bytes plus a
// MIME type. The MIME type comes from the data URL when present, otherwise it is
// sniffed, so the file part does not advertise the wrong type.
func decodeImagePayload(value string) (imagePayload, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return imagePayload{}, errors.New("image is required: send the reference image as base64 in the \"image\" field")
	}

	mimeType := ""
	payload := trimmed
	if strings.HasPrefix(strings.ToLower(trimmed), "data:") {
		comma := strings.Index(trimmed, ",")
		if comma < 0 {
			return imagePayload{}, errors.New("image data URL is missing its base64 payload")
		}
		header := trimmed[len("data:"):comma]
		if !strings.HasSuffix(strings.ToLower(header), ";base64") {
			return imagePayload{}, errors.New("image data URL must be base64 encoded")
		}
		mimeType = strings.TrimSpace(header[:len(header)-len(";base64")])
		payload = trimmed[comma+1:]
	}

	data, err := decodeBase64(payload)
	if err != nil {
		return imagePayload{}, err
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	return imagePayload{data: data, mime: mimeType}, nil
}

// decodeBase64 tolerates the padding and alphabet variants providers see in
// practice: standard, unpadded, and the URL-safe alphabet.
func decodeBase64(value string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		}
		return r
	}, value)
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(cleaned); err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("image is not valid base64")
}

func writeFilePart(writer *multipart.Writer, field string, baseName string, payload imagePayload) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, baseName+imageExtension(payload.mime)))
	header.Set("Content-Type", payload.mime)
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create form file %q: %w", field, err)
	}
	if _, err := part.Write(payload.data); err != nil {
		return fmt.Errorf("write form file %q: %w", field, err)
	}
	return nil
}

// imageExtension maps the MIME type to a filename extension. Upstreams decode
// the bytes, but a filename that contradicts the payload makes captured
// requests confusing to read back.
func imageExtension(mimeType string) string {
	base := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	switch base {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	}
	if extensions, err := mime.ExtensionsByType(base); err == nil && len(extensions) > 0 {
		return extensions[0]
	}
	return ".png"
}

// scalarFormValue flattens one JSON value into a form field. Nested objects and
// arrays have no form-data representation without inventing a convention, so
// they are skipped rather than silently stringified.
func scalarFormValue(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return "", false
	}
	switch trimmed[0] {
	case '"':
		var value string
		if err := common.Unmarshal(trimmed, &value); err != nil {
			return "", false
		}
		return value, true
	case '{', '[':
		return "", false
	default:
		return string(trimmed), true
	}
}
