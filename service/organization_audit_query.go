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

	"github.com/QuantumNous/new-api/model"
)

type OrganizationAuditQueryRequest struct {
	Offset           int
	Limit            int
	OrganizationId   int
	OrganizationSlug string
	OperatorUserId   int
	TargetType       string
	TargetId         int
	ActionType       string
	StartTimestamp   int64
	EndTimestamp     int64
}

func ListOrganizationAuditLogs(operatorUserId, organizationId int, accessMode string, req OrganizationAuditQueryRequest) ([]*model.OrganizationAuditLog, int64, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, 0, err
	}
	if !actor.Capabilities.CanViewAudit {
		return nil, 0, errors.New("permission denied")
	}
	return queryOrganizationAuditLogs(req.withOrganizationId(organizationId))
}

func ListAllOrganizationAuditLogs(operatorUserId int, req OrganizationAuditQueryRequest) ([]*model.OrganizationAuditLog, int64, error) {
	if !model.IsAdmin(operatorUserId) {
		return nil, 0, errors.New("permission denied")
	}
	return queryOrganizationAuditLogs(req)
}

func (req OrganizationAuditQueryRequest) withOrganizationId(organizationId int) OrganizationAuditQueryRequest {
	req.OrganizationId = organizationId
	return req
}

func queryOrganizationAuditLogs(req OrganizationAuditQueryRequest) ([]*model.OrganizationAuditLog, int64, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	tx := model.DB.Model(&model.OrganizationAuditLog{})
	if req.OrganizationId > 0 {
		tx = tx.Where("organization_id = ?", req.OrganizationId)
	}
	if req.OrganizationSlug != "" {
		tx = tx.Where("organization_slug = ?", req.OrganizationSlug)
	}
	if req.OperatorUserId > 0 {
		tx = tx.Where("operator_user_id = ?", req.OperatorUserId)
	}
	if req.TargetType != "" {
		tx = tx.Where("target_type = ?", req.TargetType)
	}
	if req.TargetId > 0 {
		tx = tx.Where("target_id = ?", req.TargetId)
	}
	if req.ActionType != "" {
		tx = tx.Where("action_type = ?", req.ActionType)
	}
	if req.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", req.StartTimestamp)
	}
	if req.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", req.EndTimestamp)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []*model.OrganizationAuditLog
	if err := tx.Order("id desc").Limit(req.Limit).Offset(req.Offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
