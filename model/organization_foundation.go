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
	"errors"
	"strings"

	"gorm.io/gorm"
)

const (
	OrganizationDisableSourceSelf     = "self"
	OrganizationDisableSourcePlatform = "platform"

	OrganizationRecordStatusActive  = "active"
	OrganizationRecordStatusCleared = "cleared"

	OrganizationTokenBlockerStatusActive  = "active"
	OrganizationTokenBlockerStatusCleared = "cleared"

	OrganizationIdempotencyStatusProcessing = "processing"
	OrganizationIdempotencyStatusSucceeded  = "succeeded"
	OrganizationIdempotencyStatusFailed     = "failed"

	OrganizationBillingSessionStatusPreConsumed = "pre_consumed"
	OrganizationBillingSessionStatusRepairing   = "repairing"
	OrganizationBillingSessionStatusSettled     = "settled"
	OrganizationBillingSessionStatusRefunded    = "refunded"
	OrganizationBillingSessionStatusFailed      = "failed"

	OrganizationBillingRecordTypeSettle     = "settle"
	OrganizationBillingRecordTypePreConsume = "pre_consume"
	OrganizationBillingRecordTypeRefund     = "refund"
	OrganizationBillingRecordTypeAdjustment = "adjustment"
)

type UserAccountContext struct {
	UserId      int    `json:"user_id" gorm:"primaryKey;not null"`
	ContextType string `json:"context_type" gorm:"type:varchar(16);not null;default:'personal'"`
	ContextId   int    `json:"context_id" gorm:"not null;default:0"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;default:0"`
}

type OrganizationDisableRecord struct {
	Id               int    `json:"id"`
	OrganizationId   int    `json:"organization_id" gorm:"index;not null"`
	Source           string `json:"source" gorm:"type:varchar(16);index;not null"`
	Status           string `json:"status" gorm:"type:varchar(16);index;not null;default:'active'"`
	DisabledByUserId int    `json:"disabled_by_user_id" gorm:"index;not null"`
	DisabledReason   string `json:"disabled_reason" gorm:"type:varchar(255);default:''"`
	DisabledAt       int64  `json:"disabled_at" gorm:"bigint;index;default:0"`
	ClearedByUserId  int    `json:"cleared_by_user_id" gorm:"index;default:0"`
	ClearedReason    string `json:"cleared_reason" gorm:"type:varchar(255);default:''"`
	ClearedAt        int64  `json:"cleared_at" gorm:"bigint;default:0"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt        int64  `json:"updated_at" gorm:"bigint;index"`
}

type OrganizationTokenSystemBlocker struct {
	Id             int    `json:"id"`
	TokenId        int    `json:"token_id" gorm:"index;not null"`
	OrganizationId int    `json:"organization_id" gorm:"index;not null"`
	Reason         string `json:"reason" gorm:"type:varchar(64);index;not null"`
	RefType        string `json:"ref_type" gorm:"type:varchar(32);index;default:''"`
	RefId          int    `json:"ref_id" gorm:"index;default:0"`
	Status         string `json:"status" gorm:"type:varchar(16);index;not null;default:'active'"`
	PreviousStatus int    `json:"previous_status" gorm:"default:0"`
	OperatorUserId int    `json:"operator_user_id" gorm:"index;default:0"`
	ClearanceLevel int    `json:"clearance_level" gorm:"default:0"`
	DisabledAt     int64  `json:"disabled_at" gorm:"bigint;index;default:0"`
	ClearedAt      int64  `json:"cleared_at" gorm:"bigint;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;index"`
}

type OrganizationIdempotencyRecord struct {
	Id             int    `json:"id"`
	OrganizationId int    `json:"organization_id" gorm:"index;not null"`
	OperationType  string `json:"operation_type" gorm:"type:varchar(64);index;not null"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(191);uniqueIndex;not null"`
	RequestHash    string `json:"request_hash" gorm:"type:varchar(128);not null;default:''"`
	Status         string `json:"status" gorm:"type:varchar(16);index;not null;default:'processing'"`
	ResultJson     string `json:"result_json" gorm:"type:text"`
	ErrorCode      string `json:"error_code" gorm:"type:varchar(64);default:''"`
	CreatedBy      int    `json:"created_by" gorm:"index;not null"`
	ExpiresAt      int64  `json:"expires_at" gorm:"bigint;index;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;index"`
}

func (record *OrganizationIdempotencyRecord) BeforeSave(tx *gorm.DB) error {
	if strings.TrimSpace(record.IdempotencyKey) == "" {
		return errors.New("idempotency key is required")
	}
	return nil
}

type OrganizationBillingSession struct {
	Id                       int    `json:"id"`
	OrganizationId           int    `json:"organization_id" gorm:"index;not null"`
	IdempotencyKey           string `json:"idempotency_key" gorm:"type:varchar(191);uniqueIndex;not null"`
	RequestId                string `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	TaskId                   string `json:"task_id" gorm:"type:varchar(191);index;default:''"`
	Status                   string `json:"status" gorm:"type:varchar(16);index;not null;default:'pre_consumed'"`
	PreConsumedQuota         int    `json:"pre_consumed_quota" gorm:"not null;default:0"`
	SettledQuota             int    `json:"settled_quota" gorm:"not null;default:0"`
	RefundedQuota            int    `json:"refunded_quota" gorm:"not null;default:0"`
	TokenPreConsumedQuota    int    `json:"token_pre_consumed_quota" gorm:"not null;default:0"`
	TokenRemainDeductedQuota int    `json:"token_remain_deducted_quota" gorm:"not null;default:0"`
	TokenUnlimitedQuota      bool   `json:"token_unlimited_quota" gorm:"not null;default:false"`
	TokenRefunded            bool   `json:"token_refunded" gorm:"not null;default:false"`
	TokenId                  int    `json:"token_id" gorm:"index;default:0"`
	TokenName                string `json:"token_name" gorm:"type:varchar(255);default:''"`
	ChannelId                int    `json:"channel_id" gorm:"index;default:0"`
	ResponsibleUserId        int    `json:"responsible_user_id" gorm:"index;default:0"`
	CreatorUserId            int    `json:"creator_user_id" gorm:"index;default:0"`
	ModelName                string `json:"model_name" gorm:"type:varchar(255);index;default:''"`
	Group                    string `json:"group" gorm:"column:group_name;type:varchar(64);index;default:''"`
	ErrorMessage             string `json:"error_message" gorm:"type:text"`
	RepairAttempts           int    `json:"repair_attempts" gorm:"not null;default:0"`
	LastRepairError          string `json:"last_repair_error" gorm:"type:text"`
	LastHeartbeatAt          int64  `json:"last_heartbeat_at" gorm:"bigint;index;default:0"`
	ExpiresAt                int64  `json:"expires_at" gorm:"bigint;index;default:0"`
	SettledAt                int64  `json:"settled_at" gorm:"bigint;default:0"`
	RefundedAt               int64  `json:"refunded_at" gorm:"bigint;default:0"`
	CreatedAt                int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                int64  `json:"updated_at" gorm:"bigint;index"`
}

func (session *OrganizationBillingSession) BeforeSave(tx *gorm.DB) error {
	if strings.TrimSpace(session.IdempotencyKey) == "" {
		return errors.New("billing session idempotency key is required")
	}
	return nil
}

type OrganizationBillingRecord struct {
	Id                int    `json:"id"`
	OrganizationId    int    `json:"organization_id" gorm:"index;not null"`
	SessionId         int    `json:"session_id" gorm:"index;default:0"`
	RecordKey         string `json:"record_key" gorm:"type:varchar(191);uniqueIndex;not null"`
	RequestId         string `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	TaskId            string `json:"task_id" gorm:"type:varchar(191);index;default:''"`
	RecordType        string `json:"record_type" gorm:"type:varchar(16);index;not null"`
	QuotaDelta        int    `json:"quota_delta" gorm:"not null;default:0"`
	UsedQuotaDelta    int    `json:"used_quota_delta" gorm:"not null;default:0"`
	UsageQuota        int    `json:"usage_quota" gorm:"not null;default:0"`
	QuotaBefore       int    `json:"quota_before" gorm:"not null;default:0"`
	QuotaAfter        int    `json:"quota_after" gorm:"not null;default:0"`
	UsedQuotaBefore   int    `json:"used_quota_before" gorm:"not null;default:0"`
	UsedQuotaAfter    int    `json:"used_quota_after" gorm:"not null;default:0"`
	TokenId           int    `json:"token_id" gorm:"index;default:0"`
	TokenName         string `json:"token_name" gorm:"type:varchar(255);default:''"`
	ResponsibleUserId int    `json:"responsible_user_id" gorm:"index;default:0"`
	CreatorUserId     int    `json:"creator_user_id" gorm:"index;default:0"`
	ModelName         string `json:"model_name" gorm:"type:varchar(255);index;default:''"`
	Group             string `json:"group" gorm:"column:group_name;type:varchar(64);index;default:''"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"not null;default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"not null;default:0"`
	TokenCount        int    `json:"token_count" gorm:"not null;default:0"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`

	ResponsibleUsername    string `json:"responsible_username,omitempty" gorm:"-"`
	ResponsibleDisplayName string `json:"responsible_display_name,omitempty" gorm:"-"`
	LedgerQuotaDelta       int    `json:"ledger_quota_delta" gorm:"-"`
}

func (record *OrganizationBillingRecord) BeforeSave(tx *gorm.DB) error {
	if strings.TrimSpace(record.RecordKey) == "" {
		return errors.New("billing record key is required")
	}
	return nil
}
