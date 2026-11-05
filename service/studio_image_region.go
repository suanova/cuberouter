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
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	xdraw "golang.org/x/image/draw"
)

// CompositeStudioImage preserves reference pixels outside the selected region.
// The result is reviewed after compositing, and its hash binds the final PNG.
func CompositeStudioImage(candidate, reference, mask []byte) ([]byte, error) {
	failure := errors.New("studio_region_invalid")
	if len(candidate) > 12<<20 || len(reference) > 10<<20 || len(mask) > 10<<20 {
		return nil, failure
	}
	for i, data := range [][]byte{reference, mask, candidate} {
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		limit := int64(2097152)
		if i == 2 {
			limit = 4194304
		}
		if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > limit {
			return nil, failure
		}
	}
	base, _, err := image.Decode(bytes.NewReader(reference))
	if err != nil {
		return nil, failure
	}
	selection, err := png.Decode(bytes.NewReader(mask))
	if err != nil {
		return nil, failure
	}
	generated, err := png.Decode(bytes.NewReader(candidate))
	if err != nil {
		return nil, failure
	}
	bounds := base.Bounds()
	if bounds.Dx() != selection.Bounds().Dx() || bounds.Dy() != selection.Bounds().Dy() || int64(bounds.Dx())*int64(bounds.Dy()) > 2097152 {
		return nil, failure
	}
	target := image.Rect(0, 0, bounds.Dx(), bounds.Dy())
	result := image.NewNRGBA(target)
	draw.Draw(result, target, base, bounds.Min, draw.Src)
	resized := image.NewNRGBA(target)
	xdraw.CatmullRom.Scale(resized, target, generated, generated.Bounds(), draw.Src, nil)
	hasSelection := false
	for y := 0; y < target.Dy(); y++ {
		for x := 0; x < target.Dx(); x++ {
			_, _, _, alpha := selection.At(x+selection.Bounds().Min.X, y+selection.Bounds().Min.Y).RGBA()
			hasSelection = hasSelection || alpha < 65535
			if alpha == 65535 {
				continue
			}
			if alpha == 0 {
				result.SetNRGBA(x, y, resized.NRGBAAt(x, y))
				continue
			}
			// Interpolate premultiplied pixels by mask coverage. Unlike Over,
			// source replacement can clear selected transparent output pixels.
			gr, gg, gb, ga := resized.At(x, y).RGBA()
			br, bg, bb, ba := result.At(x, y).RGBA()
			coverage := uint32(65535) - alpha
			result.Set(x, y, color.RGBA64{
				R: uint16((gr*coverage + br*alpha) / 65535),
				G: uint16((gg*coverage + bg*alpha) / 65535),
				B: uint16((gb*coverage + bb*alpha) / 65535),
				A: uint16((ga*coverage + ba*alpha) / 65535),
			})
		}
	}
	if !hasSelection {
		return nil, failure
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, result); err != nil {
		return nil, err
	}
	if encoded.Len() > 12<<20 {
		return nil, failure
	}
	return encoded.Bytes(), nil
}
