package ratio_setting

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

type VideoPriceRow struct {
	Resolution   string  `json:"resolution"`
	NormalPrice  float64 `json:"normal_price"`
	OffPeakPrice float64 `json:"off_peak_price"`
}

type VideoPriceTable struct {
	Rows []VideoPriceRow `json:"rows"`
}

var videoPriceMap = types.NewRWMap[string, *VideoPriceTable]()

func UpdateVideoPriceByJSONString(jsonStr string) error {
	var m map[string]*VideoPriceTable
	if err := common.UnmarshalJsonStr(jsonStr, &m); err != nil {
		return err
	}
	for model, table := range m {
		if table == nil || len(table.Rows) == 0 {
			return fmt.Errorf("video price table for %s is empty", model)
		}
		for _, row := range table.Rows {
			if strings.TrimSpace(row.Resolution) == "" || row.NormalPrice <= 0 || row.OffPeakPrice <= 0 {
				return fmt.Errorf("invalid video price row for %s: %+v", model, row)
			}
		}
	}
	// 校验全部通过后整体替换;失败时保持旧配置不变。
	// ReplaceAll 单次加锁原子交换,读方不会观察到空表中间态。
	videoPriceMap.ReplaceAll(m)
	return nil
}

func GetVideoPrice(model string) (*VideoPriceTable, bool) {
	return videoPriceMap.Get(model)
}

// VideoPriceAnchor 返回表内 normal_price 最高的行作为锚点(保证 size 系数 ≤ 1)。
func VideoPriceAnchor(t *VideoPriceTable) float64 {
	anchor := 0.0
	for _, row := range t.Rows {
		if row.NormalPrice > anchor {
			anchor = row.NormalPrice
		}
	}
	return anchor
}

// VideoPriceModelPrice 返回按次计费的美元价格:锚点 ¥/秒 → USD per-call。
func VideoPriceModelPrice(t *VideoPriceTable) float64 {
	return VideoPriceAnchor(t) / USD2RMB
}

type OffPeakWindow struct {
	StartHour int    `json:"start_hour"`
	EndHour   int    `json:"end_hour"`
	Timezone  string `json:"timezone"`
}

var (
	offPeakWindowMu sync.RWMutex
	offPeakWindow   = OffPeakWindow{StartHour: 22, EndHour: 8, Timezone: "Asia/Shanghai"}
)

func UpdateOffPeakWindowByJSONString(jsonStr string) error {
	var w OffPeakWindow
	if err := common.UnmarshalJsonStr(jsonStr, &w); err != nil {
		return err
	}
	if w.StartHour < 0 || w.StartHour > 23 || w.EndHour < 0 || w.EndHour > 23 {
		return fmt.Errorf("off-peak hours must be in [0,23]: %+v", w)
	}
	if w.Timezone == "" {
		w.Timezone = "Asia/Shanghai"
	}
	// 时区必须是可加载的 IANA 名;无法加载的配置会让 IsOffPeakHour 静默失效,
	// 导致错峰价永不生效,这里直接拒绝
	if _, err := time.LoadLocation(w.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", w.Timezone, err)
	}
	offPeakWindowMu.Lock()
	offPeakWindow = w
	offPeakWindowMu.Unlock()
	return nil
}

func GetOffPeakWindow() OffPeakWindow {
	offPeakWindowMu.RLock()
	defer offPeakWindowMu.RUnlock()
	return offPeakWindow
}

// IsOffPeakHour 半开区间 [start, end);start == end 视为无错峰;跨零点合法。
func IsOffPeakHour(now time.Time, w OffPeakWindow) bool {
	if w.StartHour == w.EndHour {
		return false
	}
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		return false
	}
	hour := now.In(loc).Hour()
	if w.StartHour < w.EndHour {
		return hour >= w.StartHour && hour < w.EndHour
	}
	return hour >= w.StartHour || hour < w.EndHour
}
