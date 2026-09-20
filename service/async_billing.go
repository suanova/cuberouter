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
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// OrganizationAsyncTaskBillingTTLSeconds 是异步任务账本会话的租约时长。
//
// 任务提交之后要等上游跑完才知道实际费用，这期间会话不能过期，否则修复协程
// 会把预扣当成崩溃残留退掉；轮询间隔又远比流式长，所以这里给一个比流式超时
// 宽得多的下限。
func OrganizationAsyncTaskBillingTTLSeconds() int64 {
	ttl := int64(constant.StreamingTimeout) * 12
	if ttl < 7200 {
		return 7200
	}
	return ttl
}

// TaskBillingRelayInfo 从已落库的异步任务还原出一个只够计费用的 RelayInfo。
//
// 轮询阶段早已没有 gin.Context，请求上下文里的作用域信息只剩这几列，
// 所以必须先把旧行归一化（NormalizeTaskBillingScope），否则一个 0 值作用域
// 会被当成「非组织计费」，把组织的退款打到个人钱包上。
func TaskBillingRelayInfo(task *model.Task) *relaycommon.RelayInfo {
	model.NormalizeTaskBillingScope(task)
	return &relaycommon.RelayInfo{
		UserId:                        task.UserId,
		TokenId:                       task.TokenId,
		TokenKey:                      task.TokenKey,
		TokenUnlimited:                task.TokenUnlimited,
		UsingGroup:                    task.Group,
		RequestId:                     task.RequestId,
		ChannelMeta:                   &relaycommon.ChannelMeta{ChannelId: task.ChannelId},
		ScopeType:                     task.ScopeType,
		ScopeId:                       task.ScopeId,
		BillingAccountType:            task.BillingAccountType,
		BillingAccountId:              task.BillingAccountId,
		OrganizationId:                task.OrganizationId,
		ActorUserId:                   task.ActorUserId,
		CreatorUserId:                 task.CreatorUserId,
		ResponsibleUserId:             task.ResponsibleUserId,
		OrganizationBillingSessionId:  task.OrganizationBillingSessionId,
		OrganizationBillingSessionKey: task.OrganizationBillingSessionKey,
	}
}

// MidjourneyBillingRelayInfo 是 TaskBillingRelayInfo 的 Midjourney 版本。
func MidjourneyBillingRelayInfo(task *model.Midjourney) *relaycommon.RelayInfo {
	model.NormalizeMidjourneyBillingScope(task)
	return &relaycommon.RelayInfo{
		UserId:                        task.UserId,
		TokenId:                       task.TokenId,
		TokenKey:                      task.TokenKey,
		TokenUnlimited:                task.TokenUnlimited,
		UsingGroup:                    task.Group,
		RequestId:                     task.RequestId,
		ChannelMeta:                   &relaycommon.ChannelMeta{ChannelId: task.ChannelId},
		ScopeType:                     task.ScopeType,
		ScopeId:                       task.ScopeId,
		BillingAccountType:            task.BillingAccountType,
		BillingAccountId:              task.BillingAccountId,
		OrganizationId:                task.OrganizationId,
		ActorUserId:                   task.ActorUserId,
		CreatorUserId:                 task.CreatorUserId,
		ResponsibleUserId:             task.ResponsibleUserId,
		OrganizationBillingSessionId:  task.OrganizationBillingSessionId,
		OrganizationBillingSessionKey: task.OrganizationBillingSessionKey,
	}
}

// TouchTaskBillingSession 续租任务提交时留下的组织账本会话。
func TouchTaskBillingSession(task *model.Task) error {
	if task == nil {
		return nil
	}
	return TouchOrganizationBillingSession(TaskBillingRelayInfo(task), OrganizationAsyncTaskBillingTTLSeconds())
}

// TouchMidjourneyBillingSession 续租 Midjourney 任务的组织账本会话。
func TouchMidjourneyBillingSession(task *model.Midjourney) error {
	if task == nil {
		return nil
	}
	return TouchOrganizationBillingSession(MidjourneyBillingRelayInfo(task), OrganizationAsyncTaskBillingTTLSeconds())
}

// SettleAsyncTaskOrganizationQuota 用任务上游返回的实际额度结算组织账本会话。
//
// 与个人路径不同，组织这边没有「差额定点补扣」这一步：账本会话本身就是
// 预扣，直接按实际金额结算即可，多退少补都在同一个事务里完成。
// 返回 true 表示这次调用确实走了组织结算，调用方不应再动个人额度。
func SettleAsyncTaskOrganizationQuota(task *model.Task, actualQuota int) (bool, error) {
	if task == nil || task.OrganizationBillingSessionId <= 0 {
		return false, nil
	}
	relayInfo := TaskBillingRelayInfo(task)
	if !IsOrganizationBilling(relayInfo) {
		return false, nil
	}
	if actualQuota < 0 {
		return false, nil
	}
	if err := (&OrganizationFunding{relayInfo: relayInfo}).SettleWithTokenActual(actualQuota); err != nil {
		return true, err
	}
	task.Quota = actualQuota
	return true, nil
}

// SettleMidjourneyOrganizationQuota 是 SettleAsyncTaskOrganizationQuota 的 Midjourney 版本。
func SettleMidjourneyOrganizationQuota(task *model.Midjourney, actualQuota int) (bool, error) {
	if task == nil || task.OrganizationBillingSessionId <= 0 {
		return false, nil
	}
	relayInfo := MidjourneyBillingRelayInfo(task)
	if !IsOrganizationBilling(relayInfo) {
		return false, nil
	}
	if actualQuota < 0 {
		return false, nil
	}
	if err := (&OrganizationFunding{relayInfo: relayInfo}).SettleWithTokenActual(actualQuota); err != nil {
		return true, err
	}
	task.Quota = actualQuota
	return true, nil
}

// RefundTaskOrganizationQuota 退还一条组织异步任务的预扣。
//
// 返回 true 表示这条任务属于组织计费且退款已经交给组织账本处理，
// 调用方必须跳过个人钱包/订阅的退款分支，否则会退两次钱。
func RefundTaskOrganizationQuota(task *model.Task) (bool, error) {
	if task == nil {
		return false, nil
	}
	relayInfo := TaskBillingRelayInfo(task)
	if !IsOrganizationBilling(relayInfo) {
		return false, nil
	}
	if task.OrganizationBillingSessionId > 0 {
		return true, (&OrganizationFunding{relayInfo: relayInfo}).RefundWithToken()
	}
	// 没有账本会话就没有预扣可退，反向记一笔即可：组织用量与令牌额度在
	// 同一个事务里回退，个人侧一分未动。
	return true, PostConsumeQuota(relayInfo, -task.Quota, 0, false)
}

// RefundMidjourneyOrganizationQuota 是 RefundTaskOrganizationQuota 的 Midjourney 版本。
//
// Midjourney 走的是「提交即结算」：提交时直接扣组织额度，失败时再退，
// 所以这里通常没有账本会话，靠负额度的 post-consume 反向记账。
func RefundMidjourneyOrganizationQuota(task *model.Midjourney) (bool, error) {
	if task == nil {
		return false, nil
	}
	relayInfo := MidjourneyBillingRelayInfo(task)
	if !IsOrganizationBilling(relayInfo) {
		return false, nil
	}
	if task.OrganizationBillingSessionId > 0 {
		return true, (&OrganizationFunding{relayInfo: relayInfo}).RefundWithToken()
	}
	return true, PostConsumeQuota(relayInfo, -task.Quota, 0, false)
}
