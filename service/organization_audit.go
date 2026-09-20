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
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

var (
	organizationAuditSecretTextPattern          = regexp.MustCompile(`(?i)(?:bearer\s+)?sk-[A-Za-z0-9_-]{8,}`)
	organizationAuditMaskedSecretPattern        = regexp.MustCompile(`^(?:\*{3}|[^*]{6}\*{10}[^*]{6})$`)
	organizationAuditSensitiveAssignmentPattern = regexp.MustCompile(`(?i)\b(?:api[_-]?key|authorization|auth[_-]?header|mj[_-]?api[_-]?secret|x[_-]?api[_-]?key|x[_-]?goog[_-]?api[_-]?key|token[_-]?hash|token[_-]?ciphertext|client[_-]?secret|access[_-]?token|refresh[_-]?token|invite[_-]?token|raw[_-]?token|ciphertext|encrypted[_-]?key|encrypted[_-]?token)\s*[:=]\s*\S+`)
	organizationAuditContentAssignmentPattern   = regexp.MustCompile(`(?i)\b(?:prompt|prompt[_-]?en|response|request[_-]?body|response[_-]?body|content|messages)\b\s*[:=]`)
)

const (
	organizationAuditActionCreate                    = "organization.create"
	organizationAuditActionUpdate                    = "organization.update"
	organizationAuditActionDisable                   = "organization.disable"
	organizationAuditActionEnable                    = "organization.enable"
	organizationAuditActionDissolve                  = "organization.dissolve"
	organizationAuditActionQuotaAdjust               = "organization.quota_adjust"
	organizationAuditActionMemberUpdate              = "organization.member.update"
	organizationAuditActionMemberAdd                 = "organization.member.add"
	organizationAuditActionMemberRemove              = "organization.member.remove"
	organizationAuditActionMemberExit                = "organization.member.exit"
	organizationAuditActionMemberKeyTransfer         = "organization.member.key_transfer"
	organizationAuditActionMemberKeyTransferBlocked  = "organization.member.key_transfer_blocked"
	organizationAuditActionOwnerTransfer             = "organization.owner.transfer"
	organizationAuditActionTokenCreate               = "organization.token.create"
	organizationAuditActionTokenUpdate               = "organization.token.update"
	organizationAuditActionTokenDelete               = "organization.token.delete"
	organizationAuditActionTokenResponsibilityUpdate = "organization.token.responsibility_update"
	organizationAuditActionInviteCreate              = "organization.invite.create"
	organizationAuditActionInviteResend              = "organization.invite.resend"
	organizationAuditActionInviteReopen              = "organization.invite.reopen"
	organizationAuditActionInviteAccept              = "organization.invite.accept"
	organizationAuditActionInviteRevoke              = "organization.invite.revoke"
	organizationAuditActionBillingRepairFailed       = "organization.billing.repair_failed"

	organizationAuditReasonMemberDemoted = "member_demoted"
	organizationAuditReasonMemberRemoved = "member_removed"
	organizationAuditOperatorRoleSystem  = "system"
)

type OrganizationAuditRequestMetadata struct {
	IP        string
	UserAgent string
}

func recordOrganizationAudit(tx *gorm.DB, organization *model.Organization, operatorUserId int, operatorRole string, actionType string, targetType string, targetId int, beforeData any, afterData any, reason string, requestMetadata ...OrganizationAuditRequestMetadata) error {
	var operator model.User
	if operatorUserId > 0 {
		_ = tx.Select("id", "username", "display_name").Where("id = ?", operatorUserId).First(&operator).Error
	}

	beforeText, err := marshalAuditData(beforeData)
	if err != nil {
		return err
	}
	afterText, err := marshalAuditData(afterData)
	if err != nil {
		return err
	}

	targetName, targetMetadata, err := buildOrganizationAuditTargetSnapshot(tx, targetType, targetId, beforeData, afterData)
	if err != nil {
		return err
	}
	if targetName == "" && targetType == "organization" {
		targetName = organization.Name
	}
	targetName = sanitizeOrganizationAuditText(targetName)
	reason = sanitizeOrganizationAuditText(reason)

	metadata := firstOrganizationAuditRequestMetadata(requestMetadata)
	log := model.OrganizationAuditLog{
		OrganizationId:      organization.Id,
		OrganizationName:    organization.Name,
		OrganizationSlug:    organization.Slug,
		OperatorUserId:      operatorUserId,
		OperatorUsername:    operator.Username,
		OperatorDisplayName: operator.DisplayName,
		OperatorRole:        operatorRole,
		ActionType:          actionType,
		TargetType:          targetType,
		TargetId:            targetId,
		TargetName:          targetName,
		TargetMetadata:      targetMetadata,
		BeforeData:          beforeText,
		AfterData:           afterText,
		Reason:              reason,
		Ip:                  metadata.IP,
		UserAgent:           metadata.UserAgent,
		CreatedAt:           common.GetTimestamp(),
	}
	return tx.Create(&log).Error
}

func firstOrganizationAuditRequestMetadata(values []OrganizationAuditRequestMetadata) OrganizationAuditRequestMetadata {
	if len(values) == 0 {
		return OrganizationAuditRequestMetadata{}
	}
	return OrganizationAuditRequestMetadata{
		IP:        truncateOrganizationAuditMetadata(strings.TrimSpace(values[0].IP), 64),
		UserAgent: truncateOrganizationAuditMetadata(strings.TrimSpace(values[0].UserAgent), 512),
	}
}

func truncateOrganizationAuditMetadata(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength]
}

func recordOrganizationMemberKeyTransferBlockedAudit(organizationId, operatorUserId int, operatorRole string, targetUserId, transferToUserId int, operationReason string, operationErr error, requestMetadata ...OrganizationAuditRequestMetadata) error {
	var blockedErr *OrganizationOperationBlockedError
	if !errors.As(operationErr, &blockedErr) {
		return nil
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var organization model.Organization
		if err := tx.Where("id = ?", organizationId).First(&organization).Error; err != nil {
			return err
		}
		var target model.OrganizationMember
		if err := tx.Where("organization_id = ? AND user_id = ?", organizationId, targetUserId).First(&target).Error; err != nil {
			return err
		}
		reason := truncateOrganizationAuditMetadata(sanitizeOrganizationAuditText(blockedErr.Error()), 255)
		after := map[string]any{
			"id":                  target.Id,
			"user_id":             target.UserId,
			"role":                target.Role,
			"status":              target.Status,
			"transfer_to_user_id": transferToUserId,
			"blockers":            blockedErr.Blockers,
			"operation_reason":    strings.TrimSpace(operationReason),
		}
		return recordOrganizationAudit(tx, &organization, operatorUserId, operatorRole, organizationAuditActionMemberKeyTransferBlocked, "member", target.Id, target, after, reason, requestMetadata...)
	})
}

func recordOrganizationBillingRepairFailedAudit(sessionId int, repairErr error) error {
	if sessionId <= 0 || repairErr == nil {
		return nil
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var session model.OrganizationBillingSession
		if err := tx.Where("id = ?", sessionId).First(&session).Error; err != nil {
			return err
		}
		var organization model.Organization
		if err := tx.Where("id = ?", session.OrganizationId).First(&organization).Error; err != nil {
			return err
		}
		summary := truncateOrganizationAuditMetadata(sanitizeOrganizationAuditText(repairErr.Error()), 255)
		after := map[string]any{
			"session_id":   session.Id,
			"request_id":   session.RequestId,
			"status":       session.Status,
			"repair_error": summary,
		}
		return recordOrganizationAudit(tx, &organization, 0, organizationAuditOperatorRoleSystem, organizationAuditActionBillingRepairFailed, "billing_session", session.Id, nil, after, summary)
	})
}

func buildOrganizationAuditTargetSnapshot(tx *gorm.DB, targetType string, targetId int, values ...any) (string, string, error) {
	metadata := map[string]any{}
	for _, value := range values {
		mergeOrganizationAuditSnapshot(metadata, value)
	}

	switch targetType {
	case "organization":
		if metadata["name"] != nil {
			return auditTargetSnapshotResult(fmt.Sprint(metadata["name"]), metadata)
		}
	case "invite":
		if targetId > 0 {
			var invite model.OrganizationInvite
			if err := tx.Select("id", "target_email", "role", "status", "inviter_user_id", "accepted_user_id", "expired_at", "accepted_at", "revoked_at").Where("id = ?", targetId).First(&invite).Error; err == nil {
				mergeOrganizationAuditSnapshot(metadata, invite)
			} else if err != nil && err != gorm.ErrRecordNotFound {
				return "", "", err
			}
		}
		if email := firstAuditString(metadata, "target_email"); email != "" {
			mergeOrganizationAuditUserByEmail(tx, metadata, "target", email)
		}
		return auditTargetSnapshotResult(firstAuditString(metadata, "target_email"), metadata)
	case "member":
		userId := intFromAuditValue(metadata["user_id"])
		if userId == 0 && targetId > 0 {
			var member model.OrganizationMember
			if err := tx.Select("id", "user_id", "role", "status", "invited_by").Where("id = ?", targetId).First(&member).Error; err == nil {
				mergeOrganizationAuditSnapshot(metadata, member)
				userId = member.UserId
			} else if err != nil && err != gorm.ErrRecordNotFound {
				return "", "", err
			}
		}
		if userId > 0 {
			if err := mergeOrganizationAuditUserById(tx, metadata, "", userId); err != nil {
				return "", "", err
			}
		}
		return auditTargetSnapshotResult(firstAuditString(metadata, "username", "display_name", "email"), metadata)
	case "token":
		if targetId > 0 {
			var token model.Token
			if err := tx.Select("id", "key", "name", "creator_user_id", "responsible_user_id", "user_id", "visibility", "status").Unscoped().Where("id = ?", targetId).First(&token).Error; err == nil {
				mergeOrganizationAuditSnapshot(metadata, map[string]any{
					"id":                  token.Id,
					"api_key":             formatOrganizationAuditAPIKey(token.Key),
					"name":                token.Name,
					"creator_user_id":     token.CreatorUserId,
					"responsible_user_id": token.ResponsibleUserId,
					"user_id":             token.UserId,
					"visibility":          token.Visibility,
					"status":              token.Status,
				})
			} else if err != nil && err != gorm.ErrRecordNotFound {
				return "", "", err
			}
		}
		if responsibleUserId := intFromAuditValue(metadata["responsible_user_id"]); responsibleUserId > 0 {
			if err := mergeOrganizationAuditUserById(tx, metadata, "responsible", responsibleUserId); err != nil {
				return "", "", err
			}
		}
		if creatorUserId := intFromAuditValue(metadata["creator_user_id"]); creatorUserId > 0 {
			if err := mergeOrganizationAuditUserById(tx, metadata, "creator", creatorUserId); err != nil {
				return "", "", err
			}
		}
		return auditTargetSnapshotResult(firstAuditString(metadata, "api_key", "name"), metadata)
	case "quota_adjustment":
		if targetId > 0 {
			var adjustment model.OrganizationQuotaAdjustment
			if err := tx.Select("id", "quota_delta", "quota_before", "quota_after", "reason").Where("id = ?", targetId).First(&adjustment).Error; err == nil {
				mergeOrganizationAuditSnapshot(metadata, adjustment)
			} else if err != nil && err != gorm.ErrRecordNotFound {
				return "", "", err
			}
		}
		return auditTargetSnapshotResult(firstAuditString(metadata, "reason"), metadata)
	}
	return auditTargetSnapshotResult(firstAuditString(metadata, "name", "target_email", "username", "display_name"), metadata)
}

func auditTargetSnapshotResult(targetName string, metadata map[string]any) (string, string, error) {
	metadataText, err := marshalAuditMetadata(metadata)
	return sanitizeOrganizationAuditText(targetName), metadataText, err
}

func mergeOrganizationAuditUserByEmail(tx *gorm.DB, metadata map[string]any, prefix string, email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}
	var user model.User
	if err := tx.Select("id", "username", "display_name", "email").Where("LOWER(email) = ?", email).First(&user).Error; err == nil {
		mergeOrganizationAuditUserSnapshot(metadata, prefix, user)
	}
}

func mergeOrganizationAuditUserById(tx *gorm.DB, metadata map[string]any, prefix string, userId int) error {
	var user model.User
	if err := tx.Select("id", "username", "display_name", "email").Where("id = ?", userId).First(&user).Error; err == nil {
		mergeOrganizationAuditUserSnapshot(metadata, prefix, user)
		return nil
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}

func mergeOrganizationAuditUserSnapshot(metadata map[string]any, prefix string, user model.User) {
	fields := map[string]any{
		"user_id":      user.Id,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"email":        user.Email,
	}
	if prefix != "" {
		fields = map[string]any{
			prefix + "_user_id":      user.Id,
			prefix + "_username":     user.Username,
			prefix + "_display_name": user.DisplayName,
			prefix + "_email":        user.Email,
		}
	}
	mergeOrganizationAuditSnapshot(metadata, fields)
}

func formatOrganizationAuditAPIKey(key string) string {
	return maskOrganizationAuditSecret(formatOrganizationAuditFullAPIKey(key))
}

func maskOrganizationAuditSnapshotSecret(key string, value any) string {
	secret := fmt.Sprint(value)
	if normalizeOrganizationAuditKey(key) == "apikey" {
		return formatOrganizationAuditAPIKey(secret)
	}
	return maskOrganizationAuditSecret(secret)
}

func formatOrganizationAuditFullAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(key), "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	if !strings.HasPrefix(key, "sk-") {
		key = "sk-" + key
	}
	return key
}

func maskOrganizationAuditSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(secret), "bearer ") {
		secret = strings.TrimSpace(secret[7:])
	}
	prefix := ""
	if strings.HasPrefix(secret, "sk-") {
		prefix = "sk-"
		secret = strings.TrimPrefix(secret, "sk-")
	}
	if organizationAuditMaskedSecretPattern.MatchString(secret) {
		return prefix + secret
	}
	if len(secret) <= 12 {
		return prefix + "***"
	}
	return prefix + secret[:6] + "**********" + secret[len(secret)-6:]
}

func mergeOrganizationAuditSnapshot(target map[string]any, value any) {
	if value == nil {
		return
	}
	data, err := common.Marshal(value)
	if err != nil {
		return
	}
	var snapshot map[string]any
	if err := common.Unmarshal(data, &snapshot); err != nil {
		return
	}
	for key, snapshotValue := range snapshot {
		if isSensitiveOrganizationAuditSnapshotKey(key) || isOrganizationAuditContentKey(key) || snapshotValue == nil || snapshotValue == "" {
			continue
		}
		if isOrganizationAuditMaskedSecretKey(key) {
			masked := maskOrganizationAuditSnapshotSecret(key, snapshotValue)
			if masked != "" {
				target[key] = masked
			}
			continue
		}
		snapshotValue = sanitizeOrganizationAuditDataWithKey(key, snapshotValue)
		if snapshotValue == nil || snapshotValue == "" {
			continue
		}
		target[key] = snapshotValue
	}
}

func isSensitiveOrganizationAuditSnapshotKey(key string) bool {
	switch normalizeOrganizationAuditKey(key) {
	case "key", "token", "password", "sessiontoken", "accesstoken", "refreshtoken", "secret", "clientsecret", "tokenhash", "tokenciphertext", "ciphertext", "encryptedkey", "encryptedtoken":
		return true
	default:
		return false
	}
}

func isOrganizationAuditMaskedSecretKey(key string) bool {
	switch normalizeOrganizationAuditKey(key) {
	case "apikey", "authorization", "authheader", "mjapisecret", "xapikey", "xgoogapikey":
		return true
	default:
		return false
	}
}

func isOrganizationAuditContentKey(key string) bool {
	switch normalizeOrganizationAuditKey(key) {
	case "prompt", "prompten", "response", "content", "requestbody", "responsebody", "body", "input", "output", "messages", "data":
		return true
	default:
		return false
	}
}

func normalizeOrganizationAuditKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	replacer := strings.NewReplacer("_", "", "-", "", " ", "", ".", "")
	return replacer.Replace(key)
}

func intFromAuditValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var result int
		_, _ = fmt.Sscanf(v, "%d", &result)
		return result
	default:
		return 0
	}
}

func firstAuditString(metadata map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := fmt.Sprint(metadata[key]); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func marshalAuditMetadata(metadata map[string]any) (string, error) {
	if len(metadata) == 0 {
		return "", nil
	}
	sanitized, _ := sanitizeOrganizationAuditDataWithKey("", metadata).(map[string]any)
	if len(sanitized) == 0 {
		return "", nil
	}
	data, err := common.Marshal(sanitized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func marshalAuditData(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	var decoded any
	if err := common.Unmarshal(data, &decoded); err == nil {
		data, err = common.Marshal(sanitizeOrganizationAuditData(decoded))
		if err != nil {
			return "", err
		}
	}
	return string(data), nil
}

func sanitizeOrganizationAuditData(value any) any {
	return sanitizeOrganizationAuditDataWithKey("", value)
}

func sanitizeOrganizationAuditDataWithKey(parentKey string, value any) any {
	switch v := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(v))
		for key, item := range v {
			if isSensitiveOrganizationAuditSnapshotKey(key) || isOrganizationAuditContentKey(key) {
				continue
			}
			if isOrganizationAuditMaskedSecretKey(key) {
				masked := maskOrganizationAuditSnapshotSecret(key, item)
				if masked != "" {
					sanitized[key] = masked
				}
				continue
			}
			item = sanitizeOrganizationAuditDataWithKey(key, item)
			if item != nil && item != "" {
				sanitized[key] = item
			}
		}
		return sanitized
	case []any:
		sanitized := make([]any, 0, len(v))
		for _, item := range v {
			sanitized = append(sanitized, sanitizeOrganizationAuditDataWithKey(parentKey, item))
		}
		return sanitized
	case string:
		if isOrganizationAuditMaskedSecretKey(parentKey) {
			return maskOrganizationAuditSecret(v)
		}
		if isOrganizationAuditContentKey(parentKey) {
			return nil
		}
		return sanitizeOrganizationAuditText(v)
	default:
		return value
	}
}

func sanitizeOrganizationAuditText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if organizationAuditContentAssignmentPattern.MatchString(value) {
		return "[redacted]"
	}
	value = organizationAuditSecretTextPattern.ReplaceAllStringFunc(value, maskOrganizationAuditSecret)
	value = organizationAuditSensitiveAssignmentPattern.ReplaceAllStringFunc(value, func(match string) string {
		separatorIndex := strings.IndexAny(match, ":=")
		if separatorIndex < 0 {
			return "[redacted]"
		}
		return match[:separatorIndex+1] + "[redacted]"
	})
	return value
}
