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

import "gorm.io/gorm"

const organizationScopeMigrationBatchSize = 500

type organizationScopeMigrationSpec struct {
	model      any
	table      string
	fields     []string
	needsWhere string
	updates    map[string]any
}

func prepareOrganizationScopeMigration(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	for _, spec := range organizationScopeMigrationSpecs() {
		if !db.Migrator().HasTable(spec.model) {
			continue
		}
		for _, field := range spec.fields {
			if !db.Migrator().HasColumn(spec.model, field) {
				if err := db.Migrator().AddColumn(spec.model, field); err != nil {
					return err
				}
			}
		}
		if err := backfillOrganizationScopeRows(db, spec); err != nil {
			return err
		}
	}
	return nil
}

func backfillOrganizationScopeRows(db *gorm.DB, spec organizationScopeMigrationSpec) error {
	lastId := 0
	for {
		var ids []int
		if err := db.Table(spec.table).
			Where("id > ? AND (scope_type IS NULL OR scope_type = '' OR scope_type = ?)", lastId, AccountContextTypePersonal).
			Where(spec.needsWhere).
			Order("id ASC").
			Limit(organizationScopeMigrationBatchSize).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			return tx.Table(spec.table).Where("id IN ?", ids).Updates(spec.updates).Error
		}); err != nil {
			return err
		}
		lastId = ids[len(ids)-1]
	}
}

func organizationScopeMigrationSpecs() []organizationScopeMigrationSpec {
	personalScopeUpdates := func(includeBilling, includeActor bool) map[string]any {
		updates := map[string]any{
			"scope_type":          gorm.Expr("CASE WHEN scope_type IS NULL OR scope_type = '' THEN ? ELSE scope_type END", AccountContextTypePersonal),
			"scope_id":            gorm.Expr("CASE WHEN scope_id IS NULL OR scope_id = 0 THEN user_id ELSE scope_id END"),
			"organization_id":     0,
			"creator_user_id":     gorm.Expr("CASE WHEN creator_user_id IS NULL OR creator_user_id = 0 THEN user_id ELSE creator_user_id END"),
			"responsible_user_id": gorm.Expr("CASE WHEN responsible_user_id IS NULL OR responsible_user_id = 0 THEN user_id ELSE responsible_user_id END"),
		}
		if includeBilling {
			updates["billing_account_type"] = gorm.Expr("CASE WHEN billing_account_type IS NULL OR billing_account_type = '' THEN ? ELSE billing_account_type END", AccountContextTypePersonal)
			updates["billing_account_id"] = gorm.Expr("CASE WHEN billing_account_id IS NULL OR billing_account_id = 0 THEN user_id ELSE billing_account_id END")
		}
		if includeActor {
			updates["actor_user_id"] = gorm.Expr("CASE WHEN actor_user_id IS NULL OR actor_user_id = 0 THEN user_id ELSE actor_user_id END")
		}
		return updates
	}

	tokenUpdates := personalScopeUpdates(false, false)
	tokenUpdates["visibility"] = gorm.Expr("CASE WHEN visibility IS NULL OR visibility = '' THEN ? ELSE visibility END", TokenVisibilityPrivate)
	return []organizationScopeMigrationSpec{
		{
			model:      &Token{},
			table:      "tokens",
			fields:     []string{"ScopeType", "ScopeId", "Visibility", "OrganizationId", "CreatorUserId", "ResponsibleUserId"},
			needsWhere: "scope_id IS NULL OR scope_id = 0 OR visibility IS NULL OR visibility = '' OR organization_id IS NULL OR organization_id <> 0 OR creator_user_id IS NULL OR creator_user_id = 0 OR responsible_user_id IS NULL OR responsible_user_id = 0",
			updates:    tokenUpdates,
		},
		{
			model:      &Task{},
			table:      "tasks",
			fields:     []string{"ScopeType", "ScopeId", "BillingAccountType", "BillingAccountId", "OrganizationId", "ActorUserId", "CreatorUserId", "ResponsibleUserId"},
			needsWhere: "scope_id IS NULL OR scope_id = 0 OR billing_account_type IS NULL OR billing_account_type = '' OR billing_account_id IS NULL OR billing_account_id = 0 OR organization_id IS NULL OR organization_id <> 0 OR actor_user_id IS NULL OR actor_user_id = 0 OR creator_user_id IS NULL OR creator_user_id = 0 OR responsible_user_id IS NULL OR responsible_user_id = 0",
			updates:    personalScopeUpdates(true, true),
		},
		{
			model:      &Midjourney{},
			table:      "midjourneys",
			fields:     []string{"ScopeType", "ScopeId", "BillingAccountType", "BillingAccountId", "OrganizationId", "ActorUserId", "CreatorUserId", "ResponsibleUserId"},
			needsWhere: "scope_id IS NULL OR scope_id = 0 OR billing_account_type IS NULL OR billing_account_type = '' OR billing_account_id IS NULL OR billing_account_id = 0 OR organization_id IS NULL OR organization_id <> 0 OR actor_user_id IS NULL OR actor_user_id = 0 OR creator_user_id IS NULL OR creator_user_id = 0 OR responsible_user_id IS NULL OR responsible_user_id = 0",
			updates:    personalScopeUpdates(true, true),
		},
		{
			model:      &QuotaData{},
			table:      "quota_data",
			fields:     []string{"ScopeType", "ScopeId", "BillingAccountType", "BillingAccountId", "OrganizationId", "ResponsibleUserId"},
			needsWhere: "scope_id IS NULL OR scope_id = 0 OR billing_account_type IS NULL OR billing_account_type = '' OR billing_account_id IS NULL OR billing_account_id = 0 OR organization_id IS NULL OR organization_id <> 0 OR responsible_user_id IS NULL OR responsible_user_id = 0",
			updates: map[string]any{
				"scope_type":           gorm.Expr("CASE WHEN scope_type IS NULL OR scope_type = '' THEN ? ELSE scope_type END", AccountContextTypePersonal),
				"scope_id":             gorm.Expr("CASE WHEN scope_id IS NULL OR scope_id = 0 THEN user_id ELSE scope_id END"),
				"billing_account_type": gorm.Expr("CASE WHEN billing_account_type IS NULL OR billing_account_type = '' THEN ? ELSE billing_account_type END", AccountContextTypePersonal),
				"billing_account_id":   gorm.Expr("CASE WHEN billing_account_id IS NULL OR billing_account_id = 0 THEN user_id ELSE billing_account_id END"),
				"organization_id":      0,
				"responsible_user_id":  gorm.Expr("CASE WHEN responsible_user_id IS NULL OR responsible_user_id = 0 THEN user_id ELSE responsible_user_id END"),
			},
		},
		{
			model:      &Log{},
			table:      "logs",
			fields:     []string{"ScopeType", "ScopeId", "BillingAccountType", "BillingAccountId", "OrganizationId", "ActorUserId", "CreatorUserId", "ResponsibleUserId"},
			needsWhere: "scope_id IS NULL OR scope_id = 0 OR billing_account_type IS NULL OR billing_account_type = '' OR billing_account_id IS NULL OR billing_account_id = 0 OR organization_id IS NULL OR organization_id <> 0 OR actor_user_id IS NULL OR actor_user_id = 0 OR creator_user_id IS NULL OR creator_user_id = 0 OR responsible_user_id IS NULL OR responsible_user_id = 0",
			updates:    personalScopeUpdates(true, true),
		},
	}
}
