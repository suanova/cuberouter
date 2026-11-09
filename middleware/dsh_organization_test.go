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
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dshOrganizationFixture struct {
	user       model.User
	firstOrg   model.Organization
	firstChan  model.Channel
	secondOrg  model.Organization
	secondChan model.Channel
	unboundOrg model.Organization
	otherChan  model.Channel
	outsider   model.User
}

// setupDshOrganizationFixture 建两个已开通组织（各自绑定一个渠道）和一个
// 在用户白名单里但没有绑定渠道的组织：后者用于验证"未开通则回退"。
func setupDshOrganizationFixture(t *testing.T) dshOrganizationFixture {
	t.Helper()
	setupOrganizationMiddlewareTestDB(t,
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.Channel{},
	)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })

	fixture := dshOrganizationFixture{}
	fixture.user = createDshMiddlewareTestUser(t, "dsh-org-member", common.RoleCommonUser)
	fixture.outsider = createDshMiddlewareTestUser(t, "dsh-org-outsider", common.RoleCommonUser)
	fixture.firstOrg = model.Organization{Name: "DSH First", Slug: "dsh-middleware-first", Status: model.OrganizationStatusActive}
	fixture.secondOrg = model.Organization{Name: "DSH Second", Slug: "dsh-middleware-second", Group: "vip", Status: model.OrganizationStatusActive}
	fixture.unboundOrg = model.Organization{Name: "DSH Unbound", Slug: "dsh-middleware-unbound", Status: model.OrganizationStatusActive}
	for _, organization := range []*model.Organization{&fixture.firstOrg, &fixture.secondOrg, &fixture.unboundOrg} {
		require.NoError(t, model.DB.Create(organization).Error)
	}
	for _, organizationId := range []int{fixture.firstOrg.Id, fixture.secondOrg.Id, fixture.unboundOrg.Id} {
		require.NoError(t, model.DB.Create(&model.OrganizationMember{
			OrganizationId: organizationId,
			UserId:         fixture.user.Id,
			Role:           model.OrganizationRoleMember,
			Status:         model.OrganizationMemberStatusActive,
		}).Error)
	}

	fixture.firstChan = createDshMiddlewareTestChannel(t, "dsh-org-first", common.ChannelStatusEnabled)
	fixture.secondChan = createDshMiddlewareTestChannel(t, "dsh-org-second", common.ChannelStatusEnabled)
	fixture.otherChan = createDshMiddlewareTestChannel(t, "dsh-org-other", common.ChannelStatusEnabled)

	setDshAllowedOrganizationsForMiddlewareTest(t, []int{
		fixture.firstOrg.Id,
		fixture.secondOrg.Id,
		fixture.unboundOrg.Id,
	})
	setDshOrganizationBindingsForMiddlewareTest(t, map[int]int{
		fixture.firstOrg.Id:  fixture.firstChan.Id,
		fixture.secondOrg.Id: fixture.secondChan.Id,
	})
	return fixture
}

type dshOrganizationObservation struct {
	organizationId   int
	usingGroup       string
	userGroup        string
	billingAccountId int
	pin              dto.ChannelPin
	pinFound         bool
	echoedHeader     string
}

// runDshOrganization 用真实中间件链跑一次 POST /pg/agent/tasks 的入口段。
func runDshOrganization(t *testing.T, headers map[string]string, userId int) (*httptest.ResponseRecorder, *dshOrganizationObservation) {
	t.Helper()
	observation := &dshOrganizationObservation{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", userId)
		common.SetContextKey(c, constant.ContextKeyActorUserId, userId)
		for key, value := range headers {
			c.Request.Header.Set(key, value)
		}
	})
	router.POST("/pg/agent/tasks", DshOrganization(), func(c *gin.Context) {
		observation.organizationId = common.GetContextKeyInt(c, constant.ContextKeyOrganizationId)
		observation.usingGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		observation.userGroup = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		observation.billingAccountId = common.GetContextKeyInt(c, constant.ContextKeyBillingAccountId)
		observation.pin, observation.pinFound, _ = service.GetChannelConstraints(c).ResolvedPin()
		observation.echoedHeader = c.Writer.Header().Get(dshOrganizationIdHeader)
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/pg/agent/tasks", nil))
	return recorder, observation
}

func TestDshOrganizationMiddleware(t *testing.T) {
	fixture := setupDshOrganizationFixture(t)

	t.Run("without a preference the lowest enabled organization is used", func(t *testing.T) {
		recorder, observation := runDshOrganization(t, nil, fixture.user.Id)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, fixture.firstOrg.Id, observation.organizationId)
		require.Equal(t, "default", observation.usingGroup)
		require.Equal(t, "default", observation.userGroup)
		// 任务按组织额度计费，不能落到个人钱包。
		require.Equal(t, fixture.firstOrg.Id, observation.billingAccountId)
		require.True(t, observation.pinFound)
		require.Equal(t, dto.PinSourceOrganization, observation.pin.Source)
		require.Equal(t, fixture.firstChan.Id, observation.pin.ChannelId)
		require.Equal(t, strconv.Itoa(fixture.firstOrg.Id), observation.echoedHeader)
	})

	// 会话级固定：同一会话的后续请求必须落回同一个容器。
	t.Run("the session header selects the organization", func(t *testing.T) {
		recorder, observation := runDshOrganization(t, map[string]string{
			dshOrganizationIdHeader: strconv.Itoa(fixture.secondOrg.Id),
		}, fixture.user.Id)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, fixture.secondOrg.Id, observation.organizationId)
		// 分组跟着组织走，"vip" 组织的成员用 vip 分组选路。
		require.Equal(t, "vip", observation.usingGroup)
		require.Equal(t, "vip", observation.userGroup)
		require.Equal(t, fixture.secondChan.Id, observation.pin.ChannelId)
	})

	t.Run("the account context header selects the organization", func(t *testing.T) {
		recorder, observation := runDshOrganization(t, map[string]string{
			"X-Account-Context-Type": model.AccountContextTypeOrganization,
			"X-Account-Context-Id":   strconv.Itoa(fixture.secondOrg.Id),
		}, fixture.user.Id)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, fixture.secondOrg.Id, observation.organizationId)
	})

	// 页面停在尚未开通 DSH 的组织上下文时不该变成死页。
	t.Run("an account context for a disabled organization falls back", func(t *testing.T) {
		recorder, observation := runDshOrganization(t, map[string]string{
			"X-Account-Context-Type": model.AccountContextTypeOrganization,
			"X-Account-Context-Id":   strconv.Itoa(fixture.unboundOrg.Id),
		}, fixture.user.Id)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, fixture.firstOrg.Id, observation.organizationId)
	})

	t.Run("a session header for an organization the user left is rejected", func(t *testing.T) {
		require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
			Where("organization_id = ? AND user_id = ?", fixture.secondOrg.Id, fixture.user.Id).
			Update("status", model.OrganizationMemberStatusExited).Error)
		t.Cleanup(func() {
			_ = model.DB.Model(&model.OrganizationMember{}).
				Where("organization_id = ? AND user_id = ?", fixture.secondOrg.Id, fixture.user.Id).
				Update("status", model.OrganizationMemberStatusActive).Error
		})

		recorder, _ := runDshOrganization(t, map[string]string{
			dshOrganizationIdHeader: strconv.Itoa(fixture.secondOrg.Id),
		}, fixture.user.Id)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshOrganizationUnavailable))
	})

	t.Run("a malformed session header is rejected instead of ignored", func(t *testing.T) {
		recorder, _ := runDshOrganization(t, map[string]string{
			dshOrganizationIdHeader: "not-a-number",
		}, fixture.user.Id)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("a user without any enabled organization is rejected", func(t *testing.T) {
		recorder, _ := runDshOrganization(t, nil, fixture.outsider.Id)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshOrganizationUnavailable))
	})

	t.Run("a user whose bound channel is unavailable is rejected", func(t *testing.T) {
		setDshOrganizationBindingsForMiddlewareTest(t, map[int]int{
			fixture.firstOrg.Id: fixture.otherChan.Id,
		})
		t.Cleanup(func() {
			setDshOrganizationBindingsForMiddlewareTest(t, map[int]int{
				fixture.firstOrg.Id:  fixture.firstChan.Id,
				fixture.secondOrg.Id: fixture.secondChan.Id,
			})
		})
		require.NoError(t, model.DB.Model(&model.Channel{}).
			Where("id = ?", fixture.otherChan.Id).
			Update("status", common.ChannelStatusManuallyDisabled).Error)
		t.Cleanup(func() {
			_ = model.DB.Model(&model.Channel{}).
				Where("id = ?", fixture.otherChan.Id).
				Update("status", common.ChannelStatusEnabled).Error
		})
		setDshAllowedOrganizationsForMiddlewareTest(t, []int{fixture.firstOrg.Id})

		recorder, _ := runDshOrganization(t, nil, fixture.user.Id)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshOrganizationChannelUnavailable))
	})
}

func TestDshUploadOrganizationMiddleware(t *testing.T) {
	fixture := setupDshOrganizationFixture(t)

	runUpload := func(t *testing.T, headers map[string]string, userId int) (*httptest.ResponseRecorder, *dshOrganizationObservation) {
		t.Helper()
		observation := &dshOrganizationObservation{}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("id", userId)
			for key, value := range headers {
				c.Request.Header.Set(key, value)
			}
		})
		router.POST("/api/dsh/uploads", DshUploadOrganization(), func(c *gin.Context) {
			observation.organizationId = common.GetContextKeyInt(c, constant.ContextKeyOrganizationId)
			observation.usingGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
			observation.pinFound = len(service.GetChannelConstraints(c).Pins) > 0
			c.JSON(http.StatusOK, gin.H{"success": true})
		})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/dsh/uploads", nil))
		return recorder, observation
	}

	// 上传不选渠道、不计费：只写组织 ID，不建立计费上下文也不加 pin。
	t.Run("writes only the organization id", func(t *testing.T) {
		recorder, observation := runUpload(t, nil, fixture.user.Id)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, fixture.firstOrg.Id, observation.organizationId)
		require.Empty(t, observation.usingGroup)
		require.False(t, observation.pinFound)
	})

	t.Run("honors the session header", func(t *testing.T) {
		recorder, observation := runUpload(t, map[string]string{
			dshOrganizationIdHeader: strconv.Itoa(fixture.secondOrg.Id),
		}, fixture.user.Id)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, fixture.secondOrg.Id, observation.organizationId)
	})

	t.Run("rejects a user without an enabled organization", func(t *testing.T) {
		recorder, _ := runUpload(t, nil, fixture.outsider.Id)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshOrganizationUnavailable))
	})
}
