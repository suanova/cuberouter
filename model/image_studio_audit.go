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
	"os"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

const ImageStudioAuditRetention = 30 * 24 * time.Hour

// Images never enter this table. Hashes identify exact submitted/result bytes;
// they cannot recover images or identify a person independently of the account.
type ImageStudioAudit struct {
	ID          uint   `json:"id"`
	JobID       string `json:"job_id" gorm:"size:32;uniqueIndex"`
	UserID      int    `json:"user_id" gorm:"index"`
	Username    string `json:"username" gorm:"size:64"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at" gorm:"index"`
	Model       string `json:"model" gorm:"size:128"`
	ChannelID   int    `json:"channel_id"`
	State       string `json:"state" gorm:"size:32"`
	UserPrompt  string `json:"user_prompt" gorm:"type:text"`
	ModelPrompt string `json:"model_prompt" gorm:"type:text"`
	Parameters  string `json:"parameters" gorm:"type:text"`
	InputHashes string `json:"input_sha256" gorm:"type:text"`
	OutputHash  string `json:"output_sha256" gorm:"size:64;index"`
}

func ImageStudioAuditEnabled() bool { return os.Getenv("IMAGE_STUDIO_AUDIT_ENABLED") == "true" }

func studioAuditDB() *gorm.DB {
	// SQL trace/error logs must not create unbounded copies of retained prompts.
	return DB.Session(&gorm.Session{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
}

func CreateImageStudioAudit(a *ImageStudioAudit) error {
	now := time.Now()
	a.CreatedAt, a.ExpiresAt = now.Unix(), now.Add(ImageStudioAuditRetention).Unix()
	return studioAuditDB().Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "job_id"}}, DoNothing: true}).Create(a).Error
}

func UpdateImageStudioAudit(job string, user int, values map[string]any) error {
	return studioAuditDB().Model(&ImageStudioAudit{}).Where("job_id = ? AND user_id = ? AND expires_at > ?", job, user, time.Now().Unix()).Updates(values).Error
}

func FindImageStudioAudits(job, hash string, user int) ([]ImageStudioAudit, error) {
	q := studioAuditDB().Where("expires_at > ?", time.Now().Unix())
	if job != "" {
		q = q.Where("job_id = ?", job)
	}
	if hash != "" {
		q = q.Where("output_hash = ?", hash)
	}
	if user > 0 {
		q = q.Where("user_id = ?", user)
	}
	var rows []ImageStudioAudit
	err := q.Order("id DESC").Limit(50).Find(&rows).Error
	return rows, err
}

func PurgeExpiredImageStudioAudits(now int64) error {
	return studioAuditDB().Where("expires_at <= ?", now).Delete(&ImageStudioAudit{}).Error
}

var studioAuditCleaner sync.Once

func StartImageStudioAuditCleanup() {
	studioAuditCleaner.Do(func() {
		go func() {
			for {
				if err := PurgeExpiredImageStudioAudits(time.Now().Unix()); err != nil {
					common.SysError("image studio audit expiry cleanup failed")
				}
				time.Sleep(5 * time.Minute)
			}
		}()
	})
}
