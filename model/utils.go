package model

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	BatchUpdateTypeUserQuota = iota
	BatchUpdateTypeTokenQuota
	BatchUpdateTypeUsedQuota
	BatchUpdateTypeChannelUsedQuota
	BatchUpdateTypeRequestCount
	BatchUpdateTypeTotalPromptTokens
	BatchUpdateTypeTotalCompletionTokens
	BatchUpdateTypeTotalCacheTokens
	BatchUpdateTypeCount // if you add a new type, you need to add a new map and a new lock
)

var batchUpdateStores []map[int]int
var batchUpdateLocks []sync.Mutex

func init() {
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateStores = append(batchUpdateStores, make(map[int]int))
		batchUpdateLocks = append(batchUpdateLocks, sync.Mutex{})
	}
}

func InitBatchUpdater() {
	gopool.Go(func() {
		for {
			time.Sleep(time.Duration(common.BatchUpdateInterval) * time.Second)
			batchUpdate()
		}
	})
}

func addNewRecord(type_ int, id int, value int) {
	batchUpdateLocks[type_].Lock()
	defer batchUpdateLocks[type_].Unlock()
	old, ok := batchUpdateStores[type_][id]
	if !ok {
		batchUpdateStores[type_][id] = value
		return
	}

	sum := old + value
	if (value > 0 && sum < old) || (value < 0 && sum > old) {
		common.SysError(fmt.Sprintf("batch update overflow: type=%d id=%d old=%d value=%d", type_, id, old, value))
		if value > 0 {
			sum = math.MaxInt
		} else {
			sum = math.MinInt
		}
	}
	batchUpdateStores[type_][id] = sum
}

func batchUpdate() {
	// check if there's any data to update
	hasData := false
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		if len(batchUpdateStores[i]) > 0 {
			hasData = true
			batchUpdateLocks[i].Unlock()
			break
		}
		batchUpdateLocks[i].Unlock()
	}

	if !hasData {
		return
	}

	common.SysLog("batch update started")
	stores := make([]map[int]int, BatchUpdateTypeCount)
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		stores[i] = batchUpdateStores[i]
		batchUpdateStores[i] = make(map[int]int)
		batchUpdateLocks[i].Unlock()
	}

	for i, store := range stores {
		if i == BatchUpdateTypeUserQuota || i == BatchUpdateTypeUsedQuota || i == BatchUpdateTypeRequestCount {
			continue
		}
		for key, value := range store {
			var err error
			switch i {
			case BatchUpdateTypeTokenQuota:
				err = increaseTokenQuota(key, value)
			case BatchUpdateTypeChannelUsedQuota:
				err = updateChannelUsedQuota(key, value)
			case BatchUpdateTypeTotalPromptTokens:
				err = updateUserTotalPromptTokens(key, int64(value))
			case BatchUpdateTypeTotalCompletionTokens:
				err = updateUserTotalCompletionTokens(key, int64(value))
			case BatchUpdateTypeTotalCacheTokens:
				err = updateUserTotalCacheTokens(key, int64(value))
			}
			if err != nil {
				// 落库失败只丢这一条：把增量塞回缓冲、下个周期重试。丢一轮就是永久
				// 幻影——DB 从此偏高，之后每次水合都把丢掉的扣减复活成可用余额。
				common.SysError(fmt.Sprintf("failed to batch update (type=%d id=%d delta=%d, will retry): %s", i, key, value, err.Error()))
				addNewRecord(i, key, value)
			}
		}
	}

	userQuotaStore := stores[BatchUpdateTypeUserQuota]
	usedQuotaStore := stores[BatchUpdateTypeUsedQuota]
	requestCountStore := stores[BatchUpdateTypeRequestCount]

	userIDs := make(map[int]struct{}, len(userQuotaStore)+len(usedQuotaStore)+len(requestCountStore))
	for key := range userQuotaStore {
		userIDs[key] = struct{}{}
	}
	for key := range usedQuotaStore {
		userIDs[key] = struct{}{}
	}
	for key := range requestCountStore {
		userIDs[key] = struct{}{}
	}
	for key := range userIDs {
		quota, usedQuota, requestCount := userQuotaStore[key], usedQuotaStore[key], requestCountStore[key]
		if err := updateUserQuotaUsedQuotaAndRequestCount(key, quota, usedQuota, requestCount); err != nil {
			// 用户三件套是合并成一条 UPDATE 写的，失败时三个分量一起回填（零增量跳过，
			// 避免往缓冲里塞无意义的 0 条目）。
			common.SysError(fmt.Sprintf("failed to batch update user quota (id=%d quota=%d used_quota=%d request_count=%d, will retry): %s",
				key, quota, usedQuota, requestCount, err.Error()))
			if quota != 0 {
				addNewRecord(BatchUpdateTypeUserQuota, key, quota)
			}
			if usedQuota != 0 {
				addNewRecord(BatchUpdateTypeUsedQuota, key, usedQuota)
			}
			if requestCount != 0 {
				addNewRecord(BatchUpdateTypeRequestCount, key, requestCount)
			}
		}
	}
	common.SysLog("batch update finished")
}

func RecordExist(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func shouldUpdateRedis(fromDB bool, err error) bool {
	return common.RedisEnabled && fromDB && err == nil
}
