package ratio_setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

// ImagePriceTiers 是图片按张价目表的固定画质档位(与图像请求的 quality 字段取值一致)。
var ImagePriceTiers = []string{"fast", "standard", "high"}

// ImagePriceRow 是图片按张价目表的一个画质档位价格。
// Tier 必须是 ImagePriceTiers 之一(大小写不敏感);
// Price 为该档位下单张图片的美元价格(与视频表同口径,USD 存储)。
type ImagePriceRow struct {
	Tier  string  `json:"tier"`
	Price float64 `json:"price"`
}

// ImagePriceTable 是单个模型的图片按张价目表:画质档位 → 单张美元价。
// 计费 = 锚点(最高价行)× 档位系数(档位价/锚点)× 张数 n。
// 请求 quality 缺省或不在表内时按锚点(最贵档)计费。
type ImagePriceTable struct {
	Rows []ImagePriceRow `json:"rows"`
}

var imagePriceMap = types.NewRWMap[string, *ImagePriceTable]()

// ParseImagePriceMap 解析 ImagePrice option 的顶层 map(模型名 → 价格表)。
func ParseImagePriceMap(jsonStr string) (map[string]*ImagePriceTable, error) {
	m := make(map[string]*ImagePriceTable)
	if strings.TrimSpace(jsonStr) == "" {
		return m, nil
	}
	if err := common.UnmarshalJsonStr(jsonStr, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func UpdateImagePriceByJSONString(jsonStr string) error {
	m, err := ParseImagePriceMap(jsonStr)
	if err != nil {
		return err
	}
	for model, table := range m {
		if table == nil || len(table.Rows) == 0 {
			return fmt.Errorf("image price table for %s is empty", model)
		}
		seen := make(map[string]struct{}, len(table.Rows))
		for i := range table.Rows {
			row := &table.Rows[i]
			row.Tier = NormalizeImagePriceTier(row.Tier)
			if !IsValidImagePriceTier(row.Tier) {
				return fmt.Errorf("invalid image price tier for %s: %q (want one of %v)",
					model, row.Tier, ImagePriceTiers)
			}
			if row.Price <= 0 {
				return fmt.Errorf("invalid image price row for %s: %+v", model, row)
			}
			if _, dup := seen[row.Tier]; dup {
				return fmt.Errorf("duplicate image price tier for %s: %q", model, row.Tier)
			}
			seen[row.Tier] = struct{}{}
		}
	}
	// 校验全部通过后整体替换;失败时保持旧配置不变。
	// ReplaceAll 单次加锁原子交换,读方不会观察到空表中间态。
	imagePriceMap.ReplaceAll(m)
	return nil
}

func GetImagePrice(model string) (*ImagePriceTable, bool) {
	return imagePriceMap.Get(model)
}

// NormalizeImagePriceTier 归一化画质档位:去首尾空白、小写。
func NormalizeImagePriceTier(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// IsValidImagePriceTier 判断 tier 是否为固定画质档位之一(调用前需已归一化或传入规范值)。
func IsValidImagePriceTier(tier string) bool {
	for _, t := range ImagePriceTiers {
		if tier == t {
			return true
		}
	}
	return false
}

// ImagePriceAnchor 返回表内最高价行作为锚点(保证档位系数 ≤ 1)。
// 表外档位按锚点(最贵档)计费,与视频按秒表的保守口径一致。
func ImagePriceAnchor(t *ImagePriceTable) float64 {
	anchor := 0.0
	for _, row := range t.Rows {
		if row.Price > anchor {
			anchor = row.Price
		}
	}
	return anchor
}

// ImagePriceTierRatio 返回给定画质档位的计费系数(档位价 ÷ 锚点,≤ 1)。
// 档位未配置(或为空)时返回 0,调用方不加系数,即按锚点计费。
func ImagePriceTierRatio(t *ImagePriceTable, tier string) float64 {
	anchor := ImagePriceAnchor(t)
	if anchor <= 0 {
		return 0
	}
	key := NormalizeImagePriceTier(tier)
	if key == "" {
		return 0
	}
	for _, row := range t.Rows {
		if row.Tier == key {
			return row.Price / anchor
		}
	}
	return 0
}
