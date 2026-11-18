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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// testCubyAppDatabaseContract runs the Cuby app schema and its queries against
// one real database: an existing database gains the two tables without losing
// rows, a second startup changes nothing, the unique constraints hold, and the
// account answer follows users, organizations, memberships and app status.
func testCubyAppDatabaseContract(t *testing.T, db *gorm.DB, recorder *migrationSQLRecorder) {
	t.Helper()
	db.Logger = recorder
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// One connection: the PostgreSQL scratch schema is a per-connection setting.
	sqlDB.SetMaxOpenConns(1)

	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })

	// A database as the previous release left it: accounts and organizations
	// exist, the Cuby app tables do not.
	require.NoError(t, db.AutoMigrate(&User{}, &Organization{}, &OrganizationMember{}))
	holder := User{Username: "cuby-holder", Password: "not-a-real-password", DisplayName: "Holder", Email: "holder@example.com", AffCode: "cubyholder"}
	other := User{Username: "cuby-other", Password: "not-a-real-password", AffCode: "cubyother"}
	require.NoError(t, db.Create(&holder).Error)
	require.NoError(t, db.Create(&other).Error)
	harbour := Organization{Name: "Harbour Ltd", Slug: "harbour", CreatedBy: other.Id}
	leftBehind := Organization{Name: "Left Behind", Slug: "left-behind", CreatedBy: other.Id}
	closed := Organization{Name: "Closed Shop", Slug: "closed-shop", CreatedBy: other.Id, Status: OrganizationStatusDisabled}
	for _, organization := range []*Organization{&harbour, &leftBehind, &closed} {
		require.NoError(t, db.Create(organization).Error)
	}
	require.NoError(t, db.Create(&[]OrganizationMember{
		{OrganizationId: harbour.Id, UserId: holder.Id, Role: OrganizationRoleMember, Status: OrganizationMemberStatusActive},
		{OrganizationId: leftBehind.Id, UserId: holder.Id, Role: OrganizationRoleMember, Status: OrganizationMemberStatusRemoved},
		{OrganizationId: closed.Id, UserId: holder.Id, Role: OrganizationRoleMember, Status: OrganizationMemberStatusActive},
	}).Error)
	require.False(t, db.Migrator().HasTable(&CubyApp{}))
	require.False(t, db.Migrator().HasTable(&CubyAppGrant{}))

	migrate := func() {
		require.NoError(t, db.AutoMigrate(&CubyApp{}, &CubyAppGrant{}))
	}
	migrate()
	assert.True(t, db.Migrator().HasIndex(&CubyAppGrant{}, "idx_cuby_app_grants_app"))
	assert.True(t, db.Migrator().HasIndex(&CubyAppGrant{}, "idx_cuby_app_grants_holder"))
	var users, organizations, members int64
	require.NoError(t, db.Model(&User{}).Count(&users).Error)
	require.NoError(t, db.Model(&Organization{}).Count(&organizations).Error)
	require.NoError(t, db.Model(&OrganizationMember{}).Count(&members).Error)
	assert.Equal(t, []int64{2, 3, 3}, []int64{users, organizations, members}, "the migration must leave existing rows alone")

	// A second startup must not touch the schema again.
	recorder.reset()
	migrate()
	var repeated []string
	for _, statement := range recorder.snapshot() {
		normalized := strings.ToUpper(strings.TrimSpace(statement))
		if strings.HasPrefix(normalized, "CREATE") || strings.HasPrefix(normalized, "ALTER") || strings.HasPrefix(normalized, "DROP") {
			repeated = append(repeated, statement)
		}
	}
	assert.Empty(t, repeated, "a second startup must not repeat any Cuby app DDL")

	apps := []CubyApp{
		{AppId: "ai-company", Name: "AI company", Status: CubyAppStatusEnabled},
		{AppId: "financebuddy", Name: "FinanceBuddy", Status: CubyAppStatusEnabled},
		{AppId: "notes", Name: "Notes", Status: CubyAppStatusEnabled},
		{AppId: "reports", Name: "Reports", Status: CubyAppStatusDisabled},
	}
	require.NoError(t, db.Create(&apps).Error)
	require.Error(t, db.Create(&CubyApp{AppId: "ai-company", Status: CubyAppStatusEnabled}).Error,
		"one app id must not be registered twice")

	grants := []CubyAppGrant{
		// Twice over: to the account itself and through an organization.
		{AppId: "ai-company", SubjectType: CubyAppGrantSubjectOrganization, SubjectId: harbour.Id},
		{AppId: "ai-company", SubjectType: CubyAppGrantSubjectUser, SubjectId: holder.Id},
		// Only through organizations the account cannot use: it was removed
		// from one, and the other is disabled.
		{AppId: "financebuddy", SubjectType: CubyAppGrantSubjectOrganization, SubjectId: leftBehind.Id},
		{AppId: "financebuddy", SubjectType: CubyAppGrantSubjectOrganization, SubjectId: closed.Id},
		// Somebody else's.
		{AppId: "financebuddy", SubjectType: CubyAppGrantSubjectUser, SubjectId: other.Id},
		{AppId: "notes", SubjectType: CubyAppGrantSubjectOrganization, SubjectId: harbour.Id},
		// Given, but the app is switched off for everyone.
		{AppId: "reports", SubjectType: CubyAppGrantSubjectUser, SubjectId: holder.Id},
	}
	require.NoError(t, db.Create(&grants).Error)
	require.Error(t, db.Create(&CubyAppGrant{AppId: "notes", SubjectType: CubyAppGrantSubjectOrganization, SubjectId: harbour.Id}).Error,
		"one app must not be given to the same holder twice")

	accountApps, err := GetAccountCubyApps(holder.Id)
	require.NoError(t, err)
	assert.Equal(t, []AccountCubyApp{
		{Id: "ai-company", Sources: []CubyAppSource{
			{Type: CubyAppGrantSubjectUser},
			{Type: CubyAppGrantSubjectOrganization, Id: harbour.Id, Name: "Harbour Ltd"},
		}},
		{Id: "notes", Sources: []CubyAppSource{
			{Type: CubyAppGrantSubjectOrganization, Id: harbour.Id, Name: "Harbour Ltd"},
		}},
	}, accountApps)

	otherApps, err := GetAccountCubyApps(other.Id)
	require.NoError(t, err)
	assert.Equal(t, []AccountCubyApp{
		{Id: "financebuddy", Sources: []CubyAppSource{{Type: CubyAppGrantSubjectUser}}},
	}, otherApps)

	nobodyApps, err := GetAccountCubyApps(other.Id + 1000)
	require.NoError(t, err)
	assert.Empty(t, nobodyApps)
	assert.NotNil(t, nobodyApps, "an account without apps gets an empty list, not null")

	counts, err := CountCubyAppGrants()
	require.NoError(t, err)
	countByHolder := map[string]int64{}
	for _, count := range counts {
		countByHolder[count.AppId+"/"+count.SubjectType] = count.Count
	}
	assert.Equal(t, map[string]int64{
		"ai-company/organization":   1,
		"ai-company/user":           1,
		"financebuddy/organization": 2,
		"financebuddy/user":         1,
		"notes/organization":        1,
		"reports/user":              1,
	}, countByHolder)

	views, total, err := ListCubyAppGrants("financebuddy", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, views, 3)
	holders := map[string]string{}
	for _, view := range views {
		if view.SubjectType == CubyAppGrantSubjectOrganization {
			holders[view.OrganizationName] = view.OrganizationStatus
			continue
		}
		holders[view.Username] = view.Email
	}
	assert.Equal(t, map[string]string{
		"Left Behind": OrganizationStatusActive,
		"Closed Shop": OrganizationStatusDisabled,
		"cuby-other":  "",
	}, holders)
	_, total, err = ListCubyAppGrants("financebuddy", 2, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total, "the total does not depend on the page")

	require.NoError(t, DeleteCubyAppWithGrants(&apps[0]))
	var left int64
	require.NoError(t, db.Model(&CubyAppGrant{}).Where("app_id = ?", "ai-company").Count(&left).Error)
	assert.Zero(t, left, "deleting an app must delete its grants")
	accountApps, err = GetAccountCubyApps(holder.Id)
	require.NoError(t, err)
	require.Len(t, accountApps, 1)
	assert.Equal(t, "notes", accountApps[0].Id)
}

func TestCubyAppDatabaseContractSQLite(t *testing.T) {
	recorder := &migrationSQLRecorder{}
	dsn := fmt.Sprintf("file:cuby_app_contract_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	testCubyAppDatabaseContract(t, db, recorder)
}

func TestCubyAppDatabaseContractConfiguredDatabases(t *testing.T) {
	tests := []struct {
		name string
		env  string
		open func(*testing.T, string) *gorm.DB
	}{
		{name: "mysql", env: "TEST_MYSQL_DSN", open: mysqlScratchDatabase},
		{name: "postgres", env: "TEST_POSTGRES_DSN", open: postgresScratchSchema},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(test.env))
			if dsn == "" {
				t.Skip(test.env + " is not configured")
			}
			testCubyAppDatabaseContract(t, test.open(t, dsn), &migrationSQLRecorder{})
		})
	}
}
