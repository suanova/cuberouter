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
	"github.com/QuantumNous/new-api/common"
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
		task.OriginalQuota = organizationTaskOriginalQuota(task)
	}
	if err := HydrateOrganizationTaskNames(tasks); err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// organizationTaskOriginalQuota reverses the effective group ratio the task was
// charged with. The ratio already folds in the user-model discount, so the
// result is the list price the task would have cost without either.
func organizationTaskOriginalQuota(task *model.Task) int {
	if task == nil {
		return 0
	}
	if task.Quota <= 0 {
		return task.Quota
	}
	context := task.PrivateData.BillingContext
	if context == nil || context.GroupRatio <= 0 {
		return task.Quota
	}
	return common.QuotaRound(float64(task.Quota) / context.GroupRatio)
}

// HydrateOrganizationTaskNames fills the display names for rows written before
// the task stored them. The stored snapshot wins when present; a deleted token
// or user is still resolved through Unscoped so an old row keeps a readable
// name instead of falling back to an id.
func HydrateOrganizationTaskNames(tasks []*model.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	tokenIds := make([]int, 0)
	userIds := make([]int, 0)
	seenTokens := map[int]struct{}{}
	seenUsers := map[int]struct{}{}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.TokenName == "" && task.TokenId > 0 {
			if _, ok := seenTokens[task.TokenId]; !ok {
				seenTokens[task.TokenId] = struct{}{}
				tokenIds = append(tokenIds, task.TokenId)
			}
		}
		if task.ResponsibleName == "" && task.ResponsibleUserId > 0 {
			if _, ok := seenUsers[task.ResponsibleUserId]; !ok {
				seenUsers[task.ResponsibleUserId] = struct{}{}
				userIds = append(userIds, task.ResponsibleUserId)
			}
		}
		if task.CreatorName == "" && task.CreatorUserId > 0 {
			if _, ok := seenUsers[task.CreatorUserId]; !ok {
				seenUsers[task.CreatorUserId] = struct{}{}
				userIds = append(userIds, task.CreatorUserId)
			}
		}
	}

	tokenNames := make(map[int]string, len(tokenIds))
	if len(tokenIds) > 0 {
		var tokens []model.Token
		if err := model.DB.Unscoped().Select("id", "name").Where("id IN ?", tokenIds).Find(&tokens).Error; err != nil {
			return err
		}
		for _, token := range tokens {
			if token.Name != "" {
				tokenNames[token.Id] = token.Name
			}
		}
	}

	userNames := make(map[int]string, len(userIds))
	if len(userIds) > 0 {
		var users []model.User
		if err := model.DB.Unscoped().Select("id", "username", "display_name").Where("id IN ?", userIds).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			name := user.DisplayName
			if name == "" {
				name = user.Username
			}
			if name != "" {
				userNames[user.Id] = name
			}
		}
	}

	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.TokenName == "" {
			task.TokenName = tokenNames[task.TokenId]
		}
		if task.ResponsibleName == "" {
			task.ResponsibleName = userNames[task.ResponsibleUserId]
		}
		if task.CreatorName == "" {
			task.CreatorName = userNames[task.CreatorUserId]
		}
	}
	return nil
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
