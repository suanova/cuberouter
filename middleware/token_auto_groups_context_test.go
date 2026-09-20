package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTokenAutoGroupsContext 建一个带用户记录的上下文。
// SetupContextForToken 现在会先解析令牌作用域（个人令牌要读用户缓存/用户表），
// 所以这里不能再只给一个空 gin.Context，否则会在 Redis/DB 为 nil 时 panic。
func newTokenAutoGroupsContext(t *testing.T) (*gin.Context, int) {
	t.Helper()
	setupOrganizationMiddlewareTestDB(t, &model.User{})
	user := model.User{Username: "auto-groups-context", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "auto-groups-context"}
	require.NoError(t, model.DB.Create(&user).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx, user.Id
}

func TestSetupContextForTokenPreservesCustomAutoGroupsOrder(t *testing.T) {
	ctx, userId := newTokenAutoGroupsContext(t)
	token := &model.Token{Id: 1, UserId: userId, AutoGroups: `["vip","default"]`}

	require.NoError(t, SetupContextForToken(ctx, token))
	value, ok := common.GetContextKey(ctx, constant.ContextKeyTokenAutoGroups)
	require.True(t, ok)
	assert.Equal(t, []string{"vip", "default"}, value)
}

func TestSetupContextForTokenTreatsStoredEmptyArrayAsInheritance(t *testing.T) {
	ctx, userId := newTokenAutoGroupsContext(t)
	token := &model.Token{Id: 1, UserId: userId, AutoGroups: `[]`}

	require.NoError(t, SetupContextForToken(ctx, token))
	_, ok := common.GetContextKey(ctx, constant.ContextKeyTokenAutoGroups)
	assert.False(t, ok)
}

func TestSetupContextForTokenMalformedAutoGroupsFailsClosed(t *testing.T) {
	ctx, userId := newTokenAutoGroupsContext(t)
	token := &model.Token{Id: 1, UserId: userId, AutoGroups: `not-json`}

	require.NoError(t, SetupContextForToken(ctx, token))
	value, ok := common.GetContextKey(ctx, constant.ContextKeyTokenAutoGroups)
	require.True(t, ok)
	assert.Equal(t, []string{}, value)
}
