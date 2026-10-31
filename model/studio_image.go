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
package model

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"time"

	"gorm.io/gorm/clause"
)

// StudioImageAudit belongs to the main DB. It never stores image bytes or keys.
type StudioImageAudit struct {
	JobID       string `json:"job_id" gorm:"primaryKey;size:32"`
	UserID      int    `json:"user_id" gorm:"index"`
	Model       string `json:"model" gorm:"size:255"`
	UserPrompt  string `json:"user_prompt" gorm:"type:text"`
	ModelPrompt string `json:"model_prompt" gorm:"type:text"`
	Parameters  string `json:"parameters" gorm:"type:text"`
	InputHashes string `json:"input_hashes" gorm:"type:text"`
	OutputHash  string `json:"output_hash" gorm:"size:64"`
	Review      string `json:"review" gorm:"type:text"`
	ChannelID   int    `json:"channel_id"`
	State       string `json:"state" gorm:"size:32"`
	Failure     string `json:"error,omitempty" gorm:"size:64"`
	CreatedAt   int64  `json:"created_at" gorm:"index"`
}

// StudioImageLease fences physical GPU capacity across gateway replicas. A
// failed/uncertain relay keeps its lease until expiry instead of reusing a GPU.
type StudioImageLease struct {
	Resource  string `gorm:"primaryKey;size:64"`
	Holder    string `gorm:"size:64"`
	ExpiresAt int64  `gorm:"index"`
	LastUsed  int64
}

func AcquireStudioImageLease(resource, holder string, now time.Time, duration time.Duration) (bool, error) {
	row := StudioImageLease{Resource: resource}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return false, err
	}
	result := DB.Model(&StudioImageLease{}).Where("resource = ? AND expires_at <= ?", resource, now.UnixMilli()).
		Updates(map[string]any{"holder": holder, "expires_at": now.Add(duration).UnixMilli(), "last_used": now.UnixMilli()})
	return result.RowsAffected == 1, result.Error
}

func ReleaseStudioImageLease(resource, holder string) error {
	return DB.Model(&StudioImageLease{}).Where("resource = ? AND holder = ?", resource, holder).
		Updates(map[string]any{"holder": "", "expires_at": int64(0)}).Error
}

func RecordStudioImageAudit(row *StudioImageAudit) error {
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error
}

func StudioImageAccountBusy(owner int, now time.Time) (bool, error) {
	var count int64
	err := DB.Model(&StudioImageLease{}).Where("resource = ? AND expires_at > ?", fmt.Sprintf("account:%d", owner), now.UnixMilli()).Count(&count).Error
	return count > 0, err
}

func StudioImageWorkerChannels(ids []int, group, name string) ([]*Channel, error) {
	var rows []*Channel
	err := DB.Model(&Channel{}).Select("channels.*").Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("channels.id IN ?", ids).Where(map[string]any{"channels.status": common.ChannelStatusEnabled, "abilities.group": group, "abilities.model": name, "abilities.enabled": true}).Find(&rows).Error
	return rows, err
}

func UpdateStudioImageAudit(job string, updates map[string]any) error {
	return DB.Model(&StudioImageAudit{}).Where("job_id = ?", job).Updates(updates).Error
}

func ListStudioImageAudit(before int64, user int, beforeID ...string) ([]StudioImageAudit, error) {
	query := DB.Where("created_at >= ?", time.Now().Add(-30*24*time.Hour).UnixMilli())
	if before > 0 {
		if len(beforeID) > 0 && beforeID[0] != "" {
			query = query.Where("(created_at < ? OR (created_at = ? AND job_id < ?))", before, before, beforeID[0])
		} else {
			query = query.Where("created_at < ?", before)
		}
	}
	if user > 0 {
		query = query.Where("user_id = ?", user)
	}
	rows := make([]StudioImageAudit, 0)
	err := query.Order("created_at DESC").Order("job_id DESC").Limit(100).Find(&rows).Error
	return rows, err
}

func PruneStudioImageAudits(now time.Time) error {
	return DB.Where("created_at < ?", now.Add(-30*24*time.Hour).UnixMilli()).Delete(&StudioImageAudit{}).Error
}
