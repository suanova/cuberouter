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
package service

import "github.com/QuantumNous/new-api/model"

type OrganizationDetail struct {
	Organization *model.Organization       `json:"organization"`
	Member       *model.OrganizationMember `json:"member"`
	Actor        *OrganizationActorContext `json:"actor"`
}

func GetOrganizationDetailForUserWithAccessMode(userId int, organizationId int, accessMode string, allowDissolvedRead bool) (*OrganizationDetail, error) {
	actor, err := GetOrganizationActorContextForAccessMode(userId, organizationId, accessMode, allowDissolvedRead)
	if err != nil {
		return nil, err
	}
	return &OrganizationDetail{Organization: actor.Organization, Member: actor.Member, Actor: actor}, nil
}
