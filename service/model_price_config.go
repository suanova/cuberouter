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
	"fmt"
	"math"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/channelconfig"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ModelPriceApplyResult summarizes one declarative pricing write.
type ModelPriceApplyResult struct {
	// Models is the number of models whose price was written.
	Models int `json:"models"`
	// Written maps each option that was persisted to its update count.
	Written map[string]int `json:"written"`
	// Warnings reports prices that cuberouter will not bill as configured.
	Warnings []string `json:"warnings"`
}

// ApplyModelPrices merges the given prices into the global option tables.
//
// Every table is read-copied, merged and written back as a whole because
// UpdateOption replaces the value, so writing only the new entries would drop
// the built-in defaults and every other model. Writes are serialized by the
// same lock used by the CSV import; a failed table write is reported but does
// not stop the remaining tables, and the caller may safely retry.
func ApplyModelPrices(prices map[string]channelconfig.ModelPrice) (*ModelPriceApplyResult, error) {
	result := &ModelPriceApplyResult{
		Written:  map[string]int{},
		Warnings: []string{},
	}
	if len(prices) == 0 {
		return result, nil
	}

	names := make([]string, 0, len(prices))
	for name := range prices {
		names = append(names, name)
	}
	sort.Strings(names)

	modelRatios := make(map[string]float64)
	completionRatios := make(map[string]float64)
	cacheRatios := make(map[string]float64)
	createCacheRatios := make(map[string]float64)
	modelPrices := make(map[string]float64)

	for _, name := range names {
		price := prices[name]
		result.Models++
		if price.PerRequest != nil {
			modelPrices[name] = *price.PerRequest
			continue
		}
		modelRatios[name] = price.ModelRatio
		completionRatios[name] = price.CompletionRatio
		if price.CacheRatio != nil {
			cacheRatios[name] = *price.CacheRatio
		}
		if price.CreateCacheRatio != nil {
			createCacheRatios[name] = *price.CreateCacheRatio
		}
		if info := ratio_setting.GetCompletionRatioInfo(name); info.Locked && !closeEnough(info.Ratio, price.CompletionRatio) {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"model %q: the built-in completion ratio %.6g overrides the configured %.6g, so the output price may not match the config",
				name, info.Ratio, price.CompletionRatio,
			))
		}
	}

	ratioWriteMu.Lock()
	defer ratioWriteMu.Unlock()

	tables := []struct {
		option  string
		updates map[string]float64
		current func() map[string]float64
	}{
		{"ModelRatio", modelRatios, ratio_setting.GetModelRatioCopy},
		{"CompletionRatio", completionRatios, ratio_setting.GetCompletionRatioCopy},
		{"CacheRatio", cacheRatios, ratio_setting.GetCacheRatioCopy},
		{"CreateCacheRatio", createCacheRatios, ratio_setting.GetCreateCacheRatioCopy},
		{"ModelPrice", modelPrices, ratio_setting.GetModelPriceCopy},
	}

	var writeErrors []error
	for _, table := range tables {
		if len(table.updates) == 0 {
			continue
		}
		merged := mergeMaps(table.current(), table.updates)
		encoded, err := common.Marshal(merged)
		if err != nil {
			// Never call UpdateOption with an empty value: it would wipe the
			// global price table.
			writeErrors = append(writeErrors, fmt.Errorf("marshal %s: %w", table.option, err))
			continue
		}
		if err := model.UpdateOption(table.option, string(encoded)); err != nil {
			writeErrors = append(writeErrors, fmt.Errorf("save %s: %w", table.option, err))
			continue
		}
		result.Written[table.option] = len(table.updates)
	}
	return result, errors.Join(writeErrors...)
}

func closeEnough(left, right float64) bool {
	return math.Abs(left-right) <= 1e-9
}
