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
package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// organizationMemberOrphanRepairBatchSize 控制每批删除的行数：MySQL 上一次性删除
// 大量行会长时间持锁，必须分批。
const organizationMemberOrphanRepairBatchSize = 500

// prepareOrganizationMemberOrphanRepair 清理"账号已不存在"的成员行。
//
// 判据选的是可证明的那一个：账号不存在，成员关系就不可能成立。另一个候选判据
// （成员行 created_at 早于账号 created_at）在多节点部署下会误报——common.GetTimestamp()
// 是各节点自己的墙钟，管理员在 B 节点给 A 节点创建的账号加成员时两个时间戳来自不同
// 时钟——因此它只报告、不删除。
func prepareOrganizationMemberOrphanRepair(db *gorm.DB) error {
	total := 0
	for {
		var ids []int
		if err := db.Model(&OrganizationMember{}).
			Select("organization_members.id").
			Joins("LEFT JOIN users ON users.id = organization_members.user_id").
			Where("users.id IS NULL").
			Limit(organizationMemberOrphanRepairBatchSize).
			Pluck("organization_members.id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		if err := db.Where("id IN ?", ids).Delete(&OrganizationMember{}).Error; err != nil {
			return err
		}
		total += len(ids)
		if len(ids) < organizationMemberOrphanRepairBatchSize {
			break
		}
	}
	if total > 0 {
		common.SysLog(fmt.Sprintf("organization member orphan repair: removed %d row(s) whose account no longer exists", total))
	}

	// 只报告不删除：时钟偏差可能让合法成员行看起来更早。
	var suspicious int64
	if err := db.Model(&OrganizationMember{}).
		Joins("JOIN users ON users.id = organization_members.user_id").
		Where("organization_members.created_at > 0 AND organization_members.created_at < users.created_at").
		Count(&suspicious).Error; err != nil {
		return err
	}
	if suspicious > 0 {
		common.SysError(fmt.Sprintf("organization member audit: %d row(s) predate their account and need manual review (id reuse or cross-node clock skew; not deleted automatically)", suspicious))
	}
	return nil
}
