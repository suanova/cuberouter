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
	AdmissionNonce   string `json:"-" gorm:"size:32"`
	RequestHash      string `json:"-" gorm:"size:64"`
	Source           string `json:"source" gorm:"size:16;index"`
	TokenID          int    `json:"token_id"`
	TokenName        string `json:"token_name" gorm:"size:64"`
	RequestID        string `json:"request_id" gorm:"size:80;index"`
	UsingGroup       string `json:"using_group" gorm:"size:64"`
	ModerationPolicy string `json:"moderation_policy" gorm:"size:16"`
	ID               uint   `json:"id"`
	JobID            string `json:"job_id" gorm:"size:32;uniqueIndex"`
	UserID           int    `json:"user_id" gorm:"index"`
	Username         string `json:"username" gorm:"size:64"`
	CreatedAt        int64  `json:"created_at"`
	ExpiresAt        int64  `json:"expires_at" gorm:"index"`
	Model            string `json:"model" gorm:"size:128"`
	ChannelID        int    `json:"channel_id"`
	State            string `json:"state" gorm:"size:32"`
	UserPrompt       string `json:"user_prompt" gorm:"type:text"`
	ModelPrompt      string `json:"model_prompt" gorm:"type:text"`
	Parameters       string `json:"parameters" gorm:"type:text"`
	InputHashes      string `json:"input_sha256" gorm:"type:text"`
	OutputHash       string `json:"output_sha256" gorm:"size:64;index"`
	ModerationState  string `json:"moderation_state" gorm:"size:32"`
	ModerationResult string `json:"moderation_result" gorm:"type:text"`
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

func ClaimImageStudioAudit(a *ImageStudioAudit) (bool, error) {
	// MySQL clientFoundRows can report 1 for a no-op ON DUPLICATE KEY update.
	// Compare a unique insert nonce instead of relying on RowsAffected.
	a.AdmissionNonce = common.GetUUID()
	if err := CreateImageStudioAudit(a); err != nil {
		return false, err
	}
	var stored ImageStudioAudit
	if err := studioAuditDB().Select("admission_nonce").Where("job_id = ?", a.JobID).First(&stored).Error; err != nil {
		return false, err
	}
	return stored.AdmissionNonce == a.AdmissionNonce, nil
}

func UpdateImageStudioAudit(job string, user int, values map[string]any) error {
	q := studioAuditDB().Model(&ImageStudioAudit{}).Where("job_id = ? AND user_id = ? AND expires_at > ?", job, user, time.Now().Unix())
	updated := q.Updates(values)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected == 0 {
		// MySQL may report zero for an unchanged row. Distinguish that from a
		// missing/expired audit: a release must not silently lose its record.
		var count int64
		if err := q.Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

func FindImageStudioAudits(job, hash string, user int) ([]ImageStudioAudit, error) {
	return PageImageStudioAudits(job, hash, user, 0, "", "")
}

func PageImageStudioAudits(job, hash string, user int, before uint64, source, moderation string) ([]ImageStudioAudit, error) {
	q := studioAuditDB().Where("expires_at > ?", time.Now().Unix())
	if before > 0 {
		q = q.Where("id < ?", before)
	}
	if source == "studio" {
		q = q.Where("source = ? OR source = ? OR source IS NULL", "studio", "")
	} else if source != "" {
		q = q.Where("source = ?", source)
	}
	if moderation != "" {
		q = q.Where("moderation_state = ?", moderation)
	}
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
