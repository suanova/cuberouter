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
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Optional DSNs must point at DISPOSABLE databases. CI supplies real engine services.
func TestImageStudioDatabaseMatrix(t *testing.T) {
	engines := []struct {
		name      string
		dialector gorm.Dialector
	}{
		{"sqlite", sqlite.Open(filepath.Join(t.TempDir(), "audit.db"))},
	}
	if dsn := os.Getenv("STUDIO_TEST_MYSQL_DSN"); dsn != "" {
		engines = append(engines, struct {
			name      string
			dialector gorm.Dialector
		}{"mysql", mysql.Open(dsn)})
	}
	if dsn := os.Getenv("STUDIO_TEST_POSTGRES_DSN"); dsn != "" {
		engines = append(engines, struct {
			name      string
			dialector gorm.Dialector
		}{"postgres", postgres.Open(dsn)})
	}
	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			db, err := gorm.Open(engine.dialector, &gorm.Config{})
			require.NoError(t, err)
			var version string
			versionSQL := "SELECT version()"
			if engine.name == "sqlite" {
				versionSQL = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
			t.Logf("%s version: %s", engine.name, version)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { sqlDB.Close() })
			old := DB
			DB = db
			t.Cleanup(func() { DB = old })
			for _, upgrade := range []bool{false, true} {
				require.NoError(t, db.Migrator().DropTable(&ImageStudioAudit{}))
				if upgrade {
					// Persisted fields match release v1.0.0; preserve existing account/channel data.
					require.NoError(t, db.AutoMigrate(&User{}, &Channel{}, &Ability{}, &Option{}))
					require.NoError(t, db.Create(&User{Id: 987001, Username: "studio-upgrade", Status: 1, Group: "image-studio"}).Error)
					require.NoError(t, db.Create(&Channel{Id: 987001, Name: "existing-image-channel", Type: 1, Status: 1, Key: "test-only"}).Error)
				}
				require.NoError(t, db.AutoMigrate(&ImageStudioAudit{}))
				require.NoError(t, CreateImageStudioAudit(&ImageStudioAudit{JobID: "matrix-job", UserID: 987001, Username: "studio-upgrade", ModelPrompt: "保留原圖 🌸", State: "queued"}))
				require.NoError(t, db.AutoMigrate(&ImageStudioAudit{}))
				require.NoError(t, db.AutoMigrate(&ImageStudioAudit{}))
				require.NoError(t, CreateImageStudioAudit(&ImageStudioAudit{JobID: "matrix-job", UserID: 987001, ModelPrompt: "must not overwrite"}))
				require.True(t, db.Migrator().HasIndex(&ImageStudioAudit{}, "idx_image_studio_audits_job_id"))
				rows, err := FindImageStudioAudits("matrix-job", "", 987001)
				require.NoError(t, err)
				require.Len(t, rows, 1)
				require.Equal(t, "保留原圖 🌸", rows[0].ModelPrompt)
				require.Equal(t, int64(ImageStudioAuditRetention.Seconds()), rows[0].ExpiresAt-rows[0].CreatedAt)
				require.NoError(t, UpdateImageStudioAudit("matrix-job", 987001, map[string]any{"channel_id": 987001, "state": "completed", "output_hash": "test-hash"}))
				rows, err = FindImageStudioAudits("", "test-hash", 987001)
				require.NoError(t, err)
				require.Len(t, rows, 1)
				require.NoError(t, PurgeExpiredImageStudioAudits(time.Now().Add(ImageStudioAuditRetention+time.Minute).Unix()))
				var count int64
				require.NoError(t, db.Model(&ImageStudioAudit{}).Count(&count).Error)
				require.Zero(t, count)
				if upgrade {
					var user User
					require.NoError(t, db.First(&user, 987001).Error)
					require.Equal(t, "studio-upgrade", user.Username)
					var channel Channel
					require.NoError(t, db.First(&channel, 987001).Error)
					require.Equal(t, "existing-image-channel", channel.Name)
				}
			}
		})
	}
}
