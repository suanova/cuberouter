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
package controller

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// registerWithVerifiedEmail 走完整的注册处理器：注册验证码、带上它发请求。
func registerWithVerifiedEmail(t *testing.T, username, email, code string) *httptest.ResponseRecorder {
	t.Helper()
	require.NoError(t, common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose))
	engine := gin.New()
	engine.POST("/api/user/register", Register)
	body, err := common.Marshal(map[string]any{
		"username":          username,
		"password":          "Secret-password1",
		"email":             email,
		"verification_code": code,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/user/register", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	return recorder
}

func setupRegisterJoinTestEnv(t *testing.T) {
	t.Helper()
	oldRegister, oldPwdReg := common.RegisterEnabled, common.PasswordRegisterEnabled
	oldEmailVerify := common.EmailVerificationEnabled
	oldQuota, oldGenToken := common.QuotaForNewUser, constant.GenerateDefaultToken
	common.RegisterEnabled, common.PasswordRegisterEnabled = true, true
	common.EmailVerificationEnabled = true
	common.QuotaForNewUser, constant.GenerateDefaultToken = 0, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled = oldRegister, oldPwdReg
		common.EmailVerificationEnabled = oldEmailVerify
		common.QuotaForNewUser, constant.GenerateDefaultToken = oldQuota, oldGenToken
	})
}

// 只在邮箱验证分支内、且邮箱已被验证码证明过，才允许自动加入组织。
func TestRegisterAutoJoinsWhitelistedDomain(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)

	owner := model.User{Username: "auto-join-owner", AffCode: "auto-join-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "Auto Join Org", Slug: "auto-join-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "*.enterprise.com",
		PatternNormalized: "*.enterprise.com",
		CreatedBy:         owner.Id,
	}).Error)

	recorder := registerWithVerifiedEmail(t, "auto-join-user", "newcomer@mail.enterprise.com", "111111")
	require.Equal(t, 200, recorder.Code, recorder.Body.String())
	// 响应不泄露组织：未认证调用者不该能通过注册接口探测组织是否存在。
	assert.NotContains(t, recorder.Body.String(), organization.Name)

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "auto-join-user").First(&registered).Error)
	var member model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, registered.Id).First(&member).Error)
	assert.Equal(t, model.OrganizationRoleMember, member.Role)
	assert.Equal(t, model.OrganizationMemberStatusActive, member.Status)
}

// 非白名单邮箱照常注册，但不进任何组织。
func TestRegisterLeavesUnmatchedEmailPersonal(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)

	recorder := registerWithVerifiedEmail(t, "personal-user", "outsider@example.com", "222222")
	require.Equal(t, 200, recorder.Code, recorder.Body.String())

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "personal-user").First(&registered).Error)
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", registered.Id).Count(&count).Error)
	assert.Zero(t, count)
}

// 命中规则但组织已停用：注册必须成功，只是不加入——一条失效规则不能把整个域的
// 注册全部卡死。
func TestRegisterSucceedsWhenMatchedOrganizationIsDisabled(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)

	owner := model.User{Username: "disabled-org-owner", AffCode: "disabled-org-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "Disabled Join Org", Slug: "disabled-join-org", Status: model.OrganizationStatusDisabled, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "*.enterprise.com",
		PatternNormalized: "*.enterprise.com",
		CreatedBy:         owner.Id,
	}).Error)

	recorder := registerWithVerifiedEmail(t, "disabled-org-user", "newcomer@enterprise.com", "333333")
	require.Equal(t, 200, recorder.Code, recorder.Body.String())

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "disabled-org-user").First(&registered).Error)
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", registered.Id).Count(&count).Error)
	assert.Zero(t, count)
}

// 邮箱验证关闭时不存在已验证的邮箱，规则不得生效。
func TestRegisterSkipsAutoJoinWhenEmailVerificationIsOff(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)
	common.EmailVerificationEnabled = false

	owner := model.User{Username: "no-verify-owner", AffCode: "no-verify-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "No Verify Org", Slug: "no-verify-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "*.enterprise.com",
		PatternNormalized: "*.enterprise.com",
		CreatedBy:         owner.Id,
	}).Error)

	// 邮箱验证关闭时注册不需要验证码。
	engine := gin.New()
	engine.POST("/api/user/register", Register)
	body, err := common.Marshal(map[string]any{
		"username": "no-verify-user",
		"password": "Secret-password1",
		"email":    "newcomer@enterprise.com",
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/user/register", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	require.Equal(t, 200, recorder.Code, recorder.Body.String())

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "no-verify-user").First(&registered).Error)
	assert.Equal(t, "", registered.Email, "关闭邮箱验证时邮箱不入库")
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", registered.Id).Count(&count).Error)
	assert.Zero(t, count)
}

// 加入失败必须把整个注册事务回滚干净：不只是没有成员关系，连用户行都不能留下——
// 半个人不应该存在，否则这个用户名/邮箱此后既注册不了，又对应一个不存在的账号。
func TestRegisterRollsBackUserWhenAutoJoinFails(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)

	owner := model.User{Username: "rollback-owner", AffCode: "rollback-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "Rollback Org", Slug: "rollback-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "*.enterprise.com",
		PatternNormalized: "*.enterprise.com",
		CreatedBy:         owner.Id,
	}).Error)

	// 删掉规则表是为了逼出一次真实的数据库错误：加入路径只把这种错误往上抛
	// （组织停用、邮箱无命中、解析歧义都只跳过），而解析器只在邮箱非空时查这张表，
	// 所以删除它正好只打断加入这一段。
	require.NoError(t, model.DB.Migrator().DropTable(&model.OrganizationJoinRule{}))

	recorder := registerWithVerifiedEmail(t, "rollback-user", "newcomer@enterprise.com", "444444")
	require.Equal(t, 200, recorder.Code, recorder.Body.String())
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success, "加入失败时注册必须整体失败：%s", recorder.Body.String())
	// 失败必须来自加入那一步（错误里点名了被删的表）：否则一旦将来有更早的校验
	// 把请求挡在插入之前，这个用例会在"根本没走到插入"的情况下假通过。
	assert.Contains(t, response.Message, "organization_join_rules")

	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("username = ?", "rollback-user").Count(&count).Error)
	assert.Zero(t, count, "加入失败必须连用户行一起回滚")
}

// OAuth 的创建路径不走自动加入：provider 给的邮箱没有任何验证证明，一旦挂上去，
// 任何人都能用未验证的邮箱冒充组织成员——而成员能读到组织的 API key（花组织的额度）。
// 这里不搭假的 OAuth provider，而是照着 controller/oauth.go 的创建块走一遍它调用的
// 那两步模型调用（事务内 InsertWithTx，提交后 FinalizeOAuthUserCreation）；
// 邮箱命中一条有效规则，且邮箱验证开着（规则最可能生效的配置），断言零成员关系。
func TestOAuthUserCreationPathDoesNotAutoJoin(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)

	owner := model.User{Username: "oauth-owner", AffCode: "oauth-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "OAuth Org", Slug: "oauth-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "*.enterprise.com",
		PatternNormalized: "*.enterprise.com",
		CreatedBy:         owner.Id,
	}).Error)

	user := model.User{
		Username:    "oauth-created-user",
		DisplayName: "OAuth User",
		Email:       "unverified@enterprise.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		return user.InsertWithTx(tx, 0)
	}))
	user.FinalizeOAuthUserCreation(0)

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "oauth-created-user").First(&registered).Error)
	require.Equal(t, "unverified@enterprise.com", registered.Email, "OAuth 路径照常写入邮箱")
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", registered.Id).Count(&count).Error)
	assert.Zero(t, count, "公共创建路径不得按邮箱自动加入组织")
}
