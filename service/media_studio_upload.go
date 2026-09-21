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
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const StudioUploadMaxBytes int64 = 10 * 1024 * 1024

type StudioUploadConfig struct {
	Endpoint, Bucket, Region, AccessKey, SecretKey string
}

func LoadStudioUploadConfig() (StudioUploadConfig, error) {
	cfg := StudioUploadConfig{
		Endpoint: os.Getenv("MEDIA_STUDIO_S3_ENDPOINT"), Bucket: os.Getenv("MEDIA_STUDIO_S3_BUCKET"),
		Region: os.Getenv("MEDIA_STUDIO_S3_REGION"), AccessKey: os.Getenv("MEDIA_STUDIO_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("MEDIA_STUDIO_S3_SECRET_KEY"),
	}
	return cfg, cfg.Validate()
}

func (cfg StudioUploadConfig) Validate() error {
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return errors.New("configure an S3 origin without a path, query or credentials")
	}
	ip := net.ParseIP(endpoint.Hostname())
	if endpoint.Scheme != "https" && !(endpoint.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return errors.New("S3 origin requires HTTPS (HTTP is allowed only for literal loopback addresses)")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`).MatchString(cfg.Bucket) || strings.Contains(cfg.Bucket, "..") || net.ParseIP(cfg.Bucket) != nil {
		return errors.New("configure a valid private S3 bucket")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`).MatchString(cfg.Region) || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return errors.New("configure the S3 region and server credentials")
	}
	return nil
}

type StudioUploadTicket struct {
	UploadURL string            `json:"upload_url"`
	ImageURL  string            `json:"image_url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt int64             `json:"expires_at"`
}

// PresignStudioUpload grants access to one random, owner-prefixed object only.
// No user-supplied key, path, bucket, or download target is accepted.
func PresignStudioUpload(ctx context.Context, cfg StudioUploadConfig, owner int, contentType string, size int64, now time.Time) (StudioUploadTicket, error) {
	var ticket StudioUploadTicket
	if err := cfg.Validate(); err != nil {
		return ticket, err
	}
	extension, ok := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[contentType]
	if owner <= 0 || !ok || size <= 0 || size > StudioUploadMaxBytes {
		return ticket, errors.New("upload must be PNG, JPEG or WebP between 1 byte and 10 MB")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return ticket, err
	}
	objectURL := strings.TrimRight(cfg.Endpoint, "/") + "/" + cfg.Bucket + fmt.Sprintf("/media-studio/uploads/%d/", owner) + hex.EncodeToString(random) + extension
	credentials := aws.Credentials{AccessKeyID: cfg.AccessKey, SecretAccessKey: cfg.SecretKey}
	signer := v4.NewSigner(func(options *v4.SignerOptions) { options.DisableURIPathEscaping = true })
	put, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, nil)
	if err != nil {
		return ticket, err
	}
	put.ContentLength = size
	put.Header.Set("Content-Type", contentType)
	query := put.URL.Query()
	query.Set("X-Amz-Expires", "300")
	put.URL.RawQuery = query.Encode()
	ticket.UploadURL, _, err = signer.PresignHTTP(ctx, credentials, put, "UNSIGNED-PAYLOAD", "s3", cfg.Region, now)
	if err != nil {
		return ticket, err
	}
	get, err := http.NewRequestWithContext(ctx, http.MethodGet, objectURL, nil)
	if err != nil {
		return ticket, err
	}
	query = get.URL.Query()
	query.Set("X-Amz-Expires", "3600")
	query.Set("response-content-type", contentType)
	query.Set("response-cache-control", "private, no-store")
	get.URL.RawQuery = query.Encode()
	ticket.ImageURL, _, err = signer.PresignHTTP(ctx, credentials, get, "UNSIGNED-PAYLOAD", "s3", cfg.Region, now)
	ticket.Headers = map[string]string{"Content-Type": contentType}
	ticket.ExpiresAt = now.Add(5 * time.Minute).Unix()
	return ticket, err
}
