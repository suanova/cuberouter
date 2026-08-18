package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// BillingAggRow 账单报表聚合行。
// DayKey 为"本地自然日"序号(自 Unix 纪元起的天数,按服务器本地时区截断);
// 仅按日查询填充,汇总查询留 0。日期字符串由调用方通过 DayKey 还原。
//
// CacheTokens 恒为 0（占位）：本仓库 Log 表没有 cache_tokens 列，缓存 token
// 数据记录在日志 other JSON 中，跨库（SQLite/MySQL/PG）JSON 提取不可行，
// 报表暂以占位 0 返回，调用方不应把该字段当作实测值。
type BillingAggRow struct {
	ModelName        string
	DayKey           int64
	RequestCount     int
	PromptTokens     int
	CompletionTokens int
	CacheTokens      int
	Quota            int
}

// localUTCOffsetSeconds 返回服务器本地时区相对 UTC 的偏移秒数。
// 用于把 Unix 秒按本地自然日截断(避免依赖各数据库的时区函数差异)。
func localUTCOffsetSeconds() int {
	_, offset := time.Now().In(time.Local).Zone()
	return offset
}

// dayKeyExpr 返回计算"本地自然日序号"的 SQL 表达式片段:
//
//	FLOOR((created_at + <offset>) / 86400.0)
//
// created_at 为 Unix 秒 bigint。offset 由参数绑定,使三库(PG/MySQL/SQLite)
// 行为完全一致,且不依赖 DB session 时区。DST 时区在切换日会有 ±1 天偏差,
// 当前部署为 Asia/Shanghai(无 DST),可安全使用。
const daySeconds = 86400.0

// GetUserBillingAgg 按 (username, 时间区间) 聚合消费日志(type=2),
// 返回按 (model_name, 本地自然日) 分组的明细。调用方如需仅按模型汇总,
// 传 withDaily=false 即只 GROUP BY model_name(DayKey 留 0)。
func GetUserBillingAgg(username string, startTs, endTs int64, withDaily bool) ([]BillingAggRow, error) {
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}
	offset := localUTCOffsetSeconds()

	tx := LOG_DB.Table("logs").
		Where("type = ?", LogTypeConsume).
		Where("username = ?", username).
		Where("created_at >= ?", startTs).
		Where("created_at <= ?", endTs)

	var rows []BillingAggRow
	if withDaily {
		tx = tx.Select("model_name, FLOOR((created_at + ?) / ?) AS day_key, "+
			"count(*) AS request_count, "+
			"sum(prompt_tokens) AS prompt_tokens, "+
			"sum(completion_tokens) AS completion_tokens, "+
			"0 AS cache_tokens, "+
			"sum(quota) AS quota", offset, daySeconds).
			Group("model_name, day_key").
			Order("day_key, model_name")
	} else {
		tx = tx.Select("model_name, " +
			"count(*) AS request_count, " +
			"sum(prompt_tokens) AS prompt_tokens, " +
			"sum(completion_tokens) AS completion_tokens, " +
			"0 AS cache_tokens, " +
			"sum(quota) AS quota").
			Group("model_name").
			Order("model_name")
	}

	if err := tx.Scan(&rows).Error; err != nil {
		common.SysError("failed to query billing agg: " + err.Error())
		return nil, errors.New("查询账单数据失败")
	}
	if rows == nil {
		rows = []BillingAggRow{}
	}
	return rows, nil
}

// BillingDayKeyToDate 把本地自然日序号还原为 YYYY-MM-DD(本地时区)。
func BillingDayKeyToDate(dayKey int64) string {
	// dayKey*86400 落在某个 UTC 自然日的 00:00,本地时区下仍属同一日。
	return time.Unix(dayKey*int64(daySeconds), 0).In(time.Local).Format("2006-01-02")
}

// ReconciliationRow 计费对账报表按用户聚合行。
// CacheTokens 恒为 0（占位），原因同 BillingAggRow：Log 表无 cache_tokens 列。
type ReconciliationRow struct {
	UserId           int
	Username         string
	RequestCount     int
	PromptTokens     int
	CompletionTokens int
	CacheTokens      int
	Quota            int
	ModelCount       int
}

// GetReconciliationReport 按时间区间汇总所有有消费记录的用户(type=2),
// 每用户一行:总请求次数/token 用量/quota/涉及模型数。按 quota 降序。
// 用户概要(display_name/group/role)由调用方按 user_id 批量补齐。
func GetReconciliationReport(startTs, endTs int64) ([]ReconciliationRow, error) {
	// 一次 SQL:按 user_id 聚合用量,并用 count(distinct model_name) 算模型数
	var rows []ReconciliationRow
	err := LOG_DB.Table("logs").
		Select("user_id, "+
			"MAX(username) AS username, "+
			"count(*) AS request_count, "+
			"sum(prompt_tokens) AS prompt_tokens, "+
			"sum(completion_tokens) AS completion_tokens, "+
			"0 AS cache_tokens, "+
			"sum(quota) AS quota, "+
			"count(distinct model_name) AS model_count").
		Where("type = ?", LogTypeConsume).
		Where("created_at >= ?", startTs).
		Where("created_at <= ?", endTs).
		Group("user_id").
		Order("quota desc").
		Scan(&rows).Error
	if err != nil {
		common.SysError("failed to query reconciliation: " + err.Error())
		return nil, errors.New("查询对账数据失败")
	}
	if rows == nil {
		rows = []ReconciliationRow{}
	}
	return rows, nil
}
