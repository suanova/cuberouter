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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"gorm.io/gorm"
)

type InviteView struct {
	model.OrganizationInvite
	InviterUsername    string `json:"inviter_username"`
	InviterDisplayName string `json:"inviter_display_name"`
}

type CreateInviteRequest struct {
	Email          string
	Role           string
	ForceRotate    bool
	IdempotencyKey string
}

type OrganizationInviteListRequest struct {
	Offset int
	Limit  int
}

type InvitePublicView struct {
	Id                 int    `json:"id"`
	OrganizationId     int    `json:"organization_id"`
	OrganizationName   string `json:"organization_name"`
	OrganizationSlug   string `json:"organization_slug"`
	Role               string `json:"role"`
	TargetEmail        string `json:"target_email"`
	Status             string `json:"status"`
	InviterUserId      int    `json:"inviter_user_id"`
	InviterUsername    string `json:"inviter_username"`
	InviterDisplayName string `json:"inviter_display_name"`
	CurrentUserId      int    `json:"current_user_id"`
	CurrentUserEmail   string `json:"current_user_email"`
	EmailMatched       bool   `json:"email_matched"`
	ExpiredAt          int64  `json:"expired_at"`
}

var sendOrganizationInviteEmail = func(subject string, receiver string, content string) error {
	return common.SendEmailWithTimeout(subject, receiver, content, organizationInviteSMTPTimeout())
}

var (
	ErrOrganizationInviteDeliveryInProgress = errors.New("organization invite delivery in progress")
	ErrOrganizationInviteRecipientRejected  = errors.New("organization invite recipient rejected")
	ErrOrganizationInviteAlreadySent        = errors.New("pending invite already sent")
)

type OrganizationInviteDeliveryError struct {
	InviteID       int
	DeliveryStatus string
	Cause          error
}

func (e *OrganizationInviteDeliveryError) Error() string {
	return "organization invite email delivery failed"
}

func (e *OrganizationInviteDeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

const defaultOrganizationInviteTTLSeconds = 7 * 24 * 60 * 60

func organizationInviteExpiredAt(now int64) int64 {
	ttlSeconds := common.GetEnvOrDefault("ORGANIZATION_INVITE_TTL_SECONDS", defaultOrganizationInviteTTLSeconds)
	if ttlSeconds <= 0 {
		return 0
	}
	return now + int64(ttlSeconds)
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

const organizationInviteForceRotateOperation = "organization_invite_force_rotate"
const organizationInviteDeliveryLeaseSeconds int64 = 5 * 60
const organizationInviteDeliveryProcessingTTLSeconds = organizationInviteDeliveryLeaseSeconds
const organizationInviteDeliveryErrorCode = "organization_invite_delivery_failed"

const (
	defaultOrganizationInviteSMTPTimeoutSeconds = 30
	minOrganizationInviteSMTPTimeoutSeconds     = 1
	maxOrganizationInviteSMTPTimeoutSeconds     = 240
)

func organizationInviteSMTPTimeout() time.Duration {
	seconds := common.GetEnvOrDefault("SMTP_TIMEOUT_SECONDS", defaultOrganizationInviteSMTPTimeoutSeconds)
	if seconds < minOrganizationInviteSMTPTimeoutSeconds || seconds > maxOrganizationInviteSMTPTimeoutSeconds {
		common.SysError(fmt.Sprintf("SMTP_TIMEOUT_SECONDS must be between %d and %d, using default value: %d", minOrganizationInviteSMTPTimeoutSeconds, maxOrganizationInviteSMTPTimeoutSeconds, defaultOrganizationInviteSMTPTimeoutSeconds))
		seconds = defaultOrganizationInviteSMTPTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func organizationInviteDeliveryStatusForError(err error) string {
	var deliveryErr *common.EmailDeliveryError
	if !errors.As(err, &deliveryErr) {
		return model.OrganizationInviteDeliveryUnknown
	}
	if deliveryErr.Stage == common.EmailDeliveryStageRecipient && deliveryErr.SMTPCode >= 500 && deliveryErr.SMTPCode <= 599 {
		return model.OrganizationInviteDeliveryRejected
	}
	if deliveryErr.Stage == common.EmailDeliveryStageContent {
		return model.OrganizationInviteDeliveryUnknown
	}
	return model.OrganizationInviteDeliveryFailed
}

type organizationInviteIdempotencyResult struct {
	InviteId int `json:"invite_id"`
}

type organizationInviteIdempotencyState struct {
	Record         *model.OrganizationIdempotencyRecord
	ReplayInviteId int
	ResumeInviteId int
}

type persistedOrganizationEmailInvite struct {
	Invite              model.OrganizationInvite
	RawToken            string
	IdempotencyRecordId int
}

func organizationInviteForceRotateRequestHash(operatorUserId, organizationId int, targetEmail string, role string) (string, error) {
	data, err := common.Marshal(map[string]any{
		"operation_type":   organizationInviteForceRotateOperation,
		"operator_user_id": operatorUserId,
		"organization_id":  organizationId,
		"target_email":     targetEmail,
		"role":             role,
	})
	if err != nil {
		return "", err
	}
	return common.GenerateHMAC(string(data)), nil
}

func prepareOrganizationInviteForceRotateIdempotencyWithTx(tx *gorm.DB, operatorUserId, organizationId int, idempotencyKey string, requestHash string, now int64) (*organizationInviteIdempotencyState, error) {
	var existing model.OrganizationIdempotencyRecord
	err := model.LockForUpdate(tx).Where("idempotency_key = ?", idempotencyKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record := &model.OrganizationIdempotencyRecord{
			OrganizationId: organizationId,
			OperationType:  organizationInviteForceRotateOperation,
			IdempotencyKey: idempotencyKey,
			RequestHash:    requestHash,
			Status:         model.OrganizationIdempotencyStatusProcessing,
			CreatedBy:      operatorUserId,
			ExpiresAt:      now + organizationInviteDeliveryProcessingTTLSeconds,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tx.Create(record).Error; err != nil {
			return nil, err
		}
		return &organizationInviteIdempotencyState{Record: record}, nil
	}
	if err != nil {
		return nil, err
	}
	if existing.OrganizationId != organizationId || existing.CreatedBy != operatorUserId || existing.OperationType != organizationInviteForceRotateOperation || existing.RequestHash != requestHash {
		return nil, errors.New("organization idempotency conflict")
	}
	var result organizationInviteIdempotencyResult
	if err := common.Unmarshal([]byte(existing.ResultJson), &result); err != nil || result.InviteId <= 0 {
		return nil, errors.New("organization idempotency conflict")
	}
	if existing.Status == model.OrganizationIdempotencyStatusSucceeded {
		return &organizationInviteIdempotencyState{Record: &existing, ReplayInviteId: result.InviteId}, nil
	}
	if existing.Status == model.OrganizationIdempotencyStatusProcessing && existing.ExpiresAt > now {
		return nil, errors.New("organization idempotency conflict")
	}
	if existing.Status != model.OrganizationIdempotencyStatusProcessing && existing.Status != model.OrganizationIdempotencyStatusFailed {
		return nil, errors.New("organization idempotency conflict")
	}
	existing.Status = model.OrganizationIdempotencyStatusProcessing
	existing.ErrorCode = ""
	existing.ExpiresAt = now + organizationInviteDeliveryProcessingTTLSeconds
	existing.UpdatedAt = now
	if err := tx.Model(&existing).Select("status", "error_code", "expires_at", "updated_at").Updates(&existing).Error; err != nil {
		return nil, err
	}
	return &organizationInviteIdempotencyState{Record: &existing, ResumeInviteId: result.InviteId}, nil
}

func saveOrganizationInviteIdempotencyProgressWithTx(tx *gorm.DB, record *model.OrganizationIdempotencyRecord, inviteId int, now int64) error {
	if record == nil {
		return nil
	}
	resultJson, err := common.Marshal(organizationInviteIdempotencyResult{InviteId: inviteId})
	if err != nil {
		return err
	}
	record.Status = model.OrganizationIdempotencyStatusProcessing
	record.ResultJson = string(resultJson)
	record.ErrorCode = ""
	record.ExpiresAt = now + organizationInviteDeliveryProcessingTTLSeconds
	record.UpdatedAt = now
	return tx.Model(record).Select("status", "result_json", "error_code", "expires_at", "updated_at").Updates(record).Error
}

func ListOrganizationInvites(operatorUserId, organizationId int, accessMode string) ([]InviteView, error) {
	invites, _, err := ListOrganizationInvitesPaged(operatorUserId, organizationId, accessMode, OrganizationInviteListRequest{})
	return invites, err
}

func ListOrganizationInvitesPaged(operatorUserId, organizationId int, accessMode string, req OrganizationInviteListRequest) ([]InviteView, int64, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, 0, errors.New("invalid organization invite request")
	}
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, 0, err
	}
	if !actor.Capabilities.CanViewInvites {
		return nil, 0, errors.New("permission denied")
	}
	if actor.Organization.Status == model.OrganizationStatusDissolved && !actor.IsPlatformAdmin {
		return nil, 0, errors.New("organization dissolved")
	}
	if _, err := repairStaleOrganizationInviteDeliveries(model.DB, organizationId, common.GetTimestamp()); err != nil {
		return nil, 0, err
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	inviteTable := model.OrganizationInvite{}.TableName()
	var invites []InviteView
	query := model.DB.Table(inviteTable).
		Select(inviteTable+".*, users.username AS inviter_username, users.display_name AS inviter_display_name").
		Joins("LEFT JOIN users ON users.id = "+inviteTable+".inviter_user_id").
		Where(inviteTable+".organization_id = ?", organizationId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = query.Order(inviteTable + ".id desc")
	if req.Limit > 0 {
		query = query.Limit(req.Limit).Offset(req.Offset)
	}
	err = query.Scan(&invites).Error
	return invites, total, err
}

func repairStaleOrganizationInviteDeliveries(tx *gorm.DB, organizationId int, now int64) (int64, error) {
	if tx == nil || organizationId <= 0 {
		return 0, nil
	}
	result := tx.Model(&model.OrganizationInvite{}).
		Where("organization_id = ? AND status = ? AND delivery_status = ? AND updated_at <= ?", organizationId, model.OrganizationInviteStatusPending, model.OrganizationInviteDeliveryPending, now-organizationInviteDeliveryLeaseSeconds).
		Updates(map[string]any{
			"delivery_status":     model.OrganizationInviteDeliveryUnknown,
			"last_delivery_error": "lease_expired:unknown",
			"updated_at":          now,
		})
	return result.RowsAffected, result.Error
}

func CreateEmailInvite(operatorUserId, organizationId int, accessMode string, req CreateInviteRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*model.OrganizationInvite, error) {
	persisted, err := persistOrganizationEmailInvite(operatorUserId, organizationId, accessMode, req, auditMetadata...)
	if err != nil {
		return nil, err
	}
	if persisted.RawToken == "" {
		invite := persisted.Invite
		invite.Token = ""
		return &invite, nil
	}
	if err := deliverOrganizationEmailInvite(persisted); err != nil {
		return nil, err
	}
	invite := persisted.Invite
	invite.Token = persisted.RawToken
	return &invite, nil
}

func normalizeOrganizationInviteRequest(req CreateInviteRequest) (string, string, string, error) {
	targetEmail := NormalizeEmail(req.Email)
	if targetEmail == "" || !strings.Contains(targetEmail, "@") {
		return "", "", "", errors.New("invalid invite email")
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = model.OrganizationRoleMember
	}
	if role != model.OrganizationRoleAdmin && role != model.OrganizationRoleMember {
		return "", "", "", errors.New("invalid organization member role")
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if req.ForceRotate {
		var err error
		idempotencyKey, err = requireOrganizationIdempotencyKey(idempotencyKey)
		if err != nil {
			return "", "", "", err
		}
	}
	return targetEmail, role, idempotencyKey, nil
}

func persistOrganizationEmailInvite(operatorUserId, organizationId int, accessMode string, req CreateInviteRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*persistedOrganizationEmailInvite, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, errors.New("invalid organization invite request")
	}
	targetEmail, role, idempotencyKey, err := normalizeOrganizationInviteRequest(req)
	if err != nil {
		return nil, err
	}
	var requestHash string
	if req.ForceRotate {
		requestHash, err = organizationInviteForceRotateRequestHash(operatorUserId, organizationId, targetEmail, role)
		if err != nil {
			return nil, err
		}
	}
	now := common.GetTimestamp()
	var persisted *persistedOrganizationEmailInvite
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityManageInvitations)
		if err != nil {
			return err
		}
		if decision.Role == OrganizationPolicyRolePlatformAdmin || decision.Role == OrganizationPolicyRolePlatformRoot {
			return errors.New("permission denied")
		}
		if _, err := repairStaleOrganizationInviteDeliveries(tx, organizationId, now); err != nil {
			return err
		}
		expiredAt := organizationInviteExpiredAt(now)
		var existingInvite model.OrganizationInvite
		hasExistingInvite := false
		auditAction := organizationAuditActionInviteCreate
		auditReason := ""
		err = model.LockForUpdate(tx).Where("organization_id = ? AND target_email = ?", organizationId, targetEmail).First(&existingInvite).Error
		if err == nil {
			hasExistingInvite = true
			if NormalizeEmail(existingInvite.TargetEmail) != targetEmail {
				return errors.New("organization idempotency conflict")
			}
			if existingInvite.Status == model.OrganizationInviteStatusPending && (existingInvite.ExpiredAt == 0 || existingInvite.ExpiredAt > now) {
				switch existingInvite.DeliveryStatus {
				case model.OrganizationInviteDeliveryPending:
					return ErrOrganizationInviteDeliveryInProgress
				case model.OrganizationInviteDeliveryRejected:
					return ErrOrganizationInviteRecipientRejected
				case model.OrganizationInviteDeliverySent:
					if !req.ForceRotate {
						return ErrOrganizationInviteAlreadySent
					}
				}
				auditAction = organizationAuditActionInviteResend
				if req.ForceRotate {
					auditReason = "resend"
				} else {
					auditReason = "retry delivery"
				}
			} else {
				auditAction = organizationAuditActionInviteReopen
				auditReason = "reopen from " + existingInvite.Status
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var idempotencyState *organizationInviteIdempotencyState
		if req.ForceRotate {
			idempotencyState, err = prepareOrganizationInviteForceRotateIdempotencyWithTx(tx, operatorUserId, organizationId, idempotencyKey, requestHash, now)
			if err != nil {
				return err
			}
			if idempotencyState.ReplayInviteId > 0 {
				if !hasExistingInvite || existingInvite.Id != idempotencyState.ReplayInviteId {
					return errors.New("organization idempotency conflict")
				}
				existingInvite.Token = ""
				persisted = &persistedOrganizationEmailInvite{Invite: existingInvite, IdempotencyRecordId: idempotencyState.Record.Id}
				return nil
			}
			if idempotencyState.ResumeInviteId > 0 && (!hasExistingInvite || existingInvite.Id != idempotencyState.ResumeInviteId) {
				return errors.New("organization idempotency conflict")
			}
		}
		var existingUser model.User
		if err := tx.Select("id").Where("LOWER(email) = ?", targetEmail).First(&existingUser).Error; err == nil {
			var existingMember model.OrganizationMember
			if err := tx.Where("organization_id = ? AND user_id = ? AND status IN ?", organizationId, existingUser.Id, []string{model.OrganizationMemberStatusActive, model.OrganizationMemberStatusDisabled}).First(&existingMember).Error; err == nil {
				return errors.New("user already joined organization")
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		token, err := generateOrganizationInviteToken()
		if err != nil {
			return err
		}
		if hasExistingInvite {
			before := existingInvite
			existingInvite.Type = model.OrganizationInviteTypeEmail
			existingInvite.Role = role
			existingInvite.Token = token
			existingInvite.TokenHash = model.HashOrganizationInviteToken(token)
			existingInvite.Status = model.OrganizationInviteStatusPending
			existingInvite.InviterUserId = operatorUserId
			existingInvite.AcceptedUserId = 0
			existingInvite.UpdatedAt = now
			existingInvite.ExpiredAt = expiredAt
			existingInvite.AcceptedAt = 0
			existingInvite.RevokedAt = 0
			existingInvite.Reason = ""
			existingInvite.DeliveryStatus = model.OrganizationInviteDeliveryPending
			existingInvite.DeliveryAttempts++
			existingInvite.LastDeliveryError = ""
			existingInvite.DeliveredAt = 0
			if err := tx.Model(&existingInvite).Select("type", "role", "token_hash", "status", "inviter_user_id", "accepted_user_id", "updated_at", "expired_at", "accepted_at", "revoked_at", "reason", "delivery_status", "delivery_attempts", "last_delivery_error", "delivered_at").Updates(&existingInvite).Error; err != nil {
				return err
			}
			if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, auditAction, "invite", existingInvite.Id, before, existingInvite, auditReason, auditMetadata...); err != nil {
				return err
			}
			if idempotencyState != nil {
				if err := saveOrganizationInviteIdempotencyProgressWithTx(tx, idempotencyState.Record, existingInvite.Id, now); err != nil {
					return err
				}
			}
			persisted = &persistedOrganizationEmailInvite{Invite: existingInvite, RawToken: token}
			if idempotencyState != nil {
				persisted.IdempotencyRecordId = idempotencyState.Record.Id
			}
			return nil
		}
		candidate := &model.OrganizationInvite{OrganizationId: organizationId, Type: model.OrganizationInviteTypeEmail, TargetEmail: targetEmail, Role: role, Token: token, TokenHash: model.HashOrganizationInviteToken(token), Status: model.OrganizationInviteStatusPending, InviterUserId: operatorUserId, CreatedAt: now, UpdatedAt: now, ExpiredAt: expiredAt, DeliveryStatus: model.OrganizationInviteDeliveryPending, DeliveryAttempts: 1}
		if err := tx.Create(candidate).Error; err != nil {
			return err
		}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionInviteCreate, "invite", candidate.Id, nil, candidate, "", auditMetadata...); err != nil {
			return err
		}
		if idempotencyState != nil {
			if err := saveOrganizationInviteIdempotencyProgressWithTx(tx, idempotencyState.Record, candidate.Id, now); err != nil {
				return err
			}
		}
		persisted = &persistedOrganizationEmailInvite{Invite: *candidate, RawToken: token}
		if idempotencyState != nil {
			persisted.IdempotencyRecordId = idempotencyState.Record.Id
		}
		return nil
	})
	return persisted, err
}

func deliverOrganizationEmailInvite(persisted *persistedOrganizationEmailInvite) error {
	if persisted == nil || persisted.Invite.Id <= 0 || persisted.RawToken == "" {
		return errors.New("invalid organization invite delivery")
	}
	var lifecycleErr error
	var subject string
	var receiver string
	var content string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var organization model.Organization
		if err := model.LockForUpdate(tx).Where("id = ?", persisted.Invite.OrganizationId).First(&organization).Error; err != nil {
			return err
		}
		var invite model.OrganizationInvite
		if err := model.LockForUpdate(tx).Where("id = ? AND organization_id = ?", persisted.Invite.Id, organization.Id).First(&invite).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		if organization.Status == model.OrganizationStatusDissolved {
			if invite.Status == model.OrganizationInviteStatusPending {
				invite.Status = model.OrganizationInviteStatusRevoked
				invite.RevokedAt = now
				invite.UpdatedAt = now
				if err := tx.Model(&invite).Select("status", "revoked_at", "updated_at").Updates(&invite).Error; err != nil {
					return err
				}
			}
			persisted.Invite = invite
			lifecycleErr = errors.New("organization dissolved")
			return nil
		}
		if organization.Status != model.OrganizationStatusActive {
			lifecycleErr = errors.New("organization disabled")
			return nil
		}
		if invite.Status != model.OrganizationInviteStatusPending {
			return errors.New("invite is not pending")
		}
		if invite.TokenHash != model.HashOrganizationInviteToken(persisted.RawToken) {
			return errors.New("organization idempotency conflict")
		}
		if invite.DeliveryStatus == model.OrganizationInviteDeliverySent {
			persisted.Invite = invite
			return nil
		}
		if invite.DeliveryStatus != model.OrganizationInviteDeliveryPending {
			return errors.New("organization invite delivery is not pending")
		}
		inviteURL := strings.TrimRight(system_setting.ServerAddress, "/") + "/organization/invite/" + persisted.RawToken
		subject = fmt.Sprintf("%s organization invitation", organization.Name)
		receiver = invite.TargetEmail
		content = buildOrganizationInviteEmailContent(organization.Name, invite.Role, inviteURL)
		persisted.Invite = invite
		return nil
	})
	if err != nil {
		return err
	}
	if lifecycleErr != nil || subject == "" {
		return lifecycleErr
	}
	if sendErr := sendOrganizationInviteEmail(subject, receiver, content); sendErr != nil {
		deliveryStatus := organizationInviteDeliveryStatusForError(sendErr)
		if err := finalizeOrganizationInviteDelivery(persisted, deliveryStatus, organizationInviteDeliveryErrorSummary(sendErr, deliveryStatus), 0); err != nil {
			return err
		}
		return &OrganizationInviteDeliveryError{InviteID: persisted.Invite.Id, DeliveryStatus: deliveryStatus, Cause: sendErr}
	}
	return finalizeOrganizationInviteDelivery(persisted, model.OrganizationInviteDeliverySent, "", common.GetTimestamp())
}

func completeOrganizationInviteDeliveryIdempotencyWithTx(tx *gorm.DB, recordId, inviteId int, now int64) error {
	if recordId <= 0 {
		return nil
	}
	var record model.OrganizationIdempotencyRecord
	if err := model.LockForUpdate(tx).Where("id = ?", recordId).First(&record).Error; err != nil {
		return err
	}
	if record.Status == model.OrganizationIdempotencyStatusSucceeded {
		return nil
	}
	if record.OperationType != organizationInviteForceRotateOperation || record.Status != model.OrganizationIdempotencyStatusProcessing {
		return errors.New("organization idempotency conflict")
	}
	resultJson, err := common.Marshal(organizationInviteIdempotencyResult{InviteId: inviteId})
	if err != nil {
		return err
	}
	record.Status = model.OrganizationIdempotencyStatusSucceeded
	record.ResultJson = string(resultJson)
	record.ErrorCode = ""
	record.ExpiresAt = 0
	record.UpdatedAt = now
	return tx.Model(&record).Select("status", "result_json", "error_code", "expires_at", "updated_at").Updates(&record).Error
}

func finalizeOrganizationInviteDelivery(persisted *persistedOrganizationEmailInvite, deliveryStatus string, errorSummary string, deliveredAt int64) error {
	if persisted == nil || persisted.Invite.Id <= 0 || persisted.RawToken == "" {
		return errors.New("invalid organization invite delivery")
	}
	now := common.GetTimestamp()
	return model.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.OrganizationInvite{}).
			Where("id = ? AND organization_id = ? AND token_hash = ? AND delivery_status = ?", persisted.Invite.Id, persisted.Invite.OrganizationId, model.HashOrganizationInviteToken(persisted.RawToken), model.OrganizationInviteDeliveryPending).
			Updates(map[string]any{
				"delivery_status":     deliveryStatus,
				"last_delivery_error": errorSummary,
				"delivered_at":        deliveredAt,
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("organization idempotency conflict")
		}
		persisted.Invite.DeliveryStatus = deliveryStatus
		persisted.Invite.LastDeliveryError = errorSummary
		persisted.Invite.DeliveredAt = deliveredAt
		persisted.Invite.UpdatedAt = now
		if deliveryStatus == model.OrganizationInviteDeliverySent {
			return completeOrganizationInviteDeliveryIdempotencyWithTx(tx, persisted.IdempotencyRecordId, persisted.Invite.Id, now)
		}
		return failOrganizationInviteDeliveryIdempotencyWithTx(tx, persisted.IdempotencyRecordId, now)
	})
}

func failOrganizationInviteDeliveryIdempotencyWithTx(tx *gorm.DB, recordId int, now int64) error {
	if recordId <= 0 {
		return nil
	}
	var record model.OrganizationIdempotencyRecord
	if err := model.LockForUpdate(tx).Where("id = ?", recordId).First(&record).Error; err != nil {
		return err
	}
	if record.OperationType != organizationInviteForceRotateOperation || record.Status != model.OrganizationIdempotencyStatusProcessing {
		return errors.New("organization idempotency conflict")
	}
	record.Status = model.OrganizationIdempotencyStatusFailed
	record.ErrorCode = organizationInviteDeliveryErrorCode
	record.ExpiresAt = 0
	record.UpdatedAt = now
	return tx.Model(&record).Select("status", "error_code", "expires_at", "updated_at").Updates(&record).Error
}

func organizationInviteDeliveryErrorSummary(err error, deliveryStatus string) string {
	if err == nil {
		return ""
	}
	var deliveryErr *common.EmailDeliveryError
	if !errors.As(err, &deliveryErr) {
		return "unknown:" + deliveryStatus
	}
	stage := strings.TrimSpace(string(deliveryErr.Stage))
	if stage == "" {
		stage = "unknown"
	}
	if deliveryErr.SMTPCode > 0 {
		return fmt.Sprintf("%s:%d:%s", stage, deliveryErr.SMTPCode, deliveryStatus)
	}
	return stage + ":" + deliveryStatus
}

func GetInviteByToken(token string, currentUserId int) (*InvitePublicView, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("invalid invite token")
	}
	var invite model.OrganizationInvite
	if err := model.DB.Where("token_hash IN ?", model.OrganizationInviteTokenHashes(token)).First(&invite).Error; err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	if invite.Status != model.OrganizationInviteStatusPending {
		return nil, errors.New("invite is not pending")
	}
	if invite.ExpiredAt > 0 && invite.ExpiredAt <= now {
		invite.Status = model.OrganizationInviteStatusExpired
		invite.UpdatedAt = now
		if err := model.DB.Model(&invite).Select("status", "updated_at").Updates(&invite).Error; err != nil {
			return nil, err
		}
		return nil, errors.New("invite expired")
	}
	var organization model.Organization
	if err := model.DB.Where("id = ?", invite.OrganizationId).First(&organization).Error; err != nil {
		return nil, err
	}
	var inviter model.User
	_ = model.DB.Select("id", "username", "display_name").Where("id = ?", invite.InviterUserId).First(&inviter).Error
	view := &InvitePublicView{Id: invite.Id, OrganizationId: invite.OrganizationId, OrganizationName: organization.Name, OrganizationSlug: organization.Slug, Role: invite.Role, TargetEmail: invite.TargetEmail, Status: invite.Status, InviterUserId: invite.InviterUserId, InviterUsername: inviter.Username, InviterDisplayName: inviter.DisplayName, CurrentUserId: currentUserId, ExpiredAt: invite.ExpiredAt}
	if currentUserId > 0 {
		var user model.User
		if err := model.DB.Select("id", "email").Where("id = ?", currentUserId).First(&user).Error; err == nil {
			view.CurrentUserEmail = user.Email
			view.EmailMatched = NormalizeEmail(user.Email) == NormalizeEmail(invite.TargetEmail)
		}
	}
	return view, nil
}

func AcceptInvite(token string, userId int, auditMetadata ...OrganizationAuditRequestMetadata) error {
	token = strings.TrimSpace(token)
	if token == "" || userId <= 0 {
		return errors.New("invalid invite acceptance request")
	}
	var lookup struct {
		OrganizationId int
	}
	if err := model.DB.Model(&model.OrganizationInvite{}).Select("organization_id").Where("token_hash IN ?", model.OrganizationInviteTokenHashes(token)).First(&lookup).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var organization model.Organization
		if err := model.LockForUpdate(tx).Where("id = ?", lookup.OrganizationId).First(&organization).Error; err != nil {
			return err
		}
		var invite model.OrganizationInvite
		if err := model.LockForUpdate(tx).Where("organization_id = ? AND token_hash IN ?", organization.Id, model.OrganizationInviteTokenHashes(token)).First(&invite).Error; err != nil {
			return err
		}
		if invite.Status != model.OrganizationInviteStatusPending {
			return errors.New("invite is not pending")
		}
		if invite.ExpiredAt > 0 && invite.ExpiredAt <= now {
			invite.Status = model.OrganizationInviteStatusExpired
			invite.UpdatedAt = now
			if err := tx.Model(&invite).Select("status", "updated_at").Updates(&invite).Error; err != nil {
				return err
			}
			return errors.New("invite expired")
		}
		if organization.Status == model.OrganizationStatusDisabled {
			return errors.New("organization disabled")
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		var user model.User
		if err := tx.Select("id", "email").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if NormalizeEmail(user.Email) != NormalizeEmail(invite.TargetEmail) {
			return errors.New("invite email mismatch")
		}
		before := invite
		result := tx.Model(&model.OrganizationInvite{}).
			Where("id = ? AND status = ?", invite.Id, model.OrganizationInviteStatusPending).
			Updates(map[string]any{"status": model.OrganizationInviteStatusAccepted, "accepted_user_id": userId, "accepted_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("invite is not pending")
		}
		invite.Status = model.OrganizationInviteStatusAccepted
		invite.AcceptedUserId = userId
		invite.AcceptedAt = now
		invite.UpdatedAt = now
		if err := ReactivateOrganizationMember(tx, invite.OrganizationId, userId, invite.Role, invite.InviterUserId); err != nil {
			return err
		}
		actor, err := getOrganizationActorContextWithTxForAccessMode(tx, userId, organization.Id, OrganizationAccessModeWorkspace, true)
		if err != nil {
			return err
		}
		return recordOrganizationAudit(tx, &organization, userId, actor.OperatorRoleForAudit, organizationAuditActionInviteAccept, "invite", invite.Id, before, invite, "", auditMetadata...)
	})
}

func RevokeInvite(operatorUserId, organizationId int, accessMode string, inviteId int, reason string, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 || inviteId <= 0 {
		return errors.New("invalid organization invite request")
	}
	now := common.GetTimestamp()
	return model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityManageInvitations)
		if err != nil {
			return err
		}
		var invite model.OrganizationInvite
		if err := model.LockForUpdate(tx).Where("id = ? AND organization_id = ?", inviteId, organizationId).First(&invite).Error; err != nil {
			return err
		}
		if invite.Status != model.OrganizationInviteStatusPending {
			return errors.New("only pending invite can be revoked")
		}
		before := invite
		invite.Status = model.OrganizationInviteStatusRevoked
		invite.RevokedAt = now
		invite.UpdatedAt = now
		invite.Reason = strings.TrimSpace(reason)
		if err := tx.Model(&invite).Select("status", "revoked_at", "updated_at", "reason").Updates(&invite).Error; err != nil {
			return err
		}
		return recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionInviteRevoke, "invite", invite.Id, before, invite, invite.Reason, auditMetadata...)
	})
}

func generateOrganizationInviteToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func buildOrganizationInviteEmailContent(organizationName string, role string, inviteURL string) string {
	return fmt.Sprintf("<p>You have been invited to join <strong>%s</strong> as <strong>%s</strong>.</p><p><a href=\"%s\">Accept invitation</a></p>", html.EscapeString(organizationName), html.EscapeString(role), html.EscapeString(inviteURL))
}
