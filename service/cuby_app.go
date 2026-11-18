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

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

var (
	// ErrCubyAppsNotManaged means the switch is off. The Cuby client must be able
	// to tell this apart from "this account has no apps", so the route answers it
	// with HTTP 404 instead of an empty list.
	ErrCubyAppsNotManaged    = errors.New("this instance does not manage Cuby apps")
	ErrCubyAppNotFound       = errors.New("cuby app not found")
	ErrCubyAppIdTaken        = errors.New("cuby app id already exists")
	ErrCubyAppGrantNotFound  = errors.New("cuby app grant not found")
	ErrCubyAppGrantExists    = errors.New("cuby app grant already exists")
	ErrCubyAppHolderNotFound = errors.New("cuby app grant holder not found")
)

// The id rule of cuby-app.json: 2 to 40 lower-case letters, digits or hyphens,
// not starting with a hyphen. The Cuby client drops any id that breaks it.
var cubyAppIdPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)

const (
	cubyAppMaxNameLength        = 64
	cubyAppMaxDescriptionLength = 255
)

func ListAccountCubyApps(userId int) ([]model.AccountCubyApp, error) {
	if !operation_setting.IsCubyAppManagementEnabled() {
		return nil, ErrCubyAppsNotManaged
	}
	return model.GetAccountCubyApps(userId)
}

// SetCubyAppManagementEnabled persists the switch before it takes effect in
// memory, so the other nodes pick it up and a failed write changes nothing.
func SetCubyAppManagementEnabled(enabled bool) error {
	return model.UpdateOptionsBulk(map[string]string{
		operation_setting.CubyAppSettingEnabledOptionKey: strconv.FormatBool(enabled),
	})
}

type CubyAppView struct {
	model.CubyApp
	UserGrantCount         int64 `json:"user_grant_count"`
	OrganizationGrantCount int64 `json:"organization_grant_count"`
}

func ListCubyAppsForManagement() ([]CubyAppView, error) {
	var apps []model.CubyApp
	if err := model.DB.Order("app_id asc").Find(&apps).Error; err != nil {
		return nil, err
	}
	counts, err := model.CountCubyAppGrants()
	if err != nil {
		return nil, err
	}
	views := make([]CubyAppView, 0, len(apps))
	indexByAppId := make(map[string]int, len(apps))
	for i, app := range apps {
		views = append(views, CubyAppView{CubyApp: app})
		indexByAppId[app.AppId] = i
	}
	for _, count := range counts {
		i, ok := indexByAppId[count.AppId]
		if !ok {
			continue
		}
		if count.SubjectType == model.CubyAppGrantSubjectOrganization {
			views[i].OrganizationGrantCount = count.Count
		} else {
			views[i].UserGrantCount = count.Count
		}
	}
	return views, nil
}

type CreateCubyAppRequest struct {
	AppId       string
	Name        string
	Description string
}

func CreateCubyApp(operatorUserId int, req CreateCubyAppRequest) (*model.CubyApp, error) {
	appId := strings.TrimSpace(req.AppId)
	if !cubyAppIdPattern.MatchString(appId) {
		return nil, errors.New("invalid cuby app id: use 2 to 40 lower-case letters, digits or hyphens")
	}
	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)
	if name == "" {
		name = appId
	}
	if utf8.RuneCountInString(name) > cubyAppMaxNameLength {
		return nil, errors.New("cuby app name too long")
	}
	if utf8.RuneCountInString(description) > cubyAppMaxDescriptionLength {
		return nil, errors.New("cuby app description too long")
	}
	var existing int64
	if err := model.DB.Model(&model.CubyApp{}).Where("app_id = ?", appId).Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrCubyAppIdTaken
	}
	now := common.GetTimestamp()
	app := &model.CubyApp{
		AppId:       appId,
		Name:        name,
		Description: description,
		Status:      model.CubyAppStatusEnabled,
		CreatedBy:   operatorUserId,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := model.DB.Create(app).Error; err != nil {
		// Two administrators registering the same id at once: the unique index
		// decides, and the loser gets the same answer as the check above.
		var taken int64
		if countErr := model.DB.Model(&model.CubyApp{}).Where("app_id = ?", appId).Count(&taken).Error; countErr == nil && taken > 0 {
			return nil, ErrCubyAppIdTaken
		}
		return nil, err
	}
	return app, nil
}

// UpdateCubyAppRequest carries only what the administrator changed; a nil field
// is left as it is. The app id is not here: it cannot be changed.
type UpdateCubyAppRequest struct {
	Name        *string
	Description *string
	Status      *string
}

func UpdateCubyApp(id int, req UpdateCubyAppRequest) (*model.CubyApp, error) {
	app, err := getCubyApp(id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, errors.New("cuby app name is required")
		}
		if utf8.RuneCountInString(name) > cubyAppMaxNameLength {
			return nil, errors.New("cuby app name too long")
		}
		updates["name"] = name
	}
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if utf8.RuneCountInString(description) > cubyAppMaxDescriptionLength {
			return nil, errors.New("cuby app description too long")
		}
		updates["description"] = description
	}
	if req.Status != nil {
		if *req.Status != model.CubyAppStatusEnabled && *req.Status != model.CubyAppStatusDisabled {
			return nil, errors.New("invalid cuby app status")
		}
		updates["status"] = *req.Status
	}
	if len(updates) == 0 {
		return app, nil
	}
	updates["updated_at"] = common.GetTimestamp()
	if err := model.DB.Model(&model.CubyApp{}).Where("id = ?", app.Id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return getCubyApp(id)
}

func DeleteCubyApp(id int) error {
	app, err := getCubyApp(id)
	if err != nil {
		return err
	}
	return model.DeleteCubyAppWithGrants(app)
}

func ListCubyAppGrants(id int, pageInfo *common.PageInfo) ([]model.CubyAppGrantView, int64, error) {
	app, err := getCubyApp(id)
	if err != nil {
		return nil, 0, err
	}
	return model.ListCubyAppGrants(app.AppId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
}

// CreateCubyAppGrant gives an app to a user or to an organization. The holder
// must exist: a grant to a mistyped id would sit there and silently do nothing.
func CreateCubyAppGrant(operatorUserId int, id int, subjectType string, subjectId int) (*model.CubyAppGrant, error) {
	app, err := getCubyApp(id)
	if err != nil {
		return nil, err
	}
	if subjectId <= 0 {
		return nil, ErrCubyAppHolderNotFound
	}
	switch subjectType {
	case model.CubyAppGrantSubjectUser:
		if _, err := model.GetUserById(subjectId, false); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrCubyAppHolderNotFound
			}
			return nil, err
		}
	case model.CubyAppGrantSubjectOrganization:
		var organizations int64
		if err := model.DB.Model(&model.Organization{}).
			Where("id = ? AND status <> ?", subjectId, model.OrganizationStatusDissolved).
			Count(&organizations).Error; err != nil {
			return nil, err
		}
		if organizations == 0 {
			return nil, ErrCubyAppHolderNotFound
		}
	default:
		return nil, errors.New("invalid cuby app grant holder type")
	}

	holder := model.DB.Model(&model.CubyAppGrant{}).
		Where("app_id = ? AND subject_type = ? AND subject_id = ?", app.AppId, subjectType, subjectId)
	var existing int64
	if err := holder.Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrCubyAppGrantExists
	}
	grant := &model.CubyAppGrant{
		AppId:       app.AppId,
		SubjectType: subjectType,
		SubjectId:   subjectId,
		CreatedBy:   operatorUserId,
		CreatedAt:   common.GetTimestamp(),
	}
	if err := model.DB.Create(grant).Error; err != nil {
		var taken int64
		if countErr := model.DB.Model(&model.CubyAppGrant{}).
			Where("app_id = ? AND subject_type = ? AND subject_id = ?", app.AppId, subjectType, subjectId).
			Count(&taken).Error; countErr == nil && taken > 0 {
			return nil, ErrCubyAppGrantExists
		}
		return nil, err
	}
	return grant, nil
}

func DeleteCubyAppGrant(id int, grantId int) error {
	app, err := getCubyApp(id)
	if err != nil {
		return err
	}
	result := model.DB.Where("id = ? AND app_id = ?", grantId, app.AppId).Delete(&model.CubyAppGrant{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCubyAppGrantNotFound
	}
	return nil
}

func getCubyApp(id int) (*model.CubyApp, error) {
	var app model.CubyApp
	if err := model.DB.First(&app, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCubyAppNotFound
		}
		return nil, err
	}
	return &app, nil
}
