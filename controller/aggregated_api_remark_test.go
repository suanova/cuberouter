package controller

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Issue #88: 聚合 API 创建用户 remark 组合规则单元测试
func TestBuildCreateUserRemark(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		remark        string
		userValidity  string
		expected      string
		expectedHasRx bool // 是否应触发 remark 更新
	}{
		{
			name:          "仅传 user_validity_保持原有行为",
			remark:        "",
			userValidity:  "2026-12-31",
			expected:      "账户有效期至 2026-12-31",
			expectedHasRx: true,
		},
		{
			name:          "仅传 remark_不做 remark 更新_由 Insert 直接写入",
			remark:        "运营渠道A用户",
			userValidity:  "",
			expected:      "",
			expectedHasRx: false,
		},
		{
			name:          "两者同时传入_组合不覆盖用户备注",
			remark:        "运营渠道A用户",
			userValidity:  "2026-12-31",
			expected:      "运营渠道A用户；账户有效期至 2026-12-31",
			expectedHasRx: true,
		},
		{
			name:          "user_validity 解析失败_不更新 remark",
			remark:        "运营渠道A用户",
			userValidity:  "2026/12/31",
			expected:      "",
			expectedHasRx: false,
		},
		{
			name:          "user_validity 非法格式_不更新 remark",
			remark:        "",
			userValidity:  "not-a-date",
			expected:      "",
			expectedHasRx: false,
		},
		{
			name:          "remark 与有效性拼接后接近列上限",
			remark:        "备注",
			userValidity:  "2099-01-01",
			expected:      "备注；账户有效期至 2099-01-01",
			expectedHasRx: true,
		},
		{
			// Issue #89 TC13：remark 已达 255 字符且同传 user_validity 时，
			// 拼接值超 varchar(255) 上限 → 截断为 255 字符，保留用户 remark 头部，不报错。
			name:          "拼接后超过255字符_截断为255_保留用户备注头部",
			remark:        strings.Repeat("备", 255),
			userValidity:  "2026-12-31",
			expected:      strings.Repeat("备", 255),
			expectedHasRx: true,
		},
		{
			// Issue #89：拼接后恰好 256 字符 → 截断为 255（前 238 个「备」 + 分号 + 16 字符有效期段）。
			name:          "拼接后恰好256字符_截断为255",
			remark:        strings.Repeat("备", 238),
			userValidity:  "2026-12-31",
			expected:      strings.Repeat("备", 238) + "；账户有效期至 2026-12-3",
			expectedHasRx: true,
		},
		{
			// Issue #89：拼接后恰好 255 字符 → 不截断，保持原值。
			name:          "拼接后恰好255字符_不截断",
			remark:        strings.Repeat("备", 237),
			userValidity:  "2026-12-31",
			expected:      strings.Repeat("备", 237) + "；账户有效期至 2026-12-31",
			expectedHasRx: true,
		},
		{
			// Issue #89：多字节字符截断必须按字符（rune）而非字节，不得出现半个字符。
			name:          "多字节字符截断_按字符不按字节",
			remark:        strings.Repeat("运", 255),
			userValidity:  "2026-12-31",
			expected:      strings.Repeat("运", 255),
			expectedHasRx: true,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := buildCreateUserRemark(tc.remark, tc.userValidity)
			require.Equal(t, tc.expectedHasRx, ok, "ok = %v, want %v", ok, tc.expectedHasRx)
			if tc.expectedHasRx {
				assert.Equal(t, tc.expected, got)
				// Issue #89 不变量：最终写入的 remark 不超过 255 字符
				assert.LessOrEqual(t, utf8.RuneCountInString(got), 255)
			}
		})
	}
}
