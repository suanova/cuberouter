package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
)

// GetUserInvitees GET /api/user/:id/invitees?p=&page_size=
// 返回 id 用户邀请的下级用户分页列表（仅 InviteeBrief 字段）。
// 已通过 AdminAuth：管理员不受邀请范围限制，可查看任意用户的邀请记录。
func GetUserInvitees(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	pageInfo := common.GetPageQuery(c)
	users, total, err := model.GetUserInvitees(id, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	items := make([]dto.InviteeBrief, 0, len(users))
	for _, u := range users {
		items = append(items, dto.InviteeBrief{
			Id:        u.Id,
			Username:  u.Username,
			Email:     u.Email,
			Phone:     common.MaskPhone(u.Phone),
			Status:    u.Status,
			Group:     u.Group,
			Role:      u.Role,
			CreatedAt: u.CreatedAt,
		})
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}
