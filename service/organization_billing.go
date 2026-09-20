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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"gorm.io/gorm"
)

// 组织账本：一次组织请求的额度要动两处——组织的 used_quota 和令牌的 remain_quota。
// 两处必须落在同一个事务里，否则崩溃在中间就会出现「组织扣了但令牌没扣」的漂移。
//
// 预扣阶段先落一条 billing_session 作为账本凭据（idempotency_key 唯一索引兜住重放），
// 结算/退款再照着这条 session 反向修正，差额记进 billing_records 供对账。
// 进程崩溃留下的孤儿 session 由 organization_billing_repair_task.go 按租约回收退款。

var (
	errOrganizationBillingSessionConflict      = errors.New("organization billing session conflict")
	errOrganizationBillingSessionInsertFailure = errors.New("organization billing session insert failed")
)

// SQLite 同一时刻只允许一个写者。把本进程内的组织计费写操作串起来，
// 避免并发请求在升级事务时反复撞 "database is locked"。
var organizationBillingSQLiteTransactionMu sync.Mutex

// OrganizationBillingRepairLeaseSeconds 是修复协程对一条待修复会话的租约时长。
// 超时未完成说明修复者自己也挂了，别的事务可以重新认领。
const OrganizationBillingRepairLeaseSeconds int64 = 60

// IsOrganizationBilling 判断这次请求是否由组织账户扣费。
// 两个条件缺一不可：类型是组织，且 id 有效——只信类型会让 BillingAccountId 为 0
// 的请求去查一个不存在的组织。
func IsOrganizationBilling(relayInfo *relaycommon.RelayInfo) bool {
	return relayInfo != nil && relayInfo.BillingAccountType == model.AccountContextTypeOrganization && relayInfo.BillingAccountId > 0
}

// PreConsumeOrganizationQuota 是「只校验不落账」的预检，给那些稍后会走
// 完整事务的调用方做提前失败用。真正的扣减在 OrganizationFunding 里。
func PreConsumeOrganizationQuota(relayInfo *relaycommon.RelayInfo, quota int) error {
	if quota < 0 {
		return errors.New("quota cannot be negative")
	}
	if !IsOrganizationBilling(relayInfo) || quota == 0 {
		return nil
	}
	var organization model.Organization
	if err := model.DB.Where("id = ?", relayInfo.BillingAccountId).First(&organization).Error; err != nil {
		return err
	}
	if organization.Status != model.OrganizationStatusActive {
		return errors.New("organization is not active")
	}
	if organization.Quota-organization.UsedQuota < quota {
		return errors.New("organization quota is not enough")
	}
	relayInfo.OrganizationId = organization.Id
	relayInfo.OrganizationQuota = organization.Quota - organization.UsedQuota
	return nil
}

func ConsumeOrganizationQuota(relayInfo *relaycommon.RelayInfo, quota int) error {
	return ConsumeOrganizationQuotaWithRequestCount(relayInfo, quota, quota > 0)
}

func ConsumeOrganizationQuotaWithRequestCount(relayInfo *relaycommon.RelayInfo, quota int, incrementRequestCount bool) error {
	if !IsOrganizationBilling(relayInfo) || quota == 0 {
		return nil
	}
	return runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		if _, _, err := updateOrganizationUsedQuotaTx(tx, relayInfo.BillingAccountId, quota, incrementRequestCount, quota > 0, common.GetTimestamp()); err != nil {
			return err
		}
		return syncOrganizationQuotaFromTx(tx, relayInfo)
	})
}

func consumeOrganizationAndTokenQuotaWithRequestCount(relayInfo *relaycommon.RelayInfo, quota int, incrementRequestCount bool) error {
	if !IsOrganizationBilling(relayInfo) || quota == 0 {
		return nil
	}
	err := runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		if _, _, err := updateOrganizationUsedQuotaTx(tx, relayInfo.BillingAccountId, quota, incrementRequestCount, quota > 0, common.GetTimestamp()); err != nil {
			return err
		}
		if !relayInfo.IsPlayground {
			var err error
			if quota > 0 {
				_, err = DecreaseTokenQuotaTx(tx, relayInfo, quota)
			} else {
				_, err = IncreaseTokenQuotaTx(tx, relayInfo, -quota)
			}
			if err != nil {
				return err
			}
		}
		return syncOrganizationQuotaFromTx(tx, relayInfo)
	})
	if err == nil {
		invalidateOrganizationTokenCaches(relayInfo.TokenKey)
	}
	return err
}

// organizationQuotaUpdateFailure 在被条件更新挡下来之后，回头看一眼到底是
// 「组织被封了」还是「额度不够」，好让调用方给出可诊断的错误而不是裸的 0 行更新。
func organizationQuotaUpdateFailure(tx *gorm.DB, organizationId int) error {
	var organization model.Organization
	if err := tx.Where("id = ?", organizationId).First(&organization).Error; err != nil {
		return err
	}
	if organization.Status != model.OrganizationStatusActive {
		return errors.New("organization is not active")
	}
	return errors.New("organization quota is not enough")
}

// organizationBillingSessionIdentity 生成账本会话的幂等键。
//
// 优先用请求 ID；异步任务在提交请求和任务完成时是两个时刻，用的是同一个
// OriginTaskID，于是退化为任务维度。Operation 后缀让同一请求上的附加扣费
// （比如违规罚金）各占一条账，不会互相顶掉。
func organizationBillingSessionIdentity(relayInfo *relaycommon.RelayInfo) (key string, requestId string, taskId string, err error) {
	if relayInfo == nil {
		return "", "", "", errors.New("relayInfo is nil")
	}
	requestId = strings.TrimSpace(relayInfo.RequestId)
	operation := strings.TrimSpace(relayInfo.OrganizationBillingOperation)
	suffix := ""
	if operation != "" {
		suffix = ":" + operation
	}
	if requestId != "" {
		return "request:" + requestId + suffix, requestId, "", nil
	}
	if relayInfo.TaskRelayInfo != nil {
		taskId = strings.TrimSpace(relayInfo.TaskRelayInfo.OriginTaskID)
	}
	if taskId != "" {
		return "task:" + taskId + suffix, "", taskId, nil
	}
	return "", "", "", errors.New("organization billing idempotency key is required")
}

// organizationBillingSessionMatches 判断已有会话是不是「同一笔账的重放」。
// 金额、令牌、请求标识任一不同都说明这是键冲突而不是重放，必须报错而不是复用。
func organizationBillingSessionMatches(session model.OrganizationBillingSession, relayInfo *relaycommon.RelayInfo, amount int, requestId string, taskId string) bool {
	if relayInfo == nil {
		return false
	}
	return session.OrganizationId == relayInfo.BillingAccountId &&
		session.PreConsumedQuota == amount &&
		session.TokenId == relayInfo.TokenId &&
		session.RequestId == requestId &&
		session.TaskId == taskId
}

func syncOrganizationQuotaFromTx(tx *gorm.DB, relayInfo *relaycommon.RelayInfo) error {
	if relayInfo == nil || relayInfo.BillingAccountId == 0 {
		return nil
	}
	var organization model.Organization
	if err := tx.Where("id = ?", relayInfo.BillingAccountId).First(&organization).Error; err != nil {
		return err
	}
	relayInfo.OrganizationId = organization.Id
	relayInfo.OrganizationQuota = organization.Quota - organization.UsedQuota
	return nil
}

// updateOrganizationUsedQuotaTx 是组织用量唯一的写入口。
//
// 关键在 WHERE 上：额度不足和组织非活跃这两个判定都塞进条件里，
// 由数据库原子地决定行是否被更新，而不是先读再判再写——并发下后者会超扣。
// requireActive 只在增方向打开：退款时组织可能已经被封甚至被解散，
// 那笔钱仍然必须退回去，否则用户的额度就被平台吞了。
//
// 返回 before/after 两份快照供账本记录，before 由 after 反推，
// 保证与 UPDATE 的语义完全一致（包括 used_quota 被 CASE 夹在 0 的情况）。
func updateOrganizationUsedQuotaTx(tx *gorm.DB, organizationId int, delta int, incrementRequestCount bool, requireActive bool, now int64) (model.Organization, model.Organization, error) {
	if delta == 0 && !incrementRequestCount {
		var organization model.Organization
		query := tx.Where("id = ?", organizationId)
		if requireActive {
			query = query.Where("status = ?", model.OrganizationStatusActive)
		}
		if err := query.First(&organization).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) && requireActive {
				return model.Organization{}, model.Organization{}, organizationQuotaUpdateFailure(tx, organizationId)
			}
			return model.Organization{}, model.Organization{}, err
		}
		return organization, organization, nil
	}

	updates := map[string]any{"updated_at": now}
	query := tx.Model(&model.Organization{}).Where("id = ?", organizationId)
	if requireActive {
		query = query.Where("status = ?", model.OrganizationStatusActive)
	}
	switch {
	case delta > 0:
		updates["used_quota"] = gorm.Expr("used_quota + ?", delta)
		query = query.Where("used_quota + ? <= quota", delta)
	case delta < 0:
		updates["used_quota"] = gorm.Expr("CASE WHEN used_quota + ? < 0 THEN 0 ELSE used_quota + ? END", delta, delta)
	}
	if incrementRequestCount {
		updates["request_count"] = gorm.Expr("request_count + ?", 1)
	}

	result := query.Updates(updates)
	if result.Error != nil {
		return model.Organization{}, model.Organization{}, result.Error
	}
	if result.RowsAffected == 0 {
		return model.Organization{}, model.Organization{}, organizationQuotaUpdateFailure(tx, organizationId)
	}

	var after model.Organization
	if err := tx.Where("id = ?", organizationId).First(&after).Error; err != nil {
		return model.Organization{}, model.Organization{}, err
	}
	before := after
	before.UsedQuota = after.UsedQuota - delta
	if delta < 0 && before.UsedQuota < 0 {
		before.UsedQuota = 0
	}
	if incrementRequestCount && before.RequestCount > 0 {
		before.RequestCount--
	}
	return before, after, nil
}

// runOrganizationBillingTransaction 在 SQLite 上串行化组织计费写操作并重试忙错误。
//
// SQLite 没有行级锁也没有 SELECT FOR UPDATE，两个事务同时想写就直接有一个
// 拿到 SQLITE_BUSY。重试是唯一可行的策略，指数退避让先到的那个尽快放手。
func runOrganizationBillingTransaction(fn func(tx *gorm.DB) error) error {
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		organizationBillingSQLiteTransactionMu.Lock()
		defer organizationBillingSQLiteTransactionMu.Unlock()
	}

	const maxAttempts = 20
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = model.DB.Transaction(fn)
		if err == nil || !isSQLiteBusyError(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return err
}

func isSQLiteBusyError(err error) bool {
	if err == nil || !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "sqlite_busy")
}

func createOrganizationBillingRecordTx(tx *gorm.DB, record *model.OrganizationBillingRecord) error {
	if record == nil {
		return nil
	}
	return tx.Create(record).Error
}

func buildOrganizationBillingRecordFromSession(session model.OrganizationBillingSession, recordKey string, recordType string, usedQuotaDelta int, usageQuota int, before model.Organization, after model.Organization, now int64) model.OrganizationBillingRecord {
	return model.OrganizationBillingRecord{
		OrganizationId:    session.OrganizationId,
		SessionId:         session.Id,
		RecordKey:         recordKey,
		RequestId:         session.RequestId,
		TaskId:            session.TaskId,
		RecordType:        recordType,
		QuotaDelta:        0,
		UsedQuotaDelta:    usedQuotaDelta,
		UsageQuota:        usageQuota,
		QuotaBefore:       before.Quota,
		QuotaAfter:        after.Quota,
		UsedQuotaBefore:   before.UsedQuota,
		UsedQuotaAfter:    after.UsedQuota,
		TokenId:           session.TokenId,
		TokenName:         session.TokenName,
		ResponsibleUserId: session.ResponsibleUserId,
		CreatorUserId:     session.CreatorUserId,
		ModelName:         session.ModelName,
		Group:             session.Group,
		CreatedAt:         now,
	}
}

// createOrganizationQuotaAdjustmentBillingRecordTx 把平台管理员的额度调整也记进账本，
// 否则组织账单里会出现「额度涨了但没有来源」的空洞。Key 由调整自身的幂等键派生，
// 重复提交同一笔调整不会重复入账。
func createOrganizationQuotaAdjustmentBillingRecordTx(tx *gorm.DB, adjustment *model.OrganizationQuotaAdjustment) error {
	if adjustment == nil {
		return nil
	}
	record := &model.OrganizationBillingRecord{
		OrganizationId:    adjustment.OrganizationId,
		RecordKey:         "adjust:" + adjustment.IdempotencyKey,
		RecordType:        model.OrganizationBillingRecordTypeAdjustment,
		QuotaDelta:        adjustment.QuotaDelta,
		UsedQuotaDelta:    0,
		UsageQuota:        0,
		QuotaBefore:       adjustment.QuotaBefore,
		QuotaAfter:        adjustment.QuotaAfter,
		UsedQuotaBefore:   adjustment.UsedQuota,
		UsedQuotaAfter:    adjustment.UsedQuota,
		ResponsibleUserId: adjustment.OperatorUserId,
		CreatedAt:         adjustment.CreatedAt,
	}
	return createOrganizationBillingRecordTx(tx, record)
}

func syncOrganizationBillingSessionToRelayInfo(relayInfo *relaycommon.RelayInfo, session *model.OrganizationBillingSession) {
	if relayInfo == nil || session == nil {
		return
	}
	relayInfo.OrganizationBillingSessionId = session.Id
	relayInfo.OrganizationBillingSessionKey = session.IdempotencyKey
}

// organizationBillingSessionTTLSeconds 是会话的存活时长。
// 取流式超时的两倍：心跳协程按流式间隔续租，留一倍余量，
// 正常请求不会因为一次心跳抖动就被修复协程当成孤儿退款。
func organizationBillingSessionTTLSeconds() int64 {
	ttl := int64(constant.StreamingTimeout) * 2
	if ttl <= 0 {
		return 60
	}
	return ttl
}

// reuseOrganizationBillingSession 处理幂等键已经存在的两条路：
// 同参数的并发重放（复用已有会话）和参数不同的键冲突（报错）。
//
// 这里刻意不加行锁：MySQL 在唯一索引上对不存在的键加锁会退化成间隙锁，
// 两个并发插入互相等待直接死锁。让唯一约束去裁决插入竞争，重试路径再兜底。
func (f *OrganizationFunding) reuseOrganizationBillingSession(tx *gorm.DB, key string, amount int, requestId string, taskId string) (bool, error) {
	var existing model.OrganizationBillingSession
	err := tx.Where("idempotency_key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !organizationBillingSessionMatches(existing, f.relayInfo, amount, requestId, taskId) {
		return true, errOrganizationBillingSessionConflict
	}
	f.session = &existing
	f.tokenConsumed = existing.TokenPreConsumedQuota
	syncOrganizationBillingSessionToRelayInfo(f.relayInfo, &existing)
	return true, syncOrganizationQuotaFromTx(tx, f.relayInfo)
}

// PreConsumeWithToken 预扣一次组织请求：落会话、加组织用量、扣令牌额度，一个事务。
//
// 令牌额度按会话创建那一刻的无限额快照扣：结算时可能已经读不到令牌
// （组织下线、令牌被删），那时仍要用同一个口径退，所以额度要记在会话上。
func (f *OrganizationFunding) PreConsumeWithToken(amount int) error {
	if f == nil || f.relayInfo == nil || !IsOrganizationBilling(f.relayInfo) {
		return nil
	}
	if amount < 0 {
		return errors.New("quota cannot be negative")
	}
	key, requestId, taskId, err := organizationBillingSessionIdentity(f.relayInfo)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	err = runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		token, err := loadTokenForQuotaTx(tx, f.relayInfo)
		if err != nil {
			return err
		}
		if reused, err := f.reuseOrganizationBillingSession(tx, key, amount, requestId, taskId); err != nil {
			return err
		} else if reused {
			return nil
		}
		tokenId := f.relayInfo.TokenId
		tokenName := ""
		tokenUnlimited := f.relayInfo.TokenUnlimited
		if token != nil {
			tokenId = token.Id
			tokenName = token.Name
			tokenUnlimited = token.UnlimitedQuota
		}
		tokenDeducted := amount
		if f.relayInfo.IsPlayground || tokenUnlimited {
			tokenDeducted = 0
		}
		channelId := 0
		if f.relayInfo.ChannelMeta != nil {
			channelId = f.relayInfo.ChannelId
		}
		session := &model.OrganizationBillingSession{
			OrganizationId:           f.relayInfo.BillingAccountId,
			IdempotencyKey:           key,
			RequestId:                requestId,
			TaskId:                   taskId,
			Status:                   model.OrganizationBillingSessionStatusPreConsumed,
			PreConsumedQuota:         amount,
			TokenPreConsumedQuota:    tokenDeducted,
			TokenRemainDeductedQuota: tokenDeducted,
			TokenUnlimitedQuota:      tokenUnlimited,
			TokenId:                  tokenId,
			TokenName:                tokenName,
			ChannelId:                channelId,
			ResponsibleUserId:        f.relayInfo.ResponsibleUserId,
			CreatorUserId:            f.relayInfo.CreatorUserId,
			ModelName:                f.relayInfo.OriginModelName,
			Group:                    f.relayInfo.UsingGroup,
			LastHeartbeatAt:          now,
			ExpiresAt:                now + organizationBillingSessionTTLSeconds(),
			CreatedAt:                now,
			UpdatedAt:                now,
		}
		if err := tx.Create(session).Error; err != nil {
			return fmt.Errorf("%w: %v", errOrganizationBillingSessionInsertFailure, err)
		}
		before, after, err := updateOrganizationUsedQuotaTx(tx, f.relayInfo.BillingAccountId, amount, false, true, now)
		if err != nil {
			return err
		}
		deducted, err := decreaseTokenQuotaWithSnapshotTx(tx, f.relayInfo, amount, tokenUnlimited)
		if err != nil {
			return err
		}
		// 无限额令牌不会真的扣额度，把会话上的口径改成实际值，退款才算得对。
		if deducted != tokenDeducted {
			if err := tx.Model(session).Updates(map[string]any{
				"token_pre_consumed_quota":    deducted,
				"token_remain_deducted_quota": deducted,
				"updated_at":                  now,
			}).Error; err != nil {
				return err
			}
			session.TokenPreConsumedQuota = deducted
			session.TokenRemainDeductedQuota = deducted
		}
		if amount > 0 {
			record := buildOrganizationBillingRecordFromSession(*session, "pre_consume:"+strconv.Itoa(session.Id), model.OrganizationBillingRecordTypePreConsume, amount, 0, before, after, now)
			if err := createOrganizationBillingRecordTx(tx, &record); err != nil {
				return err
			}
		}
		f.session = session
		f.tokenConsumed = deducted
		syncOrganizationBillingSessionToRelayInfo(f.relayInfo, session)
		return syncOrganizationQuotaFromTx(tx, f.relayInfo)
	})
	if err != nil {
		// 插入撞唯一约束 = 有并发请求先落了同一条会话。事务已经回滚，
		// 出事务重新读一次，命中就当作自己的预扣成功。
		if errors.Is(err, errOrganizationBillingSessionInsertFailure) {
			reused, replayErr := f.reuseOrganizationBillingSession(model.DB, key, amount, requestId, taskId)
			if replayErr != nil {
				return replayErr
			}
			if reused {
				invalidateOrganizationTokenCaches(f.relayInfo.TokenKey)
				return nil
			}
		}
		return err
	}
	invalidateOrganizationTokenCaches(f.relayInfo.TokenKey)
	return nil
}

// ReserveMoreWithToken 在已经落账的会话上追加预扣。
//
// 用途是「提交之后才算准」的加价：异步任务在拿回上游任务 ID 之后才确定最终
// 报价，分层计费的自动分组重试也可能切到更贵的组。这时会话已经建好，
// 重新走 PreConsumeWithToken 会撞上「同键不同额」的幂等冲突，只能就地加码。
//
// delta <= 0 由调用方保证；会话已经结算/退款/正在修复时静默跳过——
// 那些状态下账已经动过，再加码只会让两边对不上。
func (f *OrganizationFunding) ReserveMoreWithToken(delta int) error {
	if f == nil || f.relayInfo == nil || !IsOrganizationBilling(f.relayInfo) || delta <= 0 {
		return nil
	}
	if f.session == nil {
		if err := f.loadSession(); err != nil {
			return err
		}
	}
	now := common.GetTimestamp()
	err := runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		var session model.OrganizationBillingSession
		if err := model.LockForUpdate(tx).Where("id = ?", f.session.Id).First(&session).Error; err != nil {
			return err
		}
		switch session.Status {
		case model.OrganizationBillingSessionStatusPreConsumed, model.OrganizationBillingSessionStatusFailed:
			// 仍可加码：失败态会被修复协程按 refunded_quota 重算，不会漏退。
		default:
			f.session = &session
			f.tokenConsumed = session.TokenRemainDeductedQuota
			syncOrganizationBillingSessionToRelayInfo(f.relayInfo, &session)
			return nil
		}
		tokenRelayInfo := organizationBillingTokenQuotaRelayInfo(&session, f.relayInfo)
		before, after, err := updateOrganizationUsedQuotaTx(tx, session.OrganizationId, delta, false, true, now)
		if err != nil {
			return err
		}
		deducted, err := decreaseTokenQuotaWithSnapshotTx(tx.Unscoped(), tokenRelayInfo, delta, session.TokenUnlimitedQuota)
		if err != nil {
			return err
		}
		session.PreConsumedQuota += delta
		session.TokenPreConsumedQuota += deducted
		session.TokenRemainDeductedQuota += deducted
		session.LastHeartbeatAt = now
		session.ExpiresAt = now + organizationBillingSessionTTLSeconds()
		session.UpdatedAt = now
		if err := tx.Model(&session).Updates(map[string]any{
			"pre_consumed_quota":          session.PreConsumedQuota,
			"token_pre_consumed_quota":    session.TokenPreConsumedQuota,
			"token_remain_deducted_quota": session.TokenRemainDeductedQuota,
			"last_heartbeat_at":           session.LastHeartbeatAt,
			"expires_at":                  session.ExpiresAt,
			"updated_at":                  session.UpdatedAt,
		}).Error; err != nil {
			return err
		}
		// RecordKey 要唯一：用加码后的累计预扣额做后缀，同一个会话的每次加码各留一条。
		record := buildOrganizationBillingRecordFromSession(session, "reserve:"+strconv.Itoa(session.Id)+":"+strconv.Itoa(session.PreConsumedQuota), model.OrganizationBillingRecordTypePreConsume, delta, 0, before, after, now)
		if err := createOrganizationBillingRecordTx(tx, &record); err != nil {
			return err
		}
		f.session = &session
		f.tokenConsumed = session.TokenRemainDeductedQuota
		syncOrganizationBillingSessionToRelayInfo(f.relayInfo, &session)
		return syncOrganizationQuotaFromTx(tx, f.relayInfo)
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(f.relayInfo.TokenKey)
	return nil
}

// SettleWithToken 把「预扣额度 + 差额」交给 SettleWithTokenActual。
// delta 是实际用量减预扣量，负数就是退。
func (f *OrganizationFunding) SettleWithToken(delta int) error {
	if f == nil || f.relayInfo == nil || !IsOrganizationBilling(f.relayInfo) {
		return nil
	}
	if f.session == nil {
		if err := f.loadSession(); err != nil {
			return err
		}
	}
	return f.SettleWithTokenActual(f.session.PreConsumedQuota + delta)
}

// validateOrganizationSettledQuota 让重复结算变成幂等操作：
// 结算过且金额一致就直接放行，金额不一致说明同一把幂等键被用来结两笔不同的账，报错。
func validateOrganizationSettledQuota(session model.OrganizationBillingSession, actualQuota int) error {
	if session.Status != model.OrganizationBillingSessionStatusSettled {
		return nil
	}
	if session.SettledQuota != actualQuota {
		return errors.New("organization idempotency conflict")
	}
	return nil
}

func (f *OrganizationFunding) SettleWithTokenActual(actualQuota int) error {
	if f == nil || f.relayInfo == nil || !IsOrganizationBilling(f.relayInfo) {
		return nil
	}
	if actualQuota < 0 {
		return errors.New("settled quota cannot be negative")
	}
	if f.session == nil {
		if err := f.loadSession(); err != nil {
			return err
		}
	}
	if err := validateOrganizationSettledQuota(*f.session, actualQuota); err != nil {
		return err
	}
	return f.settleWithTokenActual(actualQuota)
}

func (f *OrganizationFunding) settleWithTokenActual(actualQuota int) error {
	now := common.GetTimestamp()
	err := runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		var session model.OrganizationBillingSession
		if err := model.LockForUpdate(tx).Where("id = ?", f.session.Id).First(&session).Error; err != nil {
			return err
		}
		if err := validateOrganizationSettledQuota(session, actualQuota); err != nil {
			return err
		}
		if session.Status == model.OrganizationBillingSessionStatusSettled {
			f.session = &session
			f.tokenConsumed = session.TokenRemainDeductedQuota
			syncOrganizationBillingSessionToRelayInfo(f.relayInfo, &session)
			return nil
		}
		if session.Status == model.OrganizationBillingSessionStatusRefunded {
			return errors.New("organization billing session already refunded")
		}
		tokenRelayInfo := organizationBillingTokenQuotaRelayInfo(&session, f.relayInfo)
		delta := actualQuota - session.PreConsumedQuota
		incrementRequestCount := actualQuota > 0
		// 只有增方向要求组织活跃：实际用量比预扣少时，退还要能穿过已封禁的组织。
		before, after, err := updateOrganizationUsedQuotaTx(tx, session.OrganizationId, delta, incrementRequestCount, delta > 0, now)
		if err != nil {
			return err
		}
		tokenDelta := 0
		if delta > 0 {
			tokenDelta, err = decreaseTokenQuotaWithSnapshotTx(tx.Unscoped(), tokenRelayInfo, delta, session.TokenUnlimitedQuota)
		} else if delta < 0 {
			tokenDelta, err = increaseTokenQuotaWithSnapshotTx(tx.Unscoped(), tokenRelayInfo, -delta, session.TokenUnlimitedQuota)
			tokenDelta = -tokenDelta
		}
		if err != nil {
			return err
		}
		// 违规罚金不经过上游转发，渠道用量没人记，只能在这里补。
		if f.relayInfo.OrganizationBillingOperation == organizationBillingOperationViolationFee && session.ChannelId > 0 {
			if err := tx.Model(&model.Channel{}).Where("id = ?", session.ChannelId).Update("used_quota", gorm.Expr("used_quota + ?", actualQuota)).Error; err != nil {
				return err
			}
		}
		session.Status = model.OrganizationBillingSessionStatusSettled
		session.SettledQuota = actualQuota
		session.TokenRemainDeductedQuota += tokenDelta
		if session.TokenRemainDeductedQuota < 0 {
			session.TokenRemainDeductedQuota = 0
		}
		session.SettledAt = now
		session.UpdatedAt = now
		if err := tx.Model(&session).Updates(map[string]any{
			"status":                      session.Status,
			"settled_quota":               session.SettledQuota,
			"token_remain_deducted_quota": session.TokenRemainDeductedQuota,
			"settled_at":                  session.SettledAt,
			"updated_at":                  session.UpdatedAt,
		}).Error; err != nil {
			return err
		}
		record := buildOrganizationBillingRecordFromSession(session, "settle:"+strconv.Itoa(session.Id), model.OrganizationBillingRecordTypeSettle, delta, actualQuota, before, after, now)
		if err := createOrganizationBillingRecordTx(tx, &record); err != nil {
			return err
		}
		f.session = &session
		f.tokenConsumed = session.TokenRemainDeductedQuota
		syncOrganizationBillingSessionToRelayInfo(f.relayInfo, &session)
		if err := syncOrganizationQuotaFromTx(tx, tokenRelayInfo); err != nil {
			return err
		}
		f.relayInfo.OrganizationId = tokenRelayInfo.OrganizationId
		f.relayInfo.OrganizationQuota = tokenRelayInfo.OrganizationQuota
		return nil
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(f.relayInfo.TokenKey)
	return nil
}

// RefundWithToken 把还没结算掉的会话整笔退回组织。
//
// 已结算的会话要按 settled 而不是预扣金额退——中间那次结算可能已经改过账。
// RefundedQuota 让重复调用只退差额，令牌额度则靠 TokenRefunded 标志保证只退一次。
func (f *OrganizationFunding) RefundWithToken() error {
	if f == nil || f.relayInfo == nil || !IsOrganizationBilling(f.relayInfo) {
		return nil
	}
	if f.session == nil {
		if err := f.loadSession(); err != nil {
			return err
		}
	}
	now := common.GetTimestamp()
	err := runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		var session model.OrganizationBillingSession
		if err := model.LockForUpdate(tx).Where("id = ?", f.session.Id).First(&session).Error; err != nil {
			return err
		}
		if session.Status == model.OrganizationBillingSessionStatusRefunded {
			f.session = &session
			f.tokenConsumed = 0
			syncOrganizationBillingSessionToRelayInfo(f.relayInfo, &session)
			return nil
		}
		tokenRelayInfo := organizationBillingTokenQuotaRelayInfo(&session, f.relayInfo)
		refundBasis := session.PreConsumedQuota
		wasSettled := session.Status == model.OrganizationBillingSessionStatusSettled
		if wasSettled {
			refundBasis = session.SettledQuota
		}
		refundQuota := refundBasis - session.RefundedQuota
		if refundQuota < 0 {
			refundQuota = 0
		}
		// requireActive=false：退款必须穿过已封禁/已解散的组织，
		// 否则平台一封号就能把在途的预扣据为己有。
		before, after, err := updateOrganizationUsedQuotaTx(tx, session.OrganizationId, -refundQuota, false, false, now)
		if err != nil {
			return err
		}
		tokenRefunded := 0
		tokenRefundQuota := session.TokenRemainDeductedQuota
		if session.TokenUnlimitedQuota {
			tokenRefundQuota = refundQuota
		}
		if tokenRefundQuota > 0 && !session.TokenRefunded {
			tokenRefunded, err = increaseTokenQuotaWithSnapshotTx(tx.Unscoped(), tokenRelayInfo, tokenRefundQuota, session.TokenUnlimitedQuota)
			if err != nil {
				return err
			}
		}
		session.Status = model.OrganizationBillingSessionStatusRefunded
		session.RefundedQuota += refundQuota
		session.TokenRemainDeductedQuota -= tokenRefunded
		if session.TokenRemainDeductedQuota < 0 {
			session.TokenRemainDeductedQuota = 0
		}
		session.TokenRefunded = true
		session.RefundedAt = now
		session.UpdatedAt = now
		if err := tx.Model(&session).Updates(map[string]any{
			"status":                      session.Status,
			"refunded_quota":              session.RefundedQuota,
			"token_remain_deducted_quota": session.TokenRemainDeductedQuota,
			"token_refunded":              session.TokenRefunded,
			"refunded_at":                 session.RefundedAt,
			"updated_at":                  session.UpdatedAt,
		}).Error; err != nil {
			return err
		}
		// 已结算的会话退款要从用量里减回去；只预扣过的不算用量，usage 保持 0。
		usageQuota := 0
		if wasSettled {
			usageQuota = -refundQuota
		}
		record := buildOrganizationBillingRecordFromSession(session, "refund:"+strconv.Itoa(session.Id), model.OrganizationBillingRecordTypeRefund, -refundQuota, usageQuota, before, after, now)
		if err := createOrganizationBillingRecordTx(tx, &record); err != nil {
			return err
		}
		f.session = &session
		f.tokenConsumed = session.TokenRemainDeductedQuota
		syncOrganizationBillingSessionToRelayInfo(f.relayInfo, &session)
		if err := syncOrganizationQuotaFromTx(tx, tokenRelayInfo); err != nil {
			return err
		}
		f.relayInfo.OrganizationId = tokenRelayInfo.OrganizationId
		f.relayInfo.OrganizationQuota = tokenRelayInfo.OrganizationQuota
		return nil
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(f.relayInfo.TokenKey)
	return nil
}

// loadSession 找回本请求对应的账本会话。
//
// relayInfo 上带着会话 id 和 key 时按两者一起查：重试会复制 RelayInfo，
// 只按 id 查可能命中另一个作用域下的会话。没有时退化为按幂等键查。
func (f *OrganizationFunding) loadSession() error {
	var session model.OrganizationBillingSession
	query := model.DB
	if f.relayInfo != nil && f.relayInfo.OrganizationBillingSessionId > 0 {
		sessionKey := strings.TrimSpace(f.relayInfo.OrganizationBillingSessionKey)
		if sessionKey == "" {
			return errors.New("organization billing session key is required")
		}
		query = query.Where("id = ? AND idempotency_key = ?", f.relayInfo.OrganizationBillingSessionId, sessionKey)
	} else {
		key, _, _, err := organizationBillingSessionIdentity(f.relayInfo)
		if err != nil {
			return err
		}
		query = query.Where("idempotency_key = ?", key)
	}
	if err := query.First(&session).Error; err != nil {
		return fmt.Errorf("organization billing session not found: %w", err)
	}
	f.session = &session
	f.tokenConsumed = session.TokenRemainDeductedQuota
	syncOrganizationBillingSessionToRelayInfo(f.relayInfo, &session)
	return nil
}

// TouchOrganizationBillingSession 续租。流式请求每收到一个 chunk 调一次，
// 只要还在往下游吐数据，修复协程就不会把这笔在途的预扣当成崩溃残留退掉。
func TouchOrganizationBillingSession(relayInfo *relaycommon.RelayInfo, ttlSeconds int64) error {
	if relayInfo == nil || relayInfo.OrganizationBillingSessionId == 0 {
		return nil
	}
	if ttlSeconds <= 0 {
		ttlSeconds = int64(constant.StreamingTimeout)
		if ttlSeconds <= 0 {
			ttlSeconds = 60
		}
	}
	now := common.GetTimestamp()
	return model.DB.Model(&model.OrganizationBillingSession{}).
		Where("id = ? AND idempotency_key = ? AND status = ?", relayInfo.OrganizationBillingSessionId, relayInfo.OrganizationBillingSessionKey, model.OrganizationBillingSessionStatusPreConsumed).
		UpdateColumns(map[string]any{"last_heartbeat_at": now, "expires_at": now + ttlSeconds, "updated_at": now}).Error
}

// ClaimExpiredOrganizationBillingSessions 认领超时的会话，认领即续租。
//
// 两类候选：正常过期的 pre_consumed/failed，以及租约已经超时的 repairing——
// 后一类是「修复者自己也崩了」留下的，别的实例要能接手。
// 认领靠带条件的 UPDATE 抢占，RowsAffected=0 就是没抢到。
func ClaimExpiredOrganizationBillingSessions(limit int, now int64) ([]model.OrganizationBillingSession, error) {
	if limit <= 0 {
		limit = 100
	}
	staleRepairingBefore := now - OrganizationBillingRepairLeaseSeconds
	var candidates []model.OrganizationBillingSession
	err := model.DB.
		Where("((status IN ? AND expires_at > 0 AND expires_at <= ?) OR (status = ? AND last_heartbeat_at <= ?))", []string{model.OrganizationBillingSessionStatusPreConsumed, model.OrganizationBillingSessionStatusFailed}, now, model.OrganizationBillingSessionStatusRepairing, staleRepairingBefore).
		Order("expires_at asc, id asc").
		Limit(limit).
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	claimed := make([]model.OrganizationBillingSession, 0, len(candidates))
	for _, candidate := range candidates {
		session, err := claimExpiredOrganizationBillingSession(candidate.Id, now)
		if err != nil {
			return nil, err
		}
		if session == nil {
			continue
		}
		claimed = append(claimed, *session)
	}
	return claimed, nil
}

func claimExpiredOrganizationBillingSession(sessionId int, now int64) (*model.OrganizationBillingSession, error) {
	staleRepairingBefore := now - OrganizationBillingRepairLeaseSeconds
	result := model.DB.Model(&model.OrganizationBillingSession{}).
		Where("id = ? AND ((status IN ? AND expires_at > 0 AND expires_at <= ?) OR (status = ? AND last_heartbeat_at <= ?))", sessionId, []string{model.OrganizationBillingSessionStatusPreConsumed, model.OrganizationBillingSessionStatusFailed}, now, model.OrganizationBillingSessionStatusRepairing, staleRepairingBefore).
		UpdateColumns(map[string]any{
			"status":            model.OrganizationBillingSessionStatusRepairing,
			"repair_attempts":   gorm.Expr("repair_attempts + ?", 1),
			"last_heartbeat_at": now,
			"expires_at":        now + OrganizationBillingRepairLeaseSeconds,
			"updated_at":        now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	var session model.OrganizationBillingSession
	if err := model.DB.First(&session, sessionId).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func RepairExpiredOrganizationBillingSessions(limit int, now int64) (int, error) {
	claimed, err := ClaimExpiredOrganizationBillingSessions(limit, now)
	if err != nil {
		return 0, err
	}
	return RepairOrganizationBillingSessions(claimed, now)
}

// RepairOrganizationBillingSessions 把认领到的会话退款。
//
// 认领到修复之后状态可能又被别人动过，这里重新读一遍确认仍是 repairing 再动手；
// 单条失败不影响其余会话，记下第一个错误继续处理，修复是尽力而为的。
func RepairOrganizationBillingSessions(sessions []model.OrganizationBillingSession, now int64) (int, error) {
	repaired := 0
	var firstErr error
	for i := range sessions {
		session := sessions[i]
		if err := model.DB.First(&session, session.Id).Error; err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if session.Status != model.OrganizationBillingSessionStatusRepairing {
			continue
		}
		relayInfo := organizationBillingRelayInfoFromSession(&session)
		funding := &OrganizationFunding{relayInfo: relayInfo, session: &session, tokenConsumed: session.TokenRemainDeductedQuota}
		if err := funding.RefundWithToken(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			recordOrganizationBillingRepairFailure(session.Id, err, now)
			continue
		}
		repaired++
	}
	return repaired, firstErr
}

// organizationBillingRelayInfoFromSession 从账本会话还原一个够用的 RelayInfo。
// 修复协程没有任何请求上下文，退款所需的全部信息都只能从会话里读回来。
func organizationBillingRelayInfoFromSession(session *model.OrganizationBillingSession) *relaycommon.RelayInfo {
	if session == nil {
		return nil
	}
	return &relaycommon.RelayInfo{
		UserId:                        session.ResponsibleUserId,
		TokenId:                       session.TokenId,
		TokenUnlimited:                session.TokenUnlimitedQuota,
		ScopeType:                     model.AccountContextTypeOrganization,
		ScopeId:                       session.OrganizationId,
		BillingAccountType:            model.AccountContextTypeOrganization,
		BillingAccountId:              session.OrganizationId,
		OrganizationId:                session.OrganizationId,
		CreatorUserId:                 session.CreatorUserId,
		ResponsibleUserId:             session.ResponsibleUserId,
		OriginModelName:               session.ModelName,
		UsingGroup:                    session.Group,
		RequestId:                     session.RequestId,
		OrganizationBillingSessionId:  session.Id,
		OrganizationBillingSessionKey: session.IdempotencyKey,
	}
}

func organizationBillingTokenQuotaRelayInfo(session *model.OrganizationBillingSession, current *relaycommon.RelayInfo) *relaycommon.RelayInfo {
	relayInfo := organizationBillingRelayInfoFromSession(session)
	if relayInfo == nil {
		return nil
	}
	if current != nil {
		relayInfo.IsPlayground = current.IsPlayground
	}
	return relayInfo
}

func recordOrganizationBillingRepairFailure(sessionId int, repairErr error, now int64) {
	transitioned, err := markOrganizationBillingSessionRepairFailed(sessionId, repairErr, now)
	if err != nil {
		common.SysError("failed to mark organization billing repair failed: " + err.Error())
		return
	}
	if !transitioned {
		return
	}
	if err := recordOrganizationBillingRepairFailedAudit(sessionId, repairErr); err != nil {
		common.SysError("failed to record organization billing repair failure audit: " + err.Error())
	}
}

// markOrganizationBillingSessionRepairFailed 只在状态仍是 repairing 时置为 failed，
// 否则会把另一个实例刚修好的会话又打回失败。
func markOrganizationBillingSessionRepairFailed(sessionId int, repairErr error, now int64) (bool, error) {
	message := ""
	if repairErr != nil {
		message = repairErr.Error()
	}
	result := model.DB.Model(&model.OrganizationBillingSession{}).
		Where("id = ? AND status = ?", sessionId, model.OrganizationBillingSessionStatusRepairing).
		UpdateColumns(map[string]any{
			"status":            model.OrganizationBillingSessionStatusFailed,
			"error_message":     message,
			"last_repair_error": message,
			"expires_at":        now + OrganizationBillingRepairLeaseSeconds,
			"updated_at":        now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
