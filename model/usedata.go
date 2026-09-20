package model

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// QuotaData 柱状图数据
//
// idx_quota_data_account_context 覆盖这一整行身份的全部维度：既包括分析维度
// （分组/令牌/渠道/节点），也包括作用域与账单归属。原因见 upsertQuotaData：
// 多节点并发的缓存刷写都走 ON CONFLICT，冲突目标必须与行的真实身份完全一致，
// 少一列就会把两条本应独立的记录合并掉、丢掉那一维的归属。
//
// 这个索引故意不写成 gorm 的 uniqueIndex 标签，而是由
// ensureQuotaDataAccountContextIndex 显式建出来：SQLite 上只要表里带 uniqueIndex
// 标签，每次 AutoMigrate 都会整表重建，quota_data 在真实部署里可以很大，
// 每次启动复制一遍不可接受（tokens 表已经因为同一个原因长期如此）。
// logs.billing_event_key 走的是同一条路子。
type QuotaData struct {
	Id        int    `json:"id"`
	UserID    int    `json:"user_id" gorm:"index"`
	Username  string `json:"username" gorm:"index:idx_qdt_model_user_name,priority:2;size:64;default:''"`
	ModelName string `json:"model_name" gorm:"index:idx_qdt_model_user_name,priority:1;size:64;default:''"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index:idx_qdt_created_at,priority:2"`
	UseGroup  string `json:"use_group" gorm:"index;size:64;default:''"`
	TokenID   int    `json:"token_id" gorm:"index;default:0"`
	ChannelID int    `json:"channel_id" gorm:"index;default:0"`
	NodeName  string `json:"node_name" gorm:"index;size:64;default:''"`
	TokenUsed int    `json:"token_used" gorm:"default:0"`
	Count     int    `json:"count" gorm:"default:0"`
	Quota     int    `json:"quota" gorm:"default:0"`

	ScopeType          string `json:"scope_type" gorm:"type:varchar(16);index;default:'personal'"`
	ScopeId            int    `json:"scope_id" gorm:"index;default:0"`
	BillingAccountType string `json:"billing_account_type" gorm:"type:varchar(16);index;default:'personal'"`
	BillingAccountId   int    `json:"billing_account_id" gorm:"index;default:0"`
	OrganizationId     int    `json:"organization_id" gorm:"index;default:0"`
	ResponsibleUserId  int    `json:"responsible_user_id" gorm:"index;default:0"`
}

func NormalizeQuotaDataScope(quotaData *QuotaData) {
	if quotaData == nil {
		return
	}
	if quotaData.ScopeType == "" {
		quotaData.ScopeType = AccountContextTypePersonal
	}
	if quotaData.ScopeId == 0 && quotaData.ScopeType == AccountContextTypePersonal {
		quotaData.ScopeId = quotaData.UserID
	}
	if quotaData.BillingAccountType == "" {
		quotaData.BillingAccountType = AccountContextTypePersonal
	}
	if quotaData.BillingAccountId == 0 && quotaData.BillingAccountType == AccountContextTypePersonal {
		quotaData.BillingAccountId = quotaData.UserID
	}
	if quotaData.ResponsibleUserId == 0 && quotaData.ScopeType == AccountContextTypePersonal {
		quotaData.ResponsibleUserId = quotaData.UserID
	}
}

type QuotaDataLogParams struct {
	UserID    int
	Username  string
	ModelName string
	Quota     int
	CreatedAt int64
	TokenUsed int
	UseGroup  string
	TokenID   int
	ChannelID int
	NodeName  string

	// 作用域与账单归属。组织请求的用量要按组织记账，缺省（零值）会退化成个人，
	// 于是组织的消耗会出现在责任人的个人看板上，而组织那边看不到。
	ScopeType          string
	ScopeId            int
	BillingAccountType string
	BillingAccountId   int
	OrganizationId     int
	ResponsibleUserId  int
}

func UpdateQuotaData() {
	for {
		if common.DataExportEnabled {
			common.SysLog("正在更新数据看板数据...")
			SaveQuotaDataCache()
		}
		time.Sleep(time.Duration(common.DataExportInterval) * time.Minute)
	}
}

var CacheQuotaData = make(map[string]*QuotaData)
var CacheQuotaDataLock = sync.Mutex{}

// quotaDataCacheKey 生成缓存聚合键，必须与 idx_quota_data_account_context 覆盖同一组维度。
//
// 作用域必须在键里：同一个用户同一小时既有个人消费又有组织消费，只按
// 用户/模型/时间做键会把两笔合并成一条，之后无论写进哪一边都是错的。
func quotaDataCacheKey(quotaData *QuotaData) string {
	NormalizeQuotaDataScope(quotaData)
	return fmt.Sprintf("%d\x00%s\x00%s\x00%d\x00%s\x00%d\x00%d\x00%s\x00%s\x00%d\x00%s\x00%d\x00%d\x00%d",
		quotaData.UserID,
		quotaData.Username,
		quotaData.ModelName,
		quotaData.CreatedAt,
		quotaData.UseGroup,
		quotaData.TokenID,
		quotaData.ChannelID,
		quotaData.NodeName,
		quotaData.ScopeType,
		quotaData.ScopeId,
		quotaData.BillingAccountType,
		quotaData.BillingAccountId,
		quotaData.OrganizationId,
		quotaData.ResponsibleUserId,
	)
}

func logQuotaDataCache(quotaData *QuotaData) {
	key := quotaDataCacheKey(quotaData)
	count := quotaData.Count
	quota := quotaData.Quota
	tokenUsed := quotaData.TokenUsed
	cachedQuotaData, ok := CacheQuotaData[key]
	if ok {
		cachedQuotaData.Count += count
		cachedQuotaData.Quota += quota
		cachedQuotaData.TokenUsed += tokenUsed
		quotaData = cachedQuotaData
	}
	CacheQuotaData[key] = quotaData
}

func LogQuotaData(params QuotaDataLogParams) {
	// 只精确到小时
	createdAt := params.CreatedAt - (params.CreatedAt % 3600)
	quotaData := &QuotaData{
		UserID:    params.UserID,
		Username:  params.Username,
		ModelName: params.ModelName,
		CreatedAt: createdAt,
		UseGroup:  params.UseGroup,
		TokenID:   params.TokenID,
		ChannelID: params.ChannelID,
		NodeName:  params.NodeName,
		Count:     1,
		Quota:     params.Quota,
		TokenUsed: params.TokenUsed,

		ScopeType:          params.ScopeType,
		ScopeId:            params.ScopeId,
		BillingAccountType: params.BillingAccountType,
		BillingAccountId:   params.BillingAccountId,
		OrganizationId:     params.OrganizationId,
		ResponsibleUserId:  params.ResponsibleUserId,
	}

	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	logQuotaDataCache(quotaData)
}

func SaveQuotaDataCache() {
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	size := len(CacheQuotaData)
	// 缓存里已经按完整身份聚合过了，落库直接 upsert：不再「先查再插」，
	// 那个模式在多节点下会各自查到「不存在」然后各插一行，同一小时的用量翻倍。
	for _, quotaData := range CacheQuotaData {
		if err := upsertQuotaData(quotaData); err != nil {
			common.SysLog(fmt.Sprintf("upsertQuotaData error: %s", err))
		}
	}
	CacheQuotaData = make(map[string]*QuotaData)
	common.SysLog(fmt.Sprintf("保存数据看板数据成功，共保存%d条数据", size))
}

// quotaDataAccountContextColumnNames 是 idx_quota_data_account_context 的列定义，
// 顺序即索引顺序。upsertQuotaData 的 ON CONFLICT 目标直接由它派生：两者分开写的话，
// 改了一处忘了另一处，ON CONFLICT 就匹配不上索引，重复写入会变成插入冲突。
var quotaDataAccountContextColumnNames = []string{
	"user_id",
	"username",
	"model_name",
	"created_at",
	"use_group",
	"token_id",
	"channel_id",
	"node_name",
	"scope_type",
	"scope_id",
	"billing_account_type",
	"billing_account_id",
	"organization_id",
	"responsible_user_id",
}

const quotaDataAccountContextIndexName = "idx_quota_data_account_context"

func quotaDataConflictColumns() []clause.Column {
	columns := make([]clause.Column, 0, len(quotaDataAccountContextColumnNames))
	for _, name := range quotaDataAccountContextColumnNames {
		columns = append(columns, clause.Column{Name: name})
	}
	return columns
}

// ensureQuotaDataAccountContextIndex 幂等地补上唯一索引，并在建不出来时先合并重复行。
//
// 索引不在结构体标签里（原因见 QuotaData 的注释），AutoMigrate 因此不会建它，
// 而是由 ensureOrganizationBillingIndexes 在迁移流程里显式调用。
//
// 老库上大概率带着重复行：升级前的写路径是「先查后插」（见 main 分支的
// SaveQuotaDataCache），两个节点同时刷同一个时间桶时都会查到「不存在」而各插一行，
// 而当时没有任何唯一约束拦着。这些行在升级后正好落在同一个索引键上，直接建索引会
// 失败，而这里的失败会拦住 master 启动——上线即不可用。所以先合并再建：合并是按
// 行身份求和，看板口径不变，只是把重复行并成一行的过程提前了。
//
// 任何自己建模的测试库只要会走到 SaveQuotaDataCache，也必须调这一句，
// 否则 ON CONFLICT 找不到冲突目标、写入静默失败。
func ensureQuotaDataAccountContextIndex(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("quota_data") {
		return nil
	}
	if db.Migrator().HasIndex("quota_data", quotaDataAccountContextIndexName) {
		return nil
	}
	err := ensureModelUniqueMultiColumnIndex(db, "quota_data", quotaDataAccountContextIndexName, quotaDataAccountContextColumnNames...)
	if err == nil {
		return nil
	}
	// 建失败最常见的原因就是重复行。先按行身份合并，再试一次；仍然失败说明是
	// 别的原因（权限、磁盘、列类型），把错误原样抛出去让启动停下来。
	if mergeErr := mergeQuotaDataDuplicateRows(db); mergeErr != nil {
		return fmt.Errorf("ensure %s: create index failed (%v), merging duplicate rows failed: %w", quotaDataAccountContextIndexName, err, mergeErr)
	}
	if retryErr := ensureModelUniqueMultiColumnIndex(db, "quota_data", quotaDataAccountContextIndexName, quotaDataAccountContextColumnNames...); retryErr != nil {
		return fmt.Errorf("ensure %s: create index failed after merging duplicate rows: %w", quotaDataAccountContextIndexName, retryErr)
	}
	return nil
}

// quotaDataAccountContextKey 是 idx_quota_data_account_context 的键值，字段顺序与
// quotaDataAccountContextColumnNames 一致——合并重复行时要按这组列在 Go 侧分组。
type quotaDataAccountContextKey struct {
	UserID             int    `gorm:"column:user_id"`
	Username           string `gorm:"column:username"`
	ModelName          string `gorm:"column:model_name"`
	CreatedAt          int64  `gorm:"column:created_at"`
	UseGroup           string `gorm:"column:use_group"`
	TokenID            int    `gorm:"column:token_id"`
	ChannelID          int    `gorm:"column:channel_id"`
	NodeName           string `gorm:"column:node_name"`
	ScopeType          string `gorm:"column:scope_type"`
	ScopeId            int    `gorm:"column:scope_id"`
	BillingAccountType string `gorm:"column:billing_account_type"`
	BillingAccountId   int    `gorm:"column:billing_account_id"`
	OrganizationId     int    `gorm:"column:organization_id"`
	ResponsibleUserId  int    `gorm:"column:responsible_user_id"`
}

// mergeQuotaDataDuplicateRows 把同一索引键上的多行并成一行。
//
// 保留 id 最小的那行并累加 count/quota/token_used，其余删除：这三个值是各节点各刷
// 一部分用量后加起来的，求和正是看板原本要显示的数。删除放在同一个事务里，中途失败
// 不会留下「已经并进 keeper 但重复行还在」的半成品。
//
// 循环直到不再有重复组。每轮至少删掉一行，所以一定会终止；分批是为了别让一次
// 查询把整张表的重复键都拖进内存。
func mergeQuotaDataDuplicateRows(db *gorm.DB) error {
	const batchSize = 200
	groupColumns := strings.Join(quotaDataAccountContextColumnNames, ", ")
	for {
		var keys []quotaDataAccountContextKey
		if err := db.Table("quota_data").
			Select(groupColumns).
			Group(groupColumns).
			Having("COUNT(*) > 1").
			Limit(batchSize).
			Find(&keys).Error; err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}
		for _, key := range keys {
			if err := mergeQuotaDataGroup(db, key); err != nil {
				return err
			}
		}
	}
}

// quotaDataGroupConditions 是「同一索引键」的等值条件，列顺序与
// quotaDataAccountContextColumnNames 一致，占位符逐个对应 quotaDataGroupArguments。
//
// 列都非 NULL（作用域列由 prepareOrganizationScopeMigration 回填，分析列本来就由
// Create 写入），所以等值比较就够了：真出现 NULL 时条件匹配不到那个组，
// mergeQuotaDataGroup 会直接报错而不是空转——那说明前面的假设不成立，需要人来查。
func quotaDataGroupConditions() string {
	conditions := make([]string, 0, len(quotaDataAccountContextColumnNames))
	for _, column := range quotaDataAccountContextColumnNames {
		conditions = append(conditions, column+" = ?")
	}
	return strings.Join(conditions, " AND ")
}

func quotaDataGroupArguments(key quotaDataAccountContextKey) []any {
	return []any{
		key.UserID, key.Username, key.ModelName, key.CreatedAt, key.UseGroup,
		key.TokenID, key.ChannelID, key.NodeName, key.ScopeType, key.ScopeId,
		key.BillingAccountType, key.BillingAccountId, key.OrganizationId, key.ResponsibleUserId,
	}
}

func mergeQuotaDataGroup(db *gorm.DB, key quotaDataAccountContextKey) error {
	var rows []QuotaData
	if err := db.Table("quota_data").
		Select("id, count, quota, token_used").
		Where(quotaDataGroupConditions(), quotaDataGroupArguments(key)...).
		Order("id").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) < 2 {
		return fmt.Errorf("quota_data duplicate group %+v matched %d rows, expected at least 2", key, len(rows))
	}
	keeper := rows[0]
	count, quota, tokenUsed := keeper.Count, keeper.Quota, keeper.TokenUsed
	duplicateIds := make([]int, 0, len(rows)-1)
	for _, row := range rows[1:] {
		count += row.Count
		quota += row.Quota
		tokenUsed += row.TokenUsed
		duplicateIds = append(duplicateIds, row.Id)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("quota_data").Where("id = ?", keeper.Id).
			Updates(map[string]any{"count": count, "quota": quota, "token_used": tokenUsed}).Error; err != nil {
			return err
		}
		return tx.Table("quota_data").Where("id IN ?", duplicateIds).Delete(&QuotaData{}).Error
	})
}

// upsertQuotaData 按完整行身份累加。
//
// 累加表达式里的列必须写成 clause.Column{Table: ...}：PostgreSQL 在
// ON CONFLICT DO UPDATE 里要求 SET 右侧的列引用带表名限定，写成裸列名会直接
// 报 ambiguous column，而 SQLite/MySQL 对此宽容，本地跑不出来。
func upsertQuotaData(quotaData *QuotaData) error {
	NormalizeQuotaDataScope(quotaData)
	return DB.Clauses(clause.OnConflict{
		Columns: quotaDataConflictColumns(),
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":      gorm.Expr("? + ?", clause.Column{Table: "quota_data", Name: "count"}, quotaData.Count),
			"quota":      gorm.Expr("? + ?", clause.Column{Table: "quota_data", Name: "quota"}, quotaData.Quota),
			"token_used": gorm.Expr("? + ?", clause.Column{Table: "quota_data", Name: "token_used"}, quotaData.TokenUsed),
		}),
	}).Table("quota_data").Create(quotaData).Error
}

// personalQuotaDataScopeQuery 收窄到个人账单。
//
// 用 billing_account_id 而不是 user_id：组织请求的 user_id 是操作者本人，
// 只有账单归属列才分得清这笔钱是谁出的。
func personalQuotaDataScopeQuery(db *gorm.DB, userId int) *gorm.DB {
	return db.Where("billing_account_type = ? AND billing_account_id = ?", AccountContextTypePersonal, userId)
}

func organizationQuotaDataScopeQuery(db *gorm.DB, organizationId int) *gorm.DB {
	return db.Where("billing_account_type = ? AND billing_account_id = ?", AccountContextTypeOrganization, organizationId)
}

// GetQuotaDataByUsername 按用户名取个人看板数据，管理员在 /api/data 上用。
//
// 同样按账单归属收窄：组织消费把这个人记为责任人，但钱是组织出的，
// 混进来会让「这个用户花了多少」的报表凭空多出一块。组织维度走
// GetQuotaDataByOrganizationId。
func GetQuotaDataByUsername(username string, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	var user User
	if err = DB.Where("username = ?", username).First(&user).Error; err != nil {
		return quotaDatas, err
	}
	// 从quota_data表中查询数据
	err = personalQuotaDataScopeQuery(DB.Table("quota_data"), user.Id).
		Select("user_id, username, model_name, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Where("username = ? and created_at >= ? and created_at <= ?", username, startTime, endTime).
		Group("user_id, username, model_name, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

// GetQuotaDataByUserId 返回该用户的个人看板数据。
//
// 按账单归属收窄，组织消费不算进来——组织请求的 user_id 也会是这个人（他是责任人），
// 不按 billing_account_id 过滤的话，组织的花销会出现在他的个人用量曲线上。
func GetQuotaDataByUserId(userId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = personalQuotaDataScopeQuery(DB.Table("quota_data"), userId).
		Select("user_id, username, model_name, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("user_id, username, model_name, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

// GetQuotaDataByOrganizationId 返回某个组织的看板数据，维度与个人看板一致。
func GetQuotaDataByOrganizationId(organizationId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = organizationQuotaDataScopeQuery(DB.Table("quota_data"), organizationId).
		Select("user_id, username, model_name, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("user_id, username, model_name, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataGroupByUser(startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = DB.Table("quota_data").
		Select("username, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("username, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetAllQuotaDates(startTime int64, endTime int64, username string) (quotaData []*QuotaData, err error) {
	if username != "" {
		return GetQuotaDataByUsername(username, startTime, endTime)
	}
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	// only select model_name, sum(count) as count, sum(quota) as quota, model_name, created_at from quota_data group by model_name, created_at;
	//err = DB.Table("quota_data").Where("created_at >= ? and created_at <= ?", startTime, endTime).Find(&quotaDatas).Error
	err = DB.Table("quota_data").Select("model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, created_at").Where("created_at >= ? and created_at <= ?", startTime, endTime).Group("model_name, created_at").Find(&quotaDatas).Error
	return quotaDatas, err
}
