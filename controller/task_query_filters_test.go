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
package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type taskQueryFilterResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int64            `json:"total"`
		Items []map[string]any `json:"items"`
	} `json:"data"`
}

func setupTaskQueryFilterControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	initModelListColumnNames(t)
	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/task-query-filter-controller.db?_pragma=busy_timeout(30000)"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Task{}))

	alice := model.User{Username: "tq-alice", Password: "x", AffCode: "tq-alice"}
	bob := model.User{Username: "tq-bob", Password: "x", AffCode: "tq-bob"}
	require.NoError(t, db.Create(&alice).Error)
	require.NoError(t, db.Create(&bob).Error)
	require.NoError(t, db.Create([]model.Task{
		{TaskID: "tq-1", UserId: alice.Id, Platform: "kling", TokenName: "key-a", RequestId: "req-1", Properties: model.Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "tq-2", UserId: bob.Id, Platform: "suno", TokenName: "key-b", RequestId: "req-2", Properties: model.Properties{OriginModelName: "music-v2"}},
	}).Error)
	return db
}

func getAllTaskQuery(t *testing.T, query string) taskQueryFilterResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("role", common.RoleAdminUser)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/task?"+query, nil)
	GetAllTask(c)

	var response taskQueryFilterResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestGetAllTaskFiltersQueryParams(t *testing.T) {
	setupTaskQueryFilterControllerTestDB(t)

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"model name", "model_name=wan3.0-video", "tq-1"},
		{"token name", "token_name=key-b", "tq-2"},
		{"request id", "request_id=req-2", "tq-2"},
		{"platform", "platform=suno", "tq-2"},
		{"username", "username=tq-alice", "tq-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := getAllTaskQuery(t, tc.query)
			require.True(t, response.Success, response.Message)
			require.Equal(t, int64(1), response.Data.Total)
			require.Len(t, response.Data.Items, 1)
			require.Equal(t, tc.want, response.Data.Items[0]["task_id"])
		})
	}
}

func TestGetAllTaskRejectsMalformedWildcard(t *testing.T) {
	setupTaskQueryFilterControllerTestDB(t)

	response := getAllTaskQuery(t, "model_name=a%25%25")
	require.False(t, response.Success)
	require.Contains(t, response.Message, "连续的 %")
}
