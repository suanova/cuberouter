package controller

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCampaignTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled = oldRedisEnabled
	})
	// Drain in-flight campaign dispatch goroutines before restoring the global
	// handles: cleanup is LIFO, so this drain runs before the restore above.
	t.Cleanup(service.DrainCampaignDispatches)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Redemption{}, &model.Log{},
		&model.Campaign{}, &model.CampaignParticipant{}, &model.CampaignReward{}))
}

type campaignResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// callCampaignHandler builds a gin context with a JSON body plus the given context
// values/path params, invokes the handler directly (no middleware), and parses the body.
func callCampaignHandler(t *testing.T, method, target string, ctxVals map[string]any,
	params map[string]string, body any, handler gin.HandlerFunc) campaignResponse {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var raw []byte
	if body != nil {
		var err error
		raw, err = common.Marshal(body)
		require.NoError(t, err)
	}
	c.Request = httptest.NewRequest(method, target, bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	for k, v := range ctxVals {
		c.Set(k, v)
	}
	for k, v := range params {
		c.Params = append(c.Params, gin.Param{Key: k, Value: v})
	}
	handler(c)
	var resp campaignResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

var campaignCtlUserSeq = 950000

func seedCampaignCtlUser(t *testing.T, email string) *model.User {
	t.Helper()
	campaignCtlUserSeq++
	id := campaignCtlUserSeq
	username := fmt.Sprintf("campaign-ctl-%d", id)
	user := &model.User{
		Id: id, Username: username, Password: "x", DisplayName: username,
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		AffCode: fmt.Sprintf("aff-%d", id), Email: email,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func TestAddCampaign(t *testing.T) {
	setupCampaignTestDB(t)
	gin.SetMode(gin.TestMode)
	admin := seedCampaignCtlUser(t, "")
	ctx := map[string]any{"id": admin.Id}

	t.Run("validation matrix", func(t *testing.T) {
		invitee := seedCampaignCtlUser(t, "")
		cases := []struct {
			name        string
			body        map[string]any
			wantSuccess bool
			wantMessage string // empty = assert success flag only
		}{
			{"empty name", map[string]any{"name": "", "type": "phone_filled"}, false, "活动名称不能为空"},
			{"empty type", map[string]any{"name": "x", "type": ""}, false, "活动类型不能为空"},
			{"invalid type", map[string]any{"name": "x", "type": "bogus"}, false, ""},
			{"invitation without invitee", map[string]any{
				"name": "x", "type": "invitation", "config_json": `{"quota":100}`}, false, "邀请活动必须指定关联用户"},
			{"invitation with nonexistent invitee", map[string]any{
				"name": "x", "type": "invitation", "config_json": `{"quota":100,"invitee_user_id":99999999}`}, false, "关联用户不存在"},
			{"invitation with zero quota", map[string]any{
				"name": "x", "type": "invitation",
				"config_json": fmt.Sprintf(`{"quota":0,"invitee_user_id":%d}`, invitee.Id)}, false, "奖励额度必须大于0"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				resp := callCampaignHandler(t, "POST", "/api/campaign/", ctx, nil, tc.body, AddCampaign)
				assert.False(t, resp.Success)
				if tc.wantMessage != "" {
					assert.Contains(t, resp.Message, tc.wantMessage)
				}
			})
		}
	})

	t.Run("defaults to draft and forces redemption_count=1", func(t *testing.T) {
		invitee := seedCampaignCtlUser(t, "")
		resp := callCampaignHandler(t, "POST", "/api/campaign/", ctx, nil, map[string]any{
			"name": "inv-draft", "type": "invitation",
			"config_json": fmt.Sprintf(
				`{"quota":100,"invitee_user_id":%d,"code_count":2,"redemption_count":5}`, invitee.Id),
		}, AddCampaign)
		require.True(t, resp.Success, resp.Message)

		var campaign model.Campaign
		require.NoError(t, model.DB.First(&campaign).Error)
		assert.Equal(t, model.CampaignStatusDraft, campaign.Status, "status 0 defaults to Draft")
		cfg, err := campaign.ParseCampaignConfig()
		require.NoError(t, err)
		assert.Equal(t, 1, cfg.RedemptionCount, "invitation config forces redemption_count=1")
		available, err := model.CountAvailableRedemptionCodesByOwner(campaign.Id, invitee.Id)
		require.NoError(t, err)
		assert.Zero(t, available, "draft campaigns must not generate codes")
	})

	t.Run("active invitation generates its code pool", func(t *testing.T) {
		invitee := seedCampaignCtlUser(t, "")
		resp := callCampaignHandler(t, "POST", "/api/campaign/", ctx, nil, map[string]any{
			"name": "inv-active", "type": "invitation", "status": model.CampaignStatusActive,
			"start_at":    common.GetTimestamp() - 60,
			"config_json": fmt.Sprintf(`{"quota":100,"invitee_user_id":%d,"code_count":2}`, invitee.Id),
		}, AddCampaign)
		require.True(t, resp.Success, resp.Message)

		var created model.Campaign
		require.NoError(t, model.DB.Where("name = ?", "inv-active").First(&created).Error)
		available, err := model.CountAvailableRedemptionCodesByOwner(created.Id, invitee.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(2), available, "active invitation campaign generates code_count codes on create")
	})
}

func TestUpdateCampaignStatus(t *testing.T) {
	setupCampaignTestDB(t)
	gin.SetMode(gin.TestMode)
	admin := seedCampaignCtlUser(t, "")
	ctx := map[string]any{"id": admin.Id}
	invitee := seedCampaignCtlUser(t, "")

	campaign := &model.Campaign{
		Name: "status-target", Type: model.CampaignTypeInvitation, Status: model.CampaignStatusDraft,
		ConfigJson: fmt.Sprintf(`{"quota":100,"invitee_user_id":%d,"code_count":2}`, invitee.Id),
	}
	require.NoError(t, campaign.Insert())
	params := map[string]string{"id": fmt.Sprintf("%d", campaign.Id)}

	resp := callCampaignHandler(t, "PUT", "/api/campaign/:id/status", ctx, params,
		map[string]any{"status": 99}, UpdateCampaignStatus)
	assert.False(t, resp.Success, "status 99 must be rejected")

	resp = callCampaignHandler(t, "PUT", "/api/campaign/:id/status", ctx, params,
		map[string]any{"status": model.CampaignStatusActive}, UpdateCampaignStatus)
	require.True(t, resp.Success, resp.Message)
	available, err := model.CountAvailableRedemptionCodesByOwner(campaign.Id, invitee.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(2), available, "activating an invitation campaign generates its codes")
}

func TestResendCampaignRewardEmail(t *testing.T) {
	setupCampaignTestDB(t)
	gin.SetMode(gin.TestMode)
	admin := seedCampaignCtlUser(t, "")
	ctx := map[string]any{"id": admin.Id}
	oldServer, oldAccount := common.SMTPServer, common.SMTPAccount
	common.SMTPServer, common.SMTPAccount = "", ""
	t.Cleanup(func() { common.SMTPServer, common.SMTPAccount = oldServer, oldAccount })

	campaign := &model.Campaign{Name: "resend", Type: model.CampaignTypePhoneFilled, Status: model.CampaignStatusActive}
	require.NoError(t, campaign.Insert())
	paramsFor := func(id int) map[string]string { return map[string]string{"id": fmt.Sprintf("%d", id)} }

	seedReward := func(status, redemptionId int, userId int) *model.CampaignReward {
		t.Helper()
		r := &model.CampaignReward{CampaignId: campaign.Id, UserId: userId, RedemptionId: redemptionId,
			Quota: 10, Status: status, DispatchedAt: common.GetTimestamp()}
		require.NoError(t, model.CreateCampaignReward(r))
		return r
	}

	pending := seedReward(model.CampaignRewardStatusPending, 0, admin.Id)
	resp := callCampaignHandler(t, "POST", "/api/campaign/rewards/:id/resend", ctx, paramsFor(pending.Id), nil, ResendCampaignRewardEmail)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "该奖励未成功发放兑换码，无法补发邮件")

	noCode := seedReward(model.CampaignRewardStatusDispatched, 0, admin.Id)
	resp = callCampaignHandler(t, "POST", "/api/campaign/rewards/:id/resend", ctx, paramsFor(noCode.Id), nil, ResendCampaignRewardEmail)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "该奖励未成功发放兑换码，无法补发邮件")

	userNoEmail := seedCampaignCtlUser(t, "")
	code := &model.Redemption{UserId: userNoEmail.Id, Key: common.GetUUID(), Status: common.RedemptionCodeStatusEnabled,
		Name: "x", Quota: 10, CreatedTime: common.GetTimestamp()}
	require.NoError(t, code.Insert())
	noEmail := seedReward(model.CampaignRewardStatusDispatched, code.Id, userNoEmail.Id)
	resp = callCampaignHandler(t, "POST", "/api/campaign/rewards/:id/resend", ctx, paramsFor(noEmail.Id), nil, ResendCampaignRewardEmail)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "目标用户未绑定邮箱")

	userWithEmail := seedCampaignCtlUser(t, "resend@example.com")
	withEmail := seedReward(model.CampaignRewardStatusDispatched, code.Id, userWithEmail.Id)
	resp = callCampaignHandler(t, "POST", "/api/campaign/rewards/:id/resend", ctx, paramsFor(withEmail.Id), nil, ResendCampaignRewardEmail)
	assert.False(t, resp.Success, "SMTP is unconfigured: the send failure must be surfaced")
	assert.Contains(t, resp.Message, "邮件发送失败：")
	reloaded, err := model.GetCampaignRewardById(withEmail.Id)
	require.NoError(t, err)
	assert.NotEmpty(t, reloaded.EmailError, "the send error must be persisted for admin visibility")
}

func TestUpdateCampaign(t *testing.T) {
	setupCampaignTestDB(t)
	gin.SetMode(gin.TestMode)
	admin := seedCampaignCtlUser(t, "")
	ctx := map[string]any{"id": admin.Id}

	newCampaign := func() *model.Campaign {
		campaign := &model.Campaign{
			Name: "orig", Description: "desc", Type: model.CampaignTypePhoneFilled,
			Status: model.CampaignStatusDraft, StartAt: 111, EndAt: 222,
			ConfigJson: `{"quota":50,"expire_days":7}`,
		}
		require.NoError(t, campaign.Insert())
		return campaign
	}

	t.Run("rejects a type change", func(t *testing.T) {
		campaign := newCampaign()
		resp := callCampaignHandler(t, "PUT", "/api/campaign/", ctx, nil, map[string]any{
			"id": campaign.Id, "name": "orig", "type": model.CampaignTypeInvitation,
		}, UpdateCampaign)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "活动类型创建后不可修改")
	})

	t.Run("partial request preserves omitted fields", func(t *testing.T) {
		campaign := newCampaign()
		resp := callCampaignHandler(t, "PUT", "/api/campaign/", ctx, nil, map[string]any{
			"id": campaign.Id, "name": "renamed",
		}, UpdateCampaign)
		require.True(t, resp.Success, resp.Message)

		reloaded, err := model.GetCampaignById(campaign.Id)
		require.NoError(t, err)
		assert.Equal(t, "renamed", reloaded.Name)
		assert.Equal(t, "desc", reloaded.Description, "omitted description must survive a partial update")
		assert.Equal(t, model.CampaignStatusDraft, reloaded.Status)
		assert.Equal(t, int64(111), reloaded.StartAt)
		assert.Equal(t, int64(222), reloaded.EndAt)
		assert.Equal(t, `{"quota":50,"expire_days":7}`, reloaded.ConfigJson)
	})

	t.Run("explicit zero values still apply", func(t *testing.T) {
		campaign := newCampaign()
		resp := callCampaignHandler(t, "PUT", "/api/campaign/", ctx, nil, map[string]any{
			"id": campaign.Id, "name": "orig", "end_at": 0, "description": "",
		}, UpdateCampaign)
		require.True(t, resp.Success, resp.Message)

		reloaded, err := model.GetCampaignById(campaign.Id)
		require.NoError(t, err)
		assert.Zero(t, reloaded.EndAt, "an explicitly supplied 0 must clear the end time")
		assert.Empty(t, reloaded.Description)
	})

	t.Run("rejects invalid merged status", func(t *testing.T) {
		campaign := newCampaign()
		resp := callCampaignHandler(t, "PUT", "/api/campaign/", ctx, nil, map[string]any{
			"id": campaign.Id, "status": 99,
		}, UpdateCampaign)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "无效的活动状态")
	})
}
