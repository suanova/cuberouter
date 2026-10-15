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
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestImageAPIPolicyAndAuditPaging(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	old := DB
	DB = db
	t.Cleanup(func() { DB = old })
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&ImageAPIPolicy{}, &ImageStudioAudit{}))
	policy, err := ImageAPIModeration("trial")
	require.NoError(t, err)
	require.Equal(t, "external", policy)
	require.NoError(t, SaveImageAPIPolicy(ImageAPIPolicy{GroupName: "trial", Moderation: "required", UpdatedBy: 1}))
	policy, err = ImageAPIModeration("trial")
	require.NoError(t, err)
	require.Equal(t, "required", policy)
	require.Error(t, SaveImageAPIPolicy(ImageAPIPolicy{GroupName: "trial", Moderation: "disabled"}))
	for i := 0; i < 55; i++ {
		require.NoError(t, CreateImageStudioAudit(&ImageStudioAudit{JobID: fmt.Sprintf("%032d", i), UserID: 2, Source: "api", RequestHash: "original", ModerationState: "passed"}))
	}
	require.NoError(t, CreateImageStudioAudit(&ImageStudioAudit{JobID: "old", UserID: 9, Source: "", ModerationState: "passed"}))
	rows, err := PageImageStudioAudits("", "", 2, 0, "api", "passed")
	require.NoError(t, err)
	require.Len(t, rows, 50)
	older, err := PageImageStudioAudits("", "", 2, uint64(rows[49].ID), "api", "passed")
	require.NoError(t, err)
	require.Len(t, older, 5)
	rows, err = PageImageStudioAudits("", "", 2, 0, "studio", "")
	require.NoError(t, err)
	require.Empty(t, rows, "legacy-source OR must not bypass user filter")
	created, err := ClaimImageStudioAudit(&ImageStudioAudit{JobID: fmt.Sprintf("%032d", 1), UserID: 2, RequestHash: "replaced"})
	require.NoError(t, err)
	require.False(t, created)
	rows, err = FindImageStudioAudits(fmt.Sprintf("%032d", 1), "", 2)
	require.NoError(t, err)
	require.Equal(t, "original", rows[0].RequestHash)
	require.NoError(t, db.Model(&ImageStudioAudit{}).Where("user_id = ?", 2).Update("expires_at", time.Now().Unix()-1).Error)
	rows, err = PageImageStudioAudits("", "", 2, 0, "api", "")
	require.NoError(t, err)
	require.Empty(t, rows)
}
