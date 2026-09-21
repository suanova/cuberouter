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
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestStudioUploadTicketBindsObjectAndUploadHeaders(t *testing.T) {
	cfg := StudioUploadConfig{Endpoint: "https://objects.example", Bucket: "studio-test", Region: "us-east-1", AccessKey: "test-access", SecretKey: "test-secret"}
	now := time.Date(2026, 9, 21, 4, 0, 0, 0, time.UTC)
	ticket, err := PresignStudioUpload(context.Background(), cfg, 17, "image/png", 42, now)
	require.NoError(t, err)
	upload, err := url.Parse(ticket.UploadURL)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(upload.Path, "/studio-test/media-studio/uploads/17/"))
	assert.Equal(t, "300", upload.Query().Get("X-Amz-Expires"))
	assert.Equal(t, "content-length;content-type;host", upload.Query().Get("X-Amz-SignedHeaders"))
	download, err := url.Parse(ticket.ImageURL)
	require.NoError(t, err)
	assert.Equal(t, upload.Path, download.Path)
	assert.Equal(t, "3600", download.Query().Get("X-Amz-Expires"))
	assert.NotContains(t, ticket.UploadURL, cfg.SecretKey)
	assert.Equal(t, now.Add(5*time.Minute).Unix(), ticket.ExpiresAt)
	// A receiver recomputing the signature with different bytes or media type rejects it.
	query := upload.Query()
	for _, key := range []string{"X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-Date", "X-Amz-SignedHeaders", "X-Amz-Signature"} {
		query.Del(key)
	}
	upload.RawQuery = query.Encode()
	signer := v4.NewSigner(func(options *v4.SignerOptions) { options.DisableURIPathEscaping = true })
	for _, tc := range []struct {
		name, mime string
		size       int64
		matches    bool
	}{
		{"exact headers", "image/png", 42, true}, {"different length", "image/png", 43, false}, {"different MIME", "text/html", 42, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPut, upload.String(), nil)
			require.NoError(t, err)
			request.ContentLength = tc.size
			request.Header.Set("Content-Type", tc.mime)
			signed, _, err := signer.PresignHTTP(context.Background(), aws.Credentials{AccessKeyID: cfg.AccessKey, SecretAccessKey: cfg.SecretKey}, request, "UNSIGNED-PAYLOAD", "s3", cfg.Region, now)
			require.NoError(t, err)
			assert.Equal(t, tc.matches, signed == ticket.UploadURL)
		})
	}
}
func TestStudioUploadRejectsInvalidUpload(t *testing.T) {
	cfg := StudioUploadConfig{Endpoint: "https://objects.example", Bucket: "studio-test", Region: "us-east-1", AccessKey: "test", SecretKey: "secret"}
	for _, tc := range []struct {
		name, mime string
		owner      int
		size       int64
	}{
		{"no owner", "image/png", 0, 1}, {"SVG", "image/svg+xml", 1, 1}, {"zero bytes", "image/png", 1, 0}, {"too large", "image/png", 1, StudioUploadMaxBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PresignStudioUpload(context.Background(), cfg, tc.owner, tc.mime, tc.size, time.Now())
			require.Error(t, err)
		})
	}
}
func TestStudioUploadRequiresSecureOrigin(t *testing.T) {
	for _, endpoint := range []string{"http://objects.example", "https://user:pass@objects.example", "https://objects.example/bucket", "https://objects.example?token=secret", "https://objects.example#fragment"} {
		t.Run(endpoint, func(t *testing.T) {
			cfg := StudioUploadConfig{Endpoint: endpoint, Bucket: "studio-test", Region: "us-east-1", AccessKey: "test", SecretKey: "secret"}
			require.Error(t, cfg.Validate())
		})
	}
}
