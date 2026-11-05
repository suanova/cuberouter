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
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioImageRegionPreservesOutsidePixels(t *testing.T) {
	base, candidate, mask := image.NewNRGBA(image.Rect(0, 0, 4, 4)), image.NewNRGBA(image.Rect(0, 0, 4, 4)), image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			base.Set(x, y, color.NRGBA{R: 180, A: 255})
			candidate.Set(x, y, color.NRGBA{B: 200, A: 255})
			mask.Set(x, y, color.NRGBA{A: 255})
		}
	}
	mask.Set(1, 1, color.NRGBA{})
	encode := func(img image.Image) []byte {
		var b bytes.Buffer
		require.NoError(t, png.Encode(&b, img))
		return b.Bytes()
	}
	out, err := CompositeStudioImage(encode(candidate), encode(base), encode(mask))
	require.NoError(t, err)
	decoded, err := png.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Equal(t, color.NRGBAModel.Convert(candidate.At(1, 1)), color.NRGBAModel.Convert(decoded.At(1, 1)))
	for _, p := range []image.Point{{0, 0}, {2, 1}, {3, 3}} {
		assert.Equal(t, color.NRGBAModel.Convert(base.At(p.X, p.Y)), color.NRGBAModel.Convert(decoded.At(p.X, p.Y)))
	}
	mask.Set(1, 1, color.NRGBA{A: 255})
	_, err = CompositeStudioImage(encode(candidate), encode(base), encode(mask))
	require.EqualError(t, err, "studio_region_invalid")
}

func TestStudioImageRegionReplacesTransparentSelectedPixels(t *testing.T) {
	base := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	base.Set(0, 0, color.NRGBA{R: 180, A: 255})
	base.Set(1, 0, color.NRGBA{R: 180, A: 255})
	candidate := image.NewNRGBA(base.Bounds())
	mask := image.NewNRGBA(base.Bounds())
	mask.Set(1, 0, color.NRGBA{A: 255})
	encode := func(img image.Image) []byte {
		var b bytes.Buffer
		require.NoError(t, png.Encode(&b, img))
		return b.Bytes()
	}
	out, err := CompositeStudioImage(encode(candidate), encode(base), encode(mask))
	require.NoError(t, err)
	decoded, err := png.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Equal(t, color.NRGBA{}, color.NRGBAModel.Convert(decoded.At(0, 0)))
	assert.Equal(t, base.At(1, 0), color.NRGBAModel.Convert(decoded.At(1, 0)))
}

func TestStudioImageRegionBlendsPartialMaskCoverage(t *testing.T) {
	for _, tc := range []struct {
		name      string
		candidate color.NRGBA
		want      color.NRGBA
	}{
		{"opaque candidate blends both colors", color.NRGBA{B: 255, A: 255}, color.NRGBA{R: 128, B: 127, A: 255}},
		{"transparent candidate cannot tint the base", color.NRGBA{B: 255}, color.NRGBA{R: 255, A: 128}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := image.NewNRGBA(image.Rect(0, 0, 1, 1))
			base.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
			candidate := image.NewNRGBA(base.Bounds())
			candidate.SetNRGBA(0, 0, tc.candidate)
			mask := image.NewNRGBA(base.Bounds())
			mask.SetNRGBA(0, 0, color.NRGBA{A: 128})
			encode := func(img image.Image) []byte {
				var b bytes.Buffer
				require.NoError(t, png.Encode(&b, img))
				return b.Bytes()
			}
			out, err := CompositeStudioImage(encode(candidate), encode(base), encode(mask))
			require.NoError(t, err)
			decoded, err := png.Decode(bytes.NewReader(out))
			require.NoError(t, err)
			assert.Equal(t, tc.want, color.NRGBAModel.Convert(decoded.At(0, 0)))
		})
	}
}
