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
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

const StudioReviewContext = "studio_image_review"
const StudioExecutionContext = "studio_image_execution"

type StudioReview struct {
	Protocol        string              `json:"protocol"`
	Model           string              `json:"model"`
	Revision        string              `json:"revision"`
	ImageSHA256     string              `json:"image_sha256"`
	Decision        string              `json:"decision"`
	ReleaseAllowed  *bool               `json:"release_allowed"`
	WeightsVerified bool                `json:"weights_verified"`
	Threshold       *float64            `json:"threshold"`
	Scores          map[string]*float64 `json:"scores"`
	ChannelID       int                 `json:"channel_id"`
}

var studioRevision = regexp.MustCompile(`^[a-f0-9]{40}$`)

func TestStudioImageReviewer(ctx context.Context, channel *model.Channel) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		return err
	}
	_, err, _ := requestStudioReview(ctx, *channel, encoded.Bytes())
	return err
}

func ValidateStudioReview(report StudioReview, image []byte) error {
	if report.Protocol != "image-moderation-v1" || report.Model != "google/shieldgemma-2-4b-it" ||
		!studioRevision.MatchString(report.Revision) || !report.WeightsVerified ||
		report.ImageSHA256 != fmt.Sprintf("%x", sha256.Sum256(image)) ||
		report.Threshold == nil || *report.Threshold != 0.5 || report.ReleaseAllowed == nil || len(report.Scores) != 3 {
		return errors.New("output_check_unavailable")
	}
	allow := true
	for _, category := range []string{"sexual", "violence", "dangerous"} {
		score := report.Scores[category]
		if score == nil || math.IsNaN(*score) || math.IsInf(*score, 0) || *score < 0 || *score > 1 {
			return errors.New("output_check_unavailable")
		}
		allow = allow && *score < 0.5
	}
	if *report.ReleaseAllowed != allow || (allow && report.Decision != "allow") || (!allow && report.Decision != "block") {
		return errors.New("output_check_unavailable")
	}
	if !allow {
		return errors.New("output_blocked")
	}
	return nil
}

// Admin-controlled service destinations accept TLS or local HTTP only. Review
// channels use Other for an optional PEM CA bundle; verification stays enabled.
func StudioPrivateClient(endpoint, ca string, timeout time.Duration) (*http.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid_studio_service_url")
	}
	local := u.Hostname() == "localhost"
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		local = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return nil, errors.New("studio_service_requires_tls")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if ca != "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM([]byte(ca)) {
			return nil, errors.New("invalid_studio_ca")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

func ReviewStudioPNG(ctx context.Context, job string, image []byte) (StudioReview, error) {
	var channels []model.Channel
	if err := model.DB.Where("type = ? AND status = ?", constant.ChannelTypeStudioReview, common.ChannelStatusEnabled).
		Order("priority DESC").Order("id ASC").Find(&channels).Error; err != nil {
		return StudioReview{}, errors.New("output_check_unavailable")
	}
	// Removing every reviewer still fails closed: enabled Studio has no mode
	// that can release an unchecked image, so configuration cannot clear a latch.
	var leases []model.StudioImageLease
	resources := make([]string, 0, len(channels))
	for _, channel := range channels {
		resources = append(resources, fmt.Sprintf("review:%d", channel.Id))
	}
	if model.DB.Where("resource IN ?", resources).Find(&leases).Error != nil {
		return StudioReview{}, errors.New("output_check_unavailable")
	}
	lastUsed := make(map[string]int64)
	for _, lease := range leases {
		lastUsed[lease.Resource] = lease.LastUsed
	}
	sort.SliceStable(channels, func(i, j int) bool {
		if channels[i].GetPriority() != channels[j].GetPriority() {
			return channels[i].GetPriority() > channels[j].GetPriority()
		}
		return lastUsed[fmt.Sprintf("review:%d", channels[i].Id)] < lastUsed[fmt.Sprintf("review:%d", channels[j].Id)]
	})
	for _, ch := range channels {
		resource := fmt.Sprintf("review:%d", ch.Id)
		acquired, err := model.AcquireStudioImageLease(resource, job, time.Now(), time.Minute)
		if err != nil {
			return StudioReview{}, errors.New("output_check_unavailable")
		}
		if !acquired {
			continue
		}
		report, reviewErr, transportFailed := requestStudioReview(ctx, ch, image)
		if !transportFailed {
			_ = model.ReleaseStudioImageLease(resource, job)
		}
		if reviewErr == nil || reviewErr.Error() == "output_blocked" {
			return report, reviewErr
		}
		// A transport outage can try another replica. A malformed verdict is
		// terminal: a different reviewer cannot override a protocol disagreement.
		if !transportFailed {
			return report, reviewErr
		}
	}
	return StudioReview{}, errors.New("output_check_unavailable")
}

func requestStudioReview(ctx context.Context, channel model.Channel, image []byte) (StudioReview, error, bool) {
	failure := errors.New("output_check_unavailable")
	client, err := StudioPrivateClient(channel.GetBaseURL(), channel.Other, 45*time.Second)
	if err != nil || len(channel.Key) < 32 {
		return StudioReview{}, failure, false
	}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, channel.GetBaseURL(), bytes.NewReader(image))
	if err != nil {
		return StudioReview{}, failure, false
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	req.Header.Set("Content-Type", "image/png")
	response, err := client.Do(req)
	if err != nil {
		return StudioReview{}, failure, true
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return StudioReview{}, failure, true
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	data, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	var report StudioReview
	if mediaType != "application/json" || err != nil || len(data) > 16384 || common.Unmarshal(data, &report) != nil {
		return StudioReview{}, failure, false
	}
	report.ChannelID = channel.Id
	return report, ValidateStudioReview(report, image), false
}

// GateStudioImageResponse runs before the adaptor writes any bytes or settles
// billing. The queue binds its audit callback here; normal image API calls keep
// their existing provider behavior.
func GateStudioImageResponse(c *gin.Context, response *http.Response) error {
	value, enabled := c.Get(StudioReviewContext)
	if !enabled {
		return nil
	}
	review, ok := value.(func(context.Context, []byte) error)
	if !ok || response == nil {
		return errors.New("output_check_unavailable")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (32<<20)+1))
	var payload struct {
		Data []struct {
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err != nil || len(body) > 32<<20 || common.Unmarshal(body, &payload) != nil || len(payload.Data) != 1 {
		return errors.New("output_check_unavailable")
	}
	image, err := base64.StdEncoding.Strict().DecodeString(payload.Data[0].Base64)
	if err != nil || len(image) > 12<<20 {
		return errors.New("output_check_unavailable")
	}
	config, err := png.DecodeConfig(bytes.NewReader(image))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 4194304 {
		return errors.New("output_check_unavailable")
	}
	if transformValue, exists := c.Get("studio_image_transform"); exists {
		transform, ok := transformValue.(func([]byte) ([]byte, error))
		if !ok {
			return errors.New("output_check_unavailable")
		}
		image, err = transform(image)
		if err != nil {
			return err
		}
		body, err = sjson.SetBytes(body, "data.0.b64_json", base64.StdEncoding.EncodeToString(image))
		if err != nil {
			return errors.New("output_check_unavailable")
		}
	}
	if _, err := png.Decode(bytes.NewReader(image)); err != nil {
		return errors.New("output_check_unavailable")
	}
	if err := review(c.Request.Context(), image); err != nil {
		return err
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}
