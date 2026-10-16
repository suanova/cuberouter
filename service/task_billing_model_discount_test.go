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
package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 用户×模型折扣在任务提交时快照进 BillingContext，轮询阶段按 token 重算
// （RecalculateTaskQuotaByTokens）必须沿用同一折扣；快照上线前落库的旧任务
// 没有这个字段，必须退化成原价，绝不能变成 0 折或 NaN 扣费。
const (
	taskDiscountTestModel = "task-discount-test-model"

	taskDiscountModelRatio = 2.0
	taskDiscountGroupRatio = 1.5
	// 时长倍率（OtherRatios）也参与重算。
	taskDiscountOtherMultiplier = 2.0
	taskDiscountTotalTokens     = 400

	taskDiscountSeedQuota   = 10000
	taskDiscountPreConsumed = 2000
	// 原价 = 400 × 2 × 1.5 × 2 = 2400；折扣 0.5 → 400 × 2 × 1.5 × 0.5 × 2 = 1200。
	taskDiscountFullPriceQuota = 2400
	taskDiscountHalfPriceQuota = 1200
)

type taskDiscountFixture struct {
	user model.User
	task *model.Task
}

// setupTaskDiscountFixture 建独立测试库、固定模型与分组倍率，造出一个预扣 2000
// 的任务。discount 由调用方决定：0 与旧任务落库后的 private_data 完全一致
// （json omitempty 不落该字段），非零则随快照一起持久化。
func setupTaskDiscountFixture(t *testing.T, name string, discount float64) taskDiscountFixture {
	t.Helper()
	setupServiceTestDB(t)
	// 差额结算会回写渠道用量；渠道表缺了只会让那次 UPDATE 变成一条噪音日志。
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))

	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"`+taskDiscountTestModel+`":2}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1.5}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
	})

	user := createServiceTestUser(t, name+"-user", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).
		Updates(map[string]interface{}{"quota": taskDiscountSeedQuota, "used_quota": taskDiscountPreConsumed}).Error)
	channel := model.Channel{Name: name + "-channel", Key: "sk-test", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).
		Update("used_quota", taskDiscountPreConsumed).Error)

	task := &model.Task{
		TaskID:    "task_" + name,
		UserId:    user.Id,
		ChannelId: channel.Id,
		Quota:     taskDiscountPreConsumed,
		Status:    model.TaskStatus(model.TaskStatusInProgress),
		Group:     "default",
		PrivateData: model.TaskPrivateData{
			BillingSource: BillingSourceWallet,
			BillingContext: &model.TaskBillingContext{
				ModelRatio:        taskDiscountModelRatio,
				GroupRatio:        taskDiscountGroupRatio,
				UserModelDiscount: discount,
				OtherRatios:       map[string]float64{"seconds": taskDiscountOtherMultiplier},
				OriginModelName:   taskDiscountTestModel,
			},
		},
	}
	require.NoError(t, model.DB.Create(task).Error)
	return taskDiscountFixture{user: user, task: task}
}

// reloadTask 从库里回读任务：轮询阶段拿到的是持久化后的行，折扣必须能穿过
// private_data 的 JSON 往返。
func reloadTask(t *testing.T, id int64) model.Task {
	t.Helper()
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, id).Error)
	return stored
}

func TestRecalculateTaskQuotaAppliesSnapshotDiscount(t *testing.T) {
	fixture := setupTaskDiscountFixture(t, "task-discount-applied", 0.5)
	stored := reloadTask(t, fixture.task.ID)
	require.Equal(t, 0.5, stored.PrivateData.BillingContext.UserModelDiscount)

	require.True(t, RecalculateTaskQuotaByTokens(context.Background(), &stored, taskDiscountTotalTokens))

	// 任务额度重算为折后价 1200，预扣多收的 800 退回用户钱包（10000+2000-1200）。
	assert.Equal(t, taskDiscountHalfPriceQuota, getTaskQuota(t, fixture.task.ID))
	assert.Equal(t, taskDiscountSeedQuota+taskDiscountPreConsumed-taskDiscountHalfPriceQuota, getUserQuota(t, fixture.user.Id))
	usedQuota, _ := getUserUsageAccounting(t, fixture.user.Id)
	assert.Equal(t, taskDiscountHalfPriceQuota, usedQuota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Contains(t, log.Content, "discount=0.5000")
}

func TestRecalculateTaskQuotaLegacyContextDefaultsToOne(t *testing.T) {
	for _, tc := range []struct {
		name     string
		discount float64
	}{
		// 旧任务的快照里没有 user_model_discount 字段（omitempty 不落库）。
		{name: "legacy_zero_value", discount: 0},
		// 快照被写坏或人工改动时也不能按 0 折或溢价重算。
		{name: "out_of_range_above_one", discount: 1.5},
		{name: "out_of_range_negative", discount: -0.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := setupTaskDiscountFixture(t, "task-discount-"+tc.name, tc.discount)
			stored := reloadTask(t, fixture.task.ID)
			require.Equal(t, tc.discount, stored.PrivateData.BillingContext.UserModelDiscount)

			require.True(t, RecalculateTaskQuotaByTokens(context.Background(), &stored, taskDiscountTotalTokens))

			// 一律按原价 2400 结算：预扣 2000 差 400 补扣（10000-400）。
			assert.Equal(t, taskDiscountFullPriceQuota, getTaskQuota(t, fixture.task.ID))
			assert.Equal(t, taskDiscountSeedQuota-taskDiscountFullPriceQuota+taskDiscountPreConsumed, getUserQuota(t, fixture.user.Id))
		})
	}
}

func TestTaskLogIncludesUserModelDiscount(t *testing.T) {
	for _, tc := range []struct {
		name        string
		discount    float64
		hasDiscount bool
	}{
		{name: "discount_applied", discount: 0.5, hasDiscount: true},
		{name: "no_discount_entry", discount: 1, hasDiscount: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupServiceTestDB(t)
			require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
			gin.SetMode(gin.TestMode)

			user := createServiceTestUser(t, "task-log-"+tc.name, common.RoleCommonUser)
			channel := model.Channel{Name: "task-log-" + tc.name, Key: "sk-test", Status: common.ChannelStatusEnabled}
			require.NoError(t, model.DB.Create(&channel).Error)
			task := &model.Task{
				TaskID:      "task_log_" + tc.name,
				UserId:      user.Id,
				Quota:       taskDiscountHalfPriceQuota,
				Status:      model.TaskStatus(model.TaskStatusInProgress),
				Group:       "default",
				PrivateData: model.TaskPrivateData{BillingSource: BillingSourceWallet},
			}
			require.NoError(t, model.DB.Create(task).Error)

			info := &relaycommon.RelayInfo{
				UserId:          user.Id,
				OriginModelName: taskDiscountTestModel,
				UsingGroup:      "default",
				ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channel.Id},
				TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: "GENERATE"},
				PriceData: types.PriceData{
					ModelPrice: 0.02,
					Quota:      taskDiscountHalfPriceQuota,
					GroupRatioInfo: types.GroupRatioInfo{
						GroupRatio:           taskDiscountGroupRatio * tc.discount,
						UserModelDiscount:    tc.discount,
						HasUserModelDiscount: tc.hasDiscount,
					},
				},
			}

			log := callLogTaskConsumption(t, info, task)

			var other map[string]any
			require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
			if tc.hasDiscount {
				assert.Equal(t, tc.discount, other["user_model_discount"])
			} else {
				assert.NotContains(t, other, "user_model_discount")
			}
		})
	}
}
