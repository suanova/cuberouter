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
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const StudioMaxBody = 15 * 1024 * 1024

// The private studio worker owns media, not user wallets. GPU requests still
// pass through the normal image relay; only that relay can release a result.
func StudioBridgeOrigin() string {
	return strings.TrimRight(os.Getenv("MEDIA_STUDIO_BRIDGE_URL"), "/")
}

func StudioBridgeRequest(ctx context.Context, method, path string, body []byte, contentType string) (*http.Response, error) {
	origin := StudioBridgeOrigin()
	u, err := url.Parse(origin)
	key := os.Getenv("MEDIA_STUDIO_BRIDGE_KEY")
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || len(key) < 24 {
		return nil, errors.New("media studio worker is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, method, origin+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Type", contentType)
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return client.Do(request)
}

func StudioBridgeJSON(ctx context.Context, path string, body any) error {
	raw, err := common.Marshal(body)
	if err != nil {
		return err
	}
	response, err := StudioBridgeRequest(ctx, http.MethodPost, path, raw, "application/json")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 65536))
	if response.StatusCode != http.StatusOK {
		return errors.New("studio request could not be verified")
	}
	return nil
}
