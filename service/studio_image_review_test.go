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
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioReviewBindsBytesAndAllPolicyScores(t *testing.T) {
	data := []byte("PNG bytes")
	truth, low, threshold := true, 0.1, 0.5
	valid := func() StudioReview {
		return StudioReview{Protocol: "image-moderation-v1", Model: "google/shieldgemma-2-4b-it", Revision: strings.Repeat("a", 40),
			ImageSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Decision: "allow", ReleaseAllowed: &truth, WeightsVerified: true, Threshold: &threshold,
			Scores: map[string]*float64{"sexual": &low, "violence": &low, "dangerous": &low}}
	}
	require.NoError(t, ValidateStudioReview(valid(), data))
	for _, tc := range []struct {
		name   string
		change func(*StudioReview)
		code   string
	}{
		{"wrong bytes", func(r *StudioReview) { r.ImageSHA256 = strings.Repeat("0", 64) }, "output_check_unavailable"},
		{"missing category", func(r *StudioReview) { delete(r.Scores, "violence") }, "output_check_unavailable"},
		{"unverified weights", func(r *StudioReview) { r.WeightsVerified = false }, "output_check_unavailable"},
		{"contradictory decision", func(r *StudioReview) { r.Decision = "block" }, "output_check_unavailable"},
		{"at threshold", func(r *StudioReview) {
			no := false
			r.Scores["sexual"] = &threshold
			r.ReleaseAllowed = &no
			r.Decision = "block"
		}, "output_blocked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid()
			tc.change(&r)
			require.EqualError(t, ValidateStudioReview(r, data), tc.code)
		})
	}
}

func TestStudioReviewGateNeverWritesUnreviewedOutput(t *testing.T) {
	var pngBytes bytes.Buffer
	require.NoError(t, png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	payload, err := common.Marshal(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(pngBytes.Bytes())}}})
	require.NoError(t, err)
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set(StudioReviewContext, func(context.Context, []byte) error { return fmt.Errorf("output_blocked") })
	r := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload))}
	require.EqualError(t, GateStudioImageResponse(c, r), "output_blocked")
	assert.Empty(t, writer.Body.Bytes())
	c.Set(StudioReviewContext, func(_ context.Context, data []byte) error { assert.Equal(t, pngBytes.Bytes(), data); return nil })
	r.Body = io.NopCloser(bytes.NewReader(payload))
	require.NoError(t, GateStudioImageResponse(c, r))
	approved, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, approved)
	r.Body = io.NopCloser(strings.NewReader(`{"data":[{"url":"https://unreviewed.example/image.png"}]}`))
	require.EqualError(t, GateStudioImageResponse(c, r), "output_check_unavailable")
}

func TestStudioReviewGateRejectsCorruptPixelDataBeforeReview(t *testing.T) {
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	// A complete IHDR passes DecodeConfig even when all pixel chunks are missing.
	corrupt := encoded.Bytes()[:33]
	_, err := png.DecodeConfig(bytes.NewReader(corrupt))
	require.NoError(t, err)
	payload, err := common.Marshal(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(corrupt)}}})
	require.NoError(t, err)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	reviewed := false
	c.Set(StudioReviewContext, func(context.Context, []byte) error { reviewed = true; return nil })
	r := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload))}
	require.EqualError(t, GateStudioImageResponse(c, r), "output_check_unavailable")
	assert.False(t, reviewed, "corrupt bytes must never reach the reviewer")
}
