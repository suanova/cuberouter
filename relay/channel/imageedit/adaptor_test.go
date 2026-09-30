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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A reference image is base64 for the wire form the provider accepts, so the
// happy path must hand those bytes to the upstream untouched rather than
// re-encoding a multi-megabyte payload.
const base64Reference = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func newJSONContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/images/edits", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func newRelayInfo(relayMode int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeImageEdit,
			ChannelBaseUrl: "http://provider.test",
		},
		RelayMode: relayMode,
	}
}

func convertJSON(t *testing.T, body string) (any, error) {
	t.Helper()
	var request dto.ImageRequest
	require.NoError(t, common.UnmarshalJsonStr(body, &request))
	adaptor := &Adaptor{}
	return adaptor.ConvertImageRequest(newJSONContext(), newRelayInfo(relayconstant.RelayModeImagesEdits), request)
}

func TestConvertImageRequestForwardsBase64Reference(t *testing.T) {
	gin.SetMode(gin.TestMode)

	converted, err := convertJSON(t, `{"model":"qwen-image-edit-2511","prompt":"invert","n":2,"quality":"high","image":"`+base64Reference+`"}`)
	require.NoError(t, err)

	body, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"qwen-image-edit-2511","prompt":"invert","n":2,"quality":"high","image":"`+base64Reference+`"}`, string(body))
}

func TestConvertImageRequestForwardsBase64ReferenceArray(t *testing.T) {
	gin.SetMode(gin.TestMode)

	converted, err := convertJSON(t, `{"model":"qwen-image-edit-2511","prompt":"merge","image":["`+base64Reference+`","`+base64Reference+`"]}`)
	require.NoError(t, err)

	body, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"qwen-image-edit-2511","prompt":"merge","image":["`+base64Reference+`","`+base64Reference+`"]}`, string(body))
}

// The provider base64-decodes `image` unconditionally, so a URL there surfaces
// as an unrelated "Incorrect padding" from its decoder. Reject it here instead.
func TestConvertImageRequestRejectsRemoteImageURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, err := convertJSON(t, `{"model":"qwen-image-edit-2511","prompt":"invert","image":"https://objects.test/ref.png"}`)

	require.Error(t, err)
	var apiError *types.NewAPIError
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
	assert.Equal(t, types.ErrorCodeInvalidRequest, apiError.GetErrorCode())
}

// images[].image_url is the shape Media Studio used to send; the provider reads
// the entries as strings and dies on the object with an attribute error.
func TestConvertImageRequestRejectsImagesArray(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, err := convertJSON(t, `{"model":"qwen-image-edit-2511","prompt":"invert","images":[{"image_url":"https://objects.test/ref.png"}]}`)

	require.Error(t, err)
	var apiError *types.NewAPIError
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
}

func TestConvertImageRequestRejectsMissingImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, err := convertJSON(t, `{"model":"qwen-image-edit-2511","prompt":"invert"}`)

	require.Error(t, err)
	var apiError *types.NewAPIError
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
}

func TestConvertImageRequestRejectsMultipartEdit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	request := dto.ImageRequest{Model: "qwen-image-edit-2511", Prompt: "invert"}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/images/edits", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=x")

	_, err := (&Adaptor{}).ConvertImageRequest(c, newRelayInfo(relayconstant.RelayModeImagesEdits), request)

	require.Error(t, err)
	var apiError *types.NewAPIError
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
}

// The relay turns any conversion failure into ErrorCodeConvertRequestFailed, so
// the 400 raised above only reaches the caller if NewError preserves a nested
// NewAPIError. This guards that the rejection is not flattened into a 500.
func TestRejectionSurvivesRelayErrorWrapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, err := convertJSON(t, `{"model":"qwen-image-edit-2511","prompt":"invert","image":"https://objects.test/ref.png"}`)
	require.Error(t, err)

	wrapped := types.NewError(err, types.ErrorCodeConvertRequestFailed)
	assert.Equal(t, http.StatusBadRequest, wrapped.StatusCode)
	assert.Equal(t, types.ErrorCodeInvalidRequest, wrapped.GetErrorCode())
}

func TestGetRequestURLTargetsImageEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	edits, err := adaptor.GetRequestURL(newRelayInfo(relayconstant.RelayModeImagesEdits))
	require.NoError(t, err)
	assert.Equal(t, "http://provider.test/v1/images/edits", edits)

	generations, err := adaptor.GetRequestURL(newRelayInfo(relayconstant.RelayModeImagesGenerations))
	require.NoError(t, err)
	assert.Equal(t, "http://provider.test/v1/images/generations", generations)
}

// An explicit zero in an optional scalar must survive the marshal, otherwise a
// client asking for cfg_scale=0 silently gets the provider default.
func TestConvertImageRequestKeepsExplicitZeroScalars(t *testing.T) {
	gin.SetMode(gin.TestMode)

	converted, err := convertJSON(t, `{"model":"qwen-image-edit-2511","prompt":"invert","image":"`+base64Reference+`","seed":0,"true_cfg_scale":0,"num_inference_steps":0}`)
	require.NoError(t, err)

	body, err := common.Marshal(converted)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(body, &fields))
	assert.Equal(t, "0", string(fields["seed"]))
	assert.Equal(t, "0", string(fields["true_cfg_scale"]))
	assert.Equal(t, "0", string(fields["num_inference_steps"]))
}
