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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

const (
	organizationTokenBlockerReasonManualDisabled        = "manual_disabled"
	organizationTokenBlockerReasonOrganizationSelf      = "organization_self_disabled"
	organizationTokenBlockerReasonOrganizationPlatform  = "organization_platform_disabled"
	organizationTokenBlockerReasonOrganizationDisabled  = "organization_disabled"
	organizationTokenBlockerReasonMemberDisabled        = "member_disabled"
	organizationTokenBlockerReasonUserPlatformDisabled  = "user_platform_disabled"
	organizationTokenBlockerReasonOrganizationDissolved = "organization_dissolved"

	organizationTokenBlockerRefTypeDisableRecord = "organization_disable_record"
	organizationTokenBlockerRefTypeMember        = "organization_member"
	organizationTokenBlockerRefTypeOrganization  = "organization"
	organizationTokenBlockerRefTypeOperator      = "operator_user"
	organizationTokenBlockerRefTypeUser          = "user"
)

const (
	organizationTokenBlockerClearanceMember            = 10
	organizationTokenBlockerClearanceOrganizationAdmin = 50
	organizationTokenBlockerClearancePlatform          = 80
	organizationTokenBlockerClearanceTerminal          = 100
)

func createOrganizationDisabledTokenBlockersWithTx(tx *gorm.DB, organizationId int, record model.OrganizationDisableRecord, now int64) error {
	var tokens []model.Token
	if err := tx.Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId).Find(&tokens).Error; err != nil {
		return err
	}
	reason := organizationTokenBlockerReasonForDisableSource(record.Source)
	clearance := organizationTokenBlockerClearanceForDisableSource(record.Source)
	for i := range tokens {
		if err := ensureOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], reason, organizationTokenBlockerRefTypeDisableRecord, record.Id, record.DisabledByUserId, clearance, now); err != nil {
			return err
		}
	}
	return reconcileOrganizationTokenSystemBlockersWithTx(tx, organizationId, now)
}

func createOrganizationDissolvedTokenBlockersWithTx(tx *gorm.DB, organizationId int, operatorUserId int, now int64) error {
	var tokens []model.Token
	if err := tx.Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId).Find(&tokens).Error; err != nil {
		return err
	}
	for i := range tokens {
		if err := ensureOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], organizationTokenBlockerReasonOrganizationDissolved, organizationTokenBlockerRefTypeOrganization, organizationId, operatorUserId, organizationTokenBlockerClearanceTerminal, now); err != nil {
			return err
		}
	}
	return reconcileOrganizationTokenSystemBlockersWithTx(tx, organizationId, now)
}

func createOrganizationMemberDisabledTokenBlockersWithTx(tx *gorm.DB, organizationId int, userId int, operatorUserId int, disableSource string, now int64) ([]string, error) {
	var tokens []model.Token
	if err := model.LockForUpdate(tx).
		Where("scope_type = ? AND organization_id = ? AND (user_id = ? OR responsible_user_id = ?)", model.TokenScopeOrganization, organizationId, userId, userId).
		Order("id asc").
		Find(&tokens).Error; err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(tokens))
	clearance := organizationTokenBlockerClearanceForMemberDisableSource(disableSource)
	for i := range tokens {
		if err := ensureOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], organizationTokenBlockerReasonMemberDisabled, organizationTokenBlockerRefTypeMember, userId, operatorUserId, clearance, now); err != nil {
			return nil, err
		}
		if err := reconcileOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], now); err != nil {
			return nil, err
		}
		keys = append(keys, tokens[i].Key)
	}
	return keys, nil
}

func clearOrganizationMemberDisabledTokenBlockersWithTx(tx *gorm.DB, organizationId int, userId int, now int64) ([]string, error) {
	var tokenIds []int
	if err := tx.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("organization_id = ? AND reason = ? AND ref_type = ? AND ref_id = ? AND status = ?", organizationId, organizationTokenBlockerReasonMemberDisabled, organizationTokenBlockerRefTypeMember, userId, model.OrganizationTokenBlockerStatusActive).
		Distinct("token_id").
		Order("token_id asc").
		Pluck("token_id", &tokenIds).Error; err != nil {
		return nil, err
	}
	if len(tokenIds) == 0 {
		return nil, nil
	}
	var tokens []model.Token
	if err := model.LockForUpdate(tx).
		Where("scope_type = ? AND organization_id = ? AND id IN ?", model.TokenScopeOrganization, organizationId, tokenIds).
		Order("id asc").
		Find(&tokens).Error; err != nil {
		return nil, err
	}
	if err := clearOrganizationTokenSystemBlockersWithTx(tx, organizationId, organizationTokenBlockerReasonMemberDisabled, organizationTokenBlockerRefTypeMember, []int{userId}, now); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(tokens))
	for i := range tokens {
		if err := reconcileOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], now); err != nil {
			return nil, err
		}
		keys = append(keys, tokens[i].Key)
	}
	return keys, nil
}

func UpdateOrganizationTokenBlockersForUserStatus(userId int, status int, operatorUserId int) error {
	if userId <= 0 {
		return errors.New("invalid user")
	}
	now := common.GetTimestamp()
	var affectedKeys []string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		affectedKeys, err = syncOrganizationTokenBlockersForUserStatusWithTx(tx, userId, status, operatorUserId, now)
		return err
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(affectedKeys...)
	return nil
}

func UpdateUserStatusAndOrganizationTokenBlockers(userId int, status int, operatorUserId int) error {
	if userId <= 0 {
		return errors.New("invalid user")
	}
	now := common.GetTimestamp()
	var affectedKeys []string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if status != common.UserStatusDisabled && status != common.UserStatusEnabled {
			return errors.New("invalid user status")
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userId).Update("status", status).Error; err != nil {
			return err
		}
		var err error
		affectedKeys, err = syncOrganizationTokenBlockersForUserStatusWithTx(tx, userId, status, operatorUserId, now)
		return err
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(affectedKeys...)
	return model.UpdateUserStatusCache(userId, status)
}

func syncOrganizationTokenBlockersForUserStatusWithTx(tx *gorm.DB, userId int, status int, operatorUserId int, now int64) ([]string, error) {
	affectedKeys, err := listOrganizationTokenKeysForResponsibleUserWithTx(tx, userId)
	if err != nil {
		return nil, err
	}
	switch status {
	case common.UserStatusDisabled:
		return affectedKeys, createOrganizationUserPlatformDisabledTokenBlockersWithTx(tx, userId, operatorUserId, now)
	case common.UserStatusEnabled:
		return affectedKeys, clearOrganizationUserPlatformDisabledTokenBlockersWithTx(tx, userId, now)
	default:
		return nil, errors.New("invalid user status")
	}
}

func ReconcileOrganizationTokenBlockers() error {
	now := common.GetTimestamp()
	var affectedKeys []string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		affectedKeys, err = listAllOrganizationTokenKeysWithTx(tx)
		if err != nil {
			return err
		}
		var records []model.OrganizationDisableRecord
		if err := tx.Where("status = ?", model.OrganizationRecordStatusActive).Find(&records).Error; err != nil {
			return err
		}
		for _, record := range records {
			if err := createOrganizationDisabledTokenBlockersWithTx(tx, record.OrganizationId, record, now); err != nil {
				return err
			}
		}
		var organizations []model.Organization
		if err := tx.Where("status = ?", model.OrganizationStatusDissolved).Find(&organizations).Error; err != nil {
			return err
		}
		for _, organization := range organizations {
			if err := createOrganizationDissolvedTokenBlockersWithTx(tx, organization.Id, 0, now); err != nil {
				return err
			}
		}
		var members []model.OrganizationMember
		if err := tx.Where("status = ?", model.OrganizationMemberStatusDisabled).Find(&members).Error; err != nil {
			return err
		}
		for _, member := range members {
			if _, err := createOrganizationMemberDisabledTokenBlockersWithTx(tx, member.OrganizationId, member.UserId, 0, member.DisabledSource, now); err != nil {
				return err
			}
		}
		var disabledUserIds []int
		if err := tx.Model(&model.User{}).Where("status = ?", common.UserStatusDisabled).Pluck("id", &disabledUserIds).Error; err != nil {
			return err
		}
		for _, userId := range disabledUserIds {
			if err := createOrganizationUserPlatformDisabledTokenBlockersWithTx(tx, userId, 0, now); err != nil {
				return err
			}
		}
		if err := clearStaleOrganizationTokenSystemBlockersWithTx(tx, now); err != nil {
			return err
		}
		return reconcileAllOrganizationTokenSystemBlockersWithTx(tx, now)
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(affectedKeys...)
	return nil
}

func createOrganizationUserPlatformDisabledTokenBlockersWithTx(tx *gorm.DB, userId int, operatorUserId int, now int64) error {
	var tokens []model.Token
	if err := model.LockForUpdate(tx).
		Where("scope_type = ? AND (responsible_user_id = ? OR (responsible_user_id = 0 AND user_id = ?))", model.TokenScopeOrganization, userId, userId).
		Order("id asc").
		Find(&tokens).Error; err != nil {
		return err
	}
	for i := range tokens {
		if err := ensureOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], organizationTokenBlockerReasonUserPlatformDisabled, organizationTokenBlockerRefTypeUser, userId, operatorUserId, organizationTokenBlockerClearancePlatform, now); err != nil {
			return err
		}
		if err := reconcileOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], now); err != nil {
			return err
		}
	}
	return nil
}

func clearOrganizationUserPlatformDisabledTokenBlockersWithTx(tx *gorm.DB, userId int, now int64) error {
	organizationIds, err := activeOrganizationTokenBlockerOrganizationIdsWithTx(tx, organizationTokenBlockerReasonUserPlatformDisabled, organizationTokenBlockerRefTypeUser, userId)
	if err != nil {
		return err
	}
	if len(organizationIds) == 0 {
		return nil
	}
	if err := tx.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("reason = ? AND ref_type = ? AND ref_id = ? AND status = ?", organizationTokenBlockerReasonUserPlatformDisabled, organizationTokenBlockerRefTypeUser, userId, model.OrganizationTokenBlockerStatusActive).
		Updates(map[string]any{"status": model.OrganizationTokenBlockerStatusCleared, "cleared_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	return reconcileOrganizationTokenSystemBlockersForOrganizationsWithTx(tx, organizationIds, now)
}

func ensureOrganizationTokenSystemBlockerWithTx(tx *gorm.DB, token *model.Token, reason string, refType string, refId int, operatorUserId int, clearanceLevel int, now int64) error {
	var existing model.OrganizationTokenSystemBlocker
	err := tx.Where("token_id = ? AND organization_id = ? AND reason = ? AND ref_type = ? AND ref_id = ? AND status = ?",
		token.Id, token.OrganizationId, reason, refType, refId, model.OrganizationTokenBlockerStatusActive).First(&existing).Error
	if err == nil {
		if clearanceLevel > existing.ClearanceLevel {
			return tx.Model(&existing).Updates(map[string]any{
				"operator_user_id": operatorUserId,
				"clearance_level":  clearanceLevel,
				"updated_at":       now,
			}).Error
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	previousStatus, err := organizationTokenPreviousStatusForNewBlockerWithTx(tx, token)
	if err != nil {
		return err
	}
	blocker := model.OrganizationTokenSystemBlocker{
		TokenId:        token.Id,
		OrganizationId: token.OrganizationId,
		Reason:         reason,
		RefType:        refType,
		RefId:          refId,
		Status:         model.OrganizationTokenBlockerStatusActive,
		PreviousStatus: previousStatus,
		OperatorUserId: operatorUserId,
		ClearanceLevel: clearanceLevel,
		DisabledAt:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return tx.Create(&blocker).Error
}

func organizationTokenPreviousStatusForNewBlockerWithTx(tx *gorm.DB, token *model.Token) (int, error) {
	var blockers []model.OrganizationTokenSystemBlocker
	if err := tx.Where("token_id = ? AND status = ?", token.Id, model.OrganizationTokenBlockerStatusActive).Order("id asc").Find(&blockers).Error; err != nil {
		return 0, err
	}
	for _, blocker := range blockers {
		if blocker.PreviousStatus > 0 {
			return blocker.PreviousStatus, nil
		}
	}
	if token.PreviousStatus > 0 {
		return token.PreviousStatus, nil
	}
	return token.Status, nil
}

func clearOrganizationTokenSystemBlockersWithTx(tx *gorm.DB, organizationId int, reason string, refType string, refIds []int, now int64) error {
	if len(refIds) == 0 {
		return nil
	}
	return tx.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("organization_id = ? AND reason = ? AND ref_type = ? AND ref_id IN ? AND status = ?", organizationId, reason, refType, refIds, model.OrganizationTokenBlockerStatusActive).
		Updates(map[string]any{"status": model.OrganizationTokenBlockerStatusCleared, "cleared_at": now, "updated_at": now}).Error
}

func clearStaleOrganizationTokenSystemBlockersWithTx(tx *gorm.DB, now int64) error {
	var blockers []model.OrganizationTokenSystemBlocker
	staleCheckReasons := []string{
		organizationTokenBlockerReasonOrganizationSelf,
		organizationTokenBlockerReasonOrganizationPlatform,
		organizationTokenBlockerReasonOrganizationDisabled,
		organizationTokenBlockerReasonMemberDisabled,
		organizationTokenBlockerReasonUserPlatformDisabled,
	}
	if err := tx.Where("status = ? AND reason IN ?", model.OrganizationTokenBlockerStatusActive, staleCheckReasons).Find(&blockers).Error; err != nil {
		return err
	}
	staleIds := make([]int, 0)
	for _, blocker := range blockers {
		active, err := organizationTokenBlockerSourceStillActiveWithTx(tx, blocker)
		if err != nil {
			return err
		}
		if !active {
			staleIds = append(staleIds, blocker.Id)
		}
	}
	if len(staleIds) == 0 {
		return nil
	}
	return tx.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("id IN ?", staleIds).
		Updates(map[string]any{"status": model.OrganizationTokenBlockerStatusCleared, "cleared_at": now, "updated_at": now}).Error
}

func organizationTokenBlockerSourceStillActiveWithTx(tx *gorm.DB, blocker model.OrganizationTokenSystemBlocker) (bool, error) {
	var count int64
	switch blocker.Reason {
	case organizationTokenBlockerReasonMemberDisabled:
		matches, err := organizationTokenBlockerMatchesCurrentResponsibilityWithTx(tx, blocker)
		if err != nil || !matches {
			return matches, err
		}
		if err := tx.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ? AND status = ?", blocker.OrganizationId, blocker.RefId, model.OrganizationMemberStatusDisabled).Count(&count).Error; err != nil {
			return false, err
		}
	case organizationTokenBlockerReasonUserPlatformDisabled:
		matches, err := organizationTokenBlockerMatchesCurrentResponsibilityWithTx(tx, blocker)
		if err != nil || !matches {
			return matches, err
		}
		if err := tx.Model(&model.User{}).Where("id = ? AND status = ?", blocker.RefId, common.UserStatusDisabled).Count(&count).Error; err != nil {
			return false, err
		}
	case organizationTokenBlockerReasonOrganizationSelf, organizationTokenBlockerReasonOrganizationPlatform:
		if blocker.RefId <= 0 {
			return false, nil
		}
		if err := tx.Model(&model.OrganizationDisableRecord{}).Where("id = ? AND organization_id = ? AND status = ?", blocker.RefId, blocker.OrganizationId, model.OrganizationRecordStatusActive).Count(&count).Error; err != nil {
			return false, err
		}
	case organizationTokenBlockerReasonOrganizationDisabled:
		if err := tx.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ? AND status = ?", blocker.OrganizationId, model.OrganizationRecordStatusActive).Count(&count).Error; err != nil {
			return false, err
		}
	default:
		return true, nil
	}
	return count > 0, nil
}

func organizationTokenBlockerMatchesCurrentResponsibilityWithTx(tx *gorm.DB, blocker model.OrganizationTokenSystemBlocker) (bool, error) {
	var token model.Token
	err := model.LockForUpdate(tx).
		Select("id", "user_id", "responsible_user_id").
		Where("id = ? AND scope_type = ? AND organization_id = ?", blocker.TokenId, model.TokenScopeOrganization, blocker.OrganizationId).
		First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	responsibleUserId := token.ResponsibleUserId
	if responsibleUserId == 0 {
		responsibleUserId = token.UserId
	}
	return responsibleUserId == blocker.RefId, nil
}

func reconcileOrganizationTokenSystemBlockersWithTx(tx *gorm.DB, organizationId int, now int64) error {
	var tokens []model.Token
	if err := tx.Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId).Find(&tokens).Error; err != nil {
		return err
	}
	for i := range tokens {
		if err := reconcileOrganizationTokenSystemBlockerWithTx(tx, &tokens[i], now); err != nil {
			return err
		}
	}
	return nil
}

func reconcileAllOrganizationTokenSystemBlockersWithTx(tx *gorm.DB, now int64) error {
	var organizationIds []int
	if err := tx.Model(&model.Token{}).Where("scope_type = ? AND organization_id > 0", model.TokenScopeOrganization).Distinct("organization_id").Pluck("organization_id", &organizationIds).Error; err != nil {
		return err
	}
	organizations := map[int]bool{}
	for _, organizationId := range organizationIds {
		organizations[organizationId] = true
	}
	return reconcileOrganizationTokenSystemBlockersForOrganizationsWithTx(tx, organizations, now)
}

func reconcileOrganizationTokenSystemBlockersForOrganizationsWithTx(tx *gorm.DB, organizationIds map[int]bool, now int64) error {
	for organizationId := range organizationIds {
		if organizationId <= 0 {
			continue
		}
		if err := reconcileOrganizationTokenSystemBlockersWithTx(tx, organizationId, now); err != nil {
			return err
		}
	}
	return nil
}

func reconcileOrganizationTokenSystemBlockerWithTx(tx *gorm.DB, token *model.Token, now int64) error {
	var blockers []model.OrganizationTokenSystemBlocker
	if err := tx.Where("token_id = ? AND status = ?", token.Id, model.OrganizationTokenBlockerStatusActive).Order("id asc").Find(&blockers).Error; err != nil {
		return err
	}
	activeBlockers := blockers[:0]
	staleIds := make([]int, 0)
	for _, blocker := range blockers {
		active, err := organizationTokenBlockerSourceStillActiveWithTx(tx, blocker)
		if err != nil {
			return err
		}
		if active {
			activeBlockers = append(activeBlockers, blocker)
			continue
		}
		staleIds = append(staleIds, blocker.Id)
	}
	if len(staleIds) > 0 {
		if err := tx.Model(&model.OrganizationTokenSystemBlocker{}).
			Where("id IN ?", staleIds).
			Updates(map[string]any{"status": model.OrganizationTokenBlockerStatusCleared, "cleared_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
	}
	blockers = activeBlockers
	if len(blockers) == 0 {
		restoredStatus := token.Status
		if token.PreviousStatus > 0 {
			restoredStatus = token.PreviousStatus
		}
		if token.Status == restoredStatus && !token.DisabledBySystems && token.SystemDisabledReason == "" && token.SystemDisabledRefId == 0 && token.SystemDisabledAt == 0 && token.PreviousStatus == 0 {
			return nil
		}
		return tx.Model(token).Select("status", "disabled_by_systems", "system_disabled_reason", "system_disabled_ref_id", "system_disabled_at", "previous_status", "updated_at").
			Updates(map[string]any{"status": restoredStatus, "disabled_by_systems": false, "system_disabled_reason": "", "system_disabled_ref_id": 0, "system_disabled_at": 0, "previous_status": 0, "updated_at": now}).Error
	}
	previousStatus := blockers[0].PreviousStatus
	if previousStatus == 0 {
		previousStatus = token.Status
	}
	blocker := selectedOrganizationTokenSystemBlocker(blockers)
	disabledBySystems := organizationTokenBlockerReasonIsSystem(blocker.Reason)
	reason := ""
	refId := 0
	disabledAt := int64(0)
	if disabledBySystems {
		reason = blocker.Reason
		refId = blocker.RefId
		disabledAt = blocker.DisabledAt
	}
	return tx.Model(token).Select("status", "disabled_by_systems", "system_disabled_reason", "system_disabled_ref_id", "system_disabled_at", "previous_status", "updated_at").
		Updates(map[string]any{"status": common.TokenStatusDisabled, "disabled_by_systems": disabledBySystems, "system_disabled_reason": reason, "system_disabled_ref_id": refId, "system_disabled_at": disabledAt, "previous_status": previousStatus, "updated_at": now}).Error
}

func organizationTokenBlockerReasonForDisableSource(source string) string {
	if source == model.OrganizationDisableSourcePlatform {
		return organizationTokenBlockerReasonOrganizationPlatform
	}
	return organizationTokenBlockerReasonOrganizationSelf
}

func organizationTokenBlockerClearanceForDisableSource(source string) int {
	if source == model.OrganizationDisableSourcePlatform {
		return organizationTokenBlockerClearancePlatform
	}
	return organizationTokenBlockerClearanceOrganizationAdmin
}

func organizationTokenBlockerClearanceForMemberDisableSource(source string) int {
	if source == model.OrganizationMemberDisableSourcePlatform {
		return organizationTokenBlockerClearancePlatform
	}
	return organizationTokenBlockerClearanceOrganizationAdmin
}

func selectedOrganizationTokenSystemBlocker(blockers []model.OrganizationTokenSystemBlocker) model.OrganizationTokenSystemBlocker {
	if len(blockers) == 0 {
		return model.OrganizationTokenSystemBlocker{}
	}
	selected := blockers[0]
	selectedPriority := organizationTokenBlockerReasonPriority(selected.Reason)
	for _, blocker := range blockers[1:] {
		priority := organizationTokenBlockerReasonPriority(blocker.Reason)
		if priority > selectedPriority {
			selected = blocker
			selectedPriority = priority
		}
	}
	return selected
}

func organizationTokenBlockerReasonPriority(reason string) int {
	switch reason {
	case organizationTokenBlockerReasonOrganizationDissolved:
		return 100
	case organizationTokenBlockerReasonOrganizationPlatform, organizationTokenBlockerReasonUserPlatformDisabled:
		return 80
	case organizationTokenBlockerReasonOrganizationSelf, organizationTokenBlockerReasonOrganizationDisabled:
		return 60
	case organizationTokenBlockerReasonMemberDisabled:
		return 40
	case organizationTokenBlockerReasonManualDisabled:
		return 20
	default:
		return 0
	}
}

func activeOrganizationTokenBlockerOrganizationIdsWithTx(tx *gorm.DB, reason string, refType string, refId int) (map[int]bool, error) {
	var organizationIds []int
	if err := tx.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("reason = ? AND ref_type = ? AND ref_id = ? AND status = ?", reason, refType, refId, model.OrganizationTokenBlockerStatusActive).
		Distinct("organization_id").
		Pluck("organization_id", &organizationIds).Error; err != nil {
		return nil, err
	}
	result := map[int]bool{}
	for _, organizationId := range organizationIds {
		result[organizationId] = true
	}
	return result, nil
}

func organizationTokenBlockerReasonIsSystem(reason string) bool {
	return reason != "" && reason != organizationTokenBlockerReasonManualDisabled
}

func listOrganizationTokenKeysWithTx(tx *gorm.DB, organizationId int) ([]string, error) {
	var keys []string
	err := tx.Model(&model.Token{}).
		Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId).
		Pluck("key", &keys).Error
	return keys, err
}

func listOrganizationTokenKeysForResponsibleUserWithTx(tx *gorm.DB, userId int) ([]string, error) {
	var keys []string
	err := tx.Model(&model.Token{}).
		Where("scope_type = ? AND (responsible_user_id = ? OR (responsible_user_id = 0 AND user_id = ?))", model.TokenScopeOrganization, userId, userId).
		Pluck("key", &keys).Error
	return keys, err
}

func listAllOrganizationTokenKeysWithTx(tx *gorm.DB) ([]string, error) {
	var keys []string
	err := tx.Model(&model.Token{}).
		Where("scope_type = ?", model.TokenScopeOrganization).
		Pluck("key", &keys).Error
	return keys, err
}
