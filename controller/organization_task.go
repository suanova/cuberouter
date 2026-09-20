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
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// 组织作用域的任务读取单独成文件：cuberouter 的 controller/task.go 与
// controller/midjourney.go 已经和上游分叉得比较厉害，把组织分支塞进去会让
// 将来同步这两个文件变难。

// ListOrganizationTasks 分页查询组织异步任务
//
// @Summary      组织异步任务列表
// @Description  分页返回组织内的异步任务,支持平台、任务 ID、状态、动作与时间范围过滤
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Param        platform query string false "任务平台"
// @Param        task_id query string false "任务 ID"
// @Param        status query string false "任务状态"
// @Param        action query string false "任务动作"
// @Param        start_timestamp query int false "起始时间戳(秒)"
// @Param        end_timestamp query int false "截止时间戳(秒)"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/tasks [get]
// @Router       /admin/organizations/{id}/tasks [get]
func ListOrganizationTasks(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	items, total, err := service.ListOrganizationTasks(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationTaskListRequest{
		Offset:         pageInfo.GetStartIdx(),
		Limit:          pageInfo.GetPageSize(),
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// ListOrganizationMidjourneyTasks 分页查询组织 Midjourney 任务
//
// @Summary      组织 Midjourney 任务列表
// @Description  分页返回组织内的 Midjourney 绘图任务,支持渠道、MjID 与时间范围过滤
// @Tags         组织, 组织管理
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id path int true "组织 ID"
// @Param        p query int false "页码"
// @Param        page_size query int false "页大小"
// @Param        channel_id query string false "渠道 ID"
// @Param        mj_id query string false "Midjourney 任务 ID"
// @Param        start_timestamp query string false "起始时间戳(秒)"
// @Param        end_timestamp query string false "截止时间戳(秒)"
// @Success      200 {object} dto.APIResponse
// @Router       /organizations/{id}/midjourney-tasks [get]
// @Router       /admin/organizations/{id}/midjourney-tasks [get]
func ListOrganizationMidjourneyTasks(c *gin.Context) {
	organizationId, ok := parseOrganizationId(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListOrganizationMidjourneyTasks(c.GetInt("id"), organizationId, organizationAccessMode(c), service.OrganizationMidjourneyTaskListRequest{
		Offset:         pageInfo.GetStartIdx(),
		Limit:          pageInfo.GetPageSize(),
		ChannelID:      c.Query("channel_id"),
		MjID:           c.Query("mj_id"),
		StartTimestamp: c.Query("start_timestamp"),
		EndTimestamp:   c.Query("end_timestamp"),
	})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	if setting.MjForwardUrlEnabled {
		for _, task := range items {
			task.ImageUrl = system_setting.ServerAddress + "/mj/image/" + task.MjId
		}
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}
