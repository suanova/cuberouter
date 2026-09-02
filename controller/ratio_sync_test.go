package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRatioSyncParsePricingItems(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantItems []pricingSyncItem
		wantErr   bool
	}{
		{
			name: "new wrapped shape with off_peak_window",
			data: `{
				"pricings": [
					{"model_name": "gpt-4o", "quota_type": 0, "model_ratio": 1, "completion_ratio": 0.5, "cache_ratio": 0.1, "billing_mode": "tiered_expr", "billing_expr": "x"},
					{"model_name": "viduq3-pro", "quota_type": 1, "model_price": 0.002}
				],
				"off_peak_window": {"start_hour": 22, "end_hour": 8, "timezone": "Asia/Shanghai"}
			}`,
			wantItems: []pricingSyncItem{
				{ModelName: "gpt-4o", QuotaType: 0, ModelRatio: 1, CompletionRatio: 0.5, CacheRatio: lo.ToPtr(0.1), BillingMode: billing_setting.BillingModeTieredExpr, BillingExpr: "x"},
				{ModelName: "viduq3-pro", QuotaType: 1, ModelPrice: 0.002},
			},
		},
		{
			name: "legacy bare array shape",
			data: `[
				{"model_name": "gpt-4o", "quota_type": 0, "model_ratio": 2, "completion_ratio": 0.25},
				{"model_name": "gpt-4o-mini", "quota_type": 1, "model_price": 0.001, "audio_ratio": 0.5}
			]`,
			wantItems: []pricingSyncItem{
				{ModelName: "gpt-4o", QuotaType: 0, ModelRatio: 2, CompletionRatio: 0.25},
				{ModelName: "gpt-4o-mini", QuotaType: 1, ModelPrice: 0.001, AudioRatio: lo.ToPtr(0.5)},
			},
		},
		{
			name: "null data accepted as empty",
			data: `null`,
		},
		{
			name:    "object without pricings rejected",
			data:    `{"nope": 1}`,
			wantErr: true,
		},
		{
			name:    "scalar data rejected",
			data:    `42`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := parsePricingSyncItems([]byte(tt.data))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantItems, items)
		})
	}
}
