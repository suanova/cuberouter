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
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioImageRequestBoundariesPreserveExplicitZero(t *testing.T) {
	t.Setenv("IMAGE_STUDIO_MODELS", "qwen-image-2.1")
	valid := func() studioImageSubmission {
		var p studioImageSubmission
		require.NoError(t, common.Unmarshal([]byte(`{"mode":"create","request":{"model":"qwen-image-2.1","prompt":"cat","n":1,"size":"1024x1024","quality":"standard","num_inference_steps":1,"true_cfg_scale":0,"seed":0}}`), &p))
		return p
	}
	p := valid()
	_, err := validateStudioImageSubmission(&p)
	require.NoError(t, err)
	body, err := common.Marshal(p.Request)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"true_cfg_scale":0`)
	assert.Contains(t, string(body), `"seed":0`)
	for _, tc := range []struct {
		name   string
		mutate func(*studioImageSubmission)
	}{
		{"missing steps", func(p *studioImageSubmission) { p.Request.Steps = nil }},
		{"too many steps", func(p *studioImageSubmission) { n := 61; p.Request.Steps = &n }},
		{"negative cfg", func(p *studioImageSubmission) { n := -1.0; p.Request.CFG = &n }},
		{"unsafe seed", func(p *studioImageSubmission) { n := int64(1 << 53); p.Request.Seed = &n }},
		{"invalid dimensions", func(p *studioImageSubmission) { p.Request.Size = "1313x1312" }},
		{"oversized prompt", func(p *studioImageSubmission) { p.Request.Prompt = strings.Repeat("a", 16001) }},
		{"multiple outputs", func(p *studioImageSubmission) { n := 2; p.Request.N = &n }},
		{"unconfigured model", func(p *studioImageSubmission) { p.Request.Model = "other" }},
		{"remote image", func(p *studioImageSubmission) {
			p.Mode = "edit"
			p.Request.Image = []byte(`"https://example.com/ref.png"`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid()
			tc.mutate(&p)
			_, err := validateStudioImageSubmission(&p)
			require.Error(t, err)
		})
	}
}

func TestStudioImageRequestRejectsOpaqueAndMismatchedMasksBeforeGeneration(t *testing.T) {
	t.Setenv("IMAGE_STUDIO_MODELS", "qwen-image-2.1")
	encode := func(img image.Image) string {
		var data bytes.Buffer
		require.NoError(t, png.Encode(&data, img))
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	}
	original := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	mask := image.NewNRGBA(original.Bounds())
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			mask.Set(x, y, color.NRGBA{A: 255})
		}
	}
	var p studioImageSubmission
	require.NoError(t, common.Unmarshal([]byte(`{"mode":"edit","request":{"model":"qwen-image-2.1","prompt":"cat","n":1,"size":"1024x1024","quality":"standard","num_inference_steps":1,"true_cfg_scale":0,"seed":0}}`), &p))
	ref, err := common.Marshal(encode(original))
	require.NoError(t, err)
	p.Request.Image = ref
	p.Request.Mask = encode(mask)
	_, err = validateStudioImageSubmission(&p)
	require.Error(t, err)
	p.Request.Mask = encode(image.NewNRGBA(image.Rect(0, 0, 3, 3)))
	_, err = validateStudioImageSubmission(&p)
	require.Error(t, err)
	mask.Set(1, 1, color.NRGBA{})
	p.Request.Mask = encode(mask)
	hashes, err := validateStudioImageSubmission(&p)
	require.NoError(t, err)
	assert.Len(t, hashes, 2)
}
