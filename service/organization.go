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

// 组织生命周期的服务层入口：创建、改名、启停、解散、配额调整。
// 所有写操作都在一个事务里同时落库业务数据和审计记录，事务外的
// invalidateOrganizationTokenCaches 负责让令牌缓存跟上新的封禁状态。
//
// 错误以字符串形式往外传，controller/organization.go 的 writeOrganizationError
// 负责把它们映射成 HTTP 状态码与稳定的 error code（前端按 code 分支，
// 例如 410 organization_dissolved）。改这里的文案会连带改掉对外契约。
const maxActiveOrganizationsPerUser = 20

// organizationSlugConflictRetries 是创建组织时因 slug 唯一键冲突而重开事务的次数。
// slug 由组织名派生，并发同名创建必然撞同一个 slug，一次重试就足够分出胜负。
const organizationSlugConflictRetries = 3

var ErrOrganizationNameConflict = errors.New("organization name conflict")

type CreateOrganizationRequest struct {
	Name        string
	Description string
}

type UpdateOrganizationRequest struct {
	Name        string
	Description string
	Group       *string
	Reason      string
}

type OrganizationQuotaAdjustmentRequest struct {
	QuotaDelta     int
	Reason         string
	IdempotencyKey string
}

type DissolveOrganizationRequest struct {
	ConfirmName    string
	Reason         string
	IdempotencyKey string
}

const organizationDissolveOperation = "organization_dissolve"

type UserOrganizationView struct {
	model.Organization
	Role          string                         `json:"role"`
	DisableState  OrganizationPolicyDisableState `json:"disable_state" gorm:"-"`
	CanSelfEnable bool                           `json:"can_self_enable" gorm:"-"`
	Capabilities  OrganizationActorCapabilities  `json:"capabilities" gorm:"-"`
	AccessMode    string                         `json:"access_mode" gorm:"-"`
}

func CreateOrganization(operatorUserId int, req CreateOrganizationRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*model.Organization, error) {
	name := strings.TrimSpace(req.Name)
	if operatorUserId <= 0 || name == "" {
		return nil, errors.New("invalid organization request")
	}
	normalized := model.NormalizeOrganizationName(name)

	organization := &model.Organization{}
	var err error
	for attempt := 0; attempt <= organizationSlugConflictRetries; attempt++ {
		now := common.GetTimestamp()
		organization = &model.Organization{}
		err = model.DB.Transaction(func(tx *gorm.DB) error {
			var creator model.User
			if err := model.LockForUpdate(tx).Select("id").Where("id = ?", operatorUserId).First(&creator).Error; err != nil {
				return err
			}
			count, err := countUserActiveOrganizationsWithTx(tx, operatorUserId)
			if err != nil {
				return err
			}
			if count >= maxActiveOrganizationsPerUser {
				return errors.New("organization limit exceeded")
			}
			if err := ensureOrganizationNameAvailableWithTx(tx, normalized, 0); err != nil {
				return err
			}
			slug, err := generateUniqueOrganizationSlug(tx, name)
			if err != nil {
				return err
			}
			organization = &model.Organization{
				Name:           name,
				NameNormalized: normalized,
				Slug:           slug,
				Description:    strings.TrimSpace(req.Description),
				Status:         model.OrganizationStatusActive,
				Quota:          model.OrganizationDefaultQuota,
				CreatedBy:      operatorUserId,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := tx.Create(organization).Error; err != nil {
				return err
			}
			member := &model.OrganizationMember{
				OrganizationId: organization.Id,
				UserId:         operatorUserId,
				Role:           model.OrganizationRoleOwner,
				Status:         model.OrganizationMemberStatusActive,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := tx.Create(member).Error; err != nil {
				return err
			}
			return recordOrganizationAudit(tx, organization, operatorUserId, model.OrganizationRoleOwner, organizationAuditActionCreate, "organization", organization.Id, nil, organization, "", auditMetadata...)
		})
		if err == nil {
			return organization, nil
		}
		// slug 冲突说明另一个并发请求刚占用了同一个 slug（同名时两者派生出的 slug 相同）。
		// 事务已回滚，重开一次就能重新生成带后缀的 slug；若那次其实同名，下一轮的名字校验
		// 会返回 ErrOrganizationNameConflict，落到下面的映射里。
		if !model.IsOrganizationSlugDuplicateError(err) {
			break
		}
	}
	return nil, mapOrganizationNameWriteError(err)
}

func ListUserOrganizations(userId int, includeDissolved bool) ([]UserOrganizationView, error) {
	var organizations []UserOrganizationView
	tx := model.DB.Table("organizations").
		Select("organizations.*, organization_members.role").
		Joins("JOIN organization_members ON organization_members.organization_id = organizations.id").
		Where("organization_members.user_id = ? AND organization_members.status = ?", userId, model.OrganizationMemberStatusActive)
	if !includeDissolved {
		tx = tx.Where("organizations.status <> ?", model.OrganizationStatusDissolved)
	}
	if err := tx.Order("organizations.id desc").Scan(&organizations).Error; err != nil {
		return nil, err
	}
	if len(organizations) == 0 {
		return organizations, nil
	}
	platformRole, err := getUserRoleWithTx(model.DB, userId)
	if err != nil {
		return nil, err
	}
	for i := range organizations {
		disableState := loadOrganizationPolicyDisableState(model.DB, organizations[i].Id)
		organizations[i].DisableState = disableState
		organizations[i].CanSelfEnable = disableState.CanSelfEnable
		accessMode := OrganizationAccessModeWorkspace
		if organizations[i].Status == model.OrganizationStatusDisabled {
			accessMode = OrganizationAccessModeManagement
		}
		input := OrganizationPolicyInput{
			CurrentUser:  OrganizationPolicyUser{Id: userId, PlatformRole: platformRole},
			Organization: organizationPolicyOrganizationFromModel(organizations[i].Organization),
			DisableState: disableState,
			Member:       &OrganizationPolicyMember{UserId: userId, Role: organizations[i].Role, Status: model.OrganizationMemberStatusActive},
			AccessMode:   accessMode,
		}
		decision := EvaluateOrganizationPolicy(input)
		organizations[i].AccessMode = decision.AccessMode
		if decision.Allowed {
			organizations[i].Capabilities = organizationActorCapabilitiesFromPolicy(decision)
		}
	}
	return organizations, nil
}

func GetOrganizationForMember(userId int, organizationId int, allowDissolvedRead bool) (*model.Organization, *model.OrganizationMember, error) {
	organization, member, err := getOrganizationMember(userId, organizationId)
	if err != nil {
		return nil, nil, err
	}
	if organization.Status == model.OrganizationStatusDisabled {
		return nil, nil, errors.New("organization disabled")
	}
	if !allowDissolvedRead && organization.Status == model.OrganizationStatusDissolved {
		return nil, nil, errors.New("organization dissolved")
	}
	return organization, member, nil
}

func UpdateOrganization(operatorUserId int, organizationId int, accessMode string, req UpdateOrganizationRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*model.Organization, error) {
	name, description, requestedGroup, err := validateOrganizationUpdateRequest(req)
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, errors.New("invalid organization request")
	}
	if err != nil {
		return nil, err
	}
	normalized := model.NormalizeOrganizationName(name)
	var updated *model.Organization
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityUpdateOrganization)
		if err != nil {
			return err
		}
		reason := strings.TrimSpace(req.Reason)
		if (decision.Role == organizationAuditOperatorRolePlatformAdmin || decision.Role == organizationAuditOperatorRolePlatformRoot) && reason == "" {
			return errors.New("organization update reason required")
		}
		if err := ensureOrganizationNameAvailableWithTx(tx, normalized, organizationId); err != nil {
			return err
		}
		before := *organization
		organization.Name = name
		organization.NameNormalized = normalized
		organization.Description = description
		selectFields := []string{"name", "name_normalized", "description", "updated_at"}
		if requestedGroup != nil && (decision.Role == organizationAuditOperatorRolePlatformAdmin || decision.Role == organizationAuditOperatorRolePlatformRoot) {
			organization.Group = *requestedGroup
			selectFields = append(selectFields, "group")
		}
		organization.UpdatedAt = common.GetTimestamp()
		if err := tx.Model(organization).Select(selectFields).Updates(organization).Error; err != nil {
			return err
		}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionUpdate, "organization", organization.Id, before, organization, reason, auditMetadata...); err != nil {
			return err
		}
		updated = organization
		return nil
	})
	return updated, mapOrganizationNameWriteError(err)
}

func ensureOrganizationNameAvailableWithTx(tx *gorm.DB, normalized string, excludeOrganizationId int) error {
	query := tx.Model(&model.Organization{}).Where("name_normalized = ?", normalized)
	if excludeOrganizationId > 0 {
		query = query.Where("id <> ?", excludeOrganizationId)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrOrganizationNameConflict
	}
	return nil
}

func mapOrganizationNameWriteError(err error) error {
	if model.IsOrganizationNameDuplicateError(err) {
		return ErrOrganizationNameConflict
	}
	return err
}

func DisableOrganization(operatorUserId int, organizationId int, accessMode string, confirmName string, reason string, auditMetadata ...OrganizationAuditRequestMetadata) error {
	return setOrganizationStatus(operatorUserId, organizationId, accessMode, model.OrganizationDisableSourceSelf, model.OrganizationStatusDisabled, confirmName, strings.TrimSpace(reason), auditMetadata...)
}

func EnableOrganization(operatorUserId int, organizationId int, accessMode string, confirmName string, reason string, auditMetadata ...OrganizationAuditRequestMetadata) error {
	return setOrganizationStatus(operatorUserId, organizationId, accessMode, model.OrganizationDisableSourceSelf, model.OrganizationStatusActive, confirmName, strings.TrimSpace(reason), auditMetadata...)
}

func DisableOrganizationByPlatform(operatorUserId int, organizationId int, accessMode string, confirmName string, reason string, auditMetadata ...OrganizationAuditRequestMetadata) error {
	return setOrganizationStatus(operatorUserId, organizationId, accessMode, model.OrganizationDisableSourcePlatform, model.OrganizationStatusDisabled, confirmName, strings.TrimSpace(reason), auditMetadata...)
}

func EnableOrganizationByPlatform(operatorUserId int, organizationId int, accessMode string, confirmName string, reason string, auditMetadata ...OrganizationAuditRequestMetadata) error {
	return setOrganizationStatus(operatorUserId, organizationId, accessMode, model.OrganizationDisableSourcePlatform, model.OrganizationStatusActive, confirmName, strings.TrimSpace(reason), auditMetadata...)
}

func setOrganizationStatus(operatorUserId int, organizationId int, accessMode string, source string, status string, confirmName string, reason string, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 {
		return errors.New("invalid organization request")
	}
	if status != model.OrganizationStatusActive && status != model.OrganizationStatusDisabled {
		return errors.New("invalid organization status")
	}
	if source != model.OrganizationDisableSourceSelf && source != model.OrganizationDisableSourcePlatform {
		return errors.New("invalid organization disable source")
	}
	if (source == model.OrganizationDisableSourceSelf && accessMode != OrganizationAccessModeManagement) ||
		(source == model.OrganizationDisableSourcePlatform && accessMode != OrganizationAccessModeAdmin) {
		return errors.New("permission denied")
	}
	now := common.GetTimestamp()
	var affectedCacheKeys []string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationStatusDecisionWithTx(tx, operatorUserId, organizationId, accessMode, status)
		if err != nil {
			return err
		}
		if strings.TrimSpace(confirmName) != organization.Slug {
			return errors.New("organization confirmation mismatch")
		}
		before := *organization
		action := organizationAuditActionEnable
		if status == model.OrganizationStatusDisabled {
			action = organizationAuditActionDisable
			record, _, err := ensureOrganizationDisableRecordWithTx(tx, organizationId, source, operatorUserId, reason, now)
			if err != nil {
				return err
			}
			if err := deriveOrganizationStatusFromDisableRecordsWithTx(tx, organization, now); err != nil {
				return err
			}
			if err := createOrganizationDisabledTokenBlockersWithTx(tx, organizationId, record, now); err != nil {
				return err
			}
			affectedCacheKeys, err = listOrganizationTokenKeysWithTx(tx, organizationId)
			if err != nil {
				return err
			}
		} else {
			clearSources := []string{source}
			if source == model.OrganizationDisableSourcePlatform {
				clearSources = append(clearSources, model.OrganizationDisableSourceSelf)
			}
			var clearedRecordIds []int
			for _, clearSource := range clearSources {
				ids, err := clearOrganizationDisableRecordsWithTx(tx, organizationId, clearSource, operatorUserId, reason, now)
				if err != nil {
					return err
				}
				clearedRecordIds = append(clearedRecordIds, ids...)
				if err := clearOrganizationTokenSystemBlockersWithTx(tx, organizationId, organizationTokenBlockerReasonForDisableSource(clearSource), organizationTokenBlockerRefTypeDisableRecord, ids, now); err != nil {
					return err
				}
			}
			if err := deriveOrganizationStatusFromDisableRecordsWithTx(tx, organization, now); err != nil {
				return err
			}
			if err := reconcileOrganizationTokenSystemBlockersWithTx(tx, organizationId, now); err != nil {
				return err
			}
			affectedCacheKeys, err = listOrganizationTokenKeysWithTx(tx, organizationId)
			if err != nil {
				return err
			}
			if len(clearedRecordIds) == 0 && before.Status == organization.Status {
				return nil
			}
		}
		return recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, action, "organization", organization.Id, before, organization, reason, auditMetadata...)
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(affectedCacheKeys...)
	return nil
}

func lockedOrganizationStatusDecisionWithTx(tx *gorm.DB, operatorUserId int, organizationId int, accessMode string, status string) (*model.Organization, OrganizationPolicyDecision, error) {
	requiredCapability := OrganizationCapabilityDisableOrganization
	if status == model.OrganizationStatusActive {
		requiredCapability = OrganizationCapabilityEnableOrganization
	}
	return lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, requiredCapability)
}

func ensureOrganizationDisableRecordWithTx(tx *gorm.DB, organizationId int, source string, operatorUserId int, reason string, now int64) (model.OrganizationDisableRecord, bool, error) {
	var record model.OrganizationDisableRecord
	err := model.LockForUpdate(tx).
		Where("organization_id = ? AND source = ? AND status = ?", organizationId, source, model.OrganizationRecordStatusActive).
		First(&record).Error
	if err == nil {
		return record, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return record, false, err
	}
	record = model.OrganizationDisableRecord{
		OrganizationId:   organizationId,
		Source:           source,
		Status:           model.OrganizationRecordStatusActive,
		DisabledByUserId: operatorUserId,
		DisabledReason:   reason,
		DisabledAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := tx.Create(&record).Error; err != nil {
		return record, false, err
	}
	return record, true, nil
}

func clearOrganizationDisableRecordsWithTx(tx *gorm.DB, organizationId int, source string, operatorUserId int, reason string, now int64) ([]int, error) {
	var records []model.OrganizationDisableRecord
	if err := model.LockForUpdate(tx).
		Where("organization_id = ? AND source = ? AND status = ?", organizationId, source, model.OrganizationRecordStatusActive).
		Find(&records).Error; err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	ids := make([]int, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.Id)
	}
	if err := tx.Model(&model.OrganizationDisableRecord{}).Where("id IN ?", ids).Updates(map[string]any{
		"status":             model.OrganizationRecordStatusCleared,
		"cleared_by_user_id": operatorUserId,
		"cleared_reason":     reason,
		"cleared_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func deriveOrganizationStatusFromDisableRecordsWithTx(tx *gorm.DB, organization *model.Organization, now int64) error {
	if organization.Status == model.OrganizationStatusDissolved {
		return nil
	}
	var activeRecordCount int64
	if err := tx.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ? AND status = ?", organization.Id, model.OrganizationRecordStatusActive).Count(&activeRecordCount).Error; err != nil {
		return err
	}
	status := model.OrganizationStatusActive
	if activeRecordCount > 0 {
		status = model.OrganizationStatusDisabled
	}
	if organization.Status == status {
		return nil
	}
	organization.Status = status
	organization.UpdatedAt = now
	return tx.Model(organization).Select("status", "updated_at").Updates(organization).Error
}

func DissolveOrganization(operatorUserId int, organizationId int, accessMode string, req DissolveOrganizationRequest, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 {
		return errors.New("invalid organization request")
	}
	idempotencyKey, err := requireOrganizationIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return err
	}
	confirmName := strings.TrimSpace(req.ConfirmName)
	reason := strings.TrimSpace(req.Reason)
	requestHash, err := organizationIdempotencyRequestHash(map[string]any{
		"operation_type":   organizationDissolveOperation,
		"operator_user_id": operatorUserId,
		"organization_id":  organizationId,
		"confirm_name":     confirmName,
		"reason":           reason,
	})
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	var affectedCacheKeys []string
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		organization, err := lockOrganizationForUpdateWithTx(tx, organizationId)
		if err != nil {
			return err
		}
		idempotency, err := prepareOrganizationIdempotencyWithTx(tx, operatorUserId, organizationId, organizationDissolveOperation, idempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if idempotency.Replayed {
			return nil
		}
		decision, err := organizationManagementDecisionForLockedOrganizationWithTx(tx, operatorUserId, organization, accessMode, OrganizationCapabilityDissolveOrganization)
		if err != nil {
			return err
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		if confirmName == "" || (confirmName != organization.Name && confirmName != organization.Slug) {
			return errors.New("organization confirmation mismatch")
		}
		blockers, err := organizationDissolveBlockersWithTx(tx, organizationId, idempotency.Record.Id)
		if err != nil {
			return err
		}
		if len(blockers) > 0 {
			return organizationOperationBlockedError("organization has active lifecycle blockers", blockers...)
		}
		before := *organization
		organization.Status = model.OrganizationStatusDissolved
		organization.UpdatedAt = now
		organization.DissolvedAt = now
		if err := tx.Model(organization).Select("status", "updated_at", "dissolved_at").Updates(organization).Error; err != nil {
			return err
		}
		if err := createOrganizationDissolvedTokenBlockersWithTx(tx, organizationId, operatorUserId, now); err != nil {
			return err
		}
		affectedCacheKeys, err = listOrganizationTokenKeysWithTx(tx, organizationId)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.OrganizationInvite{}).Where("organization_id = ? AND status = ?", organizationId, model.OrganizationInviteStatusPending).Updates(map[string]interface{}{"status": model.OrganizationInviteStatusRevoked, "revoked_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionDissolve, "organization", organization.Id, before, organization, reason, auditMetadata...); err != nil {
			return err
		}
		return completeOrganizationIdempotencyWithTx(tx, idempotency, map[string]any{"organization_id": organization.Id, "status": organization.Status})
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(affectedCacheKeys...)
	return nil
}

func organizationDissolveBlockersWithTx(tx *gorm.DB, organizationId, currentIdempotencyRecordId int) ([]string, error) {
	blockers := make([]string, 0, 4)
	var count int64
	if err := tx.Model(&model.Task{}).Where("organization_id = ? AND status IN ?", organizationId, []model.TaskStatus{model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued, model.TaskStatusInProgress}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		blockers = append(blockers, "active_tasks")
	}
	count = 0
	if err := tx.Model(&model.Midjourney{}).Where("organization_id = ? AND status IN ?", organizationId, []string{"NOT_START", "SUBMITTED", "QUEUED", "IN_PROGRESS"}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		blockers = append(blockers, "active_midjourney_tasks")
	}
	count = 0
	if err := tx.Model(&model.OrganizationBillingSession{}).Where("organization_id = ? AND status IN ?", organizationId, []string{model.OrganizationBillingSessionStatusPreConsumed, model.OrganizationBillingSessionStatusRepairing, model.OrganizationBillingSessionStatusFailed}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		blockers = append(blockers, "active_billing_sessions")
	}
	count = 0
	if err := tx.Model(&model.OrganizationIdempotencyRecord{}).
		Where("organization_id = ? AND status = ? AND id <> ? AND (expires_at = 0 OR expires_at > ?)", organizationId, model.OrganizationIdempotencyStatusProcessing, currentIdempotencyRecordId, common.GetTimestamp()).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		blockers = append(blockers, "processing_idempotency")
	}
	return blockers, nil
}

func AdjustOrganizationQuota(operatorUserId int, organizationId int, accessMode string, req OrganizationQuotaAdjustmentRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*model.OrganizationQuotaAdjustment, error) {
	if operatorUserId <= 0 || organizationId <= 0 || req.QuotaDelta == 0 {
		return nil, errors.New("invalid quota adjustment request")
	}
	now := common.GetTimestamp()
	idempotencyKey, err := requireOrganizationIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	var adjustment *model.OrganizationQuotaAdjustment
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(
			tx,
			operatorUserId,
			organizationId,
			accessMode,
			OrganizationCapabilityAdjustOrganizationQuota,
		)
		if err != nil {
			return err
		}
		operatorRole := decision.Role
		existingAdjustment, found, err := findReplayOrganizationQuotaAdjustmentWithTx(tx, operatorUserId, organizationId, req, idempotencyKey)
		if err != nil {
			return err
		}
		if found {
			adjustment = existingAdjustment
			return nil
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		before := *organization
		quotaAfter := organization.Quota + req.QuotaDelta
		if quotaAfter < organization.UsedQuota {
			return errors.New("quota cannot be less than used quota")
		}
		organization.Quota = quotaAfter
		organization.UpdatedAt = now
		if err := tx.Model(&organization).Select("quota", "updated_at").Updates(&organization).Error; err != nil {
			return err
		}
		adjustment = &model.OrganizationQuotaAdjustment{
			OrganizationId: organization.Id,
			OperatorUserId: operatorUserId,
			QuotaDelta:     req.QuotaDelta,
			QuotaBefore:    before.Quota,
			QuotaAfter:     quotaAfter,
			UsedQuota:      before.UsedQuota,
			Reason:         strings.TrimSpace(req.Reason),
			IdempotencyKey: idempotencyKey,
			CreatedAt:      now,
		}
		if err := tx.Create(adjustment).Error; err != nil {
			return err
		}
		if err := createOrganizationQuotaAdjustmentBillingRecordTx(tx, adjustment); err != nil {
			return err
		}
		return recordOrganizationAudit(tx, organization, operatorUserId, operatorRole, organizationAuditActionQuotaAdjust, "quota_adjustment", adjustment.Id, before, organization, adjustment.Reason, auditMetadata...)
	})
	return adjustment, err
}

func findReplayOrganizationQuotaAdjustmentWithTx(tx *gorm.DB, operatorUserId int, organizationId int, req OrganizationQuotaAdjustmentRequest, idempotencyKey string) (*model.OrganizationQuotaAdjustment, bool, error) {
	var existing model.OrganizationQuotaAdjustment
	err := model.LockForUpdate(tx).Where("idempotency_key = ?", idempotencyKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if existing.OrganizationId != organizationId ||
		existing.OperatorUserId != operatorUserId ||
		existing.QuotaDelta != req.QuotaDelta ||
		strings.TrimSpace(existing.Reason) != strings.TrimSpace(req.Reason) {
		return nil, true, errors.New("organization idempotency conflict")
	}
	return &existing, true, nil
}

func countUserActiveOrganizations(userId int) (int64, error) {
	return countUserActiveOrganizationsWithTx(model.DB, userId)
}

func countUserActiveOrganizationsWithTx(tx *gorm.DB, userId int) (int64, error) {
	var count int64
	err := tx.Model(&model.Organization{}).Where("created_by = ? AND status <> ?", userId, model.OrganizationStatusDissolved).
		Count(&count).Error
	return count, err
}

func getOrganizationMember(userId int, organizationId int) (*model.Organization, *model.OrganizationMember, error) {
	return getOrganizationMemberWithTx(model.DB, userId, organizationId)
}

func getOrganizationMemberWithTx(tx *gorm.DB, userId int, organizationId int) (*model.Organization, *model.OrganizationMember, error) {
	var organization model.Organization
	if err := tx.Where("id = ?", organizationId).First(&organization).Error; err != nil {
		return nil, nil, err
	}
	var member model.OrganizationMember
	if err := tx.Where("organization_id = ? AND user_id = ? AND status = ?", organizationId, userId, model.OrganizationMemberStatusActive).First(&member).Error; err != nil {
		return nil, nil, errors.New("permission denied")
	}
	return &organization, &member, nil
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9-]+`)

func generateUniqueOrganizationSlug(tx *gorm.DB, name string) (string, error) {
	base := strings.ToLower(strings.TrimSpace(name))
	base = nonSlugChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if len(base) < 3 {
		base = fmt.Sprintf("org-%d", common.GetTimestamp())
	}
	if len(base) > 48 {
		base = base[:48]
		base = strings.Trim(base, "-")
	}
	for i := 0; i < 20; i++ {
		slug := base
		if i > 0 {
			slug = fmt.Sprintf("%s-%d", base, common.GetTimestamp()+int64(i))
		}
		var count int64
		if err := tx.Model(&model.Organization{}).Where("slug = ?", slug).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
	}
	return "", errors.New("failed to generate organization slug")
}
