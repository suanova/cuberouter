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

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type organizationIdempotencyState struct {
	Record   *model.OrganizationIdempotencyRecord
	Replayed bool
}

func requireOrganizationIdempotencyKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", errors.New("organization idempotency key is required")
	}
	return key, nil
}

func organizationIdempotencyRequestHash(payload any) (string, error) {
	data, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	return common.GenerateHMAC(string(data)), nil
}

func prepareOrganizationIdempotencyWithTx(tx *gorm.DB, operatorUserId, organizationId int, operationType, idempotencyKey, requestHash string) (*organizationIdempotencyState, error) {
	var existing model.OrganizationIdempotencyRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("idempotency_key = ?", idempotencyKey).First(&existing).Error
	if err == nil {
		if existing.OrganizationId != organizationId || existing.CreatedBy != operatorUserId || existing.OperationType != operationType || existing.RequestHash != requestHash {
			return nil, errors.New("organization idempotency conflict")
		}
		if existing.Status != model.OrganizationIdempotencyStatusSucceeded {
			return nil, errors.New("organization idempotency conflict")
		}
		return &organizationIdempotencyState{Record: &existing, Replayed: true}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := common.GetTimestamp()
	record := &model.OrganizationIdempotencyRecord{
		OrganizationId: organizationId,
		OperationType:  operationType,
		IdempotencyKey: idempotencyKey,
		RequestHash:    requestHash,
		Status:         model.OrganizationIdempotencyStatusProcessing,
		CreatedBy:      operatorUserId,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := tx.Create(record).Error; err != nil {
		return nil, err
	}
	return &organizationIdempotencyState{Record: record}, nil
}

func completeOrganizationIdempotencyWithTx(tx *gorm.DB, state *organizationIdempotencyState, result any) error {
	if state == nil || state.Record == nil || state.Replayed {
		return nil
	}
	resultJson, err := common.Marshal(result)
	if err != nil {
		return err
	}
	state.Record.Status = model.OrganizationIdempotencyStatusSucceeded
	state.Record.ResultJson = string(resultJson)
	state.Record.UpdatedAt = common.GetTimestamp()
	return tx.Model(state.Record).Select("status", "result_json", "updated_at").Updates(state.Record).Error
}
