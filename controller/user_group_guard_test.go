package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Register（邀请继承分组）与 UpdateUser 的改分组守卫在 GetUserGroupByIdTx
// 锁定的同一行上串行化。下面的顺序场景覆盖了两个事务的全部可能交错，
// 验证不变量：邀请人与下级分组始终一致，且已有邀请记录的用户不能改分组。

func TestUpdateUserGroupGuard_RejectsGroupChangeWhenInviteeExists(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, authz.Init(db)) // UpdateUser touches the authz enforcer
	inviter := model.User{Username: "guard-inviter", Password: "password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "GUARD-AFF1"}
	require.NoError(t, db.Create(&inviter).Error)
	invitee := model.User{Username: "guard-invitee", Password: "password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		InviterId: inviter.Id, AffCode: "GUARD-AFF2"}
	require.NoError(t, db.Create(&invitee).Error)

	rec := performUpdateUserGroup(t, inviter.Id, "vip")
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success, "group change of a user with invitees must be rejected")
	assert.Equal(t, i18n.MsgUserGroupModifyForbidden, resp.Message)

	var after model.User
	require.NoError(t, db.First(&after, inviter.Id).Error)
	assert.Equal(t, "default", after.Group, "group must stay unchanged")
}

func TestUpdateUserGroupGuard_AllowsGroupChangeWithoutInvitees(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, authz.Init(db)) // UpdateUser touches the authz enforcer
	user := model.User{Username: "guard-no-invitee", Password: "password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(&user).Error)

	rec := performUpdateUserGroup(t, user.Id, "vip")
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success, resp.Message)

	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, "vip", after.Group)
}

// 注册继承邀请人最新分组：改分组成功后注册的下级落在新分组。
func TestRegisterInheritsGroupAfterInviterGroupChange(t *testing.T) {
	setupCampaignTestDB(t)
	gin.SetMode(gin.TestMode)
	// setupCampaignTestDB does not migrate the authz/user-session tables;
	// UpdateUser touches them, so migrate and initialize the enforcer here.
	require.NoError(t, model.DB.AutoMigrate(&model.CasbinRule{}, &model.AuthzRole{}, &model.UserSession{}))
	require.NoError(t, authz.Init(model.DB))

	oldRegister, oldPwdReg := common.RegisterEnabled, common.PasswordRegisterEnabled
	oldEmailVerify := common.EmailVerificationEnabled
	oldQuota, oldGenToken := common.QuotaForNewUser, constant.GenerateDefaultToken
	common.RegisterEnabled, common.PasswordRegisterEnabled = true, true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser, constant.GenerateDefaultToken = 0, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled = oldRegister, oldPwdReg
		common.EmailVerificationEnabled = oldEmailVerify
		common.QuotaForNewUser, constant.GenerateDefaultToken = oldQuota, oldGenToken
	})

	inviter := model.User{Username: "guard-inviter-2", Password: "password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "GUARD-AFF2"}
	require.NoError(t, model.DB.Create(&inviter).Error)

	// 先改分组（无邀请记录，允许）
	rec := performUpdateUserGroup(t, inviter.Id, "vip")
	require.True(t, strings.Contains(rec.Body.String(), `"success":true`))

	// 再经邀请码注册：invitee 必须继承最新分组 vip
	engine := gin.New()
	engine.POST("/api/user/register", Register)
	body, err := common.Marshal(map[string]any{
		"username": "guard-new-user", "password": "Secret-password1",
		"aff_code": inviter.AffCode,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/user/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	var regResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &regResp))
	require.True(t, regResp.Success, regResp.Message)

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "guard-new-user").First(&registered).Error)
	assert.Equal(t, "vip", registered.Group, "invitee must inherit the inviter's latest group")
	assert.Equal(t, inviter.Id, registered.InviterId)
}

// performUpdateUserGroup calls UpdateUser as a root admin to change the target's group.
func performUpdateUserGroup(t *testing.T, id int, group string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := fmt.Sprintf(`{"id":%d,"username":"x","role":1,"group":"%s"}`, id, group)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/user/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 9999)
	c.Set("role", common.RoleRootUser)
	c.Set("username", "operator")
	UpdateUser(c)
	return recorder
}
