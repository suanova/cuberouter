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
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	OrganizationInviteDeliveryPending  = "pending"
	OrganizationInviteDeliverySent     = "sent"
	OrganizationInviteDeliveryFailed   = "failed"
	OrganizationInviteDeliveryRejected = "rejected"
	OrganizationInviteDeliveryUnknown  = "unknown"
)

type OrganizationInvite struct {
	Id                int    `json:"id"`
	OrganizationId    int    `json:"organization_id" gorm:"uniqueIndex:idx_org_invite_email;index;not null"`
	Type              string `json:"type" gorm:"type:varchar(16);not null;index;default:'email'"`
	TargetEmail       string `json:"target_email" gorm:"type:varchar(255);not null;uniqueIndex:idx_org_invite_email;index"`
	Role              string `json:"role" gorm:"type:varchar(16);not null;default:'member'"`
	Token             string `json:"-" gorm:"-"`
	TokenHash         string `json:"-" gorm:"column:token_hash;type:varchar(128);not null;uniqueIndex"`
	Status            string `json:"status" gorm:"type:varchar(16);not null;index;default:'pending'"`
	InviterUserId     int    `json:"inviter_user_id" gorm:"index;not null"`
	AcceptedUserId    int    `json:"accepted_user_id" gorm:"index;default:0"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt         int64  `json:"updated_at" gorm:"bigint;index"`
	ExpiredAt         int64  `json:"expired_at" gorm:"bigint;index;default:0"`
	AcceptedAt        int64  `json:"accepted_at" gorm:"bigint;default:0"`
	RevokedAt         int64  `json:"revoked_at" gorm:"bigint;default:0"`
	Reason            string `json:"reason" gorm:"type:varchar(255);default:''"`
	DeliveryStatus    string `json:"delivery_status" gorm:"type:varchar(16);not null;index;default:'sent'"`
	DeliveryAttempts  int    `json:"delivery_attempts" gorm:"not null;default:0"`
	LastDeliveryError string `json:"-" gorm:"type:varchar(255);default:''"`
	DeliveredAt       int64  `json:"delivered_at" gorm:"bigint;default:0"`
}

func (OrganizationInvite) TableName() string {
	return "organization_invitations"
}

func HashOrganizationInviteToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func LegacyHashOrganizationInviteToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	return common.GenerateHMAC(token)
}

func OrganizationInviteTokenHashes(token string) []string {
	hash := HashOrganizationInviteToken(token)
	if hash == "" {
		return nil
	}
	legacyHash := LegacyHashOrganizationInviteToken(token)
	if legacyHash != "" && legacyHash != hash {
		return []string{hash, legacyHash}
	}
	return []string{hash}
}

func (invite *OrganizationInvite) BeforeCreate(tx *gorm.DB) error {
	if invite.DeliveryStatus == "" {
		invite.DeliveryStatus = OrganizationInviteDeliveryPending
	}
	return nil
}

func (invite *OrganizationInvite) BeforeSave(tx *gorm.DB) error {
	if invite.TokenHash == "" && invite.Token != "" {
		invite.TokenHash = HashOrganizationInviteToken(invite.Token)
	}
	return nil
}
