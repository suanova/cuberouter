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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newUsageAsyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newBillingTestDB(t)
	origLog, origDB := LOG_DB, DB
	LOG_DB, DB = db, db
	require.NoError(t, db.AutoMigrate(&Task{}, &Midjourney{}, &Token{}, &Organization{}))
	t.Cleanup(func() { LOG_DB, DB = origLog, origDB })
	return db
}

func TestGetSelfUsageTaskRows(t *testing.T) {
	db := newUsageAsyncTestDB(t)
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&Token{Id: 5, Name: "video-key", Key: "k5", UserId: 7}).Error)
	require.NoError(t, db.Create(&Organization{Id: 9, Name: "Acme", Slug: "acme", OwnerUserId: 7}).Error)
	require.NoError(t, db.Create(&[]Task{
		{TaskID: "t1", UserId: 7, ActorUserId: 7, TokenId: 5, BillingAccountType: "personal", Status: TaskStatusSuccess, Quota: 100, SubmitTime: ts, Properties: Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "t2", UserId: 7, ActorUserId: 7, TokenId: 5, BillingAccountType: "personal", Status: TaskStatusFailure, Quota: 0, SubmitTime: ts, Properties: Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "t3", UserId: 7, ActorUserId: 7, TokenId: 5, BillingAccountType: "organization", OrganizationId: 9, Status: TaskStatusInProgress, Quota: 40, SubmitTime: ts, Properties: Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "t4", UserId: 8, ActorUserId: 8, Status: TaskStatusSuccess, Quota: 999, SubmitTime: ts, Properties: Properties{OriginModelName: "wan3.0-video"}},
	}).Error)

	rows, err := GetSelfUsageTaskRows(7, ts-60, ts+60)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	personal := rows[0]
	if personal.BillingAccountType == "organization" {
		personal = rows[1]
	}
	assert.Equal(t, 2, personal.RequestCount)
	assert.Equal(t, 1, personal.SuccessCount)
	assert.Equal(t, 1, personal.FailureCount)
	assert.Equal(t, 100, personal.Quota)
	assert.Equal(t, "video-key", personal.TokenName)

	org := rows[0]
	if org.BillingAccountType == "personal" {
		org = rows[1]
	}
	assert.Equal(t, 1, org.RequestCount)
	assert.Equal(t, 0, org.SuccessCount)
	assert.Equal(t, 0, org.FailureCount)
	assert.Equal(t, 40, org.Quota)
	assert.Equal(t, "Acme", org.OrganizationName)
}

func TestGetSelfUsageMidjourneyRowsUsesMilliseconds(t *testing.T) {
	db := newUsageAsyncTestDB(t)
	tsSec := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&[]Midjourney{
		{UserId: 7, ActorUserId: 7, Action: "IMAGINE", Status: "SUCCESS", Quota: 80, SubmitTime: tsSec * 1000, BillingAccountType: "personal"},
		{UserId: 7, ActorUserId: 7, Action: "VIDEO", Status: "FAILURE", Quota: 0, SubmitTime: tsSec * 1000, BillingAccountType: "personal"},
		{UserId: 8, ActorUserId: 8, Action: "IMAGINE", Status: "SUCCESS", Quota: 999, SubmitTime: tsSec * 1000, BillingAccountType: "personal"},
	}).Error)

	rows, err := GetSelfUsageMidjourneyRows(7, tsSec-60, tsSec+60)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	byModel := map[string]SelfUsageAsyncRow{}
	for _, row := range rows {
		byModel[row.ModelName] = row
	}
	assert.Equal(t, 1, byModel["mj_imagine"].RequestCount)
	assert.Equal(t, 1, byModel["mj_imagine"].SuccessCount)
	assert.Equal(t, 80, byModel["mj_imagine"].Quota)
	assert.Equal(t, 1, byModel["mj_video"].FailureCount)
}

func TestGetSelfUsageMidjourneyRowsIncludesLastSecond(t *testing.T) {
	db := newUsageAsyncTestDB(t)
	endSec := time.Date(2026, 11, 3, 23, 59, 59, 0, time.Local).Unix()
	require.NoError(t, db.Create(&Midjourney{
		UserId: 7, ActorUserId: 7, Action: "IMAGINE", Status: "SUCCESS", Quota: 80,
		SubmitTime: endSec*1000 + 500, BillingAccountType: "personal",
	}).Error)

	rows, err := GetSelfUsageMidjourneyRows(7, endSec-3600, endSec)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 80, rows[0].Quota)
}

func TestLocalDayKeySeparatesBoundary(t *testing.T) {
	late := time.Date(2026, 11, 2, 23, 59, 59, 0, time.Local).Unix()
	early := time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local).Unix()
	assert.NotEqual(t, localDayKey(late), localDayKey(early))
}
