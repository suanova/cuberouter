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
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type OrganizationTaskListRequest struct {
	Offset         int
	Limit          int
	Platform       constant.TaskPlatform
	TaskID         string
	Action         string
	Status         string
	StartTimestamp int64
	EndTimestamp   int64
}

type OrganizationMidjourneyTaskListRequest struct {
	Offset         int
	Limit          int
	ChannelID      string
	MjID           string
	StartTimestamp string
	EndTimestamp   string
}

func ListOrganizationTasks(operatorUserId, organizationId int, accessMode string, req OrganizationTaskListRequest) ([]*model.Task, int64, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, 0, err
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	tx := model.DB.Model(&model.Task{}).
		Where("billing_account_type = ? AND billing_account_id = ? AND organization_id = ?", model.AccountContextTypeOrganization, organizationId, organizationId)
	if !actor.Capabilities.CanViewOrganizationWideData {
		tx = tx.Where("responsible_user_id = ?", operatorUserId)
	}
	if req.Platform != "" {
		tx = tx.Where("platform = ?", req.Platform)
	}
	if req.TaskID != "" {
		tx = tx.Where("task_id = ?", req.TaskID)
	}
	if req.Action != "" {
		tx = tx.Where("action = ?", req.Action)
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if req.StartTimestamp > 0 {
		tx = tx.Where("submit_time >= ?", req.StartTimestamp)
	}
	if req.EndTimestamp > 0 {
		tx = tx.Where("submit_time <= ?", req.EndTimestamp)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tasks []*model.Task
	if err := tx.Order("id desc").Limit(req.Limit).Offset(req.Offset).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	for _, task := range tasks {
		model.NormalizeTaskBillingScope(task)
	}
	return tasks, total, nil
}

func ListOrganizationMidjourneyTasks(operatorUserId, organizationId int, accessMode string, req OrganizationMidjourneyTaskListRequest) ([]*model.Midjourney, int64, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, 0, err
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	query := model.DB.Model(&model.Midjourney{}).
		Where("billing_account_type = ? AND billing_account_id = ? AND organization_id = ?", model.AccountContextTypeOrganization, organizationId, organizationId)
	if !actor.Capabilities.CanViewOrganizationWideData {
		query = query.Where("responsible_user_id = ?", operatorUserId)
	}
	if req.ChannelID != "" {
		query = query.Where("channel_id = ?", req.ChannelID)
	}
	if req.MjID != "" {
		query = query.Where("mj_id = ?", req.MjID)
	}
	if req.StartTimestamp != "" {
		query = query.Where("submit_time >= ?", req.StartTimestamp)
	}
	if req.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", req.EndTimestamp)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tasks []*model.Midjourney
	if err := query.Order("id desc").Limit(req.Limit).Offset(req.Offset).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	for _, task := range tasks {
		model.NormalizeMidjourneyBillingScope(task)
	}
	return tasks, total, nil
}
