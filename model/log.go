package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
)

func applyExplicitLogTextFilter(tx *gorm.DB, column string, value string) (*gorm.DB, error) {
	if value == "" {
		return tx, nil
	}
	if strings.Contains(value, "%") {
		condition, pattern, err := buildLogLikeCondition(column, value)
		if err != nil {
			return nil, err
		}
		return tx.Where(condition, pattern), nil
	}
	return tx.Where(column+" = ?", value), nil
}

func buildLogLikeCondition(column string, value string) (string, string, error) {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		pattern, err := sanitizeClickHouseLikePattern(value)
		if err != nil {
			return "", "", err
		}
		return column + " LIKE ?", pattern, nil
	}

	pattern, err := sanitizeLikePattern(value)
	if err != nil {
		return "", "", err
	}
	return column + " LIKE ? ESCAPE '!'", pattern, nil
}

func sanitizeClickHouseLikePattern(input string) (string, error) {
	input = strings.ReplaceAll(input, `\`, `\\`)
	input = strings.ReplaceAll(input, `_`, `\_`)

	if err := validateLikePattern(input); err != nil {
		return "", err
	}
	return input, nil
}

type Log struct {
	Id                int    `json:"id" gorm:"index:idx_created_at_id,priority:2;index:idx_user_id_id,priority:2"`
	UserId            int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_created_at_type;index:idx_logs_org_billing_type_time,priority:5;index:idx_logs_org_billing_responsible_time,priority:6"`
	Type              int    `json:"type" gorm:"index:idx_created_at_type;index:idx_logs_org_billing_type_time,priority:4;index:idx_logs_org_billing_responsible_time,priority:4"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName         string `json:"token_name" gorm:"index;default:''"`
	ModelName         string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	UseTime           int    `json:"use_time" gorm:"default:0"`
	IsStream          bool   `json:"is_stream"`
	ChannelId         int    `json:"channel" gorm:"index"`
	ChannelName       string `json:"channel_name" gorm:"->"`
	TokenId           int    `json:"token_id" gorm:"default:0;index"`
	Group             string `json:"group" gorm:"index"`
	Ip                string `json:"ip" gorm:"index;default:''"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	Other             string `json:"other"`

	ScopeType          string `json:"scope_type" gorm:"type:varchar(16);index;default:'personal'"`
	ScopeId            int    `json:"scope_id" gorm:"index;default:0"`
	BillingAccountType string `json:"billing_account_type" gorm:"type:varchar(16);index;index:idx_logs_org_billing_type_time,priority:1;index:idx_logs_org_billing_responsible_time,priority:1;default:'personal'"`
	BillingAccountId   int    `json:"billing_account_id" gorm:"index;index:idx_logs_org_billing_type_time,priority:2;index:idx_logs_org_billing_responsible_time,priority:2;default:0"`
	OrganizationId     int    `json:"organization_id" gorm:"index;index:idx_logs_org_billing_type_time,priority:3;index:idx_logs_org_billing_responsible_time,priority:3;default:0"`
	OrganizationName   string `json:"organization_name" gorm:"type:varchar(128);default:''"`
	ActorUserId        int    `json:"actor_user_id" gorm:"index;default:0"`
	CreatorUserId      int    `json:"creator_user_id" gorm:"index;default:0"`
	CreatorName        string `json:"creator_name" gorm:"type:varchar(128);default:''"`
	ResponsibleUserId  int    `json:"responsible_user_id" gorm:"index;index:idx_logs_org_billing_responsible_time,priority:5;default:0"`
	ResponsibleName    string `json:"responsible_name" gorm:"type:varchar(128);default:''"`

	OrganizationBillingSessionId  int    `json:"organization_billing_session_id" gorm:"index;default:0"`
	OrganizationBillingSessionKey string `json:"organization_billing_session_key" gorm:"type:varchar(191);index;default:''"`
	// BillingEventKey 是结算/退款的幂等键，唯一索引（logBillingEventKeyIndex）保证同一笔账只落一条日志。
	// 唯一索引刻意不用 uniqueIndex 标签声明：该标签会让 SQLite 的 AutoMigrate 每次启动都重建整张 logs 表，
	// 改为在 ensureOrganizationBillingLogIndexes 里显式、幂等地创建。
	BillingEventKey *string `json:"-" gorm:"type:varchar(191)"`

	ResponsibleUsername    string `json:"responsible_username,omitempty" gorm:"-"`
	ResponsibleDisplayName string `json:"responsible_display_name,omitempty" gorm:"-"`
}

// logBillingEventKeyIndex 是 logs.billing_event_key 上的唯一索引名，见 Log.BillingEventKey。
const logBillingEventKeyIndex = "idx_logs_billing_event_key"

type logBillingEventKeyMigrationColumn struct {
	BillingEventKey *string `gorm:"type:varchar(191)"`
}

func (logBillingEventKeyMigrationColumn) TableName() string {
	return "logs"
}

func prepareLogBillingEventKeyMigration(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&Log{}) || db.Migrator().HasColumn(&Log{}, "BillingEventKey") {
		return nil
	}
	return db.Migrator().AddColumn(&logBillingEventKeyMigrationColumn{}, "BillingEventKey")
}

func NormalizeLogScope(log *Log) {
	if log == nil {
		return
	}
	if log.ScopeType == "" {
		log.ScopeType = AccountContextTypePersonal
	}
	if log.ScopeId == 0 && log.ScopeType == AccountContextTypePersonal {
		log.ScopeId = log.UserId
	}
	if log.BillingAccountType == "" {
		log.BillingAccountType = AccountContextTypePersonal
	}
	if log.BillingAccountId == 0 && log.BillingAccountType == AccountContextTypePersonal {
		log.BillingAccountId = log.UserId
	}
	if log.CreatorUserId == 0 && log.ScopeType == AccountContextTypePersonal {
		log.CreatorUserId = log.UserId
	}
	if log.ResponsibleUserId == 0 && log.ScopeType == AccountContextTypePersonal {
		log.ResponsibleUserId = log.UserId
	}
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
	LogTypeLogin   = 7
)

func ensureLogRequestId(log *Log) {
	if log != nil && log.RequestId == "" {
		log.RequestId = common.NewRequestId()
	}
}

func createLog(log *Log) error {
	ensureLogRequestId(log)
	return LOG_DB.Create(log).Error
}

func clickHouseLogOrder(prefix string) string {
	return prefix + "created_at desc, " + prefix + "request_id desc"
}

func assignDisplayLogIds(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].Id = startIdx + i + 1
	}
}

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			// Remove diagnostics reserved for root.
			delete(otherMap, "root_info")
			// Remove operation-audit details (operator/route info), admin-only.
			delete(otherMap, "audit_info")
			// delete(otherMap, "reject_reason")
			// delete(otherMap, "stream_status")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
	}
	assignDisplayLogIds(logs, startIdx)
}

// FormatAdminLogs removes root-only diagnostics while retaining operational
// admin_info. Root callers must not pass their results through this formatter.
func FormatAdminLogs(logs []*Log) {
	for i := range logs {
		otherMap, _ := common.StrToMap(logs[i].Other)
		if otherMap == nil {
			continue
		}
		delete(otherMap, "root_info")
		logs[i].Other = common.MapToJsonStr(otherMap)
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	order := "id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("")
	}
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order(order).Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := createLog(log)
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := createLog(log); err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// buildOpField 构建语言无关的操作描述（写入 Other.op）。
// 前端依据 action(稳定操作标识) + params(结构化参数) 在渲染期用 i18n 本地化展示，
// 因此不在数据库中存储自然语言句子。
func buildOpField(action string, params map[string]interface{}) map[string]interface{} {
	op := map[string]interface{}{
		"action": action,
	}
	if len(params) > 0 {
		op["params"] = params
	}
	return op
}

// RecordLoginLog 记录用户登录成功的审计日志（type=LogTypeLogin）。
// username 由调用方传入（登录流程已持有用户对象），避免额外的数据库查询。
// content 为英文兜底文本（用于导出）；action+params 供前端本地化渲染。
// extra 可携带 login_method、user_agent 等附加信息（普通用户可见）。
func RecordLoginLog(userId int, username string, content string, ip string, action string, params map[string]interface{}, extra map[string]interface{}) {
	other := map[string]interface{}{}
	for k, v := range extra {
		other[k] = v
	}
	other["op"] = buildOpField(action, params)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeLogin,
		Content:   content,
		Ip:        ip,
		Other:     common.MapToJsonStr(other),
	}
	if err := createLog(log); err != nil {
		common.SysLog("failed to record login log: " + err.Error())
	}
}

// RecordOperationAuditLog 记录管理/高危操作审计日志（type=LogTypeManage）。
// logUserId 为日志归属者，管理审计日志应归属实际操作者；目标资源/用户放入
// action params。username 内部按 logUserId 查询。content 为英文兜底文本（供导出使用）。
// action+params 写入 Other.op，供前端本地化渲染（普通用户可见，不含敏感信息）。
// adminInfo 存放操作者身份（写入 Other.admin_info，普通用户查询时剥离）；
// auditInfo 存放路由/方法/结果等中间件兜底信息（写入 Other.audit_info，普通用户查询时剥离）。
func RecordOperationAuditLog(logUserId int, content string, ip string, action string, params map[string]interface{}, adminInfo map[string]interface{}, auditInfo map[string]interface{}) {
	username, _ := GetUsernameById(logUserId, false)
	other := map[string]interface{}{
		"op": buildOpField(action, params),
	}
	if len(adminInfo) > 0 {
		other["admin_info"] = adminInfo
	}
	if len(auditInfo) > 0 {
		other["audit_info"] = auditInfo
	}
	log := &Log{
		UserId:    logUserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeManage,
		Content:   content,
		Ip:        ip,
		Other:     common.MapToJsonStr(other),
	}
	if err := createLog(log); err != nil {
		common.SysLog("failed to record operation audit log: " + err.Error())
	}
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        callerIp,
		Other:     common.MapToJsonStr(other),
	}
	err := createLog(log)
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
	}
}

// RecordErrorLog 记录一条失败日志。
//
// 参数与 RecordConsumeLog 共用 RecordConsumeLogParams，好让作用域/归属字段
// （ScopeType、OrganizationId、ResponsibleUserId……）在两条路径上走同一套归一化，
// 否则组织请求的失败日志会落到个人视图里，也正是这个函数存在的意义。
func RecordErrorLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	NormalizeRecordConsumeLogParams(userId, &params)
	hydrateRecordConsumeLogSnapshots(userId, &params)
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, params.ChannelId, params.ModelName, params.TokenName, common.LocalLogPreview(params.Content)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          params.Content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            0,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:                     requestId,
		UpstreamRequestId:             upstreamRequestId,
		Other:                         otherStr,
		ScopeType:                     params.ScopeType,
		ScopeId:                       params.ScopeId,
		BillingAccountType:            params.BillingAccountType,
		BillingAccountId:              params.BillingAccountId,
		OrganizationId:                params.OrganizationId,
		OrganizationName:              params.OrganizationName,
		ActorUserId:                   params.ActorUserId,
		CreatorUserId:                 params.CreatorUserId,
		CreatorName:                   params.CreatorName,
		ResponsibleUserId:             params.ResponsibleUserId,
		ResponsibleName:               params.ResponsibleName,
		OrganizationBillingSessionId:  params.OrganizationBillingSessionId,
		OrganizationBillingSessionKey: params.OrganizationBillingSessionKey,
	}
	NormalizeLogScope(log)
	err := createLog(log)
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
	// 以下作用域/归属字段由 service.RelayConsumeLogParams 从 RelayInfo 上一次性带入。
	// 个人请求留空即可，NormalizeRecordConsumeLogParams 会补成 personal/调用者自己。
	ScopeType                     string `json:"scope_type"`
	ScopeId                       int    `json:"scope_id"`
	BillingAccountType            string `json:"billing_account_type"`
	BillingAccountId              int    `json:"billing_account_id"`
	OrganizationId                int    `json:"organization_id"`
	OrganizationName              string `json:"organization_name"`
	ActorUserId                   int    `json:"actor_user_id"`
	CreatorUserId                 int    `json:"creator_user_id"`
	CreatorName                   string `json:"creator_name"`
	ResponsibleUserId             int    `json:"responsible_user_id"`
	ResponsibleName               string `json:"responsible_name"`
	OrganizationBillingSessionId  int    `json:"organization_billing_session_id"`
	OrganizationBillingSessionKey string `json:"organization_billing_session_key"`
	// BillingEventKey 让同一笔账只落一条日志；非空时唯一索引冲突被当成「已记过」静默跳过。
	BillingEventKey string `json:"-"`
	// RequestIdOverride 覆盖上下文里的请求 ID。违规罚金这类附加扣费复用原请求 ID，只靠它区分。
	RequestIdOverride string `json:"-"`
}

// NormalizeRecordConsumeLogParams 把缺省的作用域补成个人。
// 造 params 的地方（尤其是新写的调用点）不必逐个记得填这六个字段。
func NormalizeRecordConsumeLogParams(userId int, params *RecordConsumeLogParams) {
	if params == nil {
		return
	}
	if params.ScopeType == "" {
		params.ScopeType = AccountContextTypePersonal
	}
	if params.ScopeId == 0 && params.ScopeType == AccountContextTypePersonal {
		params.ScopeId = userId
	}
	if params.BillingAccountType == "" {
		params.BillingAccountType = AccountContextTypePersonal
	}
	if params.BillingAccountId == 0 && params.BillingAccountType == AccountContextTypePersonal {
		params.BillingAccountId = userId
	}
	if params.CreatorUserId == 0 && params.ScopeType == AccountContextTypePersonal {
		params.CreatorUserId = userId
	}
	if params.ResponsibleUserId == 0 && params.ScopeType == AccountContextTypePersonal {
		params.ResponsibleUserId = userId
	}
}

// hydrateRecordConsumeLogSnapshots 在写日志时把组织名/用户名固化下来。
// 日志是历史快照：组织改名或成员退组之后，旧日志仍要显示当时的归属，
// 所以这里存的是字符串而不是外键。
func hydrateRecordConsumeLogSnapshots(userId int, params *RecordConsumeLogParams) {
	if params == nil || params.BillingAccountType != AccountContextTypeOrganization || params.OrganizationId == 0 {
		return
	}
	if params.OrganizationName == "" {
		var organization Organization
		if err := DB.Select("id", "name").Where("id = ?", params.OrganizationId).First(&organization).Error; err == nil {
			params.OrganizationName = organization.Name
		}
	}
	userIds := make([]int, 0, 2)
	seen := map[int]bool{}
	if params.CreatorUserId > 0 && params.CreatorName == "" {
		userIds = append(userIds, params.CreatorUserId)
		seen[params.CreatorUserId] = true
	}
	if params.ResponsibleUserId > 0 && params.ResponsibleName == "" && !seen[params.ResponsibleUserId] {
		userIds = append(userIds, params.ResponsibleUserId)
	}
	if len(userIds) == 0 {
		return
	}
	var users []User
	if err := DB.Select("id", "username", "display_name").Where("id IN ?", userIds).Find(&users).Error; err != nil {
		return
	}
	for _, user := range users {
		displayName := user.DisplayName
		if displayName == "" {
			displayName = user.Username
		}
		if user.Id == params.CreatorUserId && params.CreatorName == "" {
			params.CreatorName = displayName
		}
		if user.Id == params.ResponsibleUserId && params.ResponsibleName == "" {
			params.ResponsibleName = displayName
		}
	}
}

// syncOrganizationBillingRecordUsageFromConsumeLog 把 token 用量补进组织账本。
//
// 结算发生在拿到用量之前，账本上那条 settle 记录只知道扣了多少钱。
// 日志是唯一带 prompt/completion token 数的地方，写日志时顺带回填，
// 组织的用量明细才不至于只有金额没有 token 数。
func syncOrganizationBillingRecordUsageFromConsumeLog(params *RecordConsumeLogParams) {
	if params == nil ||
		params.BillingAccountType != AccountContextTypeOrganization ||
		params.OrganizationId == 0 ||
		params.OrganizationBillingSessionId == 0 {
		return
	}
	updates := map[string]any{
		"prompt_tokens":     params.PromptTokens,
		"completion_tokens": params.CompletionTokens,
		"token_count":       params.PromptTokens + params.CompletionTokens,
	}
	if params.TokenId > 0 {
		updates["token_id"] = params.TokenId
	}
	if params.TokenName != "" {
		updates["token_name"] = params.TokenName
	}
	if params.ModelName != "" {
		updates["model_name"] = params.ModelName
	}
	if params.Group != "" {
		updates["group_name"] = params.Group
	}
	if err := DB.Model(&OrganizationBillingRecord{}).
		Where("organization_id = ? AND session_id = ? AND record_type = ?", params.OrganizationId, params.OrganizationBillingSessionId, OrganizationBillingRecordTypeSettle).
		UpdateColumns(updates).Error; err != nil {
		common.SysError("failed to sync organization billing record usage: " + err.Error())
	}
}

// createConsumeLog 写一条消费日志。返回值 inserted=false 表示
// billing_event_key 唯一索引挡住了这次写入——说明同一笔账已经记过，
// 调用方应当静默跳过而不是当成错误。
func createConsumeLog(log *Log) (bool, error) {
	result := LOG_DB.Create(log)
	if result.Error == nil {
		return true, nil
	}
	if log != nil && log.BillingEventKey != nil && *log.BillingEventKey != "" && IsDuplicateKeyError(LOG_DB, result.Error) {
		return false, nil
	}
	return false, result.Error
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	NormalizeRecordConsumeLogParams(userId, &params)
	hydrateRecordConsumeLogSnapshots(userId, &params)
	syncOrganizationBillingRecordUsageFromConsumeLog(&params)
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	if params.RequestIdOverride != "" {
		requestId = params.RequestIdOverride
	}
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	createdAt := common.GetTimestamp()
	otherStr := common.MapToJsonStr(params.Other)
	var billingEventKey *string
	if params.BillingEventKey != "" {
		billingEventKey = &params.BillingEventKey
	}
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        createdAt,
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:                     requestId,
		UpstreamRequestId:             upstreamRequestId,
		Other:                         otherStr,
		ScopeType:                     params.ScopeType,
		ScopeId:                       params.ScopeId,
		BillingAccountType:            params.BillingAccountType,
		BillingAccountId:              params.BillingAccountId,
		OrganizationId:                params.OrganizationId,
		OrganizationName:              params.OrganizationName,
		ActorUserId:                   params.ActorUserId,
		CreatorUserId:                 params.CreatorUserId,
		CreatorName:                   params.CreatorName,
		ResponsibleUserId:             params.ResponsibleUserId,
		ResponsibleName:               params.ResponsibleName,
		OrganizationBillingSessionId:  params.OrganizationBillingSessionId,
		OrganizationBillingSessionKey: params.OrganizationBillingSessionKey,
		BillingEventKey:               billingEventKey,
	}
	NormalizeLogScope(log)
	inserted, err := createConsumeLog(log)
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
		return
	}
	if !inserted {
		return
	}
	if common.DataExportEnabled {
		LogQuotaData(QuotaDataLogParams{
			UserID:    userId,
			Username:  username,
			ModelName: params.ModelName,
			Quota:     params.Quota,
			CreatedAt: createdAt,
			TokenUsed: params.PromptTokens + params.CompletionTokens,
			UseGroup:  params.Group,
			TokenID:   params.TokenId,
			ChannelID: params.ChannelId,
			NodeName:  common.NodeName,
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
	NodeName  string // 任务发起节点；为空时回退当前节点
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	createdAt := common.GetTimestamp()
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: createdAt,
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     common.MapToJsonStr(params.Other),
	}
	err := createLog(log)
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
	if params.LogType == LogTypeConsume && common.DataExportEnabled {
		nodeName := params.NodeName
		if nodeName == "" {
			nodeName = common.NodeName
		}
		LogQuotaData(QuotaDataLogParams{
			UserID:    params.UserId,
			Username:  username,
			ModelName: params.ModelName,
			Quota:     params.Quota,
			CreatedAt: createdAt,
			UseGroup:  params.Group,
			TokenID:   params.TokenId,
			ChannelID: params.ChannelId,
			NodeName:  nodeName,
		})
	}
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", username); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	order := "logs.created_at desc, logs.id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("logs.")
	}
	err = tx.Order(order).Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		assignDisplayLogIds(logs, startIdx)
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	order := "logs.id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("logs.")
	}
	err = tx.Order(order).Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("COALESCE(sum(quota), 0) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0) tpm")

	if tx, err = applyExplicitLogTextFilter(tx, "username", username); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "username", username); err != nil {
		return stat, err
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "model_name", modelName); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "model_name", modelName); err != nil {
		return stat, err
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	var rateStat struct {
		Rpm int
		Tpm int
	}
	if err := rpmTpmQuery.Scan(&rateStat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	stat.Rpm = rateStat.Rpm
	stat.Tpm = rateStat.Tpm

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func CountOldLog(ctx context.Context, targetTimestamp int64) (int64, error) {
	var total int64
	if err := LOG_DB.WithContext(ctx).Model(&Log{}).Where("created_at < ?", targetTimestamp).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func DeleteOldLogBatch(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if nil != ctx.Err() {
		return 0, ctx.Err()
	}

	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		// ClickHouse DELETE is a heavy mutation that rewrites data parts, so
		// per-batch mutations would be pathologically slow. Remove all matching
		// rows in a single synchronous mutation regardless of limit; the reported
		// count lets the caller's progress loop complete in one pass.
		total, err := CountOldLog(ctx, targetTimestamp)
		if err != nil {
			return 0, err
		}
		if total == 0 {
			return 0, nil
		}
		if err := LOG_DB.WithContext(ctx).Exec(
			"ALTER TABLE logs DELETE WHERE created_at < ? SETTINGS mutations_sync = 1",
			targetTimestamp,
		).Error; err != nil {
			return 0, err
		}
		return total, nil
	}

	result := LOG_DB.WithContext(ctx).Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
	if nil != result.Error {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
