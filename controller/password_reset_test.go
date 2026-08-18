/*
Copyright (C) 2023-2026 QuantumNous

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
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// TestResetPasswordEmailFailureKeepsPasswordAndToken 保护"重置密码先发邮件、
// 成功后才落库新密码并消费 token"的契约：SMTP 失败时用户旧密码与重置 token
// 都必须保持原样，可重试，不会被锁在门外。
//
// 注意：fixture 不能用 setupOpsUserTestDB——它调用 i18n.Init() 初始化全局
// bundle，而本文件按字母序排在其他断言原始 i18n key 的测试之前，会改变
// 它们拿到的消息文本。
func TestResetPasswordEmailFailureKeepsPasswordAndToken(t *testing.T) {
	db := setupManageUserTestDB(t)

	// 显式强制邮件投递失败（而不是依赖环境未配置 SMTP）：若其他测试或
	// 设置项已配置 SMTP，ResetPassword 会走成功路径并改动用户状态，
	// 测试就失去确定性。保存原配置，测试结束恢复。
	prevSMTPServer, prevSMTPAccount := common.SMTPServer, common.SMTPAccount
	common.SMTPServer, common.SMTPAccount = "", ""
	t.Cleanup(func() {
		common.SMTPServer, common.SMTPAccount = prevSMTPServer, prevSMTPAccount
	})

	passwordHash, err := common.Password2Hash("OldPass123")
	require.NoError(t, err)
	user := model.User{
		Username: "reset-email-fail",
		Password: passwordHash,
		Email:    "reset-email-fail@test.com",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(&user).Error)

	// 预置有效 token（Redis 未启用，验证码走进程内存储）
	code := common.GenerateVerificationCode(0)
	common.RegisterVerificationCodeWithKey(user.Email, code, common.PasswordResetPurpose)
	require.True(t, common.VerifyCodeWithKey(user.Email, code, common.PasswordResetPurpose))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := fmt.Sprintf(`{"email":"%s","token":"%s"}`, user.Email, code)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/reset", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	ResetPassword(c)

	// SMTP 未配置 → 邮件发送失败，请求失败
	assert.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)

	// 密码未被轮换（哈希不变），token 未被消费（仍可验证）
	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Equal(t, passwordHash, stored.Password)
	assert.True(t, common.VerifyCodeWithKey(user.Email, code, common.PasswordResetPurpose))
}

// blockingSMTPServer 只处理一个 SMTP 连接：投递第一个请求的邮件并把 DATA 阶段
// 卡住（hold 住 claim），直到测试放行。期间任何额外的连接立即被关闭——这样在
// 没有 claim 保护的旧代码上，第二个并发请求会确定性地快速失败而不是挂起。
type blockingSMTPServer struct {
	host          string
	port          int
	listener      net.Listener
	emailReceived chan struct{} // 第一个邮件已捕获且 claim 仍被持有
	release       chan struct{} // 测试放行 DATA 阶段
	messages      chan string   // 捕获到的邮件正文
}

func newBlockingSMTPServer(t *testing.T) *blockingSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	s := &blockingSMTPServer{
		host:          host,
		port:          port,
		listener:      listener,
		emailReceived: make(chan struct{}, 1),
		release:       make(chan struct{}, 1),
		messages:      make(chan string, 2),
	}
	go s.serve()
	t.Cleanup(func() {
		select {
		case s.release <- struct{}{}:
		default:
		}
		_ = s.listener.Close()
	})
	return s
}

func (s *blockingSMTPServer) serve() {
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	go s.handle(conn)
	// 关闭后续的"闯入者"连接：没有 claim 保护的代码里第二个请求会连上来，
	// 这里立刻断开让它快速失败，保持测试确定性。
	for {
		extra, err := s.listener.Accept()
		if err != nil {
			return
		}
		_ = extra.Close()
	}
}

func (s *blockingSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	if err := writeSMTPReply(rw, "220 fake.local ESMTP"); err != nil {
		return
	}
	for {
		line, err := rw.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.ToUpper(strings.TrimRight(line, "\r\n"))
		switch {
		case strings.HasPrefix(command, "EHLO"):
			if err := writeSMTPReply(rw, "250-fake.local"); err != nil {
				return
			}
			if err := writeSMTPReply(rw, "250 8BITMIME"); err != nil {
				return
			}
		case strings.HasPrefix(command, "MAIL FROM:"):
			if err := writeSMTPReply(rw, "250 2.1.0 Sender OK"); err != nil {
				return
			}
		case strings.HasPrefix(command, "RCPT TO:"):
			if err := writeSMTPReply(rw, "250 2.1.5 Recipient OK"); err != nil {
				return
			}
		case command == "DATA":
			if err := writeSMTPReply(rw, "354 End data with <CR><LF>.<CR><LF>"); err != nil {
				return
			}
			var body strings.Builder
			for {
				dataLine, err := rw.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				body.WriteString(dataLine)
			}
			select {
			case s.messages <- body.String():
			default:
			}
			select {
			case s.emailReceived <- struct{}{}:
			default:
			}
			// 卡住 DATA 阶段：第一个请求此刻仍持有 claim，测试可以确定性地
			// 让第二个请求在同一 token 上失败。
			<-s.release
			if err := writeSMTPReply(rw, "250 2.0.0 Queued"); err != nil {
				return
			}
		case command == "QUIT":
			_ = writeSMTPReply(rw, "221 2.0.0 Bye")
			return
		default:
			if err := writeSMTPReply(rw, "502 5.5.1 Command not implemented"); err != nil {
				return
			}
		}
	}
}

func writeSMTPReply(rw *bufio.ReadWriter, line string) error {
	_, err := rw.WriteString(line + "\r\n")
	if err != nil {
		return err
	}
	return rw.Flush()
}

// withResetSMTP 把全局 SMTP 指向假服务器（无 TLS、无认证），测试结束恢复原配置。
func withResetSMTP(t *testing.T, host string, port int) {
	t.Helper()
	prevServer, prevPort := common.SMTPServer, common.SMTPPort
	prevSSL, prevStartTLS := common.SMTPSSLEnabled, common.SMTPStartTLSEnabled
	prevInsecure, prevForceAuth := common.SMTPInsecureSkipVerify, common.SMTPForceAuthLogin
	prevAccount, prevFrom, prevToken := common.SMTPAccount, common.SMTPFrom, common.SMTPToken

	common.SMTPServer = host
	common.SMTPPort = port
	common.SMTPSSLEnabled = false
	common.SMTPStartTLSEnabled = false
	common.SMTPInsecureSkipVerify = false
	common.SMTPForceAuthLogin = false
	common.SMTPAccount = ""
	common.SMTPFrom = "reset@test.local"
	common.SMTPToken = ""

	t.Cleanup(func() {
		common.SMTPServer, common.SMTPPort = prevServer, prevPort
		common.SMTPSSLEnabled, common.SMTPStartTLSEnabled = prevSSL, prevStartTLS
		common.SMTPInsecureSkipVerify, common.SMTPForceAuthLogin = prevInsecure, prevForceAuth
		common.SMTPAccount, common.SMTPFrom, common.SMTPToken = prevAccount, prevFrom, prevToken
	})
}

type resetResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// performResetRequest 直接驱动 ResetPassword，返回解析后的响应。不在内部调用
// t.*：它可能从并发 goroutine 中执行，而 FailNow/Error 只能发生在测试主 goroutine。
func performResetRequest(email, token string) resetResponse {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := fmt.Sprintf(`{"email":%q,"token":%q}`, email, token)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/reset", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	ResetPassword(c)
	var resp resetResponse
	_ = common.Unmarshal(recorder.Body.Bytes(), &resp)
	return resp
}

var deliveredPasswordPattern = regexp.MustCompile(`<strong>([^<]+)</strong>`)

func extractDeliveredPassword(t *testing.T, message string) string {
	t.Helper()
	m := deliveredPasswordPattern.FindStringSubmatch(message)
	require.Len(t, m, 2, "邮件正文应包含新密码: %q", message)
	return m[1]
}

// TestResetPasswordConcurrentRequestsSingleWinner 保护重置密码的并发唯一性：同一
// token 的两个并发请求，只有一个能拿到 claim 并投递邮件/落库；另一个必须立即
// 收到"链接已失效"，绝不会投出第二封内容不同的邮件——否则先投的那封密码会被
// 后一个请求轮换掉。
//
// 用阻塞式假 SMTP 把第一个请求卡在 DATA 阶段：此时 claim 已被持有，再启动第二
// 个请求，其 claim 在同一 token 上必然失败，从而把并发窗口变成确定性的先后关系。
func TestResetPasswordConcurrentRequestsSingleWinner(t *testing.T) {
	db := setupManageUserTestDB(t)
	gin.SetMode(gin.TestMode)

	server := newBlockingSMTPServer(t)
	withResetSMTP(t, server.host, server.port)

	passwordHash, err := common.Password2Hash("OldPass123")
	require.NoError(t, err)
	user := model.User{
		Username: "reset-concurrent",
		Password: passwordHash,
		Email:    "reset-concurrent@test.com",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(&user).Error)

	code := common.GenerateVerificationCode(0)
	common.RegisterVerificationCodeWithKey(user.Email, code, common.PasswordResetPurpose)

	// 第一个请求先跑：它 claim 成功、投出邮件并卡在假 SMTP 的 DATA 阶段。
	first := make(chan resetResponse, 1)
	go func() { first <- performResetRequest(user.Email, code) }()

	select {
	case <-server.emailReceived:
		// 第一个请求已投出邮件且仍持有 claim
	case <-time.After(5 * time.Second):
		t.Fatal("第一个请求未在超时前发出邮件，claim 流程可能未生效")
	}

	// 第二个并发请求必须被拒：token 已被 claim，不能重复投递/落库。
	second := performResetRequest(user.Email, code)
	assert.False(t, second.Success, "并发第二个请求必须失败")
	assert.Equal(t, resetLinkInvalidCode, second.Code, "第二个请求应返回链接失效")

	// 放行第一个请求：邮件投递完成后落库新密码并消费 token。
	server.release <- struct{}{}
	select {
	case firstResp := <-first:
		assert.True(t, firstResp.Success, "第一个请求应成功")
	case <-time.After(5 * time.Second):
		t.Fatal("第一个请求未在超时前完成")
	}

	// 恰好一封邮件，且落库的密码就是邮件里交付的那一个。
	select {
	case message := <-server.messages:
		password := extractDeliveredPassword(t, message)
		var stored model.User
		require.NoError(t, db.First(&stored, user.Id).Error)
		assert.NotEqual(t, passwordHash, stored.Password, "密码必须已被轮换")
		assert.True(t, common.ValidatePasswordAndHash(password, stored.Password),
			"落库密码必须与唯一投递邮件中的密码一致")
	default:
		t.Fatal("预期恰好一封重置邮件")
	}
	select {
	case extra := <-server.messages:
		t.Fatalf("不应投出第二封重置邮件: %q", extra)
	default:
	}

	// token 已被消费，无法重放。
	assert.False(t, common.VerifyCodeWithKey(user.Email, code, common.PasswordResetPurpose))
}
