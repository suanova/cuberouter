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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// Adaptor serves image providers whose edit endpoint takes the reference image
// inline as base64. The stock OpenAI adaptor cannot express that contract: for
// JSON edits it forwards the request verbatim, so `images: [{"image_url": …}]`
// reaches an upstream that only ever base64-decodes `image` and fails with an
// unrelated error (a missing `startswith`, or `Incorrect padding`). This
// adaptor owns the contract instead of forwarding whatever the client sent.
type Adaptor struct {
}

// imageEditRequest is the upstream wire format. Optional scalars are pointers
// so an explicit 0/false survives the marshal while an absent field is omitted.
type imageEditRequest struct {
	Model             string          `json:"model"`
	Prompt            string          `json:"prompt"`
	Image             json.RawMessage `json:"image"`
	Size              string          `json:"size,omitempty"`
	N                 *uint           `json:"n,omitempty"`
	Quality           string          `json:"quality,omitempty"`
	NumInferenceSteps *int            `json:"num_inference_steps,omitempty"`
	Seed              *int64          `json:"seed,omitempty"`
	TrueCfgScale      *float64        `json:"true_cfg_scale,omitempty"`
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	path := "/v1/images/generations"
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		path = "/v1/images/edits"
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, path, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))
	return nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.RelayMode != relayconstant.RelayModeImagesEdits {
		return request, nil
	}
	if !isJSONRequest(c) {
		return nil, badRequest(errors.New("this channel accepts JSON image edits only: send the reference image as base64 in the \"image\" field, not as a multipart upload"))
	}
	if !isEmptyJSON(request.Images) {
		return nil, badRequest(errors.New("images[] is not supported on this channel: send the reference image as base64 in the \"image\" field"))
	}
	image, err := normalizeImage(request.Image)
	if err != nil {
		return nil, badRequest(err)
	}
	return imageEditRequest{
		Model:             request.Model,
		Prompt:            request.Prompt,
		Image:             image,
		Size:              request.Size,
		N:                 request.N,
		Quality:           request.Quality,
		NumInferenceSteps: request.NumInferenceSteps,
		Seed:              request.Seed,
		TrueCfgScale:      request.TrueCfgScale,
	}, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	// The provider answers with the OpenAI image envelope (data[].b64_json), so
	// the shared handler already covers the response shape, the error envelope
	// and the per-image `n` billing ratio.
	return openai.OpenaiImageHandler(c, info, resp)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errUnsupportedRelayFormat
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errUnsupportedRelayFormat
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errUnsupportedRelayFormat
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errUnsupportedRelayFormat
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errUnsupportedRelayFormat
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errUnsupportedRelayFormat
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errUnsupportedRelayFormat
}

var errUnsupportedRelayFormat = errors.New("this channel serves image generation and editing only")

func badRequest(err error) *types.NewAPIError {
	return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
}

func isEmptyJSON(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

// isJSONRequest mirrors the guard the OpenAI adaptor uses to tell a JSON edit
// from a multipart upload. This channel has no multipart path, so anything that
// is not a JSON body is rejected rather than forwarded.
func isJSONRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json")
}

// normalizeImage validates the reference image and returns its JSON bytes
// unchanged: a reference is routinely several MB of base64, and re-encoding it
// would only copy that buffer. A single image may be a JSON string, several may
// be a JSON array of strings.
func normalizeImage(raw json.RawMessage) (json.RawMessage, error) {
	if isEmptyJSON(raw) {
		return nil, errors.New("image is required: send the reference image as base64 in the \"image\" field")
	}
	var single string
	if err := common.Unmarshal(raw, &single); err == nil {
		if err := validateImageValue(single); err != nil {
			return nil, err
		}
		return raw, nil
	}
	var multiple []string
	if err := common.Unmarshal(raw, &multiple); err == nil {
		if len(multiple) == 0 {
			return nil, errors.New("image is required: send the reference image as base64 in the \"image\" field")
		}
		for _, image := range multiple {
			if err := validateImageValue(image); err != nil {
				return nil, err
			}
		}
		return raw, nil
	}
	return nil, errors.New("image must be a base64 string or an array of base64 strings")
}

// validateImageValue rejects the shapes this channel cannot serve before they
// reach the provider, where a URL degrades into a misleading "Incorrect
// padding" from its base64 decoder.
func validateImageValue(image string) error {
	if strings.TrimSpace(image) == "" {
		return errors.New("image is required: send the reference image as base64 in the \"image\" field")
	}
	lower := strings.ToLower(image)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return errors.New("this channel does not fetch remote images: send the reference image as base64 or as a data URL")
	}
	return nil
}
