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
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStudioAuditRetentionAndIdempotency(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&ImageStudioAudit{}))
	require.NoError(t, CreateImageStudioAudit(&ImageStudioAudit{JobID: "first", UserID: 71, ModelPrompt: "original", State: "completed", OutputHash: "original-hash"}))
	require.NoError(t, CreateImageStudioAudit(&ImageStudioAudit{JobID: "first", UserID: 71, ModelPrompt: "must-not-overwrite", State: "queued"}))
	rows, err := FindImageStudioAudits("first", "", 71)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "original", rows[0].ModelPrompt)
	require.Equal(t, "completed", rows[0].State)
	require.NoError(t, CreateImageStudioAudit(&ImageStudioAudit{JobID: "expired", UserID: 71, ModelPrompt: "delete-this"}))
	now := time.Now().Unix()
	require.NoError(t, db.Model(&ImageStudioAudit{}).Where("job_id = ?", "expired").Update("expires_at", now-1).Error)
	rows, err = FindImageStudioAudits("expired", "", 71)
	require.NoError(t, err)
	require.Empty(t, rows)
	require.NoError(t, PurgeExpiredImageStudioAudits(now))
	var count int64
	require.NoError(t, db.Unscoped().Model(&ImageStudioAudit{}).Where("job_id = ?", "expired").Count(&count).Error)
	require.Zero(t, count)
	rows, err = FindImageStudioAudits("", "original-hash", 71)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
