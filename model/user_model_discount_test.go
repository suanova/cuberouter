package model

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// discountEntries builds a discount map with count unique entries whose model
// names are nameLen bytes long (zero-padded decimal), for boundary tests.
func discountEntries(count int, nameLen int, discount float64) map[string]float64 {
	discounts := make(map[string]float64, count)
	for i := 0; i < count; i++ {
		discounts[fmt.Sprintf("%0*d", nameLen, i)] = discount
	}
	return discounts
}

func TestNormalizeUserModelDiscounts(t *testing.T) {
	tests := []struct {
		name            string
		input           map[string]float64
		want            map[string]float64
		wantErr         error
		wantErrContains string
	}{
		{
			name:  "keeps valid discount",
			input: map[string]float64{"m": 0.8},
			want:  map[string]float64{"m": 0.8},
		},
		{
			name:  "drops full price entry",
			input: map[string]float64{"m": 1.0},
			want:  map[string]float64{},
		},
		{
			name:  "rounds discount to four decimals",
			input: map[string]float64{"m": 0.85678},
			want:  map[string]float64{"m": 0.8568},
		},
		{
			name:  "trims model name",
			input: map[string]float64{" m ": 0.8},
			want:  map[string]float64{"m": 0.8},
		},
		{
			name:    "rejects discount that rounds to zero",
			input:   map[string]float64{"m": 0.00004},
			wantErr: ErrUserModelDiscountRange,
		},
		{
			name:  "rounds discount up to smallest storable value",
			input: map[string]float64{"m": 0.00006},
			want:  map[string]float64{"m": 0.0001},
		},
		{
			name:  "drops discount that rounds up to full price",
			input: map[string]float64{"m": 0.99999},
			want:  map[string]float64{},
		},
		{
			name:  "drops discount that rounds down to full price",
			input: map[string]float64{"m": 1.00004},
			want:  map[string]float64{},
		},
		{
			name:    "rejects zero",
			input:   map[string]float64{"m": 0},
			wantErr: ErrUserModelDiscountRange,
		},
		{
			name:    "rejects negative",
			input:   map[string]float64{"m": -0.2},
			wantErr: ErrUserModelDiscountRange,
		},
		{
			name:    "rejects NaN",
			input:   map[string]float64{"m": math.NaN()},
			wantErr: ErrUserModelDiscountRange,
		},
		{
			name:    "rejects positive infinity",
			input:   map[string]float64{"m": math.Inf(1)},
			wantErr: ErrUserModelDiscountRange,
		},
		{
			name:            "rejects above one and names the model",
			input:           map[string]float64{"kimi-k3": 1.2},
			wantErr:         ErrUserModelDiscountRange,
			wantErrContains: "kimi-k3",
		},
		{
			name:    "rejects empty model name",
			input:   map[string]float64{"": 0.5},
			wantErr: ErrUserModelDiscountName,
		},
		{
			name:    "rejects overlong model name",
			input:   map[string]float64{strings.Repeat("a", 256): 0.5},
			wantErr: ErrUserModelDiscountName,
		},
		{
			name:    "rejects too many entries",
			input:   discountEntries(501, 4, 0.5),
			wantErr: ErrUserModelDiscountCount,
		},
		{
			name:    "rejects oversized payload",
			input:   discountEntries(500, 300, 0.5),
			wantErr: ErrUserModelDiscountSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeUserModelDiscounts(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				if tt.wantErrContains != "" {
					assert.ErrorContains(t, err, tt.wantErrContains)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseUserModelDiscounts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[string]float64
	}{
		{name: "empty string", raw: "", want: nil},
		{name: "invalid JSON", raw: "{bad", want: nil},
		{name: "skips invalid entries", raw: `{"a":0.5,"b":2,"":0.3}`, want: map[string]float64{"a": 0.5}},
		{name: "skips zero discount", raw: `{"a":0.5,"b":0}`, want: map[string]float64{"a": 0.5}},
		{name: "keeps value without rounding", raw: `{"a":0.123456}`, want: map[string]float64{"a": 0.123456}},
		{name: "keeps full price entry", raw: `{"a":1}`, want: map[string]float64{"a": 1}},
		{name: "trims model name", raw: `{" a ":0.5}`, want: map[string]float64{"a": 0.5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseUserModelDiscounts(tt.raw)
			if tt.want == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMarshalUserModelDiscounts(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		assert.Equal(t, "", MarshalUserModelDiscounts(map[string]float64{}))
	})
	t.Run("nil map", func(t *testing.T) {
		assert.Equal(t, "", MarshalUserModelDiscounts(nil))
	})
	t.Run("sorts keys", func(t *testing.T) {
		assert.Equal(t, `{"a":0.9,"b":0.5}`, MarshalUserModelDiscounts(map[string]float64{"b": 0.5, "a": 0.9}))
	})
}

func TestLookupModelDiscount(t *testing.T) {
	discounts := map[string]float64{
		"m":        0.8,
		"bad-high": 1.5,
		"bad-zero": 0,
		"bad-nan":  math.NaN(),
	}
	tests := []struct {
		name      string
		modelName string
		want      float64
	}{
		{name: "hit returns discount", modelName: "m", want: 0.8},
		{name: "miss returns full price", modelName: "other", want: 1.0},
		{name: "invalid above one returns full price", modelName: "bad-high", want: 1.0},
		{name: "invalid zero returns full price", modelName: "bad-zero", want: 1.0},
		{name: "invalid NaN returns full price", modelName: "bad-nan", want: 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, LookupModelDiscount(discounts, tt.modelName))
		})
	}

	t.Run("nil map returns full price", func(t *testing.T) {
		assert.Equal(t, 1.0, LookupModelDiscount(nil, "m"))
	})
}
