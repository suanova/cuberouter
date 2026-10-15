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
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// RAM admission is bounded per Go process; per-endpoint GPU ownership is in
// the shared SQL database. No second CPU routing/pool service is required.
var studioReviewAdmission struct {
	sync.Mutex
	count, bytes int
}
var studioReviewTurn atomic.Uint64

func admitStudioReview(size int) (func(), bool) {
	studioReviewAdmission.Lock()
	defer studioReviewAdmission.Unlock()
	if size <= 0 || size > 12<<20 || studioReviewAdmission.count >= 64 || studioReviewAdmission.bytes+size > 256<<20 {
		return nil, false
	}
	studioReviewAdmission.count++
	studioReviewAdmission.bytes += size
	return func() {
		studioReviewAdmission.Lock()
		defer studioReviewAdmission.Unlock()
		studioReviewAdmission.count--
		studioReviewAdmission.bytes -= size
	}, true
}

func studioChannelClient(ca string) (*http.Client, func()) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if ca != "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		roots.AppendCertsFromPEM([]byte(ca)) // validated before calling
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	}
	return &http.Client{Transport: transport, Timeout: 12 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, transport.CloseIdleConnections
}

func checkStudioChannelOutput(ctx context.Context, data []byte, channels []*model.Channel) studioModerationReport {
	return runStudioChannelReview(ctx, data, channels, true)
}

func runStudioChannelReview(ctx context.Context, data []byte, channels []*model.Channel, refresh bool) studioModerationReport {
	failed := studioModerationReport{State: "error", Source: "channels", Error: "output_check_unavailable"}
	release, ok := admitStudioReview(len(data))
	if !ok {
		return failed
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return failed
	}
	owner := fmt.Sprintf("%x", nonce)
	// Try every configured replica once for availability failures. This retries
	// classification only; never regenerate and never retry a block/malformed verdict.
	tried := make(map[int]bool)
	start := int(studioReviewTurn.Add(1))
	for ctx.Err() == nil {
		if len(channels) == 0 {
			return failed
		}
		// Rotate equal-priority endpoints; preserve explicit operator priority.
		offset := start % len(channels)
		ordered := append(append([]*model.Channel{}, channels[offset:]...), channels[:offset]...)
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].GetPriority() > ordered[j].GetPriority() })
		available := false
		for _, channel := range ordered {
			if tried[channel.Id] {
				continue
			}
			endpoint, ca, err := model.ValidateImageModerationChannel(channel)
			if err != nil {
				tried[channel.Id] = true
				continue
			}
			available = true
			leased, err := model.TryImageModerationLease(ctx, endpoint, owner, 60*time.Second)
			if err != nil {
				return failed
			}
			if !leased {
				continue
			}
			tried[channel.Id] = true
			// Cap audit growth even if an operator adds many broken channels.
			if len(failed.AttemptedChannels) >= 64 {
				model.ReleaseImageModerationLease(endpoint, owner, 0)
				return failed
			}
			failed.AttemptedChannels = append(failed.AttemptedChannels, channel.Id)
			client, closeClient := studioChannelClient(ca)
			report, retry, uncertain := requestStudioModeration(ctx, client, endpoint, channel.Key, data)
			closeClient()
			// A lost connection may still be executing on the GPU. Leave its
			// bounded lease until expiry; another replica can classify meanwhile.
			if !uncertain {
				cooldown := time.Duration(0)
				if retry {
					cooldown = 2 * time.Second
				}
				model.ReleaseImageModerationLease(endpoint, owner, cooldown)
			}
			if !retry {
				report.ChannelID, report.Source = channel.Id, "channels"
				report.AttemptedChannels = failed.AttemptedChannels
				return report
			}
		}
		if !available {
			return failed
		}
		select {
		case <-ctx.Done():
			return failed
		case <-time.After(150 * time.Millisecond):
		}
		if refresh {
			_, latest, err := model.ImageModerationChannels(ctx)
			if err != nil {
				return failed
			}
			channels = latest
		}
	}
	return failed
}

// The Channel page's Test action performs a real hash-bound classification of
// an in-memory, non-person fixture; it does not invoke the image generator.
func testStudioModerationChannel(channel *model.Channel) error {
	fixture := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			fixture.Set(x, y, color.RGBA{30, 100, 60, 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, fixture); err != nil {
		return fmt.Errorf("moderation test fixture unavailable")
	}
	report := runStudioChannelReview(context.Background(), data.Bytes(), []*model.Channel{channel}, false)
	if report.State != "passed" {
		return fmt.Errorf("moderation check failed: verify channel URL, service key, CA and GPU readiness")
	}
	return nil
}
