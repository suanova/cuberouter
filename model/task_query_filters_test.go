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

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTaskQueryFilterTestDB(t *testing.T) {
	t.Helper()
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()
	t.Cleanup(func() {
		common.SetDatabaseTypes(previousMain, previousLog)
		initCol()
	})

	previousDB := DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/task-query-filters.db?_pragma=busy_timeout(30000)"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, DB.AutoMigrate(&User{}, &Task{}))
}

// createTaskQueryFilterFixture returns alice and a soft-deleted bob. Task t4 has
// empty Properties so the JSON filter is exercised against NULL.
func createTaskQueryFilterFixture(t *testing.T) (User, User) {
	t.Helper()
	DB.Unscoped().Where("username IN ?", []string{"tqf-alice", "tqf-bob"}).Delete(&User{})
	DB.Where("task_id LIKE ?", "tqf-%").Delete(&Task{})

	alice := User{Username: "tqf-alice", Password: "x", AffCode: "tqf-alice"}
	bob := User{Username: "tqf-bob", Password: "x", AffCode: "tqf-bob"}
	require.NoError(t, DB.Create(&alice).Error)
	require.NoError(t, DB.Create(&bob).Error)
	require.NoError(t, DB.Delete(&bob).Error)

	tasks := []Task{
		{TaskID: "tqf-1", UserId: alice.Id, Platform: "kling", TokenName: "key-a", Group: "default", RequestId: "req-1", Properties: Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "tqf-2", UserId: alice.Id, Platform: "suno", TokenName: "key-b", Group: "vip", RequestId: "req-2", Properties: Properties{OriginModelName: "music-v2"}},
		{TaskID: "tqf-3", UserId: bob.Id, Platform: "kling", TokenName: "key-c", Group: "default", RequestId: "req-3", Properties: Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "tqf-4", UserId: alice.Id, Platform: "kling", TokenName: "key-a", Group: "default", RequestId: "req-4"},
	}
	require.NoError(t, DB.Create(&tasks).Error)
	return alice, bob
}

func runTaskQueryFilterAssertions(t *testing.T) {
	t.Helper()
	alice, _ := createTaskQueryFilterFixture(t)

	cases := []struct {
		name   string
		params SyncTaskQueryParams
		want   []string
	}{
		{"model exact", SyncTaskQueryParams{ModelName: "wan3.0-video"}, []string{"tqf-1", "tqf-3"}},
		{"model wildcard", SyncTaskQueryParams{ModelName: "%video%"}, []string{"tqf-1", "tqf-3"}},
		{"model excludes null properties", SyncTaskQueryParams{ModelName: "wan3.0-video"}, []string{"tqf-1", "tqf-3"}},
		{"malformed wildcard falls back to literal", SyncTaskQueryParams{ModelName: "%%"}, nil},
		{"token exact", SyncTaskQueryParams{TokenName: "key-b"}, []string{"tqf-2"}},
		{"group exact", SyncTaskQueryParams{Group: "vip"}, []string{"tqf-2"}},
		{"request id exact", SyncTaskQueryParams{RequestID: "req-1"}, []string{"tqf-1"}},
		{"platform exact", SyncTaskQueryParams{Platform: "suno"}, []string{"tqf-2"}},
		{"username exact", SyncTaskQueryParams{Username: "tqf-alice"}, []string{"tqf-1", "tqf-2", "tqf-4"}},
		{"username wildcard", SyncTaskQueryParams{Username: "%alice%"}, []string{"tqf-1", "tqf-2", "tqf-4"}},
		{"deleted user username", SyncTaskQueryParams{Username: "tqf-bob"}, []string{"tqf-3"}},
		{"combined filters", SyncTaskQueryParams{Username: "tqf-alice", ModelName: "wan3.0-video", Group: "default"}, []string{"tqf-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tasks := TaskGetAllTasks(0, 50, tc.params)
			ids := make([]string, 0, len(tasks))
			for _, task := range tasks {
				ids = append(ids, task.TaskID)
			}
			require.ElementsMatch(t, tc.want, ids)
			require.Equal(t, int64(len(tc.want)), TaskCountAllTasks(tc.params))
		})
	}

	t.Run("user scope keeps its union and applies new filters", func(t *testing.T) {
		params := SyncTaskQueryParams{IncludeOrganization: true, ModelName: "wan3.0-video"}
		tasks := TaskGetAllUserTask(alice.Id, 0, 50, params)
		require.Len(t, tasks, 1)
		require.Equal(t, "tqf-1", tasks[0].TaskID)
		require.Equal(t, int64(1), TaskCountAllUserTask(alice.Id, params))
	})
}

func TestTaskQueryFilters(t *testing.T) {
	setupTaskQueryFilterTestDB(t)
	runTaskQueryFilterAssertions(t)
}
