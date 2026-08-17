package controller

import (
	"encoding/csv"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
)

// ============================================================
// Ops (运营角色) User Invitee Controllers — read-only + export
// ============================================================

// opsUserExportHeaders are the CSV column headers for the ops invitee export,
// resolved per request locale.
func opsUserExportHeaders(c *gin.Context) []string {
	return []string{
		i18n.T(c, i18n.MsgOpsExportHeaderId),
		i18n.T(c, i18n.MsgOpsExportHeaderUsername),
		i18n.T(c, i18n.MsgOpsExportHeaderDisplayName),
		i18n.T(c, i18n.MsgOpsExportHeaderPhone),
		i18n.T(c, i18n.MsgOpsExportHeaderStatus),
		i18n.T(c, i18n.MsgOpsExportHeaderGroup),
		i18n.T(c, i18n.MsgOpsExportHeaderQuota),
		i18n.T(c, i18n.MsgOpsExportHeaderUsedQuota),
		i18n.T(c, i18n.MsgOpsExportHeaderRequestCount),
		i18n.T(c, i18n.MsgOpsExportHeaderPromptTokens),
		i18n.T(c, i18n.MsgOpsExportHeaderCompletionTokens),
		i18n.T(c, i18n.MsgOpsExportHeaderCreatedAt),
		i18n.T(c, i18n.MsgOpsExportHeaderAffCode),
		i18n.T(c, i18n.MsgOpsExportHeaderAffCount),
		i18n.T(c, i18n.MsgOpsExportHeaderInviterId),
	}
}

// csvSafeCell neutralizes spreadsheet formula injection: csv.Writer quotes
// fields but does not stop spreadsheet apps from evaluating a cell whose
// value begins with =, +, -, @, tab or carriage return. A leading apostrophe
// forces the cell to be treated as text.
func csvSafeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// formatOpsUserRow maps a User to one CSV row, with the status label resolved
// for the request locale. User-controlled cells are neutralized against
// formula injection.
func formatOpsUserRow(c *gin.Context, u *model.User) []string {
	createdAt := ""
	if u.CreatedAt > 0 {
		createdAt = time.Unix(u.CreatedAt, 0).Format("2006-01-02 15:04:05")
	}
	status := fmt.Sprintf("%s(%d)", i18n.T(c, i18n.MsgOpsStatusUnknown), u.Status)
	switch u.Status {
	case common.UserStatusEnabled:
		status = i18n.T(c, i18n.MsgOpsStatusEnabled)
	case common.UserStatusDisabled:
		status = i18n.T(c, i18n.MsgOpsStatusDisabled)
	}
	return []string{
		fmt.Sprintf("%d", u.Id),
		csvSafeCell(u.Username),
		csvSafeCell(u.DisplayName),
		common.MaskPhone(u.Phone),
		status,
		csvSafeCell(u.Group),
		fmt.Sprintf("%d", u.Quota),
		fmt.Sprintf("%d", u.UsedQuota),
		fmt.Sprintf("%d", u.RequestCount),
		fmt.Sprintf("%d", u.TotalPromptTokens),
		fmt.Sprintf("%d", u.TotalCompletionTokens),
		createdAt,
		csvSafeCell(u.AffCode),
		fmt.Sprintf("%d", u.AffCount),
		fmt.Sprintf("%d", u.InviterId),
	}
}

// GetOpsInvitees returns the paged invitee list of the current ops user.
func GetOpsInvitees(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	users, total, err := model.GetOpsInvitees(userId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	for _, u := range users {
		if u.Phone != "" {
			u.Phone = common.MaskPhone(u.Phone)
		}
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

// SearchOpsInvitees searches the current ops user's invitees by keyword.
func SearchOpsInvitees(c *gin.Context) {
	keyword := c.Query("keyword")
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	users, total, err := model.SearchOpsInvitees(userId, keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	for _, u := range users {
		if u.Phone != "" {
			u.Phone = common.MaskPhone(u.Phone)
		}
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

// ExportOpsInviteesRequest is the body for the ops invitee CSV export.
type ExportOpsInviteesRequest struct {
	Ids     []int  `json:"ids"`
	Keyword string `json:"keyword"`
	Format  string `json:"format"`
}

// ExportOpsInvitees streams the current ops user's invitees as a CSV file.
// The ids take precedence over the keyword when both are provided.
func ExportOpsInvitees(c *gin.Context) {
	var req ExportOpsInviteesRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if req.Format != "" && req.Format != "csv" {
		common.ApiErrorI18n(c, i18n.MsgOpsExportUnsupportedFormat, map[string]any{"Format": req.Format})
		return
	}

	userId := c.GetInt("id")
	username := c.GetString("username")

	// Fetch and validate every batch before writing the CSV header, so a
	// failed batch fails the request instead of looking like a successful
	// (partial) export.
	var users []*model.User
	var err error
	if len(req.Ids) > 0 {
		users, err = model.ExportOpsInviteesByIds(userId, req.Ids)
	} else {
		users, err = model.ExportOpsInviteesByKeyword(userId, req.Keyword)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// ASCII filename: non-ASCII Content-Disposition filenames render as
	// garbage in browsers regardless of charset hints.
	filename := fmt.Sprintf("invitees_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// UTF-8 BOM so Excel opens the CSV as UTF-8
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(c.Writer)
	defer w.Flush()
	_ = w.Write(opsUserExportHeaders(c))

	for _, u := range users {
		_ = w.Write(formatOpsUserRow(c, u))
	}
	w.Flush()

	common.SysLog(fmt.Sprintf(
		"ops user %d (%s) exported invitees count=%d",
		userId, username, len(users),
	))
}
