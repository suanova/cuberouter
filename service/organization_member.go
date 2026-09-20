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
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

type MemberView struct {
	model.OrganizationMember
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	UserStatus  int    `json:"user_status"`
}

type UpdateMemberRequest struct {
	Role             string
	Status           string
	TransferToUserId int
	Reason           string
	IdempotencyKey   string
}

type AddMemberRequest struct {
	UserId int
	Role   string
	Reason string
}

type TransferOrganizationOwnerRequest struct {
	OwnerUserId    int
	Reason         string
	IdempotencyKey string
}

type RemoveMemberRequest struct {
	TransferToUserId int
	Reason           string
	IdempotencyKey   string
}

type ExitMemberRequest struct {
	TransferToUserId int
	Reason           string
	IdempotencyKey   string
}

const (
	organizationOwnerTransferOperation = "organization_owner_transfer"
	organizationMemberDisableOperation = "organization_member_disable"
	organizationMemberRemoveOperation  = "organization_member_remove"
	organizationMemberExitOperation    = "organization_member_exit"
)

type OrganizationMemberListRequest struct {
	Offset int
	Limit  int
}

func ListOrganizationMembers(operatorUserId, organizationId int, accessMode string) ([]MemberView, error) {
	members, _, err := ListOrganizationMembersPaged(operatorUserId, organizationId, accessMode, OrganizationMemberListRequest{})
	return members, err
}

func ListOrganizationMembersPaged(operatorUserId, organizationId int, accessMode string, req OrganizationMemberListRequest) ([]MemberView, int64, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, 0, errors.New("invalid organization member request")
	}
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, 0, err
	}
	if !actor.Capabilities.CanViewOrganization {
		return nil, 0, errors.New("permission denied")
	}
	if actor.Organization.Status == model.OrganizationStatusDissolved && !actor.IsPlatformAdmin {
		return nil, 0, errors.New("organization dissolved")
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	var members []MemberView
	query := model.DB.Table("organization_members").
		Select("organization_members.*, users.username, users.display_name, users.email, users.status AS user_status").
		Joins("LEFT JOIN users ON users.id = organization_members.user_id").
		Where("organization_members.organization_id = ? AND organization_members.status NOT IN ?", organizationId, []string{model.OrganizationMemberStatusRemoved, model.OrganizationMemberStatusExited})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = query.Order("organization_members.id asc")
	if req.Limit > 0 {
		query = query.Limit(req.Limit).Offset(req.Offset)
	}
	if err := query.Scan(&members).Error; err != nil {
		return members, total, err
	}
	for index := range members {
		if members[index].UserId == actor.Organization.OwnerUserId {
			members[index].Role = model.OrganizationRoleOwner
		}
	}
	return members, total, nil
}

func AddOrganizationMember(operatorUserId, organizationId int, accessMode string, req AddMemberRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*MemberView, error) {
	if operatorUserId <= 0 || organizationId <= 0 || req.UserId <= 0 {
		return nil, errors.New("invalid organization member request")
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = model.OrganizationRoleMember
	}
	if role != model.OrganizationRoleAdmin && role != model.OrganizationRoleMember {
		return nil, errors.New("invalid organization member role")
	}
	now := common.GetTimestamp()
	var added model.OrganizationMember
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityManageMembers)
		if err != nil {
			return err
		}
		if decision.Role != OrganizationPolicyRolePlatformAdmin && decision.Role != OrganizationPolicyRolePlatformRoot {
			return errors.New("permission denied")
		}
		var user model.User
		if err := tx.Select("id", "username", "display_name", "email").Where("id = ?", req.UserId).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("user not found")
			}
			return err
		}
		var existing model.OrganizationMember
		err = model.LockForUpdate(tx).Where("organization_id = ? AND user_id = ?", organizationId, req.UserId).First(&existing).Error
		if err == nil {
			if existing.Status == model.OrganizationMemberStatusActive || existing.Status == model.OrganizationMemberStatusDisabled {
				return errors.New("user already joined organization")
			}
			before := existing
			existing.Role = role
			existing.Status = model.OrganizationMemberStatusActive
			existing.DisabledSource = ""
			existing.InvitedBy = operatorUserId
			existing.UpdatedAt = now
			existing.DisabledAt = 0
			existing.ExitedAt = 0
			existing.RemovedAt = 0
			if err := tx.Model(&existing).Select("role", "status", "disabled_source", "invited_by", "updated_at", "disabled_at", "exited_at", "removed_at").Updates(&existing).Error; err != nil {
				return err
			}
			added = existing
			return recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionMemberAdd, "member", existing.Id, before, existing, strings.TrimSpace(req.Reason), auditMetadata...)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		member := model.OrganizationMember{OrganizationId: organizationId, UserId: req.UserId, Role: role, Status: model.OrganizationMemberStatusActive, InvitedBy: operatorUserId, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		added = member
		return recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionMemberAdd, "member", member.Id, nil, member, strings.TrimSpace(req.Reason), auditMetadata...)
	})
	if err != nil {
		return nil, err
	}
	var view MemberView
	err = model.DB.Table("organization_members").
		Select("organization_members.*, users.username, users.display_name, users.email").
		Joins("LEFT JOIN users ON users.id = organization_members.user_id").
		Where("organization_members.id = ?", added.Id).
		Scan(&view).Error
	return &view, err
}

func TransferOrganizationOwner(operatorUserId, organizationId int, accessMode string, req TransferOrganizationOwnerRequest, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 || req.OwnerUserId <= 0 {
		return errors.New("invalid organization owner transfer request")
	}
	idempotencyKey, err := requireOrganizationIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	reason := strings.TrimSpace(req.Reason)
	requestHash, err := organizationIdempotencyRequestHash(map[string]any{
		"operation_type":   organizationOwnerTransferOperation,
		"operator_user_id": operatorUserId,
		"organization_id":  organizationId,
		"owner_user_id":    req.OwnerUserId,
		"reason":           reason,
	})
	if err != nil {
		return err
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		organization, err := lockOrganizationForUpdateWithTx(tx, organizationId)
		if err != nil {
			return err
		}
		idempotency, err := prepareOrganizationIdempotencyWithTx(tx, operatorUserId, organizationId, organizationOwnerTransferOperation, idempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if idempotency.Replayed {
			return nil
		}
		decision, err := organizationManagementDecisionForLockedOrganizationWithTx(tx, operatorUserId, organization, accessMode, OrganizationCapabilityTransferOwner)
		if err != nil {
			return err
		}
		if decision.Role == OrganizationPolicyRolePlatformRoot {
			var target model.OrganizationMember
			if err := model.LockForUpdate(tx).
				Where("organization_id = ? AND user_id = ? AND status = ?", organizationId, req.OwnerUserId, model.OrganizationMemberStatusActive).
				First(&target).Error; err != nil {
				return errors.New("active owner repair target required")
			}
			before := map[string]any{"owner_user_id": organization.OwnerUserId, "target_user_id": req.OwnerUserId}
			if err := tx.Model(&model.OrganizationMember{}).
				Where("organization_id = ? AND role = ? AND status = ? AND user_id <> ?", organizationId, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive, req.OwnerUserId).
				Updates(map[string]any{"role": model.OrganizationRoleAdmin, "updated_at": now}).Error; err != nil {
				return err
			}
			target.Role = model.OrganizationRoleOwner
			target.UpdatedAt = now
			if err := tx.Model(&target).Select("role", "updated_at").Updates(&target).Error; err != nil {
				return err
			}
			previousOwnerUserId := organization.OwnerUserId
			organization.OwnerUserId = req.OwnerUserId
			organization.UpdatedAt = now
			if err := tx.Model(organization).Select("owner_user_id", "updated_at").Updates(organization).Error; err != nil {
				return err
			}
			var activeOwnerCount int64
			if err := tx.Model(&model.OrganizationMember{}).Where("organization_id = ? AND role = ? AND status = ?", organizationId, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive).Count(&activeOwnerCount).Error; err != nil {
				return err
			}
			if activeOwnerCount != 1 {
				return organizationOperationBlockedError("unique active owner required")
			}
			after := map[string]any{"owner_user_id": req.OwnerUserId, "previous_owner_user_id": previousOwnerUserId}
			if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionOwnerTransfer, "member", target.Id, before, after, reason, auditMetadata...); err != nil {
				return err
			}
			return completeOrganizationIdempotencyWithTx(tx, idempotency, after)
		}
		if organization.OwnerUserId == req.OwnerUserId {
			return completeOrganizationIdempotencyWithTx(tx, idempotency, map[string]any{"owner_user_id": req.OwnerUserId})
		}
		var currentOwner model.OrganizationMember
		if err := model.LockForUpdate(tx).
			Where("organization_id = ? AND user_id = ? AND status = ?", organizationId, organization.OwnerUserId, model.OrganizationMemberStatusActive).
			First(&currentOwner).Error; err != nil {
			return organizationOperationBlockedError("active owner member required")
		}
		var target model.OrganizationMember
		if err := model.LockForUpdate(tx).
			Where("organization_id = ? AND user_id = ? AND role IN ? AND status = ?", organizationId, req.OwnerUserId, []string{model.OrganizationRoleAdmin, model.OrganizationRoleMember}, model.OrganizationMemberStatusActive).
			First(&target).Error; err != nil {
			return errors.New("active owner transfer target required")
		}
		before := map[string]any{"owner_user_id": organization.OwnerUserId, "target_user_id": req.OwnerUserId}
		currentOwner.Role = model.OrganizationRoleAdmin
		currentOwner.UpdatedAt = now
		if err := tx.Model(&currentOwner).Select("role", "updated_at").Updates(&currentOwner).Error; err != nil {
			return err
		}
		target.Role = model.OrganizationRoleOwner
		target.UpdatedAt = now
		if err := tx.Model(&target).Select("role", "updated_at").Updates(&target).Error; err != nil {
			return err
		}
		organization.OwnerUserId = req.OwnerUserId
		organization.UpdatedAt = now
		if err := tx.Model(organization).Select("owner_user_id", "updated_at").Updates(organization).Error; err != nil {
			return err
		}
		var activeOwnerCount int64
		if err := tx.Model(&model.OrganizationMember{}).Where("organization_id = ? AND role = ? AND status = ?", organizationId, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive).Count(&activeOwnerCount).Error; err != nil {
			return err
		}
		if activeOwnerCount != 1 {
			return organizationOperationBlockedError("unique active owner required")
		}
		after := map[string]any{"owner_user_id": req.OwnerUserId, "previous_owner_user_id": currentOwner.UserId}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionOwnerTransfer, "member", target.Id, before, after, reason, auditMetadata...); err != nil {
			return err
		}
		return completeOrganizationIdempotencyWithTx(tx, idempotency, after)
	})
}

func UpdateOrganizationMember(operatorUserId, organizationId int, accessMode string, targetUserId int, req UpdateMemberRequest, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 || targetUserId <= 0 {
		return errors.New("invalid organization member request")
	}
	role := strings.TrimSpace(req.Role)
	status := strings.TrimSpace(req.Status)
	if role != "" && role != model.OrganizationRoleAdmin && role != model.OrganizationRoleMember {
		return errors.New("invalid organization member role")
	}
	if status != "" && status != model.OrganizationMemberStatusActive && status != model.OrganizationMemberStatusDisabled {
		return errors.New("invalid organization member status")
	}
	if role == "" && status == "" {
		return errors.New("empty organization member update")
	}
	var idempotencyKey string
	var requestHash string
	if status == model.OrganizationMemberStatusDisabled {
		transferToUserId := 0
		if role == model.OrganizationRoleMember {
			transferToUserId = req.TransferToUserId
		}
		var err error
		idempotencyKey, err = requireOrganizationIdempotencyKey(req.IdempotencyKey)
		if err != nil {
			return err
		}
		requestHash, err = organizationIdempotencyRequestHash(map[string]any{
			"operation_type":      organizationMemberDisableOperation,
			"operator_user_id":    operatorUserId,
			"organization_id":     organizationId,
			"target_user_id":      targetUserId,
			"transfer_to_user_id": transferToUserId,
			"role":                role,
			"status":              status,
			"reason":              strings.TrimSpace(req.Reason),
		})
		if err != nil {
			return err
		}
	}
	now := common.GetTimestamp()
	var affectedCacheKeys []string
	operatorRole := ""
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityManageMembers)
		if err != nil {
			return err
		}
		operatorRole = decision.Role
		var target model.OrganizationMember
		if err := model.LockForUpdate(tx).Where("organization_id = ? AND user_id = ?", organizationId, targetUserId).First(&target).Error; err != nil {
			return err
		}
		before := target
		newRole := target.Role
		if target.Role == model.OrganizationRoleOwner && organization.OwnerUserId != target.UserId {
			newRole = model.OrganizationRoleMember
		}
		newStatus := target.Status
		if role != "" {
			newRole = role
		}
		if status != "" {
			newStatus = status
		}
		var idempotency *organizationIdempotencyState
		if status == model.OrganizationMemberStatusDisabled {
			idempotency, err = prepareOrganizationIdempotencyWithTx(tx, operatorUserId, organizationId, organizationMemberDisableOperation, idempotencyKey, requestHash)
			if err != nil {
				return err
			}
			if idempotency.Replayed {
				return nil
			}
		}
		if err := ensureOrganizationMemberTargetAllowed(decision, organization, &target); err != nil {
			return err
		}
		if target.Role == model.OrganizationRoleAdmin && newRole == model.OrganizationRoleMember {
			transferReason := strings.TrimSpace(req.Reason)
			if transferReason == "" {
				transferReason = organizationAuditReasonMemberDemoted
			}
			transferredKeys, err := transferOrganizationPublicKeysWithTx(tx, organization, target.UserId, req.TransferToUserId, operatorUserId, decision.Role, transferReason, auditMetadata...)
			if err != nil {
				return err
			}
			affectedCacheKeys = append(affectedCacheKeys, transferredKeys...)
		}
		wasDisabled := target.Status == model.OrganizationMemberStatusDisabled
		previousDisabledSource := target.DisabledSource
		newDisabledSource := target.DisabledSource
		if status == model.OrganizationMemberStatusDisabled {
			requestedSource := model.OrganizationMemberDisableSourceOrganization
			if organizationPolicyDecisionIsPlatform(decision) {
				requestedSource = model.OrganizationMemberDisableSourcePlatform
			}
			if requestedSource == model.OrganizationMemberDisableSourcePlatform || newDisabledSource == "" {
				newDisabledSource = requestedSource
			}
		}
		if newStatus == model.OrganizationMemberStatusDisabled && !wasDisabled {
			target.DisabledAt = now
		}
		if newStatus == model.OrganizationMemberStatusActive {
			target.DisabledAt = 0
			newDisabledSource = ""
		}
		target.Role = newRole
		target.Status = newStatus
		target.DisabledSource = newDisabledSource
		target.UpdatedAt = now
		if err := tx.Model(&target).Select("role", "status", "disabled_source", "updated_at", "disabled_at").Updates(&target).Error; err != nil {
			return err
		}
		if newStatus == model.OrganizationMemberStatusDisabled && (!wasDisabled || previousDisabledSource != newDisabledSource) {
			keys, err := createOrganizationMemberDisabledTokenBlockersWithTx(tx, organizationId, target.UserId, operatorUserId, newDisabledSource, now)
			if err != nil {
				return err
			}
			affectedCacheKeys = append(affectedCacheKeys, keys...)
		}
		if newStatus == model.OrganizationMemberStatusActive && wasDisabled {
			keys, err := clearOrganizationMemberDisabledTokenBlockersWithTx(tx, organizationId, target.UserId, now)
			if err != nil {
				return err
			}
			affectedCacheKeys = append(affectedCacheKeys, keys...)
		}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionMemberUpdate, "member", target.Id, before, target, strings.TrimSpace(req.Reason), auditMetadata...); err != nil {
			return err
		}
		return completeOrganizationIdempotencyWithTx(tx, idempotency, map[string]any{"member_id": target.Id, "status": target.Status, "role": target.Role})
	})
	if err != nil {
		if auditErr := recordOrganizationMemberKeyTransferBlockedAudit(organizationId, operatorUserId, operatorRole, targetUserId, req.TransferToUserId, req.Reason, err, auditMetadata...); auditErr != nil {
			common.SysError("failed to record organization member key transfer blocker audit: " + auditErr.Error())
		}
		return err
	}
	invalidateOrganizationTokenCaches(affectedCacheKeys...)
	return nil
}

func RemoveOrganizationMember(operatorUserId, organizationId int, accessMode string, targetUserId int, req RemoveMemberRequest, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 || targetUserId <= 0 {
		return errors.New("invalid organization member request")
	}
	idempotencyKey, err := requireOrganizationIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return errors.New("organization member remove reason required")
	}
	requestHash, err := organizationIdempotencyRequestHash(map[string]any{
		"operation_type":      organizationMemberRemoveOperation,
		"operator_user_id":    operatorUserId,
		"organization_id":     organizationId,
		"target_user_id":      targetUserId,
		"transfer_to_user_id": req.TransferToUserId,
		"reason":              reason,
	})
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	var affectedCacheKeys []string
	operatorRole := ""
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityManageMembers)
		if err != nil {
			return err
		}
		operatorRole = decision.Role
		var target model.OrganizationMember
		if err := model.LockForUpdate(tx).Where("organization_id = ? AND user_id = ?", organizationId, targetUserId).First(&target).Error; err != nil {
			return err
		}
		if err := ensureOrganizationMemberTargetAllowed(decision, organization, &target); err != nil {
			return err
		}
		idempotency, err := prepareOrganizationIdempotencyWithTx(tx, operatorUserId, organizationId, organizationMemberRemoveOperation, idempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if idempotency.Replayed {
			return nil
		}
		affectedCacheKeys, err = transferOrganizationKeysWithTx(tx, organization, target.UserId, req.TransferToUserId, operatorUserId, decision.Role, reason, auditMetadata...)
		if err != nil {
			return err
		}
		before := target
		target.Status = model.OrganizationMemberStatusRemoved
		target.DisabledSource = ""
		target.UpdatedAt = now
		target.RemovedAt = now
		if err := tx.Model(&target).Select("status", "disabled_source", "updated_at", "removed_at").Updates(&target).Error; err != nil {
			return err
		}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionMemberRemove, "member", target.Id, before, target, reason, auditMetadata...); err != nil {
			return err
		}
		return completeOrganizationIdempotencyWithTx(tx, idempotency, map[string]any{"member_id": target.Id, "status": target.Status})
	})
	if err != nil {
		if auditErr := recordOrganizationMemberKeyTransferBlockedAudit(organizationId, operatorUserId, operatorRole, targetUserId, req.TransferToUserId, req.Reason, err, auditMetadata...); auditErr != nil {
			common.SysError("failed to record organization member key transfer blocker audit: " + auditErr.Error())
		}
		return err
	}
	invalidateOrganizationTokenCaches(affectedCacheKeys...)
	return nil
}

func ExitOrganization(operatorUserId, organizationId int, accessMode string, req ExitMemberRequest, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 {
		return errors.New("invalid organization member request")
	}
	idempotencyKey, err := requireOrganizationIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return err
	}
	reason := strings.TrimSpace(req.Reason)
	requestHash, err := organizationIdempotencyRequestHash(map[string]any{
		"operation_type":      organizationMemberExitOperation,
		"operator_user_id":    operatorUserId,
		"organization_id":     organizationId,
		"transfer_to_user_id": req.TransferToUserId,
		"reason":              reason,
	})
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	var affectedCacheKeys []string
	operatorRole := ""
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		organization, err := lockOrganizationForUpdateWithTx(tx, organizationId)
		if err != nil {
			return err
		}
		idempotency, err := prepareOrganizationIdempotencyWithTx(tx, operatorUserId, organizationId, organizationMemberExitOperation, idempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if idempotency.Replayed {
			return nil
		}
		decision, err := organizationManagementDecisionForLockedOrganizationWithTx(tx, operatorUserId, organization, accessMode, OrganizationCapabilityExitOrganization)
		if err != nil {
			return err
		}
		operatorRole = decision.Role
		var member model.OrganizationMember
		if err := model.LockForUpdate(tx).
			Where("organization_id = ? AND user_id = ? AND status = ?", organizationId, operatorUserId, model.OrganizationMemberStatusActive).
			First(&member).Error; err != nil {
			return errors.New("permission denied")
		}
		affectedCacheKeys, err = transferOrganizationKeysWithTx(tx, organization, member.UserId, req.TransferToUserId, operatorUserId, decision.Role, reason, auditMetadata...)
		if err != nil {
			return err
		}
		before := member
		member.Status = model.OrganizationMemberStatusExited
		member.UpdatedAt = now
		member.ExitedAt = now
		if err := tx.Model(&member).Select("status", "updated_at", "exited_at").Updates(&member).Error; err != nil {
			return err
		}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionMemberExit, "member", member.Id, before, member, reason, auditMetadata...); err != nil {
			return err
		}
		return completeOrganizationIdempotencyWithTx(tx, idempotency, map[string]any{"member_id": member.Id, "status": member.Status})
	})
	if err != nil {
		if auditErr := recordOrganizationMemberKeyTransferBlockedAudit(organizationId, operatorUserId, operatorRole, operatorUserId, req.TransferToUserId, req.Reason, err, auditMetadata...); auditErr != nil {
			common.SysError("failed to record organization member key transfer blocker audit: " + auditErr.Error())
		}
		return err
	}
	invalidateOrganizationTokenCaches(affectedCacheKeys...)
	return nil
}

func ensureOrganizationMemberTargetAllowed(decision OrganizationPolicyDecision, organization *model.Organization, target *model.OrganizationMember) error {
	if organization == nil || target == nil {
		return errors.New("permission denied")
	}
	if organization.OwnerUserId == target.UserId {
		return errors.New("organization member operation forbidden")
	}
	if target.Status == model.OrganizationMemberStatusDisabled && target.DisabledSource == model.OrganizationMemberDisableSourcePlatform && !organizationPolicyDecisionIsPlatform(decision) {
		return errors.New("organization member disabled by platform")
	}
	if target.Role == model.OrganizationRoleOwner && decision.Role != model.OrganizationRoleOwner && decision.Role != OrganizationPolicyRolePlatformAdmin && decision.Role != OrganizationPolicyRolePlatformRoot {
		return errors.New("organization member operation forbidden")
	}
	if decision.Role == model.OrganizationRoleAdmin && target.Role != model.OrganizationRoleMember {
		return errors.New("organization member operation forbidden")
	}
	return nil
}

func organizationPolicyDecisionIsPlatform(decision OrganizationPolicyDecision) bool {
	return decision.Role == OrganizationPolicyRolePlatformAdmin || decision.Role == OrganizationPolicyRolePlatformRoot
}

func lockedOrganizationManagementDecisionWithTx(tx *gorm.DB, operatorUserId, organizationId int, accessMode string, capability string) (*model.Organization, OrganizationPolicyDecision, error) {
	organization, err := lockOrganizationForUpdateWithTx(tx, organizationId)
	if err != nil {
		return nil, OrganizationPolicyDecision{}, err
	}
	decision, err := organizationManagementDecisionForLockedOrganizationWithTx(tx, operatorUserId, organization, accessMode, capability)
	if err != nil {
		return nil, OrganizationPolicyDecision{}, err
	}
	return organization, decision, nil
}

func lockOrganizationForUpdateWithTx(tx *gorm.DB, organizationId int) (*model.Organization, error) {
	var organization model.Organization
	if err := model.LockForUpdate(tx).Where("id = ?", organizationId).First(&organization).Error; err != nil {
		return nil, err
	}
	return &organization, nil
}

func organizationManagementDecisionForLockedOrganizationWithTx(tx *gorm.DB, operatorUserId int, organization *model.Organization, accessMode string, capability string) (OrganizationPolicyDecision, error) {
	if organization == nil {
		return OrganizationPolicyDecision{}, errors.New("permission denied")
	}
	operatorPlatformRole, err := getUserRoleWithTx(tx, operatorUserId)
	if err != nil {
		return OrganizationPolicyDecision{}, err
	}
	input := OrganizationPolicyInput{
		CurrentUser:  OrganizationPolicyUser{Id: operatorUserId, PlatformRole: operatorPlatformRole},
		Organization: organizationPolicyOrganizationFromModel(*organization),
		DisableState: loadOrganizationPolicyDisableState(tx, organization.Id),
		AccessMode:   accessMode,
	}
	var operatorMember model.OrganizationMember
	err = model.LockForUpdate(tx).
		Where("organization_id = ? AND user_id = ? AND status = ?", organization.Id, operatorUserId, model.OrganizationMemberStatusActive).
		First(&operatorMember).Error
	if err == nil {
		input.Member = &OrganizationPolicyMember{UserId: operatorMember.UserId, Role: operatorMember.Role, Status: operatorMember.Status}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return OrganizationPolicyDecision{}, err
	}
	decision := EvaluateOrganizationPolicy(input)
	if !decision.Allowed {
		if decision.Code == types.ErrorCodeOrganizationAccessDenied {
			return OrganizationPolicyDecision{}, errors.New("permission denied")
		}
		if decision.Message != "" {
			return OrganizationPolicyDecision{}, errors.New(decision.Message)
		}
		return OrganizationPolicyDecision{}, errors.New("permission denied")
	}
	if capability != "" && !decision.HasCapability(capability) {
		if decision.Code == types.ErrorCodeOrganizationDissolved && decision.Message != "" {
			return OrganizationPolicyDecision{}, errors.New(decision.Message)
		}
		return OrganizationPolicyDecision{}, errors.New("permission denied")
	}
	return decision, nil
}

type OrganizationOperationBlockedError struct {
	Message  string
	Blockers []string
}

func (err *OrganizationOperationBlockedError) Error() string {
	if err == nil || err.Message == "" {
		return "organization operation blocked"
	}
	return "organization operation blocked: " + err.Message
}

func organizationOperationBlockedError(message string, blockers ...string) error {
	if len(blockers) == 0 && message != "" {
		blockers = []string{message}
	}
	return &OrganizationOperationBlockedError{Message: message, Blockers: blockers}
}

func ReactivateOrganizationMember(tx *gorm.DB, organizationId, userId int, role string, invitedBy int) error {
	if tx == nil || organizationId <= 0 || userId <= 0 {
		return errors.New("invalid organization member request")
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = model.OrganizationRoleMember
	}
	if role != model.OrganizationRoleAdmin && role != model.OrganizationRoleMember {
		return errors.New("invalid organization member role")
	}
	now := common.GetTimestamp()
	var member model.OrganizationMember
	err := tx.Where("organization_id = ? AND user_id = ?", organizationId, userId).First(&member).Error
	if err == nil {
		if member.Status == model.OrganizationMemberStatusDisabled {
			return errors.New("organization member disabled")
		}
		member.Role = role
		member.Status = model.OrganizationMemberStatusActive
		member.DisabledSource = ""
		member.InvitedBy = invitedBy
		member.UpdatedAt = now
		member.DisabledAt = 0
		member.ExitedAt = 0
		member.RemovedAt = 0
		return tx.Model(&member).Select("role", "status", "disabled_source", "invited_by", "updated_at", "disabled_at", "exited_at", "removed_at").Updates(&member).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	member = model.OrganizationMember{OrganizationId: organizationId, UserId: userId, Role: role, Status: model.OrganizationMemberStatusActive, InvitedBy: invitedBy, CreatedAt: now, UpdatedAt: now}
	return tx.Create(&member).Error
}

func transferOrganizationPublicKeysWithTx(tx *gorm.DB, organization *model.Organization, fromUserId, transferToUserId, operatorUserId int, operatorRole string, reason string, auditMetadata ...OrganizationAuditRequestMetadata) ([]string, error) {
	var tokens []model.Token
	if err := model.LockForUpdate(tx).
		Where("scope_type = ? AND organization_id = ? AND visibility = ? AND (user_id = ? OR responsible_user_id = ?)", model.TokenScopeOrganization, organization.Id, model.TokenVisibilityPublic, fromUserId, fromUserId).
		Order("id asc").
		Find(&tokens).Error; err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, nil
	}

	toUserId := transferToUserId
	if toUserId == 0 {
		toUserId = organization.OwnerUserId
	}
	if toUserId <= 0 || toUserId == fromUserId {
		return nil, organizationOperationBlockedError("active transfer target required")
	}
	if err := ensureOrganizationTokenResponsibleUserWithTx(tx, organization, model.TokenVisibilityPublic, toUserId); err != nil {
		return nil, organizationOperationBlockedError("active transfer target required")
	}

	tokenIds := make([]int, 0, len(tokens))
	keys := make([]string, 0, len(tokens))
	for i := range tokens {
		tokenIds = append(tokenIds, tokens[i].Id)
		keys = append(keys, tokens[i].Key)
	}
	var activeBlockers []model.OrganizationTokenSystemBlocker
	if err := model.LockForUpdate(tx).
		Where("token_id IN ? AND status = ?", tokenIds, model.OrganizationTokenBlockerStatusActive).
		Order("id asc").
		Find(&activeBlockers).Error; err != nil {
		return nil, err
	}
	activeBlockerIds := make([]int, 0, len(activeBlockers))
	for _, blocker := range activeBlockers {
		activeBlockerIds = append(activeBlockerIds, blocker.Id)
	}
	now := common.GetTimestamp()
	if err := tx.Model(&model.Token{}).
		Where("id IN ?", tokenIds).
		Updates(map[string]any{
			"user_id":             toUserId,
			"responsible_user_id": toUserId,
			"transfer_reason":     reason,
			"updated_at":          now,
		}).Error; err != nil {
		return nil, err
	}
	for i := range tokens {
		tokens[i].UserId = toUserId
		tokens[i].ResponsibleUserId = toUserId
		tokens[i].TransferReason = reason
		tokens[i].UpdatedAt = now
		if err := reconcileOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], now); err != nil {
			return nil, err
		}
	}
	clearedBlockerReasons := make([]string, 0)
	if len(activeBlockerIds) > 0 {
		var clearedBlockers []model.OrganizationTokenSystemBlocker
		if err := tx.Where("id IN ? AND status = ?", activeBlockerIds, model.OrganizationTokenBlockerStatusCleared).Order("id asc").Find(&clearedBlockers).Error; err != nil {
			return nil, err
		}
		seenReasons := map[string]bool{}
		for _, blocker := range clearedBlockers {
			if seenReasons[blocker.Reason] {
				continue
			}
			seenReasons[blocker.Reason] = true
			clearedBlockerReasons = append(clearedBlockerReasons, blocker.Reason)
		}
	}
	beforeData := map[string]any{
		"from_user_id": fromUserId,
		"token_count":  len(tokenIds),
		"token_ids":    tokenIds,
	}
	afterData := map[string]any{
		"to_user_id":              toUserId,
		"token_count":             len(tokenIds),
		"token_ids":               tokenIds,
		"cleared_blocker_reasons": clearedBlockerReasons,
	}
	if err := recordOrganizationAudit(tx, organization, operatorUserId, operatorRole, organizationAuditActionMemberKeyTransfer, "token", 0, beforeData, afterData, reason, auditMetadata...); err != nil {
		return nil, err
	}
	return keys, nil
}

func transferOrganizationKeysWithTx(tx *gorm.DB, organization *model.Organization, fromUserId, transferToUserId, operatorUserId int, operatorRole string, reason string, auditMetadata ...OrganizationAuditRequestMetadata) ([]string, error) {
	keyAuditReason := strings.TrimSpace(reason)
	if keyAuditReason == "" {
		keyAuditReason = organizationAuditReasonMemberRemoved
	}
	affectedKeys, err := transferOrganizationPublicKeysWithTx(tx, organization, fromUserId, transferToUserId, operatorUserId, operatorRole, keyAuditReason, auditMetadata...)
	if err != nil {
		return nil, err
	}
	tokenWhere := "scope_type = ? AND organization_id = ? AND visibility <> ? AND (user_id = ? OR responsible_user_id = ?)"
	var tokens []model.Token
	if err := model.LockForUpdate(tx).
		Where(tokenWhere, model.TokenScopeOrganization, organization.Id, model.TokenVisibilityPublic, fromUserId, fromUserId).
		Order("id asc").
		Find(&tokens).Error; err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return affectedKeys, nil
	}

	now := common.GetTimestamp()
	for i := range tokens {
		tokens[i].UpdatedAt = now
		if err := tx.Model(&tokens[i]).Select("updated_at").Updates(&tokens[i]).Error; err != nil {
			return nil, err
		}
		before := tokens[i]
		affectedKeys = append(affectedKeys, tokens[i].Key)
		if err := tx.Delete(&tokens[i]).Error; err != nil {
			return nil, err
		}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, operatorRole, organizationAuditActionTokenDelete, "token", tokens[i].Id, before, nil, keyAuditReason, auditMetadata...); err != nil {
			return nil, err
		}
	}
	return affectedKeys, nil
}
