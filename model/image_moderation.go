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
package model

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ImageModerationModel = "google/shieldgemma-2-4b-it"

// A durable latch: deleting or disabling the last moderator must never switch
// back to a legacy endpoint or silently release unchecked images.
type ImageModerationPolicy struct {
	ID int `gorm:"primaryKey;autoIncrement:false"`
}

// Shared by all Go replicas. Only lease metadata, never image bytes, is stored.
type ImageModerationLease struct {
	Endpoint  string `gorm:"primaryKey;size:64"`
	Owner     string `gorm:"size:64"`
	ExpiresAt int64
}

type ImageModerationSettings struct {
	CAPEM string `json:"image_moderation_ca_pem"`
}

func (channel *Channel) AfterCreate(tx *gorm.DB) error {
	if channel.Type != constant.ChannelTypeImageModeration {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ImageModerationPolicy{ID: 1}).Error
}

func ImageModerationChannels(ctx context.Context) (bool, []*Channel, error) {
	if DB == nil {
		return false, nil, errors.New("moderation database unavailable")
	}
	var policy []ImageModerationPolicy
	if err := DB.WithContext(ctx).Limit(1).Find(&policy).Error; err != nil {
		return false, nil, err
	}
	if len(policy) == 0 {
		return false, nil, nil
	}
	var channels []*Channel
	err := DB.WithContext(ctx).Where("type = ? AND status = ?", constant.ChannelTypeImageModeration, common.ChannelStatusEnabled).Order("id").Find(&channels).Error
	return true, channels, err
}

// Endpoint is a complete /v1/moderate URL. Custom CA certificates are public
// trust material; service tokens remain in the normal protected channel key.
func ValidateImageModerationChannel(channel *Channel) (string, string, error) {
	bad := errors.New("image moderation requires an HTTPS /v1/moderate URL, one service key (32+ characters), and a valid optional CA PEM")
	if channel.BaseURL == nil || channel.ChannelInfo.IsMultiKey || len(channel.Key) < 32 || len(channel.Key) > 4096 || strings.ContainsAny(channel.Key, "\r\n") {
		return "", "", bad
	}
	u, err := url.Parse(*channel.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/v1/moderate" || u.RawPath != "" {
		return "", "", bad
	}
	loopback := u.Hostname() == "localhost"
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		loopback = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return "", "", bad
	}
	var settings ImageModerationSettings
	if channel.OtherSettings != "" && common.UnmarshalJsonStr(channel.OtherSettings, &settings) != nil {
		return "", "", bad
	}
	if len(settings.CAPEM) > 64<<10 || strings.Contains(settings.CAPEM, "PRIVATE KEY") {
		return "", "", bad
	}
	if settings.CAPEM != "" && !x509.NewCertPool().AppendCertsFromPEM([]byte(settings.CAPEM)) {
		return "", "", bad
	}
	u.Host = strings.ToLower(u.Host)
	if u.Scheme == "https" && u.Port() == "443" {
		u.Host = u.Hostname()
		if strings.Contains(u.Host, ":") {
			u.Host = "[" + u.Host + "]"
		}
	}
	return u.String(), settings.CAPEM, nil
}

func TryImageModerationLease(ctx context.Context, endpoint, owner string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(endpoint)))
	now := time.Now().UnixMilli()
	lease := ImageModerationLease{Endpoint: key}
	db := DB.WithContext(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&lease).Error; err != nil {
		return false, err
	}
	r := db.Model(&ImageModerationLease{}).Where("endpoint = ? AND expires_at <= ?", key, now).
		Updates(map[string]any{"owner": owner, "expires_at": now + ttl.Milliseconds()})
	return r.RowsAffected == 1, r.Error
}

func ReleaseImageModerationLease(endpoint, owner string, cooldown time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(endpoint)))
	// Ownership check prevents a late response releasing another replica's lease.
	DB.WithContext(ctx).Model(&ImageModerationLease{}).Where("endpoint = ? AND owner = ?", key, owner).
		Updates(map[string]any{"expires_at": time.Now().Add(cooldown).UnixMilli(), "owner": ""})
}
