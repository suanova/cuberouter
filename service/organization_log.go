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
	"time"

	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

type OrganizationLogListRequest struct {
	Offset            int
	Limit             int
	ResponsibleUserId int
	TokenId           int
	Type              int
	TokenName         string
	ResponsibleName   string
	ModelName         string
	Group             string
	RequestId         string
	StartTimestamp    int64
	EndTimestamp      int64
}

type OrganizationLogStats struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func ListOrganizationLogs(operatorUserId, organizationId int, accessMode string, req OrganizationLogListRequest) ([]*model.Log, int64, error) {
	_, tx, err := buildOrganizationLogQuery(operatorUserId, organizationId, accessMode, req, true)
	if err != nil {
		return nil, 0, err
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []*model.Log
	if err := tx.Order("id desc").Limit(req.Limit).Offset(req.Offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	for _, log := range logs {
		log.Content = ""
		model.NormalizeLogScope(log)
	}
	if err := hydrateOrganizationLogResponsibleUsers(logs); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func hydrateOrganizationLogResponsibleUsers(logs []*model.Log) error {
	if len(logs) == 0 {
		return nil
	}
	responsibleUserIds := make([]int, 0, len(logs))
	seen := map[int]bool{}
	for _, log := range logs {
		if log == nil {
			continue
		}
		if log.ResponsibleUsername == "" && log.Username != "" {
			log.ResponsibleUsername = log.Username
		}
		if log.ResponsibleDisplayName == "" && log.ResponsibleName != "" {
			log.ResponsibleDisplayName = log.ResponsibleName
		}
		if log.ResponsibleUsername != "" && log.ResponsibleDisplayName != "" {
			continue
		}
		if log.ResponsibleUserId <= 0 || seen[log.ResponsibleUserId] {
			continue
		}
		seen[log.ResponsibleUserId] = true
		responsibleUserIds = append(responsibleUserIds, log.ResponsibleUserId)
	}
	if len(responsibleUserIds) == 0 {
		return nil
	}
	var users []model.User
	if err := model.DB.Select("id", "username", "display_name").Where("id IN ?", responsibleUserIds).Find(&users).Error; err != nil {
		return err
	}
	userById := make(map[int]model.User, len(users))
	for _, user := range users {
		userById[user.Id] = user
	}
	for _, log := range logs {
		if log.ResponsibleUsername != "" && log.ResponsibleDisplayName != "" {
			continue
		}
		user, ok := userById[log.ResponsibleUserId]
		if !ok {
			continue
		}
		if log.ResponsibleUsername == "" {
			log.ResponsibleUsername = user.Username
		}
		if log.ResponsibleDisplayName == "" {
			log.ResponsibleDisplayName = user.DisplayName
		}
	}
	return nil
}

func GetOrganizationLogStats(operatorUserId, organizationId int, accessMode string, req OrganizationLogListRequest) (OrganizationLogStats, error) {
	quotaQuery, err := buildOrganizationLogStatsQuery(operatorUserId, organizationId, accessMode, req, true)
	if err != nil {
		return OrganizationLogStats{}, err
	}
	rpmTpmQuery, err := buildOrganizationLogRealtimeStatsQuery(operatorUserId, organizationId, accessMode, req)
	if err != nil {
		return OrganizationLogStats{}, err
	}
	// 两个聚合各自扫进独立的临时结构体。GORM 的 Scan 会整体重置目标结构体，
	// 复用同一个变量会让第二次 Scan（只返回 rpm/tpm）把 quota 抹成 0，
	// 接口就会永远报「本区间用量为 0」。
	var quotaStats struct {
		Quota int
	}
	if err := quotaQuery.Where("type = ?", model.LogTypeConsume).Select("COALESCE(SUM(quota), 0) AS quota").Scan(&quotaStats).Error; err != nil {
		return OrganizationLogStats{}, err
	}
	var realtimeStats struct {
		Rpm int
		Tpm int
	}
	if err := rpmTpmQuery.Select("COUNT(*) AS rpm, COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS tpm").Scan(&realtimeStats).Error; err != nil {
		return OrganizationLogStats{}, err
	}
	return OrganizationLogStats{Quota: quotaStats.Quota, Rpm: realtimeStats.Rpm, Tpm: realtimeStats.Tpm}, nil
}

func buildOrganizationLogQuery(operatorUserId, organizationId int, accessMode string, req OrganizationLogListRequest, includeTimeRange bool) (*OrganizationActorContext, *gorm.DB, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, nil, err
	}
	tx, err := applyOrganizationLogFilters(model.LOG_DB.Model(&model.Log{}), operatorUserId, organizationId, actor, req, includeTimeRange)
	if err != nil {
		return nil, nil, err
	}
	return actor, tx, nil
}

func buildOrganizationLogStatsQuery(operatorUserId, organizationId int, accessMode string, req OrganizationLogListRequest, includeTimeRange bool) (*gorm.DB, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}
	return applyOrganizationLogFilters(model.LOG_DB.Model(&model.Log{}), operatorUserId, organizationId, actor, req, includeTimeRange)
}

func buildOrganizationLogRealtimeStatsQuery(operatorUserId, organizationId int, accessMode string, req OrganizationLogListRequest) (*gorm.DB, error) {
	tx, err := buildOrganizationLogStatsQuery(operatorUserId, organizationId, accessMode, req, false)
	if err != nil {
		return nil, err
	}
	return tx.Where("type = ?", model.LogTypeConsume).Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix()), nil
}

func applyOrganizationLogFilters(tx *gorm.DB, operatorUserId, organizationId int, actor *OrganizationActorContext, req OrganizationLogListRequest, includeTimeRange bool) (*gorm.DB, error) {
	tx = tx.Where("billing_account_type = ? AND billing_account_id = ? AND organization_id = ?", model.AccountContextTypeOrganization, organizationId, organizationId)
	if !actor.Capabilities.CanViewOrganizationWideData {
		tx = tx.Where("responsible_user_id = ?", operatorUserId)
	} else if req.ResponsibleUserId > 0 {
		tx = tx.Where("responsible_user_id = ?", req.ResponsibleUserId)
	} else if req.ResponsibleName != "" {
		var err error
		tx, err = applyResponsibleNameSnapshotFilter(tx, req.ResponsibleName)
		if err != nil {
			return nil, err
		}
	}
	if req.Type != model.LogTypeUnknown {
		tx = tx.Where("type = ?", req.Type)
	}
	if req.TokenId > 0 {
		tx = tx.Where("token_id = ?", req.TokenId)
	}
	if req.TokenName != "" {
		tx = tx.Where("token_name = ?", req.TokenName)
	}
	if req.ModelName != "" {
		modelNamePattern, err := model.SanitizeLikePattern(req.ModelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if req.Group != "" {
		tx = tx.Where(model.LogGroupColumn()+" = ?", req.Group)
	}
	if req.RequestId != "" {
		tx = tx.Where("request_id = ?", req.RequestId)
	}
	if includeTimeRange {
		if req.StartTimestamp > 0 {
			tx = tx.Where("created_at >= ?", req.StartTimestamp)
		}
		if req.EndTimestamp > 0 {
			tx = tx.Where("created_at <= ?", req.EndTimestamp)
		}
	}
	return tx, nil
}

func applyResponsibleNameSnapshotFilter(tx *gorm.DB, responsibleName string) (*gorm.DB, error) {
	var responsibleUserIds []int
	if err := model.DB.Model(&model.User{}).Where("username = ? OR display_name = ?", responsibleName, responsibleName).Pluck("id", &responsibleUserIds).Error; err != nil {
		return nil, err
	}
	if len(responsibleUserIds) > 0 {
		return tx.Where("(username = ? OR responsible_name = ? OR responsible_user_id IN ?)", responsibleName, responsibleName, responsibleUserIds), nil
	}
	return tx.Where("(username = ? OR responsible_name = ?)", responsibleName, responsibleName), nil
}
