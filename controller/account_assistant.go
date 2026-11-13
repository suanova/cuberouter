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
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/accountassistant"
	"github.com/QuantumNous/new-api/service/agentkit"
	"github.com/QuantumNous/new-api/setting/account_assistant_setting"

	"github.com/gin-gonic/gin"
)

const accountAssistantMaxMessageRunes = 2000

type accountAssistantChatRequest struct {
	Message string        `json:"message"`
	History []dto.Message `json:"history,omitempty"`
}

type accountAssistantEvent struct {
	Type       string `json:"type"`
	Tool       string `json:"tool,omitempty"`
	Success    bool   `json:"success,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Content    string `json:"content,omitempty"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
}

// AccountAssistantConfig 告诉前端悬浮球是否需要显示。只有 enabled 且已配置
// 模型时才对外开放，避免用户看到入口却无法使用。
func AccountAssistantConfig(c *gin.Context) {
	setting := account_assistant_setting.GetAccountAssistantSetting()
	common.ApiSuccess(c, gin.H{
		"enabled":     setting.Enabled && strings.TrimSpace(setting.Model) != "",
		"support_url": strings.TrimSpace(setting.SupportURL),
	})
}

// AccountAssistantChat 是账户助手的流式入口。
//
// 响应是自定义 SSE 事件（不是 OpenAI chunk）：tool / final / refusal / error，
// 以 data: [DONE] 结束。模型只负责选择工具与组织措辞，工具结果由服务端注入；
// 没有任何工具成功执行时返回 refusal，前端据此展示「无法回答，请联系人工」。
func AccountAssistantChat(c *gin.Context) {
	setting := account_assistant_setting.GetAccountAssistantSetting()
	if !setting.Enabled || strings.TrimSpace(setting.Model) == "" {
		common.ApiErrorMsg(c, "账户助手未启用")
		return
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		common.ApiErrorMsg(c, "未登录")
		return
	}

	var request accountAssistantChatRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "请求格式不正确")
		return
	}
	request.Message = strings.TrimSpace(request.Message)
	if request.Message == "" {
		common.ApiErrorMsg(c, "请输入问题")
		return
	}
	if utf8.RuneCountInString(request.Message) > accountAssistantMaxMessageRunes {
		common.ApiErrorMsg(c, "问题过长，请精简后重试")
		return
	}

	principal := agentkit.Principal{
		UserID:              userID,
		IsOrganizationAdmin: accountassistant.UserIsOrganizationAdmin(userID),
	}

	billingUserID := setting.BillingUserId
	if billingUserID <= 0 {
		billingUserID = userID
	}
	group := resolveAssistantGroup(billingUserID, setting.Group)

	complete := func(ctx context.Context, roundRequest *dto.GeneralOpenAIRequest) (*dto.OpenAITextResponse, error) {
		roundRequest.Model = setting.Model
		response, newAPIError := invokeRelayRound(c, roundRequest, billingUserID, group)
		if newAPIError != nil {
			return nil, newAPIError
		}
		if response == nil {
			return nil, errors.New("模型未返回响应")
		}
		return response, nil
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writeEvent := func(event accountAssistantEvent) {
		data, err := common.Marshal(event)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}
	finishStream := func() {
		fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
		c.Writer.Flush()
	}

	assistantService := accountassistant.NewService(complete)
	result, err := assistantService.Chat(
		c.Request.Context(),
		principal,
		request.Message,
		request.History,
		func(event agentkit.Event) {
			writeEvent(accountAssistantEvent{
				Type:       "tool",
				Tool:       event.Tool,
				Success:    event.Success,
				DurationMs: event.DurationMs,
			})
		},
	)
	if err != nil {
		code, message := accountAssistantError(err)
		writeEvent(accountAssistantEvent{Type: "error", Code: code, Message: message})
		finishStream()
		return
	}
	if result.Refused {
		writeEvent(accountAssistantEvent{Type: "refusal", Code: "tool_required"})
		finishStream()
		return
	}
	finalContent := strings.TrimSpace(result.Content)
	if finalContent == "" {
		writeEvent(accountAssistantEvent{
			Type:    "error",
			Code:    "empty_answer",
			Message: "助手暂时无法生成回答，请联系人工",
		})
		finishStream()
		return
	}
	writeEvent(accountAssistantEvent{Type: "final", Content: finalContent})
	finishStream()
}

func resolveAssistantGroup(billingUserID int, configuredGroup string) string {
	group := strings.TrimSpace(configuredGroup)
	if group != "" {
		return group
	}
	resolved, err := model.GetUserGroup(billingUserID, false)
	if err == nil && strings.TrimSpace(resolved) != "" {
		return resolved
	}
	return "default"
}

func accountAssistantError(err error) (code string, message string) {
	switch {
	case errors.Is(err, accountassistant.ErrDisabled):
		return "disabled", "账户助手未启用"
	case errors.Is(err, accountassistant.ErrRateLimited):
		return "rate_limited", "提问过于频繁，请稍后再试"
	case errors.Is(err, accountassistant.ErrDailyLimit):
		return "daily_limit", "今日提问次数已达上限，请联系人工"
	case errors.Is(err, accountassistant.ErrTokenBudget):
		return "token_budget", "今日助手额度已用完，请联系人工"
	case errors.Is(err, accountassistant.ErrEmptyMessage):
		return "empty_message", "请输入问题"
	default:
		common.SysLog("account assistant request failed: " + err.Error())
		return "unavailable", "账户助手暂时不可用，请稍后重试或联系人工"
	}
}
