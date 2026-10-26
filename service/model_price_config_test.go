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
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/channelconfig"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplyModelPricesPreservesPerRequestPrecedence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pricing.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	for _, table := range []struct {
		current string
		restore func(string) error
	}{
		{ratio_setting.ModelPrice2JSONString(), ratio_setting.UpdateModelPriceByJSONString},
		{ratio_setting.ModelRatio2JSONString(), ratio_setting.UpdateModelRatioByJSONString},
		{ratio_setting.CompletionRatio2JSONString(), ratio_setting.UpdateCompletionRatioByJSONString},
		{ratio_setting.CacheRatio2JSONString(), ratio_setting.UpdateCacheRatioByJSONString},
		{ratio_setting.CreateCacheRatio2JSONString(), ratio_setting.UpdateCreateCacheRatioByJSONString},
	} {
		t.Cleanup(func() { require.NoError(t, table.restore(table.current)) })
	}

	replacementPrice, cacheRead, cacheWrite := 0.5, 0.1, 1.25
	for _, tc := range []struct {
		name           string
		initialPrices  string
		perRequest     *float64
		wantWarning    bool
		wantPrice      float64
		wantPriceEntry bool
	}{
		{name: "existing price", initialPrices: `{"config-import-test":0.25}`, wantWarning: true, wantPrice: 0.25, wantPriceEntry: true},
		{name: "existing zero price", initialPrices: `{"config-import-test":0}`, wantWarning: true, wantPriceEntry: true},
		{name: "no existing price", initialPrices: `{}`},
		{name: "configured per request", initialPrices: `{"config-import-test":0.25}`, perRequest: &replacementPrice, wantPrice: 0.5, wantPriceEntry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(tc.initialPrices))
			if tc.wantPriceEntry {
				require.NoError(t, model.UpdateOption("ModelPrice", tc.initialPrices))
			}
			price := channelconfig.ModelPrice{
				ModelRatio:       2,
				CompletionRatio:  3,
				CacheRatio:       &cacheRead,
				CreateCacheRatio: &cacheWrite,
				PerRequest:       tc.perRequest,
			}
			result, err := ApplyModelPrices(map[string]channelconfig.ModelPrice{"config-import-test": price})
			require.NoError(t, err)
			assert.Equal(t, 1, result.Models)
			if tc.wantWarning {
				require.Len(t, result.Warnings, 1)
				assert.Contains(t, result.Warnings[0], `model "config-import-test"`)
				assert.Contains(t, result.Warnings[0], "ModelPrice")
				assert.Contains(t, result.Warnings[0], "takes precedence")
			} else {
				assert.Empty(t, result.Warnings)
			}
			gotPrice, exists := ratio_setting.GetModelPrice("config-import-test", false)
			assert.Equal(t, tc.wantPriceEntry, exists)
			if tc.wantPriceEntry {
				assert.Equal(t, tc.wantPrice, gotPrice)
				var option model.Option
				require.NoError(t, db.First(&option, "key = ?", "ModelPrice").Error)
				var persisted map[string]float64
				require.NoError(t, common.UnmarshalJsonStr(option.Value, &persisted))
				assert.Equal(t, tc.wantPrice, persisted["config-import-test"])
			}
			if tc.perRequest == nil {
				assert.NotContains(t, result.Written, "ModelPrice")
				for _, table := range []struct {
					name string
					want float64
				}{
					{"ModelRatio", 2}, {"CompletionRatio", 3}, {"CacheRatio", 0.1}, {"CreateCacheRatio", 1.25},
				} {
					assert.Equal(t, 1, result.Written[table.name])
					var option model.Option
					require.NoError(t, db.First(&option, "key = ?", table.name).Error)
					var persisted map[string]float64
					require.NoError(t, common.UnmarshalJsonStr(option.Value, &persisted))
					assert.Equal(t, table.want, persisted["config-import-test"])
				}
			}
		})
	}
}
