package ratio_setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

// ImagePriceRow 是图片按张价目表的一档分辨率价格。
// Price 为该分辨率下单张图片的美元价格(与视频表同口径,USD 存储)。
type ImagePriceRow struct {
	Resolution string  `json:"resolution"`
	Price      float64 `json:"price"`
}

// ImagePriceTable 是单个模型的图片按张价目表:分辨率档位 → 单张美元价。
// 计费 = 锚点(最高价行)× size 系数(档位价/锚点)× 张数 n。
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
		for i := range table.Rows {
			row := &table.Rows[i]
			row.Resolution = NormalizeImageResolution(row.Resolution)
			if row.Resolution == "" || row.Price <= 0 {
				return fmt.Errorf("invalid image price row for %s: %+v", model, row)
			}
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

// NormalizeImageResolution 归一化分辨率字符串:去空白、小写、统一分隔符
// (1024*1024 / 1024 X 1024 / 1024 x 1024 → 1024x1024),
// 使配置行与请求 size 字段可以互相匹配。
func NormalizeImageResolution(s string) string {
	t := strings.ToLower(strings.TrimSpace(s))
	t = strings.ReplaceAll(t, "*", "x")
	return strings.Join(strings.Fields(t), "")
}

// ImagePriceAnchor 返回表内最高价行作为锚点(保证 size 系数 ≤ 1)。
// 表外分辨率按锚点(最贵档)计费,与视频按秒表的保守口径一致。
func ImagePriceAnchor(t *ImagePriceTable) float64 {
	anchor := 0.0
	for _, row := range t.Rows {
		if row.Price > anchor {
			anchor = row.Price
		}
	}
	return anchor
}

// ImagePriceSizeRatio 返回给定分辨率的计费 size 系数(档位价 ÷ 锚点,≤ 1)。
// 分辨率未配置(或为空)时返回 0,调用方不加系数,即按锚点计费。
func ImagePriceSizeRatio(t *ImagePriceTable, resolution string) float64 {
	anchor := ImagePriceAnchor(t)
	if anchor <= 0 {
		return 0
	}
	key := NormalizeImageResolution(resolution)
	if key == "" {
		return 0
	}
	for _, row := range t.Rows {
		if row.Resolution == key {
			return row.Price / anchor
		}
	}
	return 0
}
