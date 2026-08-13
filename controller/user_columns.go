package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// UserColumnMeta describes a toggleable column of the admin users table.
type UserColumnMeta struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

// userColumns is the hand-maintained column metadata for the admin users
// table; the order is the default column order. Labels are stable English
// source strings that the frontend translates with t(); New columns must be
// added here and in the frontend column defs together.
var userColumns = []UserColumnMeta{
	{Key: "select", Label: "Select", Required: true},
	{Key: "id", Label: "ID", Required: true},
	{Key: "username", Label: "Username", Required: true},
	{Key: "status", Label: "Status"},
	{Key: "quota", Label: "Quota"},
	{Key: "group", Label: "Group"},
	{Key: "role", Label: "Role"},
	{Key: "invite_info", Label: "Invite Info"},
	{Key: "created_at", Label: "Created At"},
	{Key: "last_login_at", Label: "Last Login At"},
	{Key: "actions", Label: "Actions", Required: true},
}

// GetUserColumns returns the column metadata for the admin users table.
// Only reachable through AdminAuth, so the caller is an admin or higher.
func GetUserColumns(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    userColumns,
	})
}
