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
package openai

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// setupOpenAIStreamSettleDB 为异常结束部分结算用例准备进程内 SQLite：结算会写
// users / tokens / channels / logs。走 model.InitDB 让保留字列（group 等）的引用
// 格式完成初始化，与 service 包的测试夹具同源
// （service/organization_test_support_test.go）。
func setupOpenAIStreamSettleDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMasterNode := common.IsMasterNode
	originalBatchUpdate := common.BatchUpdateEnabled
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(30000)", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	// 异常结束的部分结算会走 CountTextToken（OpenAI 系模型用 tiktoken 精确数），
	// 生产在 main.go 启动时调用它初始化；测试里必须显式初始化，否则 tokenizer 为 nil。
	service.InitTokenEncoders()
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}, &model.QuotaData{},
	))

	t.Cleanup(func() {
		// 还原全局前先等请求派发的后台任务（额度通知、额度缓存刷新）结束，
		// 否则它们会和用例抢同一个全局句柄。
		service.WaitForBackgroundWork()
		model.WaitForQuotaCacheWorkers()
		if model.DB != nil {
			if sqlDB, err := model.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.IsMasterNode = originalMasterNode
		common.BatchUpdateEnabled = originalBatchUpdate
		if hadSQLDSN {
			_ = os.Setenv("SQL_DSN", originalSQLDSN)
		} else {
			_ = os.Unsetenv("SQL_DSN")
		}
	})
}

func createOpenAIStreamSettleUser(t *testing.T, quota int) model.User {
	t.Helper()
	user := model.User{
		Username:    "settle_" + common.GetUUID()[:8],
		Password:    "password",
		DisplayName: "settle",
		Status:      common.UserStatusEnabled,
		Quota:       quota,
		AffCode:     common.GetUUID()[:8],
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func latestConsumeLog(t *testing.T) model.Log {
	t.Helper()
	var entry model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeConsume).Order("id desc").First(&entry).Error)
	return entry
}

func consumeLogCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("type = ?", model.LogTypeConsume).Count(&count).Error)
	return count
}

// newAbnormalSettleStreamTest 准备一个「将异常结束」的 OpenAI chat 流式请求：
// 填好计费所需的用户、价格与 prompt 估算，返回值与 setupOaiStreamTest 一致。
func newAbnormalSettleStreamTest(t *testing.T, body io.ReadCloser, user model.User) (*gin.Context, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()
	c, resp, info := setupOaiStreamTest(t, body)
	c.Set(common.RequestIdKey, t.Name())
	info.UserId = user.Id
	info.ChannelId = 1
	// 生产路径由 TextHelper 依据 text/event-stream 设置；stream_status 等消费
	// 日志标记依赖它，夹具必须显式打开。
	info.IsStream = true
	info.StartTime = time.Now()
	info.FirstResponseTime = info.StartTime
	info.SetEstimatePromptTokens(100)
	info.PriceData = hosttypes.PriceData{
		ModelRatio:      1,
		CompletionRatio: 1,
		GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 1},
	}
	return c, resp, info
}

// oneChunkThenBlockReader 先吐出一段 SSE 数据，然后阻塞读取直到 Close：
// 模拟上游已出流但随后卡死 → 空闲超时（异常结束且 ≥1 分片）。
type oneChunkThenBlockReader struct {
	data   string
	offset int
	mu     sync.Mutex
	closed bool
}

func (r *oneChunkThenBlockReader) Read(p []byte) (int, error) {
	if r.offset < len(r.data) {
		n := copy(p, r.data[r.offset:])
		r.offset += n
		return n, nil
	}
	for {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return 0, io.EOF
		}
		r.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
}

func (r *oneChunkThenBlockReader) Close() error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	return nil
}
