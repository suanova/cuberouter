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

package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Readiness is explicitly distinct from paid inference. Never create an image,
// upload a fixture or authorize a job as a side effect of channel health checks.
func testStudioChannel(ctx context.Context, c *gin.Context, channel *model.Channel, owner int, modelName, endpoint string, stream bool) (result testResult) {
	result = testResult{context: c, testMode: "studio-readiness"}
	defer func() {
		if result.localErr != nil && result.newAPIError == nil {
			result.newAPIError = types.NewErrorWithStatusCode(result.localErr, types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
		}
	}()
	if endpoint = strings.TrimSpace(endpoint); stream || (endpoint != "" && endpoint != string(constant.EndpointTypeImageGeneration)) {
		result.localErr = errors.New("Studio supports a non-streaming readiness check. Select Auto detect with streaming off; use Image Studio to test generation or editing")
		return
	}
	if owner <= 0 || !slices.Contains(channel.GetModels(), modelName) {
		result.localErr = errors.New("Select a model configured on this studio channel")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, service.StudioBridgeOrigin()+"/studio/"+strconv.Itoa(owner)+"/config", nil)
	if err != nil {
		result.localErr = errors.New("Studio adapter URL is invalid")
		return
	}
	c.Request = request
	if apiErr := middleware.SetupContextForSelectedChannel(c, channel, modelName); apiErr != nil {
		result.localErr, result.newAPIError = apiErr, apiErr
		return
	}
	// Verify the selected channel's credentials, not the separate workflow key.
	request.Header.Set("Authorization", "Bearer "+common.GetContextKeyString(c, constant.ContextKeyChannelKey))
	settings := channel.GetSetting()
	client, err := service.GetHttpClientWithProxySettings(settings.Proxy, settings)
	if err != nil {
		result.localErr = errors.New("Studio channel proxy configuration is invalid")
		return
	}
	probe := *client
	probe.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := probe.Do(request)
	if err != nil {
		result.localErr = errors.New("Studio adapter connection failed; check the adapter and private worker connection")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		result.localErr = fmt.Errorf("Studio readiness request returned HTTP %d; check the channel key and adapter", response.StatusCode)
		return
	}
	var config struct {
		Models map[string]string `json:"models"`
		Health map[string]string `json:"health"`
	}
	if err := common.DecodeJson(io.LimitReader(response.Body, 64*1024), &config); err != nil {
		result.localErr = errors.New("Studio adapter returned invalid readiness data")
		return
	}
	mode := ""
	for _, candidate := range []string{"create", "edit"} {
		if config.Models[candidate] == modelName {
			mode = candidate
			break
		}
	}
	if mode == "" {
		result.localErr = errors.New("This model is not available on the configured studio adapter")
		return
	}
	if config.Health[mode] != "ready" || config.Health["tools"] != "ready" {
		result.localErr = errors.New("Studio model or workflow tools are not ready; check the private worker connection and model service")
	}
	return
}
