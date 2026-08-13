package controller

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterTriggersCampaigns(t *testing.T) {
	setupCampaignTestDB(t) // defined in controller/campaign_test.go (same package)
	gin.SetMode(gin.TestMode)

	// The shared-cache in-memory DSN repeats across -count=N iterations and the
	// seeded ids below are fixed (960001), so the DB must die with this test for
	// the next iteration to start empty. Cleanup order is LIFO: this close runs
	// first, then setupCampaignTestDB restores the global DB handles.
	testDB, err := model.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = testDB.Close() })

	oldRegister, oldPwdReg := common.RegisterEnabled, common.PasswordRegisterEnabled
	oldEmailVerify, oldTurnstile := common.EmailVerificationEnabled, common.TurnstileCheckEnabled
	oldQuota, oldGenToken := common.QuotaForNewUser, constant.GenerateDefaultToken
	common.RegisterEnabled, common.PasswordRegisterEnabled = true, true
	common.EmailVerificationEnabled, common.TurnstileCheckEnabled = false, false
	common.QuotaForNewUser, constant.GenerateDefaultToken = 0, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled = oldRegister, oldPwdReg
		common.EmailVerificationEnabled, common.TurnstileCheckEnabled = oldEmailVerify, oldTurnstile
		common.QuotaForNewUser, constant.GenerateDefaultToken = oldQuota, oldGenToken
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("campaign-trigger-test"))))
	engine.POST("/api/user/register", Register)

	inviter := &model.User{
		Id: 960001, Username: "trig-inviter", Password: "x", DisplayName: "trig-inviter",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "AFF-TRIG",
	}
	require.NoError(t, model.DB.Create(inviter).Error)

	invCampaign := &model.Campaign{Name: "trig-inv", Type: model.CampaignTypeInvitation,
		Status: model.CampaignStatusActive, StartAt: common.GetTimestamp() - 60,
		ConfigJson: fmt.Sprintf(`{"quota":200,"invitee_user_id":%d,"code_count":2}`, inviter.Id)}
	require.NoError(t, invCampaign.Insert())
	n, err := service.CampaignEngineInstance.GenerateInvitationCodes(invCampaign)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	phoneCampaign := &model.Campaign{Name: "trig-phone", Type: model.CampaignTypePhoneFilled,
		Status: model.CampaignStatusActive, StartAt: common.GetTimestamp() - 60,
		ConfigJson: `{"quota":50}`}
	require.NoError(t, phoneCampaign.Insert())

	body, err := common.Marshal(map[string]any{
		"username": "trig-new-user", "password": "Secret-password1",
		"phone": "13800009999", "aff_code": inviter.AffCode,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/user/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "trig-new-user").First(&registered).Error)
	assert.Equal(t, "13800009999", registered.Phone, "Phone must be bound and persisted by Register")

	require.Eventually(t, func() bool {
		var participants, rewards int64
		model.DB.Model(&model.CampaignParticipant{}).Count(&participants)
		model.DB.Model(&model.CampaignReward{}).Count(&rewards)
		return participants == 2 && rewards == 2
	}, 3*time.Second, 20*time.Millisecond)

	// Invitation dispatch credits the pool quota; phone_filled only mints a code.
	var after model.User
	require.NoError(t, model.DB.First(&after, registered.Id).Error)
	assert.Equal(t, 200, after.Quota)

	available, err := model.CountAvailableRedemptionCodesByOwner(invCampaign.Id, inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(1), available, "exactly one pool code consumed")

	var codes int64
	require.NoError(t, model.DB.Model(&model.Redemption{}).
		Where("user_id = ? AND owner_admin_id = 0", registered.Id).Count(&codes).Error)
	assert.Equal(t, int64(1), codes, "phone_filled minted one code directly to the new user")
}
