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
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestStudioImageDatabaseContracts(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var db *gorm.DB
			if dialect == "sqlite" {
				var err error
				db, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "studio.db")), &gorm.Config{})
				require.NoError(t, err)
				sqlDB, err := db.DB()
				require.NoError(t, err)
				t.Cleanup(func() { _ = sqlDB.Close() })
			} else {
				db = openOrganizationNameMigrationConfiguredDB(t, dialect)
				if dialect == "postgres" {
					// Both contenders must see the same disposable schema on
					// separate connections from the test pool.
					var schema string
					require.NoError(t, db.Raw("select current_schema()").Scan(&schema).Error)
					config, err := pgx.ParseConfig(requireEnvOrSkip(t, "TEST_POSTGRES_DSN"))
					require.NoError(t, err)
					config.RuntimeParams["search_path"] = schema
					sqlDB := stdlib.OpenDB(*config)
					t.Cleanup(func() { _ = sqlDB.Close() })
					db, err = gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
					require.NoError(t, err)
				}
			}
			original := DB
			DB = db
			t.Cleanup(func() { DB = original })
			// A pre-existing destination table and row survive introducing Studio.
			require.NoError(t, db.AutoMigrate(&User{}, &Channel{}, &Ability{}))
			require.NoError(t, db.Create(&User{Id: 1, Username: "kept", Quota: 1234}).Error)
			require.NoError(t, db.Create(&Channel{Id: 1, Name: "kept-channel", Key: "fixture-only", Type: 1, Models: "existing-model"}).Error)
			for i := 0; i < 2; i++ {
				require.NoError(t, db.AutoMigrate(&StudioImageAudit{}, &StudioImageLease{}))
			}
			var account User
			require.NoError(t, db.First(&account, 1).Error)
			assert.Equal(t, "kept", account.Username)
			assert.Equal(t, 1234, account.Quota)
			var channel Channel
			require.NoError(t, db.First(&channel, 1).Error)
			assert.Equal(t, "existing-model", channel.Models)
			require.NoError(t, db.Model(&channel).Update("status", common.ChannelStatusEnabled).Error)
			require.NoError(t, db.Create(&Ability{Group: "image-studio", Model: "existing-model", ChannelId: 1, Enabled: true}).Error)
			workers, err := StudioImageWorkerChannels([]int{1}, "image-studio", "existing-model")
			require.NoError(t, err)
			require.Len(t, workers, 1)
			assert.Equal(t, 1, workers[0].Id)
			workers, err = StudioImageWorkerChannels([]int{1}, "other-group", "existing-model")
			require.NoError(t, err)
			assert.Empty(t, workers)
			require.NoError(t, db.Model(&Ability{}).Where("channel_id = ?", 1).Update("enabled", false).Error)
			workers, err = StudioImageWorkerChannels([]int{1}, "image-studio", "existing-model")
			require.NoError(t, err)
			assert.Empty(t, workers, "revoked abilities are read from the current database")
			// Reviewer patch validation must reflect GORM's zero-value update semantics.
			baseURL := "https://reviewer.invalid/moderate"
			reviewer := &Channel{Id: 2, Type: constant.ChannelTypeStudioReview, Name: "reviewer", Key: "0123456789abcdef0123456789abcdef", BaseURL: &baseURL, Other: "kept-ca-bundle"}
			require.NoError(t, db.Create(reviewer).Error)
			require.NoError(t, db.Model(&Channel{Id: 2}).Updates(Channel{Key: "abcdef0123456789abcdef0123456789"}).Error)
			var patched Channel
			require.NoError(t, db.First(&patched, 2).Error)
			assert.Equal(t, constant.ChannelTypeStudioReview, patched.Type)
			assert.Equal(t, baseURL, patched.GetBaseURL())
			assert.Equal(t, reviewer.Other, patched.Other)
			assert.Equal(t, "abcdef0123456789abcdef0123456789", patched.Key)
			var version string
			if dialect == "sqlite" {
				require.NoError(t, db.Raw("select sqlite_version()").Scan(&version).Error)
			} else {
				require.NoError(t, db.Raw("select version()").Scan(&version).Error)
			}
			t.Logf("database version: %s", version)
			now := time.Now()
			acquired, err := AcquireStudioImageLease("gpu:1", "job-a", now, time.Minute)
			require.NoError(t, err)
			require.True(t, acquired)
			acquired, err = AcquireStudioImageLease("gpu:1", "job-b", now, time.Minute)
			require.NoError(t, err)
			assert.False(t, acquired)
			require.NoError(t, ReleaseStudioImageLease("gpu:1", "job-b"))
			acquired, err = AcquireStudioImageLease("gpu:1", "job-c", now, time.Minute)
			require.NoError(t, err)
			assert.False(t, acquired, "another job cannot unlock the holder")
			require.NoError(t, ReleaseStudioImageLease("gpu:1", "job-a"))
			acquired, err = AcquireStudioImageLease("gpu:1", "job-b", now, time.Minute)
			require.NoError(t, err)
			assert.True(t, acquired)
			// Two gateways competing for a queued account must have one winner.
			sqlDB, err := db.DB()
			require.NoError(t, err)
			if dialect == "sqlite" {
				sqlDB.SetMaxOpenConns(1)
			}
			type claim struct {
				holder   string
				acquired bool
				err      error
			}
			claims := make(chan claim, 2)
			for _, holder := range []string{"gateway-a-job", "gateway-b-job"} {
				go func() {
					acquired, err := AcquireStudioImageLease("account:7", holder, now, time.Minute)
					claims <- claim{holder, acquired, err}
				}()
			}
			var winner, loser string
			for range 2 {
				c := <-claims
				require.NoError(t, c.err)
				if c.acquired {
					require.Empty(t, winner)
					winner = c.holder
				} else {
					loser = c.holder
				}
			}
			require.NotEmpty(t, winner)
			require.NotEmpty(t, loser)
			renewed, err := RenewStudioImageLease("account:7", loser, now, time.Minute)
			require.NoError(t, err)
			assert.False(t, renewed)
			renewed, err = RenewStudioImageLease("account:7", winner, now, time.Minute)
			require.NoError(t, err)
			assert.True(t, renewed, "even a same-millisecond dispatch renews the lease")
			renewed, err = RenewStudioImageLease("account:7", winner, now.Add(2*time.Minute), time.Minute)
			require.NoError(t, err)
			assert.False(t, renewed, "an expired holder cannot renew the account")
			require.NoError(t, ReleaseStudioImageLease("account:7", winner))
			acquired, err = AcquireStudioImageLease("account:7", loser, now, time.Minute)
			require.NoError(t, err)
			assert.True(t, acquired)
			row := &StudioImageAudit{JobID: "job-a", UserID: 7, UserPrompt: "original", CreatedAt: now.UnixMilli()}
			require.NoError(t, RecordStudioImageAudit(row))
			row.UserPrompt = "overwrite"
			require.NoError(t, RecordStudioImageAudit(row))
			require.NoError(t, UpdateStudioImageAudit("job-a", map[string]any{"state": "completed"}))
			rows, hasMore, err := ListStudioImageAudit(0, 7)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.False(t, hasMore)
			assert.Equal(t, "original", rows[0].UserPrompt)
			require.NoError(t, RecordStudioImageAudit(&StudioImageAudit{JobID: "job-b", UserID: 7, CreatedAt: now.UnixMilli()}))
			rows, _, err = ListStudioImageAudit(now.UnixMilli(), 7, "job-b")
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.Equal(t, "job-a", rows[0].JobID, "a timestamp tie must not skip the next audit row")
			require.NoError(t, RecordStudioImageAudit(&StudioImageAudit{JobID: "other-account", UserID: 8, CreatedAt: now.UnixMilli()}))
			rows, _, err = ListStudioImageAudit(0, 7)
			require.NoError(t, err)
			require.Len(t, rows, 2)
			for _, row := range rows {
				assert.Equal(t, 7, row.UserID, "account filtering excludes other users' prompts")
			}
			pageRows := make([]StudioImageAudit, 101)
			for i := range pageRows {
				pageRows[i] = StudioImageAudit{JobID: fmt.Sprintf("page-%03d", i), UserID: 9, CreatedAt: now.UnixMilli()}
			}
			require.NoError(t, db.CreateInBatches(&pageRows, 50).Error)
			rows, hasMore, err = ListStudioImageAudit(0, 9)
			require.NoError(t, err)
			require.Len(t, rows, 100)
			require.True(t, hasMore)
			last := rows[len(rows)-1]
			rows, hasMore, err = ListStudioImageAudit(last.CreatedAt, 9, last.JobID)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.False(t, hasMore)
			assert.Equal(t, "page-000", rows[0].JobID)
			require.NoError(t, PruneStudioImageAudits(now.Add(31*24*time.Hour)))
			var count int64
			require.NoError(t, db.Model(&StudioImageAudit{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}
