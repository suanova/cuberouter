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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupUserModelAllowListTestDB installs an isolated in-memory SQLite database
// for the member model allow list tests and restores the global handles on
// cleanup.
func setupUserModelAllowListTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	// Organizations participate in the allow list: a membership only constrains
	// while its organization is active, so the table must exist here.
	require.NoError(t, db.AutoMigrate(&User{}, &Organization{}, &OrganizationMember{}))

	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createUserModelAllowListTestUser(t *testing.T, username string, role int) *User {
	t.Helper()
	user := User{
		Username:    username,
		Password:    "password",
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     "aff-" + username, // uniqueIndex — must be unique per user
	}
	require.NoError(t, DB.Create(&user).Error)
	return &user
}

// createUserModelAllowListTestOrganization materializes the organization row a
// membership belongs to. Only memberships governed by an active organization
// constrain a user, so every fixture membership needs one.
func createUserModelAllowListTestOrganization(t *testing.T, organizationId int, status string) *Organization {
	t.Helper()
	name := fmt.Sprintf("allowlist-org-%d", organizationId)
	organization := Organization{
		Id:        organizationId,
		Name:      name,
		Slug:      name,
		Status:    status,
		CreatedBy: 1,
	}
	require.NoError(t, DB.Create(&organization).Error)
	return &organization
}

func createUserModelAllowListTestMember(t *testing.T, userId int, organizationId int, status string, modelAccess string) *OrganizationMember {
	t.Helper()
	// A membership constrains only while its organization is active: make sure the
	// fixture's organization exists (active by default) so that restricted cases
	// cannot turn into "unrestricted" for the wrong reason.
	if err := DB.First(&Organization{}, organizationId).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		createUserModelAllowListTestOrganization(t, organizationId, OrganizationStatusActive)
	} else {
		require.NoError(t, err)
	}
	member := OrganizationMember{
		OrganizationId: organizationId,
		UserId:         userId,
		Role:           OrganizationRoleMember,
		Status:         status,
		ModelAccess:    modelAccess,
	}
	require.NoError(t, DB.Create(&member).Error)
	return &member
}

func TestIsModelAllowedByMemberAccess(t *testing.T) {
	tests := []struct {
		name      string
		allow     []string
		modelName string
		want      bool
	}{
		{name: "nil list allows everything", allow: nil, modelName: "x", want: true},
		{name: "empty list allows nothing", allow: []string{}, modelName: "x", want: false},
		{name: "exact entry matches", allow: []string{"gpt-4o", "claude-3-5-sonnet"}, modelName: "gpt-4o", want: true},
		{name: "concrete name missing from the list", allow: []string{"gpt-4o"}, modelName: "gpt-4o-mini", want: false},
		{name: "gizmo wildcard entry matches a concrete name", allow: []string{"gpt-4-gizmo-*"}, modelName: "gpt-4-gizmo-abc", want: true},
		{name: "concrete gizmo entry normalizes to the wildcard", allow: []string{"gpt-4-gizmo-my-custom"}, modelName: "gpt-4-gizmo-*", want: true},
		{name: "a star outside the gizmo family is literal", allow: []string{"claude-3-*"}, modelName: "claude-3-opus", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsModelAllowedByMemberAccess(tt.allow, tt.modelName))
		})
	}
}

func TestFilterModelsByMemberAccess(t *testing.T) {
	models := []string{"a", "b", "c"}

	// A nil allow list passes the input through unchanged.
	assert.Equal(t, models, FilterModelsByMemberAccess(nil, models))
	assert.Nil(t, FilterModelsByMemberAccess(nil, nil))

	// The allow list order does not decide the output order; the input order does.
	assert.Equal(t, []string{"a", "b"}, FilterModelsByMemberAccess([]string{"b", "a"}, models))

	// Entries outside the allow list are dropped.
	assert.Equal(t, []string{"c"}, FilterModelsByMemberAccess([]string{"c", "zzz"}, models))

	// An empty allow list permits nothing but still yields a non-nil list.
	filtered := FilterModelsByMemberAccess([]string{}, models)
	require.NotNil(t, filtered)
	assert.Empty(t, filtered)
}

func TestGetUserModelAllowList(t *testing.T) {
	setupUserModelAllowListTestDB(t)

	t.Run("platform admin is exempt despite a restricted membership", func(t *testing.T) {
		admin := createUserModelAllowListTestUser(t, "allowlist-admin", common.RoleAdminUser)
		createUserModelAllowListTestMember(t, admin.Id, 1, OrganizationMemberStatusActive, `["gpt-4o"]`)

		require.Nil(t, GetUserModelAllowList(admin.Id))
	})

	t.Run("user without a membership is unrestricted", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-no-org", common.RoleCommonUser)

		require.Nil(t, GetUserModelAllowList(user.Id))
	})

	t.Run("single restricted membership returns the sorted list", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-single", common.RoleCommonUser)
		// Stored by hand out of order, unlike the sorted payload the write path
		// stores: the getter must sort whatever the column holds.
		createUserModelAllowListTestMember(t, user.Id, 2, OrganizationMemberStatusActive, `["b","a"]`)

		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(user.Id))
	})

	t.Run("an unrestricted membership adds no constraint", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-unrestricted-row", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, user.Id, 3, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
		createUserModelAllowListTestMember(t, user.Id, 4, OrganizationMemberStatusActive, "")

		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(user.Id))
	})

	t.Run("restricted memberships intersect", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-intersect", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, user.Id, 5, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
		createUserModelAllowListTestMember(t, user.Id, 6, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"b", "c"}))

		assert.Equal(t, []string{"b"}, GetUserModelAllowList(user.Id))
	})

	t.Run("disjoint memberships yield an empty non-nil list", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-disjoint", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, user.Id, 7, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
		createUserModelAllowListTestMember(t, user.Id, 8, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"c"}))

		allow := GetUserModelAllowList(user.Id)
		require.NotNil(t, allow)
		assert.Empty(t, allow)
	})

	t.Run("non-active memberships do not participate", func(t *testing.T) {
		exitedOnly := createUserModelAllowListTestUser(t, "allowlist-exited-only", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, exitedOnly.Id, 9, OrganizationMemberStatusExited, `["a"]`)
		require.Nil(t, GetUserModelAllowList(exitedOnly.Id))

		user := createUserModelAllowListTestUser(t, "allowlist-inactive-rows", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, user.Id, 10, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
		createUserModelAllowListTestMember(t, user.Id, 11, OrganizationMemberStatusDisabled, `["c"]`)
		createUserModelAllowListTestMember(t, user.Id, 12, OrganizationMemberStatusExited, `["d"]`)
		createUserModelAllowListTestMember(t, user.Id, 13, OrganizationMemberStatusRemoved, `["e"]`)
		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(user.Id))
	})

	t.Run("memberships in inactive organizations do not participate", func(t *testing.T) {
		// A disabled or dissolved organization cannot govern its members anymore
		// (a dissolved one has no administrator left), so its restriction must not
		// outlive the organization.
		disabledOrg := createUserModelAllowListTestUser(t, "allowlist-org-disabled", common.RoleCommonUser)
		createUserModelAllowListTestOrganization(t, 50, OrganizationStatusDisabled)
		createUserModelAllowListTestMember(t, disabledOrg.Id, 50, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
		require.Nil(t, GetUserModelAllowList(disabledOrg.Id))

		dissolvedOrg := createUserModelAllowListTestUser(t, "allowlist-org-dissolved", common.RoleCommonUser)
		createUserModelAllowListTestOrganization(t, 51, OrganizationStatusDissolved)
		createUserModelAllowListTestMember(t, dissolvedOrg.Id, 51, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
		require.Nil(t, GetUserModelAllowList(dissolvedOrg.Id))

		// Only the active organization of the same user keeps constraining.
		mixed := createUserModelAllowListTestUser(t, "allowlist-org-mixed", common.RoleCommonUser)
		createUserModelAllowListTestOrganization(t, 52, OrganizationStatusActive)
		createUserModelAllowListTestMember(t, mixed.Id, 52, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
		createUserModelAllowListTestOrganization(t, 53, OrganizationStatusDisabled)
		createUserModelAllowListTestMember(t, mixed.Id, 53, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"b"}))
		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(mixed.Id))
	})

	t.Run("a membership without an organization row is unrestricted", func(t *testing.T) {
		// A dangling membership points at an organization that cannot configure or
		// lift anything, so it must not act as a permanent restriction either.
		user := createUserModelAllowListTestUser(t, "allowlist-org-missing", common.RoleCommonUser)
		require.NoError(t, DB.Create(&OrganizationMember{
			OrganizationId: 99,
			UserId:         user.Id,
			Role:           OrganizationRoleMember,
			Status:         OrganizationMemberStatusActive,
			ModelAccess:    MarshalMemberModelAccess([]string{"a"}),
		}).Error)
		require.Nil(t, GetUserModelAllowList(user.Id))
	})

	t.Run("a null model_access column counts as unrestricted", func(t *testing.T) {
		// Rows that predate the column hold NULL on all three supported
		// databases; reading them must not fail and must mean "no constraint".
		// The seed values stay valid restrictions until the NULL update, so a
		// failed update or a failed NULL scan cannot pass by accident.
		nullOnly := createUserModelAllowListTestUser(t, "allowlist-null-only", common.RoleCommonUser)
		nullMember := createUserModelAllowListTestMember(t, nullOnly.Id, 14, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"zzz"}))
		results := DB.Exec("UPDATE organization_members SET model_access = NULL WHERE id = ?", nullMember.Id)
		require.NoError(t, results.Error)
		require.EqualValues(t, 1, results.RowsAffected)
		require.Nil(t, GetUserModelAllowList(nullOnly.Id))

		user := createUserModelAllowListTestUser(t, "allowlist-null-mixed", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, user.Id, 15, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
		nullMember = createUserModelAllowListTestMember(t, user.Id, 16, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"zzz"}))
		results = DB.Exec("UPDATE organization_members SET model_access = NULL WHERE id = ?", nullMember.Id)
		require.NoError(t, results.Error)
		require.EqualValues(t, 1, results.RowsAffected)
		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(user.Id))
	})
}

func TestRefreshUserModelAllowListCacheWithoutRedis(t *testing.T) {
	setupUserModelAllowListTestDB(t)

	require.NoError(t, RefreshUserModelAllowListCache(1))
}

func TestGetUserModelAllowListCache(t *testing.T) {
	setupUserModelAllowListTestDB(t)
	server := useUserCacheMiniRedis(t)

	t.Run("first read stores the restricted list and later reads serve it", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-cache-restricted", common.RoleCommonUser)
		member := createUserModelAllowListTestMember(t, user.Id, 20, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
		key := fmt.Sprintf("user:model_allowlist:%d", user.Id)

		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(user.Id))

		raw, err := common.RDB.Get(t.Context(), key).Result()
		require.NoError(t, err)
		assert.Equal(t, `{"m":["a","b"]}`, raw)
		ttl, err := common.RDB.TTL(t.Context(), key).Result()
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))

		// The cached value is served without re-reading the database.
		require.NoError(t, DB.Model(&OrganizationMember{}).Where("id = ?", member.Id).Update("model_access", MarshalMemberModelAccess([]string{"z"})).Error)
		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(user.Id))
	})

	t.Run("an unrestricted result round-trips as u:true", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-cache-unrestricted", common.RoleCommonUser)
		key := fmt.Sprintf("user:model_allowlist:%d", user.Id)

		require.Nil(t, GetUserModelAllowList(user.Id))

		raw, err := common.RDB.Get(t.Context(), key).Result()
		require.NoError(t, err)
		assert.Equal(t, `{"u":true}`, raw)
		require.Nil(t, GetUserModelAllowList(user.Id))
	})

	t.Run("an empty restricted result round-trips as an empty non-nil list", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-cache-empty", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, user.Id, 21, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
		createUserModelAllowListTestMember(t, user.Id, 22, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"c"}))
		key := fmt.Sprintf("user:model_allowlist:%d", user.Id)

		allow := GetUserModelAllowList(user.Id)
		require.NotNil(t, allow)
		assert.Empty(t, allow)

		raw, err := common.RDB.Get(t.Context(), key).Result()
		require.NoError(t, err)
		assert.Equal(t, `{"m":[]}`, raw)

		allow = GetUserModelAllowList(user.Id)
		require.NotNil(t, allow)
		assert.Empty(t, allow)
	})

	t.Run("hand-written cache values are decoded for both shapes", func(t *testing.T) {
		restricted := createUserModelAllowListTestUser(t, "allowlist-cache-manual-list", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, restricted.Id, 23, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
		require.NoError(t, common.RedisSet(fmt.Sprintf("user:model_allowlist:%d", restricted.Id), `{"m":["z","a"]}`, time.Minute))
		assert.Equal(t, []string{"a", "z"}, GetUserModelAllowList(restricted.Id))

		unrestricted := createUserModelAllowListTestUser(t, "allowlist-cache-manual-nil", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, unrestricted.Id, 24, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
		require.NoError(t, common.RedisSet(fmt.Sprintf("user:model_allowlist:%d", unrestricted.Id), `{"u":true}`, time.Minute))
		require.Nil(t, GetUserModelAllowList(unrestricted.Id))
	})

	t.Run("the admin exemption wins over a cached restriction", func(t *testing.T) {
		admin := createUserModelAllowListTestUser(t, "allowlist-cache-admin", common.RoleAdminUser)
		require.NoError(t, common.RedisSet(fmt.Sprintf("user:model_allowlist:%d", admin.Id), `{"m":["gpt-4o"]}`, time.Minute))

		require.Nil(t, GetUserModelAllowList(admin.Id))
	})

	t.Run("the cached role exempts an administrator without reading the database", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-cache-role-admin", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, user.Id, 26, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
		// The auth cache is the authority on the role once warm; a role change
		// publishes it. Seed it as an administrator while the database row still
		// reads common — a per-request database role lookup would wrongly
		// restrict this user.
		cached := *user
		cached.Role = common.RoleAdminUser
		require.NoError(t, populateUserCache(cached))

		require.Nil(t, GetUserModelAllowList(user.Id))
	})

	t.Run("a cached non-admin role is not overridden by the database role", func(t *testing.T) {
		admin := createUserModelAllowListTestUser(t, "allowlist-cache-role-member", common.RoleAdminUser)
		createUserModelAllowListTestMember(t, admin.Id, 27, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
		cached := *admin
		cached.Role = common.RoleCommonUser
		require.NoError(t, populateUserCache(cached))

		assert.Equal(t, []string{"a"}, GetUserModelAllowList(admin.Id))
	})

	t.Run("refresh drops the entry and the next read refills it", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-cache-refresh", common.RoleCommonUser)
		member := createUserModelAllowListTestMember(t, user.Id, 25, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
		key := fmt.Sprintf("user:model_allowlist:%d", user.Id)

		require.Equal(t, []string{"a"}, GetUserModelAllowList(user.Id))
		require.True(t, server.Exists(key))

		require.NoError(t, DB.Model(&OrganizationMember{}).Where("id = ?", member.Id).Update("model_access", MarshalMemberModelAccess([]string{"b"})).Error)
		require.NoError(t, RefreshUserModelAllowListCache(user.Id))
		require.False(t, server.Exists(key))

		assert.Equal(t, []string{"b"}, GetUserModelAllowList(user.Id))
		raw, err := common.RDB.Get(t.Context(), key).Result()
		require.NoError(t, err)
		assert.Equal(t, `{"m":["b"]}`, raw)
	})

	t.Run("corrupted or ambiguous cache values fall back to the database", func(t *testing.T) {
		tests := []struct {
			name string
			raw  string
		}{
			{name: "empty object", raw: `{}`},
			{name: "null models", raw: `{"m":null}`},
			{name: "legacy top-level array", raw: `["a"]`},
		}
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				user := createUserModelAllowListTestUser(t, fmt.Sprintf("allowlist-cache-corrupt-%d", i), common.RoleCommonUser)
				createUserModelAllowListTestMember(t, user.Id, 30+i, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
				key := fmt.Sprintf("user:model_allowlist:%d", user.Id)
				require.NoError(t, common.RedisSet(key, tt.raw, time.Minute))

				// An unrecognized value must not be served as "unrestricted": it
				// is a miss, the database decides, and the entry is rewritten.
				assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(user.Id))
				raw, err := common.RDB.Get(t.Context(), key).Result()
				require.NoError(t, err)
				assert.Equal(t, `{"m":["a","b"]}`, raw)
			})
		}
	})

	t.Run("a database failure is not cached as unrestricted", func(t *testing.T) {
		user := createUserModelAllowListTestUser(t, "allowlist-cache-db-failure", common.RoleCommonUser)
		createUserModelAllowListTestMember(t, user.Id, 40, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a", "b"}))
		key := fmt.Sprintf("user:model_allowlist:%d", user.Id)

		require.NoError(t, DB.Migrator().DropTable(&OrganizationMember{}))
		require.Nil(t, GetUserModelAllowList(user.Id))
		require.False(t, server.Exists(key), "a failed database read must not be cached as unrestricted")

		// The transient failure leaves no trace: once the database recovers, the
		// restriction applies again and is cached.
		require.NoError(t, DB.AutoMigrate(&OrganizationMember{}))
		recreated := OrganizationMember{
			OrganizationId: 40,
			UserId:         user.Id,
			Role:           OrganizationRoleMember,
			Status:         OrganizationMemberStatusActive,
			ModelAccess:    MarshalMemberModelAccess([]string{"a", "b"}),
		}
		require.NoError(t, DB.Create(&recreated).Error)
		assert.Equal(t, []string{"a", "b"}, GetUserModelAllowList(user.Id))
		raw, err := common.RDB.Get(t.Context(), key).Result()
		require.NoError(t, err)
		assert.Equal(t, `{"m":["a","b"]}`, raw)
	})
}

// A refill that read the database before an invalidation must not write its
// pre-change snapshot after the invalidation: that would pin the stale value
// for the whole TTL even though the write succeeded.
func TestGetUserModelAllowListCacheRefillCannotOutrunInvalidation(t *testing.T) {
	setupUserModelAllowListTestDB(t)
	server := useUserCacheMiniRedis(t)

	user := createUserModelAllowListTestUser(t, "allowlist-cache-refill-race", common.RoleCommonUser)
	member := createUserModelAllowListTestMember(t, user.Id, 50, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
	key := fmt.Sprintf("user:model_allowlist:%d", user.Id)

	// The reader captures the fence and the pre-change database snapshot.
	version, err := getUserModelAllowListCacheVersion(user.Id)
	require.NoError(t, err)
	stale, loaded := loadMemberModelAllowListFromDB(user.Id)
	require.True(t, loaded)
	require.Equal(t, []string{"a"}, stale)

	// The admin changes the list and invalidates the cache before the reader
	// writes its snapshot back.
	require.NoError(t, DB.Model(&OrganizationMember{}).Where("id = ?", member.Id).Update("model_access", MarshalMemberModelAccess([]string{"b"})).Error)
	require.NoError(t, RefreshUserModelAllowListCache(user.Id))

	wrote, err := cacheUserModelAllowListAtVersion(user.Id, stale, version)
	require.NoError(t, err)
	assert.False(t, wrote, "a refill from before the invalidation must be discarded")
	assert.False(t, server.Exists(key), "the raced refill must not revive the cache")

	// The next read loads and caches the new restriction.
	assert.Equal(t, []string{"b"}, GetUserModelAllowList(user.Id))
	raw, err := common.RDB.Get(t.Context(), key).Result()
	require.NoError(t, err)
	assert.Equal(t, `{"m":["b"]}`, raw)
}

// Hard-deleting a user must drop the allow-list cache too: SQLite reuses the
// freed user id, and a stale entry would impose the deleted user's restriction
// on the next account that receives the id.
func TestHardDeletedUserFinalizeInvalidatesModelAllowListCache(t *testing.T) {
	setupUserModelAllowListTestDB(t)
	server := useUserCacheMiniRedis(t)

	user := createUserModelAllowListTestUser(t, "allowlist-hard-delete", common.RoleCommonUser)
	createUserModelAllowListTestMember(t, user.Id, 51, OrganizationMemberStatusActive, MarshalMemberModelAccess([]string{"a"}))
	key := fmt.Sprintf("user:model_allowlist:%d", user.Id)
	require.Equal(t, []string{"a"}, GetUserModelAllowList(user.Id))
	require.True(t, server.Exists(key))

	deleted := &HardDeletedUser{Id: user.Id, AuthVersion: 1}
	require.NoError(t, deleted.Finalize())
	assert.False(t, server.Exists(key), "hard delete must invalidate the allow list cache")
}
