package common

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"slices"
	"strings"
	"time"
)

// EmailDeliveryStage 标出投递失败发生在哪一步。
//
// 组织邀请需要按阶段决定后续动作：收件人被拒（5xx）是"这个地址不能用"，
// 属于永久失败，不能再重试；内容阶段失败则是我们这边的问题，可以重投。
// 只把 error 字符串往外抛的话，调用方就只能靠匹配文案了。
type EmailDeliveryStage string

const (
	EmailDeliveryStageConfig    EmailDeliveryStage = "config"
	EmailDeliveryStageConnect   EmailDeliveryStage = "connect"
	EmailDeliveryStageTLS       EmailDeliveryStage = "tls"
	EmailDeliveryStageAuth      EmailDeliveryStage = "auth"
	EmailDeliveryStageSender    EmailDeliveryStage = "sender"
	EmailDeliveryStageRecipient EmailDeliveryStage = "recipient"
	EmailDeliveryStageData      EmailDeliveryStage = "data"
	EmailDeliveryStageContent   EmailDeliveryStage = "content"
)

// EmailDeliveryError 带上阶段和 SMTP 状态码，供上层区分可重试与永久失败。
type EmailDeliveryError struct {
	Stage    EmailDeliveryStage
	SMTPCode int
	Err      error
}

func (e *EmailDeliveryError) Error() string {
	if e == nil || e.Err == nil {
		return "email delivery failed"
	}
	return e.Err.Error()
}

func (e *EmailDeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func wrapEmailDeliveryError(stage EmailDeliveryStage, err error) error {
	if err == nil {
		return nil
	}
	code := 0
	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) {
		code = smtpErr.Code
	}
	category := "internal"
	if code > 0 {
		category = "smtp"
	} else {
		var netErr net.Error
		if errors.As(err, &netErr) {
			if netErr.Timeout() {
				category = "timeout"
			} else {
				category = "network"
			}
		}
	}
	SysError(fmt.Sprintf("email delivery failed stage=%s code=%d category=%s", stage, code, category))
	return &EmailDeliveryError{Stage: stage, SMTPCode: code, Err: err}
}

func generateMessageID() (string, error) {
	split := strings.Split(SMTPFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := strings.Split(SMTPFrom, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(SMTPAccount) || slices.Contains(EmailLoginAuthServerList, SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	return AutoSMTPAuth(SMTPAccount, SMTPToken)
}

func shouldAuthenticateSMTP() bool {
	return SMTPAccount != "" && SMTPToken != ""
}

func smtpTLSConfig() *tls.Config {
	return &tls.Config{
		ServerName:         SMTPServer,
		InsecureSkipVerify: SMTPInsecureSkipVerify, // #nosec G402 -- admin-controlled SMTP compatibility option.
	}
}

// newSMTPClient 建立连接并按配置协商 TLS，不设截止时间。
func newSMTPClient(addr string) (*smtp.Client, error) {
	return newSMTPClientWithDeadline(addr, time.Time{})
}

// newSMTPClientWithDeadline 与 newSMTPClient 行为一致，但整条连接受 deadline 约束。
// deadline 为零值时退化成 net.Dial 的默认超时，也就是 SendEmail 一直以来的行为。
func newSMTPClientWithDeadline(addr string, deadline time.Time) (*smtp.Client, error) {
	dialer := &net.Dialer{}
	if !deadline.IsZero() {
		dialer.Timeout = time.Until(deadline)
		dialer.Deadline = deadline
	}
	if SMTPSSLEnabled || (SMTPPort == 465 && !SMTPStartTLSEnabled) {
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, smtpTLSConfig())
		if err != nil {
			return nil, err
		}
		if !deadline.IsZero() {
			if err := conn.SetDeadline(deadline); err != nil {
				_ = conn.Close()
				return nil, err
			}
		}
		client, err := smtp.NewClient(conn, SMTPServer)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		return client, nil
	}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	if !deadline.IsZero() {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	client, err := smtp.NewClient(conn, SMTPServer)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if SMTPStartTLSEnabled {
		startTLSSupported, _ := client.Extension("STARTTLS")
		if !startTLSSupported {
			_ = client.Close()
			return nil, fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(smtpTLSConfig()); err != nil {
			_ = client.Close()
			return nil, err
		}
	}

	return client, nil
}

// sendEmail 是 SendEmail / SendEmailWithTimeout 共用的投递实现。
// deadline 为零值时不受额外超时约束；每个失败点都带上阶段信息。
func sendEmail(subject string, receiver string, content string, deadline time.Time) error {
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	id, err2 := generateMessageID()
	if err2 != nil {
		return wrapEmailDeliveryError(EmailDeliveryStageConfig, err2)
	}
	if SMTPServer == "" && SMTPAccount == "" {
		return wrapEmailDeliveryError(EmailDeliveryStageConfig, fmt.Errorf("SMTP 服务器未配置"))
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	mail := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+ // 添加 Message-ID 头
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver, SystemName, SMTPFrom, encodedSubject, time.Now().Format(time.RFC1123Z), id, content))
	auth := getSMTPAuth()
	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	to := strings.Split(receiver, ";")
	client, err := newSMTPClientWithDeadline(addr, deadline)
	if err != nil {
		stage := EmailDeliveryStageConnect
		if strings.Contains(err.Error(), "STARTTLS") || strings.Contains(err.Error(), "tls:") {
			stage = EmailDeliveryStageTLS
		}
		return wrapEmailDeliveryError(stage, err)
	}
	defer client.Close()
	if shouldAuthenticateSMTP() {
		if err = client.Auth(auth); err != nil {
			return wrapEmailDeliveryError(EmailDeliveryStageAuth, err)
		}
	}
	if err = client.Mail(SMTPFrom); err != nil {
		return wrapEmailDeliveryError(EmailDeliveryStageSender, err)
	}
	for _, receiver := range to {
		if err = client.Rcpt(receiver); err != nil {
			return wrapEmailDeliveryError(EmailDeliveryStageRecipient, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return wrapEmailDeliveryError(EmailDeliveryStageData, err)
	}
	_, err = w.Write(mail)
	if err != nil {
		return wrapEmailDeliveryError(EmailDeliveryStageContent, err)
	}
	err = w.Close()
	if err != nil {
		return wrapEmailDeliveryError(EmailDeliveryStageContent, err)
	}
	err = client.Quit()
	if err != nil {
		SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
	}
	return err
}

func SendEmail(subject string, receiver string, content string) error {
	return sendEmail(subject, receiver, content, time.Time{})
}

// SendEmailWithTimeout 在给定超时内投递邮件，失败时返回带阶段的 *EmailDeliveryError。
// 组织邀请走这条路径：SMTP 卡住不能把请求也一起拖住。
func SendEmailWithTimeout(subject string, receiver string, content string, timeout time.Duration) error {
	if timeout <= 0 {
		return &EmailDeliveryError{Stage: EmailDeliveryStageConfig, Err: fmt.Errorf("email delivery timeout must be positive")}
	}
	return sendEmail(subject, receiver, content, time.Now().Add(timeout))
}
