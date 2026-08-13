package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

// CampaignEngine is the core campaign processing engine
type CampaignEngine struct {
	mu sync.RWMutex
}

// Global campaign engine instance
var CampaignEngineInstance = &CampaignEngine{}

// handleCampaignType processes a specific campaign type for a user
func (e *CampaignEngine) handleCampaignType(campaignType string, user *model.User, inviterId int) {
	// Entry points may pass a slim struct without Email (e.g. request-body users
	// from UpdateUser/UpdateSelf/CreateUser), which would silently skip reward
	// emails. Reload the authoritative record; fall back to the passed user on error.
	if full, err := model.GetUserById(user.Id, false); err == nil && full != nil {
		user = full
	}
	campaigns, err := model.GetActiveCampaignsByType(campaignType)
	if err != nil {
		common.SysError("CampaignEngine: failed to get active campaigns: " + err.Error())
		return
	}

	for _, campaign := range campaigns {
		e.processCampaignForUser(campaign, user, inviterId)
	}
}

// processCampaignForUser processes a single campaign for a user
func (e *CampaignEngine) processCampaignForUser(campaign *model.Campaign, user *model.User, inviterId int) {
	config, err := campaign.ParseCampaignConfig()
	if err != nil {
		common.SysError(fmt.Sprintf("CampaignEngine: failed to parse config for campaign %d: %v", campaign.Id, err))
		return
	}

	// Record participation, atomically enforcing MaxParticipants and
	// MaxRewardsPerUser (count checks + insert happen under a campaign row
	// lock so concurrent triggers cannot over-admit).
	participant := &model.CampaignParticipant{
		CampaignId: campaign.Id,
		UserId:     user.Id,
		EventType:  campaign.Type,
	}
	if inviterId > 0 {
		inviter, err := model.GetUserById(inviterId, false)
		if err == nil && inviter != nil {
			extra := model.ParticipantExtra{
				InviterId:   inviterId,
				InviterName: inviter.Username,
			}
			extraJson, _ := common.Marshal(extra)
			participant.ExtraJson = string(extraJson)
		}
	}
	if err := model.CreateCampaignParticipantIfAllowed(participant, config.MaxParticipants, config.MaxRewardsPerUser); err != nil {
		if errors.Is(err, model.ErrCampaignFull) || errors.Is(err, model.ErrCampaignUserRewardLimit) {
			return // limit reached, skip
		}
		common.SysError(fmt.Sprintf("CampaignEngine: failed to create participant for campaign %d: %v", campaign.Id, err))
		return
	}

	// Dispatch reward based on campaign type
	switch campaign.Type {
	case model.CampaignTypePhoneFilled:
		// phone_filled 类型由 OnPhoneFilled 入口触达本函数：用户首次填入 phone 时
		// 走 handleCampaignType → processCampaignForUser → 此分支派发奖励。
		// 通过 MaxRewardsPerUser=1 可避免 phone 清空-重填的重复发放。
		e.dispatchPhoneFilledReward(campaign, user, config)
	case model.CampaignTypeInvitation:
		e.dispatchInvitationReward(campaign, user, inviterId, config, participant.Id)
	default:
		common.SysError(fmt.Sprintf("CampaignEngine: unknown campaign type: %s", campaign.Type))
	}
}

// recordReward creates a campaign reward record and returns the created entity
// (Id 已填充)。失败时返回 nil，且仅记录日志，由调用方决定后续动作。
func (e *CampaignEngine) recordReward(campaignId int, userId int, redemptionId int, quota int, status int) *model.CampaignReward {
	reward := &model.CampaignReward{
		CampaignId:   campaignId,
		UserId:       userId,
		RedemptionId: redemptionId,
		Quota:        quota,
		Status:       status,
		DispatchedAt: common.GetTimestamp(),
	}
	if err := model.CreateCampaignReward(reward); err != nil {
		common.SysError(fmt.Sprintf("CampaignEngine: failed to record reward for campaign %d: %v", campaignId, err))
		return nil
	}
	return reward
}

// OnPhoneFilled is called when a user fills in their phone number.
// It finds matching active phone_filled campaigns and triggers reward dispatch.
func (e *CampaignEngine) OnPhoneFilled(user *model.User) {
	if user == nil || user.Id == 0 {
		return
	}

	gopool.Go(func() {
		e.handleCampaignType(model.CampaignTypePhoneFilled, user, 0)
	})
}

// dispatchPhoneFilledReward dispatches reward for phone_filled campaign type
func (e *CampaignEngine) dispatchPhoneFilledReward(campaign *model.Campaign, user *model.User, config *model.CampaignConfig) {
	quota := config.Quota
	if quota <= 0 {
		return
	}

	redemptionName := config.RedemptionName
	if redemptionName == "" {
		redemptionName = fmt.Sprintf("手机号奖励-%s", campaign.Name)
	}

	// 过期时间：config.ExpireDays > 0 时设置；0 表示不过期
	var expiredTime int64
	if config.ExpireDays > 0 {
		expiredTime = common.GetTimestamp() + int64(config.ExpireDays)*86400
	}

	key := common.GetUUID()
	redemption := &model.Redemption{
		UserId:      user.Id,
		Key:         key,
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        redemptionName,
		Quota:       quota,
		CreatedTime: common.GetTimestamp(),
		ExpiredTime: expiredTime,
	}
	if err := redemption.Insert(); err != nil {
		common.SysError(fmt.Sprintf("CampaignEngine: failed to create phone_filled redemption for user %d: %v", user.Id, err))
		e.recordReward(campaign.Id, user.Id, 0, quota, model.CampaignRewardStatusFailed)
		return
	}

	reward := e.recordReward(campaign.Id, user.Id, redemption.Id, quota, model.CampaignRewardStatusDispatched)

	model.RecordLog(user.Id, model.LogTypeTopup,
		fmt.Sprintf("通过活动「%s」获得手机号填写奖励兑换码，额度 %s", campaign.Name, logger.LogQuota(quota)))

	// 发送兑换码邮件：用户未绑定邮箱或 SMTP 未配置时静默跳过
	if reward != nil {
		gopool.Go(func() {
			SendCampaignRewardEmail(user, campaign, redemption, reward)
		})
	}
}

// buildCampaignRewardEmail builds the bilingual subject and HTML content for a
// campaign reward email. Invitation-campaign codes are redeemed and credited
// automatically at dispatch time, so their email is a credit receipt; other
// campaigns send the code with redemption instructions.
func buildCampaignRewardEmail(campaign *model.Campaign, redemption *model.Redemption) (subject string, content string) {
	enExpireDesc := "Never expires"
	zhExpireDesc := "永不过期"
	if redemption.ExpiredTime > 0 {
		timeStr := time.Unix(redemption.ExpiredTime, 0).Format("2006-01-02 15:04:05")
		enExpireDesc = timeStr
		zhExpireDesc = timeStr
	}

	if campaign.Type == model.CampaignTypeInvitation {
		zhSubject := fmt.Sprintf("%s - 活动奖励到账通知", common.SystemName)
		enSubject := fmt.Sprintf("%s - Campaign Reward Credited", common.SystemName)
		subject = common.WrapBilingualSubject(enSubject, zhSubject)
		enContent := fmt.Sprintf(
			`<p>Hello,</p>`+
				`<p>Thank you for participating in the campaign "<b>%s</b>". A reward of <b>%s</b> has been credited to your account automatically — no further action is needed.</p>`+
				`<p><b>Reference Code:</b> <code style="font-size:14px;background:#f5f5f5;padding:4px 8px;border-radius:4px;">%s</code></p>`+
				`<p style="color:#888;font-size:12px;">This email was sent automatically. Please do not reply directly.</p>`,
			campaign.Name, logger.LogQuota(redemption.Quota), redemption.Key,
		)
		zhContent := fmt.Sprintf(
			`<p>您好，</p>`+
				`<p>感谢您参与活动「<b>%s</b>」，奖励额度 <b>%s</b> 已自动充值到您的账户，无需手动兑换。</p>`+
				`<p><b>兑换码（仅作凭证）：</b><code style="font-size:14px;background:#f5f5f5;padding:4px 8px;border-radius:4px;">%s</code></p>`+
				`<p style="color:#888;font-size:12px;">此邮件由系统自动发送，请勿直接回复。</p>`,
			campaign.Name, logger.LogQuota(redemption.Quota), redemption.Key,
		)
		return subject, common.WrapBilingualContent(enContent, zhContent)
	}

	zhSubject := fmt.Sprintf("%s - 活动兑换码", common.SystemName)
	enSubject := fmt.Sprintf("%s - Campaign Redemption Code", common.SystemName)
	subject = common.WrapBilingualSubject(enSubject, zhSubject)
	enContent := fmt.Sprintf(
		`<p>Hello,</p>`+
			`<p>Thank you for participating in the campaign "<b>%s</b>". Here is your redemption code:</p>`+
			`<p><b>Redemption Code:</b> <code style="font-size:14px;background:#f5f5f5;padding:4px 8px;border-radius:4px;">%s</code></p>`+
			`<p><b>Quota:</b> %s</p>`+
			`<p><b>Valid until:</b> %s</p>`+
			`<p>Please enter the redemption code above on the "Redemption Code" page to top up your account.</p>`+
			`<p style="color:#888;font-size:12px;">This email was sent automatically. Please do not reply directly.</p>`,
		campaign.Name, redemption.Key, logger.LogQuota(redemption.Quota), enExpireDesc,
	)
	zhContent := fmt.Sprintf(
		`<p>您好，</p>`+
			`<p>感谢您参与活动「<b>%s</b>」，以下是您的兑换码：</p>`+
			`<p><b>兑换码：</b><code style="font-size:14px;background:#f5f5f5;padding:4px 8px;border-radius:4px;">%s</code></p>`+
			`<p><b>额度：</b>%s</p>`+
			`<p><b>有效期至：</b>%s</p>`+
			`<p>请在系统的「兑换码」页面输入上方兑换码完成充值。</p>`+
			`<p style="color:#888;font-size:12px;">此邮件由系统自动发送，请勿直接回复。</p>`,
		campaign.Name, redemption.Key, logger.LogQuota(redemption.Quota), zhExpireDesc,
	)
	return subject, common.WrapBilingualContent(enContent, zhContent)
}

// SendCampaignRewardEmail 发送/补发活动兑换码邮件。
//   - 用户未绑定邮箱 → 视为业务忽略（不写入失败状态，不返回错误）
//   - SMTP 失败 → 写入 EmailError；调用方据返回 error 决定提示
//   - 成功 → 写入 EmailSentAt，清空 EmailError
func SendCampaignRewardEmail(user *model.User, campaign *model.Campaign, redemption *model.Redemption, reward *model.CampaignReward) error {
	if user == nil || user.Email == "" {
		return nil // 无邮箱，跳过
	}
	subject, content := buildCampaignRewardEmail(campaign, redemption)
	if err := common.SendEmail(subject, user.Email, content); err != nil {
		_ = model.MarkRewardEmailFailed(reward.Id, err.Error())
		return err
	}
	_ = model.MarkRewardEmailSent(reward.Id, common.GetTimestamp())
	return nil
}

// OnInvitationRegister is called when a new user registers with an inviter (aff code).
// It finds matching active invitation campaigns for the inviter and triggers reward dispatch.
func (e *CampaignEngine) OnInvitationRegister(user *model.User, inviterId int) {
	if user == nil || user.Id == 0 || inviterId == 0 {
		return
	}

	gopool.Go(func() {
		e.handleCampaignType(model.CampaignTypeInvitation, user, inviterId)
	})
}

// dispatchInvitationReward dispatches reward for invitation campaign type.
// It atomically picks one available redemption code from the campaign's pool
// for the invitee, credits the new user's quota, and records the reward.
// Only dispatches if config.InviteeUserId matches the actual inviterId.
// participantId is the admission slot created before dispatch; it is released
// when the pool has no available code so the user is not counted against the
// campaign limits without receiving a reward.
func (e *CampaignEngine) dispatchInvitationReward(campaign *model.Campaign, user *model.User, inviterId int, config *model.CampaignConfig, participantId int) {
	if config.InviteeUserId == 0 {
		return
	}

	// Only dispatch from the campaign that belongs to the actual inviter
	if config.InviteeUserId != inviterId {
		return
	}

	// Atomically find one available code and dispatch it to the user
	redemption, err := model.DispatchRedemptionToUser(campaign.Id, config.InviteeUserId, user.Id)
	if err != nil {
		common.SysError(fmt.Sprintf("CampaignEngine: failed to dispatch invitation code for invitee %d to user %d: %v", config.InviteeUserId, user.Id, err))
		e.recordReward(campaign.Id, user.Id, 0, config.Quota, model.CampaignRewardStatusFailed)
		return
	}
	if redemption == nil {
		common.SysLog(fmt.Sprintf("CampaignEngine: no available invitation code for invitee %d", config.InviteeUserId))
		// Release the admission slot: without a dispatched code the user must
		// not count against MaxParticipants / MaxRewardsPerUser, otherwise a
		// later pool top-up could never reach them.
		if err := model.DeleteCampaignParticipantById(participantId); err != nil {
			common.SysError(fmt.Sprintf("CampaignEngine: failed to release participant %d for campaign %d: %v", participantId, campaign.Id, err))
		}
		return
	}

	quotaAdded := redemption.Quota
	reward := e.recordReward(campaign.Id, user.Id, redemption.Id, quotaAdded, model.CampaignRewardStatusDispatched)

	model.RecordLog(user.Id, model.LogTypeTopup,
		fmt.Sprintf("通过邀请活动「%s」自动获得兑换码充值，额度 %s", campaign.Name, logger.LogQuota(quotaAdded)))

	// Send email notification if user has email bound
	if reward != nil {
		gopool.Go(func() {
			SendCampaignRewardEmail(user, campaign, redemption, reward)
		})
	}
}

// GenerateInvitationCodes generates batch redemption codes for an invitation campaign.
// The codes are prefixed with the invitee user's username.
// Only generates codes if there are fewer available codes than codeCount (incremental).
// Called after creating or updating an invitation campaign.
// Returns the number of codes actually generated.
func (e *CampaignEngine) GenerateInvitationCodes(campaign *model.Campaign) (int, error) {
	config, err := campaign.ParseCampaignConfig()
	if err != nil {
		return 0, fmt.Errorf("解析活动配置失败: %w", err)
	}

	if config.InviteeUserId == 0 {
		return 0, errors.New("邀请活动缺少关联用户")
	}

	inviteeUser, err := model.GetUserById(config.InviteeUserId, false)
	if err != nil || inviteeUser == nil {
		return 0, errors.New("关联用户不存在")
	}

	codeCount := config.CodeCount
	if codeCount <= 0 {
		codeCount = 1
	}
	// Cap at 1000 codes per generation to prevent abuse
	if codeCount > 1000 {
		codeCount = 1000
	}

	// Incremental generation: check how many available codes already exist
	available, err := model.CountAvailableRedemptionCodesByOwner(campaign.Id, config.InviteeUserId)
	if err != nil {
		return 0, fmt.Errorf("查询已有兑换码失败: %w", err)
	}
	needGenerate := codeCount - int(available)
	if needGenerate <= 0 {
		// Already have enough codes, skip generation
		common.SysLog(fmt.Sprintf("GenerateInvitationCodes: campaign %d already has %d available codes (need %d), skipping",
			campaign.Id, available, codeCount))
		return 0, nil
	}

	quota := config.Quota
	if quota <= 0 {
		return 0, errors.New("奖励额度必须大于0")
	}

	prefix := inviteeUser.Username + "-"
	// Ensure key fits within char(32): prefix + UUID[:8] must be <= 32 chars
	// UUID[:8] = 8 chars, so prefix max = 24 chars (includes the "-")
	maxPrefixLen := 24
	if len(prefix) > maxPrefixLen {
		prefix = prefix[:maxPrefixLen]
	}
	redemptionName := config.RedemptionName
	if redemptionName == "" {
		redemptionName = fmt.Sprintf("邀请活动-%s", campaign.Name)
	}

	var expiredTime int64
	if config.ExpireDays > 0 {
		expiredTime = common.GetTimestamp() + int64(config.ExpireDays)*86400
	}

	created := 0
	for i := 0; i < needGenerate; i++ {
		key := prefix + common.GetUUID()[:8]
		redemption := &model.Redemption{
			UserId:       config.InviteeUserId,
			Key:          key,
			Status:       common.RedemptionCodeStatusEnabled,
			Name:         redemptionName,
			Quota:        quota,
			CreatedTime:  common.GetTimestamp(),
			ExpiredTime:  expiredTime,
			OwnerAdminId: config.InviteeUserId, // track which invitee's pool this belongs to
			CampaignId:   campaign.Id,          // scope the pool to this campaign
		}
		if err := redemption.Insert(); err != nil {
			common.SysError(fmt.Sprintf("GenerateInvitationCodes: failed to create code %d for campaign %d: %v", i, campaign.Id, err))
			// Continue creating remaining codes
			continue
		}
		created++
	}

	// Count actual success
	finalAvailable, _ := model.CountAvailableRedemptionCodesByOwner(campaign.Id, config.InviteeUserId)

	model.RecordCampaignLogf(campaign.CreatedBy, model.LogTypeManage,
		"邀请活动「%s」生成了 %d 个兑换码（当前共 %d 个可用），前缀: %s", campaign.Name, created, finalAvailable, prefix)

	return created, nil
}
