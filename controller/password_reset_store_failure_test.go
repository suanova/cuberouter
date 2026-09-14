package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendPasswordResetEmailSkipsDeliveryWhenCodeIsNotStored pins the contract
// that the reset link is only emailed when its token actually reached the
// store. A link whose token no instance can claim is worse than no link: the
// user believes a reset is in flight and then hits an invalid-link page. The
// outward response stays successful either way, so the endpoint does not become
// an account-existence oracle.
func TestSendPasswordResetEmailSkipsDeliveryWhenCodeIsNotStored(t *testing.T) {
	db := setupManageUserTestDB(t)

	passwordHash, err := common.Password2Hash("OldPass123")
	require.NoError(t, err)
	user := model.User{
		Username: "reset-store-fail",
		Password: passwordHash,
		Email:    "reset-store-fail@test.com",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(&user).Error)

	// Configure Redis but point it at a server that is already gone, so storing
	// the code fails the way it would if the shared store were unreachable.
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	require.NoError(t, redisClient.Ping(context.Background()).Err())
	t.Cleanup(func() { _ = redisClient.Close() })
	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, redisClient
	t.Cleanup(func() { common.RedisEnabled, common.RDB = previousRedisEnabled, previousRDB })
	redisServer.Close()

	smtp := newBlockingSMTPServer(t)
	withResetSMTP(t, smtp.host, smtp.port)
	// Never hold the DATA phase, so a send that should not happen completes and
	// is observed instead of hanging the test.
	smtp.release <- struct{}{}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/reset_password?email="+user.Email, nil)
	SendPasswordResetEmail(c)

	// SendEmail is synchronous, so any delivery has already been recorded by now.
	select {
	case <-smtp.emailReceived:
		t.Fatal("验证码未存入共享存储时不得投递重置链接：该链接永远无法被认领")
	default:
	}

	assert.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.True(t, resp.Success, "对外仍须报成功，否则该端点会成为账号存在性 oracle")
}
