package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OpsUserColumnMeta describes a toggleable column of the ops invitee table.
type OpsUserColumnMeta struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

// opsUserColumns is the hand-maintained column metadata for the ops invitee
// table; the order is the default column order. Labels are stable English
// source strings that the frontend translates with t(); New columns must be
// added here and in the frontend column defs together.
var opsUserColumns = []OpsUserColumnMeta{
	{Key: "id", Label: "ID", Required: true},
	{Key: "username", Label: "Username", Required: true},
	{Key: "display_name", Label: "Display Name"},
	{Key: "phone", Label: "Phone"},
	{Key: "role", Label: "Role"},
	{Key: "status", Label: "Status"},
	{Key: "group", Label: "Group"},
	{Key: "quota", Label: "Quota"},
	{Key: "used_quota", Label: "Used Quota"},
	{Key: "request_count", Label: "Requests"},
	{Key: "total_prompt_tokens", Label: "Prompt Tokens"},
	{Key: "total_completion_tokens", Label: "Completion Tokens"},
	{Key: "aff_code", Label: "Aff Code"},
	{Key: "aff_count", Label: "Invite Count"},
	{Key: "created_at", Label: "Created At"},
}

// GetOpsUserColumns returns the column metadata for the ops invitee table.
// Only reachable through OpsAuth, so the caller is an ops user or higher.
func GetOpsUserColumns(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    opsUserColumns,
	})
}
