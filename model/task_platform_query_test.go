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

	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openUnfinishedSyncTaskQueryTestDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	if dialect == "sqlite" {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "unfinished-sync-tasks.db")), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = sqlDB.Close() })
		return db
	}
	return openOrganizationNameMigrationConfiguredDB(t, dialect)
}

func seedUnfinishedSyncTaskQueryTask(t *testing.T, platform constant.TaskPlatform, status TaskStatus, taskID string) *Task {
	t.Helper()
	now := time.Now().Unix()
	task := &Task{
		TaskID:     taskID,
		Platform:   platform,
		UserId:     1,
		ChannelId:  1,
		Action:     constant.TaskActionImageToVideo,
		Status:     status,
		Progress:   "30%",
		SubmitTime: now,
		CreatedAt:  now,
		UpdatedAt:  now,
		PrivateData: TaskPrivateData{
			UpstreamTaskID: "upstream_" + taskID,
		},
	}
	if status == TaskStatusSuccess {
		task.Progress = "100%"
		task.FinishTime = now
	}
	require.NoError(t, DB.Create(task).Error)
	return task
}

// The async task poller must discover every platform with unfinished work even
// when one platform exceeds the per-query limit. The old global LIMIT query let
// a large platform hide every newer task from other platforms.
func TestUnfinishedSyncTaskQueriesConfiguredDatabases(t *testing.T) {
	dialects := []string{"sqlite"}
	if os.Getenv("TEST_MYSQL_DSN") != "" {
		dialects = append(dialects, "mysql")
	}
	if os.Getenv("TEST_POSTGRES_DSN") != "" {
		dialects = append(dialects, "postgres")
	}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			db := openUnfinishedSyncTaskQueryTestDB(t, dialect)
			originalDB := DB
			DB = db
			t.Cleanup(func() { DB = originalDB })
			require.NoError(t, db.AutoMigrate(&Task{}))
			var version string
			switch dialect {
			case "sqlite":
				require.NoError(t, db.Raw("select sqlite_version()").Scan(&version).Error)
			default:
				require.NoError(t, db.Raw("select version()").Scan(&version).Error)
			}
			t.Logf("%s version: %s", dialect, version)

			seedUnfinishedSyncTaskQueryTask(t, "platform-a", TaskStatusInProgress, "a-1")
			seedUnfinishedSyncTaskQueryTask(t, "platform-a", TaskStatusInProgress, "a-2")
			seedUnfinishedSyncTaskQueryTask(t, "platform-a", TaskStatusInProgress, "a-3")
			seedUnfinishedSyncTaskQueryTask(t, "platform-b", TaskStatusInProgress, "b-1")
			seedUnfinishedSyncTaskQueryTask(t, "platform-b", TaskStatusInProgress, "b-2")
			seedUnfinishedSyncTaskQueryTask(t, "platform-c", TaskStatusSuccess, "c-finished")

			platforms, err := GetUnFinishSyncTaskPlatforms()
			require.NoError(t, err)
			assert.ElementsMatch(t, []constant.TaskPlatform{"platform-a", "platform-b"}, platforms)

			limitedA, err := GetUnFinishSyncTasksByPlatform("platform-a", 2)
			require.NoError(t, err)
			require.Len(t, limitedA, 2)
			assert.Less(t, limitedA[0].ID, limitedA[1].ID)
			for _, task := range limitedA {
				assert.Equal(t, constant.TaskPlatform("platform-a"), task.Platform)
			}

			allA, err := GetUnFinishSyncTasksByPlatform("platform-a", 10)
			require.NoError(t, err)
			assert.Len(t, allA, 3)

			finishedC, err := GetUnFinishSyncTasksByPlatform("platform-c", 10)
			require.NoError(t, err)
			assert.Empty(t, finishedC)

			global := GetAllUnFinishSyncTasks(2)
			require.Len(t, global, 2)
			assert.Less(t, global[0].ID, global[1].ID)
		})
	}
}
