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

	"gorm.io/gorm"
)

const (
	CubyAppStatusEnabled  = "enabled"
	CubyAppStatusDisabled = "disabled"

	CubyAppGrantSubjectUser         = "user"
	CubyAppGrantSubjectOrganization = "organization"
)

// CubyApp is one app (plug-in) of the Cuby desktop client that this instance can
// give to accounts. AppId is the id the app's own cuby-app.json carries; Cuby
// matches on it, so it never changes once registered.
//
// Uniqueness is a column constraint (the `unique` tag) and the columns carry no
// default tags. Both are deliberate: with a unique index, or with string
// defaults, the SQLite driver rebuilds the whole table on every startup, and a
// unique column that is also part of an ordinary index does the same. Declared
// this way a restart issues no DDL on SQLite, MySQL or PostgreSQL. Every insert
// goes through GORM, which writes all columns.
type CubyApp struct {
	Id          int    `json:"id"`
	AppId       string `json:"app_id" gorm:"type:varchar(40);not null;unique"`
	Name        string `json:"name" gorm:"type:varchar(128);not null"`
	Description string `json:"description" gorm:"type:varchar(512);not null"`
	Status      string `json:"status" gorm:"type:varchar(16);not null"`
	CreatedBy   int    `json:"created_by" gorm:"not null"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

// CubyAppGrant gives one app to one user, or to one organization and through it
// to every active member.
//
// HolderKey is "<app_id>/<subject_type>/<subject_id>". Its unique constraint is
// what stops one app being given to the same holder twice; it stands in for a
// unique index over the three columns, which SQLite cannot keep without the
// rebuild described on CubyApp. A grant is never updated, only created and
// deleted, so the key cannot drift from the columns it is built from.
type CubyAppGrant struct {
	Id          int    `json:"id"`
	AppId       string `json:"app_id" gorm:"type:varchar(40);not null;index:idx_cuby_app_grants_app"`
	SubjectType string `json:"subject_type" gorm:"type:varchar(16);not null;index:idx_cuby_app_grants_holder,priority:1"`
	SubjectId   int    `json:"subject_id" gorm:"not null;index:idx_cuby_app_grants_holder,priority:2"`
	HolderKey   string `json:"-" gorm:"type:varchar(96);not null;unique"`
	CreatedBy   int    `json:"created_by" gorm:"not null"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
}

func (grant *CubyAppGrant) BeforeCreate(tx *gorm.DB) error {
	grant.HolderKey = fmt.Sprintf("%s/%s/%d", grant.AppId, grant.SubjectType, grant.SubjectId)
	return nil
}

// CubyAppSource says why an account has an app: it was given to the account
// itself, or to an organization the account is an active member of.
type CubyAppSource struct {
	Type string `json:"type"`
	Id   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// AccountCubyApp is one entry of the answer the Cuby client reads. Its JSON
// shape is a contract with that client.
type AccountCubyApp struct {
	Id      string          `json:"id"`
	Sources []CubyAppSource `json:"sources"`
}

// GetAccountCubyApps lists the enabled apps a user may use: everything given to
// the user plus everything given to an active organization the user is an
// active member of. Apps come back ordered by id, the user's own grant first.
func GetAccountCubyApps(userId int) ([]AccountCubyApp, error) {
	var organizations []Organization
	if err := DB.Model(&Organization{}).
		Select("organizations.id, organizations.name").
		Joins("JOIN organization_members ON organization_members.organization_id = organizations.id").
		Where("organization_members.user_id = ? AND organization_members.status = ? AND organizations.status = ?",
			userId, OrganizationMemberStatusActive, OrganizationStatusActive).
		Find(&organizations).Error; err != nil {
		return nil, err
	}
	organizationIds := make([]int, 0, len(organizations))
	organizationNames := make(map[int]string, len(organizations))
	for _, organization := range organizations {
		organizationIds = append(organizationIds, organization.Id)
		organizationNames[organization.Id] = organization.Name
	}

	holder := DB.Where("cuby_app_grants.subject_type = ? AND cuby_app_grants.subject_id = ?", CubyAppGrantSubjectUser, userId)
	if len(organizationIds) > 0 {
		holder = holder.Or("cuby_app_grants.subject_type = ? AND cuby_app_grants.subject_id IN ?", CubyAppGrantSubjectOrganization, organizationIds)
	}
	var grants []CubyAppGrant
	if err := DB.Model(&CubyAppGrant{}).
		Select("cuby_app_grants.*").
		Joins("JOIN cuby_apps ON cuby_apps.app_id = cuby_app_grants.app_id").
		Where("cuby_apps.status = ?", CubyAppStatusEnabled).
		Where(holder).
		// "user" sorts after "organization", so desc puts the account's own grant first.
		Order("cuby_app_grants.app_id asc, cuby_app_grants.subject_type desc, cuby_app_grants.subject_id asc").
		Find(&grants).Error; err != nil {
		return nil, err
	}

	apps := make([]AccountCubyApp, 0)
	for _, grant := range grants {
		source := CubyAppSource{Type: grant.SubjectType}
		if grant.SubjectType == CubyAppGrantSubjectOrganization {
			source.Id = grant.SubjectId
			source.Name = organizationNames[grant.SubjectId]
		}
		if last := len(apps) - 1; last >= 0 && apps[last].Id == grant.AppId {
			apps[last].Sources = append(apps[last].Sources, source)
			continue
		}
		apps = append(apps, AccountCubyApp{Id: grant.AppId, Sources: []CubyAppSource{source}})
	}
	return apps, nil
}

type CubyAppGrantCount struct {
	AppId       string
	SubjectType string
	Count       int64
}

func CountCubyAppGrants() ([]CubyAppGrantCount, error) {
	var counts []CubyAppGrantCount
	err := DB.Model(&CubyAppGrant{}).
		Select("app_id, subject_type, COUNT(*) as count").
		Group("app_id, subject_type").
		Scan(&counts).Error
	return counts, err
}

// DeleteCubyAppWithGrants removes an app from the catalog together with every
// grant of it, so no grant outlives the app it names.
func DeleteCubyAppWithGrants(app *CubyApp) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_id = ?", app.AppId).Delete(&CubyAppGrant{}).Error; err != nil {
			return err
		}
		return tx.Delete(&CubyApp{}, app.Id).Error
	})
}

// CubyAppGrantView is a grant plus what an administrator needs to recognise its
// holder. The holder fields stay empty when the user or organization is gone.
type CubyAppGrantView struct {
	CubyAppGrant
	OrganizationName   string `json:"organization_name,omitempty"`
	OrganizationStatus string `json:"organization_status,omitempty"`
	Username           string `json:"username,omitempty"`
	DisplayName        string `json:"display_name,omitempty"`
	Email              string `json:"email,omitempty"`
}

func ListCubyAppGrants(appId string, offset int, limit int) ([]CubyAppGrantView, int64, error) {
	var total int64
	if err := DB.Model(&CubyAppGrant{}).Where("app_id = ?", appId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var grants []CubyAppGrant
	if err := DB.Where("app_id = ?", appId).
		Order("subject_type asc, id desc").
		Limit(limit).Offset(offset).
		Find(&grants).Error; err != nil {
		return nil, 0, err
	}

	views := make([]CubyAppGrantView, 0, len(grants))
	var organizationIds, userIds []int
	for _, grant := range grants {
		views = append(views, CubyAppGrantView{CubyAppGrant: grant})
		if grant.SubjectType == CubyAppGrantSubjectOrganization {
			organizationIds = append(organizationIds, grant.SubjectId)
		} else {
			userIds = append(userIds, grant.SubjectId)
		}
	}
	organizationById := map[int]Organization{}
	if len(organizationIds) > 0 {
		var organizations []Organization
		if err := DB.Select("id", "name", "status").Where("id IN ?", organizationIds).Find(&organizations).Error; err != nil {
			return nil, 0, err
		}
		for _, organization := range organizations {
			organizationById[organization.Id] = organization
		}
	}
	userById := map[int]User{}
	if len(userIds) > 0 {
		var users []User
		if err := DB.Select("id", "username", "display_name", "email").Where("id IN ?", userIds).Find(&users).Error; err != nil {
			return nil, 0, err
		}
		for _, user := range users {
			userById[user.Id] = user
		}
	}
	for i := range views {
		if views[i].SubjectType == CubyAppGrantSubjectOrganization {
			organization := organizationById[views[i].SubjectId]
			views[i].OrganizationName = organization.Name
			views[i].OrganizationStatus = organization.Status
			continue
		}
		user := userById[views[i].SubjectId]
		views[i].Username = user.Username
		views[i].DisplayName = user.DisplayName
		views[i].Email = user.Email
	}
	return views, total, nil
}
