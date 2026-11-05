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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/studioqueue"
	_ "golang.org/x/image/webp"
)

type studioImageRequest struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt"`
	Size           string          `json:"size"`
	N              *int            `json:"n,omitempty"`
	Quality        string          `json:"quality"`
	Steps          *int            `json:"num_inference_steps,omitempty"`
	CFG            *float64        `json:"true_cfg_scale,omitempty"`
	Seed           *int64          `json:"seed,omitempty"`
	Image          json.RawMessage `json:"image,omitempty"`
	Mask           string          `json:"mask,omitempty"`
	ResponseFormat string          `json:"response_format"`
}

type studioImageSubmission struct {
	Mode       string             `json:"mode"`
	UserPrompt string             `json:"user_prompt"`
	Request    studioImageRequest `json:"request"`
}

func validateStudioImageSubmission(p *studioImageSubmission) ([]string, error) {
	r := &p.Request
	invalid := errors.New("invalid_studio_request")
	if !slices.Contains(service.StudioImageModels(), r.Model) ||
		!slices.Contains([]string{"create", "edit"}, p.Mode) || strings.TrimSpace(r.Prompt) == "" ||
		len([]rune(r.Prompt)) > 16000 || len([]rune(p.UserPrompt)) > 16000 || r.N == nil || *r.N != 1 ||
		r.Steps == nil || *r.Steps < 1 || *r.Steps > 60 || r.CFG == nil ||
		math.IsNaN(*r.CFG) || math.IsInf(*r.CFG, 0) || *r.CFG < 0 || *r.CFG > 10 ||
		r.Seed == nil || *r.Seed < 0 || *r.Seed > 1<<53-1 ||
		!slices.Contains([]string{"fast", "standard", "high"}, r.Quality) {
		return nil, invalid
	}
	dims := strings.Split(r.Size, "x")
	if len(dims) != 2 {
		return nil, invalid
	}
	w, errW := strconv.Atoi(dims[0])
	h, errH := strconv.Atoi(dims[1])
	if errW != nil || errH != nil || w < 256 || h < 256 || w > 1664 || h > 1664 || w%32 != 0 || h%32 != 0 || w*h > 2097152 {
		return nil, invalid
	}
	var images []string
	if len(r.Image) > 0 {
		var single string
		if common.Unmarshal(r.Image, &single) == nil {
			images = []string{single}
		} else if common.Unmarshal(r.Image, &images) != nil {
			return nil, invalid
		}
	}
	if len(images) > 10 || (p.Mode == "create" && (len(images) != 0 || r.Mask != "")) || (p.Mode == "edit" && len(images) == 0) {
		return nil, invalid
	}
	if r.Mask != "" {
		images = append(images, r.Mask)
	}
	var hashes []string
	var firstConfig image.Config
	total := 0
	for i, value := range images {
		header, encoded, found := strings.Cut(value, ",")
		if !found || !slices.Contains([]string{"data:image/png;base64", "data:image/jpeg;base64", "data:image/webp;base64"}, header) {
			return nil, invalid
		}
		data, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(data) == 0 || len(data) > 10<<20 {
			return nil, invalid
		}
		total += len(data)
		if total > 11<<20 {
			return nil, errors.New("studio_references_too_large")
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 40000000 {
			return nil, invalid
		}
		if header != "data:image/"+format+";base64" {
			return nil, invalid
		}
		if i == 0 {
			firstConfig = config
		}
		if r.Mask != "" && i == len(images)-1 {
			if format != "png" || config.Width != firstConfig.Width || config.Height != firstConfig.Height || int64(config.Width)*int64(config.Height) > 2097152 {
				return nil, invalid
			}
			selection, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, invalid
			}
			hasSelection := false
			for y := 0; y < config.Height && !hasSelection; y++ {
				for x := 0; x < config.Width; x++ {
					_, _, _, a := selection.At(x, y).RGBA()
					if a < 65535 {
						hasSelection = true
						break
					}
				}
			}
			if !hasSelection {
				return nil, invalid
			}
		}
		hashes = append(hashes, fmt.Sprintf("%x", sha256.Sum256(data)))
	}
	// Force a private, verifiable result. An upstream URL cannot bypass review.
	r.ResponseFormat = "b64_json"
	if p.UserPrompt == "" {
		p.UserPrompt = r.Prompt
	}
	return hashes, nil
}

func studioImageAudit(r studioqueue.Request, id string) (*model.StudioImageAudit, error) {
	var p studioImageSubmission
	if err := common.Unmarshal(r.Body, &p); err != nil {
		return nil, err
	}
	hashes, err := validateStudioImageSubmission(&p)
	if err != nil {
		return nil, err
	}
	parameters, err := common.Marshal(map[string]any{"mode": p.Mode, "size": p.Request.Size, "quality": p.Request.Quality,
		"steps": p.Request.Steps, "cfg": p.Request.CFG, "seed": p.Request.Seed, "references": len(hashes), "mask": p.Request.Mask != ""})
	if err != nil {
		return nil, err
	}
	inputHashes, err := common.Marshal(hashes)
	if err != nil {
		return nil, err
	}
	return &model.StudioImageAudit{JobID: id, UserID: r.Owner, Model: p.Request.Model,
		UserPrompt: p.UserPrompt, ModelPrompt: p.Request.Prompt, Parameters: string(parameters),
		InputHashes: string(inputHashes), State: "queued", CreatedAt: time.Now().UnixMilli()}, nil
}
