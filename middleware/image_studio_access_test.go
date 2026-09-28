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
package middleware

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestImageStudioUsesCurrentMembershipAndLeavesTokenGroupsAvailable(t *testing.T) {
	t.Setenv("IMAGE_STUDIO_ALLOWED_GROUPS", "image-studio")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { model.DB, common.RedisEnabled = oldDB, oldRedis; sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}))
	for _, u := range []model.User{
		{Id: 701, Username: "studio-user", Group: "image-studio", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Id: 702, Username: "regular-user", Group: "default", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Id: 703, Username: "admin-not-enrolled", Group: "default", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 704, Username: "disabled-studio", Group: "image-studio", Role: common.RoleCommonUser, Status: common.UserStatusDisabled},
	} {
		u.AffCode = strconv.Itoa(u.Id)
		require.NoError(t, db.Create(&u).Error)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		id, _ := strconv.Atoi(c.GetHeader("X-Test-Authenticated-User"))
		c.Set("id", id)
		// Simulate a stale session and caller-supplied group hints.
		c.Set("group", "image-studio")
		c.Set("user_group", "image-studio")
	})
	studio := router.Group("/api/image-studio", ImageStudioGroupAuth())
	studio.Any("/*path", func(c *gin.Context) { c.Status(204) })
	request := func(id int, method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/image-studio"+path, nil)
		r.Header.Set("X-Test-Authenticated-User", strconv.Itoa(id))
		r.Header.Set("X-Image-Group", "image-studio")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct{ id, want int }{{701, 204}, {702, 403}, {703, 403}, {704, 403}, {0, 401}} {
		for _, method := range []string{"GET", "POST"} {
			w := request(tc.id, method, "/jobs")
			require.Equal(t, tc.want, w.Code)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		}
	}
	// Revocation is effective without replacing the logged-in session.
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 701).Update("group", "default").Error)
	require.Equal(t, 403, request(701, "GET", "/session").Code)
	// An enrollment does not replace normal token group choices, and the
	// image group is not a new globally selectable token group.
	before := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(before)) })
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"Existing VIP"}`))
	require.True(t, service.GroupInUserUsableGroups("image-studio", "default"))
	require.True(t, service.GroupInUserUsableGroups("image-studio", "vip"))
	require.True(t, service.GroupInUserUsableGroups("image-studio", "image-studio"))
	require.False(t, service.GroupInUserUsableGroups("default", "image-studio"))
	// Do not fall back to the stale session if the account lookup fails.
	require.NoError(t, sqlDB.Close())
	require.Equal(t, 503, request(701, "POST", "/jobs").Code)
}
