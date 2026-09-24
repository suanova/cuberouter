/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

// 唯一索引列是 varchar(191)：MySQL 5.7.8 的 utf8mb4 索引上限是 767 字节，
// 191×4 = 764。校验必须与列宽一致，否则越界的 pattern 会在写库时炸出 500。
const organizationJoinRuleMaxPatternLength = 191

// 域名的形状：至少两个标签，标签内只允许 [a-z0-9-] 且不以连字符开头或结尾。
// 非 ASCII 会在这里被拒（国际化域名必须写成 punycode），这也顺带挡住
// 用同形字符伪装企业域名。
var organizationJoinRuleDomainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// joinRuleMatchTypeFor 判定一行输入是地址规则还是域名规则：含 '@' 即地址规则。
func joinRuleMatchTypeFor(raw string) string {
	if strings.Contains(raw, "@") {
		return model.OrganizationJoinRuleMatchTypeEmail
	}
	return model.OrganizationJoinRuleMatchTypeDomain
}

// NormalizeJoinRulePattern 把管理员输入的一行规则归一化成可比较、可存储的形式。
//
// 域名规则保留开头的 "*.", 它是语义的一部分：*.enterprise.com 命中裸域名与所有
// 子域，enterprise.com 只命中裸域名，两者是不同的规则。
func NormalizeJoinRulePattern(matchType, raw string) (string, error) {
	switch matchType {
	case model.OrganizationJoinRuleMatchTypeEmail:
		normalized := model.NormalizeEmail(raw)
		if err := common.Validate.Var(normalized, "required,email"); err != nil {
			return "", errors.New("invalid join rule email pattern")
		}
		if len(normalized) > organizationJoinRuleMaxPatternLength {
			return "", errors.New("join rule pattern is too long")
		}
		return normalized, nil
	case model.OrganizationJoinRuleMatchTypeDomain:
		pattern := strings.ToLower(strings.TrimSpace(raw))
		pattern = strings.TrimSuffix(pattern, ".")
		wildcard := strings.HasPrefix(pattern, "*.")
		base := strings.TrimPrefix(pattern, "*.")
		if !organizationJoinRuleDomainPattern.MatchString(base) {
			return "", errors.New("invalid join rule domain pattern")
		}
		normalized := base
		if wildcard {
			normalized = "*." + base
		}
		if len(normalized) > organizationJoinRuleMaxPatternLength {
			return "", errors.New("join rule pattern is too long")
		}
		return normalized, nil
	default:
		return "", errors.New("invalid join rule match type")
	}
}

// joinRuleBase 取域名规则的基准域：*.enterprise.com → enterprise.com
func joinRuleBase(patternNormalized string) string {
	return strings.TrimPrefix(patternNormalized, "*.")
}

// organizationJoinRulePublicMailboxDomains 是一份刻意保持小巧的内置公共邮箱服务商清单。
//
// 它只服务于一条提示（见 CreateOrganizationJoinRules）：163.com 这类域只有两个标签，
// 校验器当然放行，但它的注册对全世界开放，配成域名规则等于把组织白送给任何会用邮箱
// 注册的人。清单不追求完整、也不从外部维护，因为它的失败方向是安全的——漏掉一个域
// 只是少一句提示，宁缺毋滥；也正因为如此，它只能用来提示，绝不能用来拦截：
// 漏项一旦变成拒绝配置，就成了对合法域的误封。
var organizationJoinRulePublicMailboxDomains = map[string]struct{}{
	"163.com":        {},
	"126.com":        {},
	"yeah.net":       {},
	"qq.com":         {},
	"foxmail.com":    {},
	"sina.com":       {},
	"sohu.com":       {},
	"139.com":        {},
	"gmail.com":      {},
	"googlemail.com": {},
	"outlook.com":    {},
	"hotmail.com":    {},
	"live.com":       {},
	"yahoo.com":      {},
	"icloud.com":     {},
	"me.com":         {},
	"aol.com":        {},
	"proton.me":      {},
	"protonmail.com": {},
	"zoho.com":       {},
	"gmx.net":        {},
	"mail.ru":        {},
	"yandex.com":     {},
	"naver.com":      {},
}

// joinRulePublicMailboxProvider 返回基准域落在哪个公共邮箱服务商域上：
// 它本身就是清单里的一条，或是清单里某一条的子域（mail.163.com → 163.com）。
// 不是公共邮箱返回空串。
func joinRulePublicMailboxProvider(base string) string {
	labels := strings.Split(base, ".")
	for i := 0; i+1 < len(labels); i++ {
		suffix := strings.Join(labels[i:], ".")
		if _, ok := organizationJoinRulePublicMailboxDomains[suffix]; ok {
			return suffix
		}
	}
	return ""
}

// MatchJoinRule 判定一条规则是否命中归一化后的邮箱。地址规则严格全等；
// 域名规则要求域等于基准域（裸域名规则）或为其子域（通配规则）。
//
// 子域判定必须锚定在 "." 上：HasSuffix(domain, base) 会让 evil-enterprise.com
// 命中 enterprise.com，等于把整个企业域白名单送给了任何会注册近似域名的人。
func MatchJoinRule(rule *model.OrganizationJoinRule, normalizedEmail string) bool {
	if rule == nil || normalizedEmail == "" {
		return false
	}
	if rule.MatchType == model.OrganizationJoinRuleMatchTypeEmail {
		return normalizedEmail == rule.PatternNormalized
	}
	at := strings.LastIndex(normalizedEmail, "@")
	if at < 0 || at == len(normalizedEmail)-1 {
		return false
	}
	domain := normalizedEmail[at+1:]
	base := joinRuleBase(rule.PatternNormalized)
	if domain == base {
		return true
	}
	if !strings.HasPrefix(rule.PatternNormalized, "*.") {
		return false
	}
	return strings.HasSuffix(domain, "."+base)
}

// JoinRulesOverlap 判定两条规则是否可能命中同一个邮箱。域名规则之间同域或互为
// 父子域即重叠——配置期用这个函数拒绝第二个占用者，运行时才会有唯一的域名候选。
func JoinRulesOverlap(a, b *model.OrganizationJoinRule) bool {
	if a == nil || b == nil {
		return false
	}
	if a.MatchType == model.OrganizationJoinRuleMatchTypeEmail && b.MatchType == model.OrganizationJoinRuleMatchTypeEmail {
		return a.PatternNormalized == b.PatternNormalized
	}
	if a.MatchType == model.OrganizationJoinRuleMatchTypeEmail {
		return MatchJoinRule(b, a.PatternNormalized)
	}
	if b.MatchType == model.OrganizationJoinRuleMatchTypeEmail {
		return MatchJoinRule(a, b.PatternNormalized)
	}
	baseA := joinRuleBase(a.PatternNormalized)
	baseB := joinRuleBase(b.PatternNormalized)
	return baseA == baseB ||
		strings.HasSuffix(baseA, "."+baseB) ||
		strings.HasSuffix(baseB, "."+baseA)
}

// ErrOrganizationJoinRuleAmbiguous 表示一个邮箱同时命中多条域名规则。配置期互斥
// 保证它不该发生；真发生了就放弃加入并告警，绝不猜。
var ErrOrganizationJoinRuleAmbiguous = errors.New("organization join rule resolution is ambiguous")

// joinRuleDomainCandidates 列出某个域可能被哪些形态的域名规则命中：
// 每个后缀的裸域名形式与通配形式各一份。查询因此仍是一次唯一索引等值命中。
func joinRuleDomainCandidates(domain string) []string {
	labels := strings.Split(domain, ".")
	candidates := make([]string, 0, len(labels)*2)
	// 只枚举到倒数第二个标签为止：校验器要求域名至少两个标签，所以单标签后缀
	// （"com"、"*.com"）永远不可能是合法规则。把它们放进候选集只会让手写进库的
	// 脏行对每一个 .com 地址生效 —— 那不是放宽查询，那是放大一个坏行的爆炸半径。
	for i := 0; i+1 < len(labels); i++ {
		suffix := strings.Join(labels[i:], ".")
		candidates = append(candidates, suffix, "*."+suffix)
	}
	return candidates
}

// ResolveOrganizationJoinRuleWithTx 解析一个邮箱命中的唯一规则。地址规则优先；
// 域名规则用后缀候选集打唯一索引，不做全表扫描。
func ResolveOrganizationJoinRuleWithTx(tx *gorm.DB, normalizedEmail string) (*model.OrganizationJoinRule, error) {
	if tx == nil || normalizedEmail == "" {
		return nil, nil
	}
	at := strings.LastIndex(normalizedEmail, "@")
	if at < 0 || at == len(normalizedEmail)-1 {
		return nil, nil
	}

	var addressRules []model.OrganizationJoinRule
	if err := tx.Where("match_type = ? AND pattern_normalized = ?", model.OrganizationJoinRuleMatchTypeEmail, normalizedEmail).Limit(2).Find(&addressRules).Error; err != nil {
		return nil, err
	}
	if len(addressRules) > 1 {
		return nil, fmt.Errorf("%w: multiple address rules for %s", ErrOrganizationJoinRuleAmbiguous, normalizedEmail)
	}
	if len(addressRules) == 1 {
		return &addressRules[0], nil
	}

	domain := normalizedEmail[at+1:]
	var domainRules []model.OrganizationJoinRule
	if err := tx.Where("match_type = ? AND pattern_normalized IN ?", model.OrganizationJoinRuleMatchTypeDomain, joinRuleDomainCandidates(domain)).Find(&domainRules).Error; err != nil {
		return nil, err
	}
	var matched []model.OrganizationJoinRule
	for i := range domainRules {
		if MatchJoinRule(&domainRules[i], normalizedEmail) {
			matched = append(matched, domainRules[i])
		}
	}
	switch len(matched) {
	case 0:
		return nil, nil
	case 1:
		return &matched[0], nil
	default:
		ids := make([]int, 0, len(matched))
		for _, rule := range matched {
			ids = append(ids, rule.Id)
		}
		return nil, fmt.Errorf("%w: rules %v for %s", ErrOrganizationJoinRuleAmbiguous, ids, normalizedEmail)
	}
}

// JoinOrganizationByJoinRuleWithTx 依据已验证的邮箱把新用户加入命中的组织。
//
// 调用点只有注册路径（controller/user.go 的 Register，且只在邮箱验证通过的分支
// 内）。OAuth 创建路径把 provider 返回的邮箱直接写进 users.email 且不做任何验证
// （controller/oauth.go 的 OAuthEmailAlreadyTakenError 附近），一旦这里被挂到公共
// 创建路径上，任何人都能用未验证的邮箱冒充组织成员。
func JoinOrganizationByJoinRuleWithTx(tx *gorm.DB, user *model.User, verifiedEmail string) error {
	if tx == nil || user == nil || user.Id <= 0 {
		return errors.New("invalid organization join request")
	}
	normalizedEmail := model.NormalizeEmail(verifiedEmail)
	if normalizedEmail == "" {
		return nil
	}
	rule, err := ResolveOrganizationJoinRuleWithTx(tx, normalizedEmail)
	if err != nil {
		if errors.Is(err, ErrOrganizationJoinRuleAmbiguous) {
			common.SysError(fmt.Sprintf("organization join rule resolution is ambiguous, user %d stays personal: %v", user.Id, err))
			return nil
		}
		return err
	}
	if rule == nil {
		return nil
	}

	var organization model.Organization
	if err := tx.Select("id", "name", "slug", "status").Where("id = ?", rule.OrganizationId).First(&organization).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysError(fmt.Sprintf("organization %d referenced by join rule %d no longer exists", rule.OrganizationId, rule.Id))
			return nil
		}
		return err
	}
	if organization.Status != model.OrganizationStatusActive {
		// 一条失效规则不该把整个域的注册全部卡死：跳过加入，注册照常成功。
		common.SysError(fmt.Sprintf("organization %d is %s, user %d matched join rule %d but was not added", organization.Id, organization.Status, user.Id, rule.Id))
		return nil
	}

	// 同一个用户在同一事务里只可能被加入一次；这里的检查是防御性的：万一将来
	// 有人把本函数挂到第二个调用点（例如邮箱绑定），重复插入会撞上
	// idx_org_user 并连带回滚整个注册事务 —— 与本函数其余分支"只跳过、不阻断"
	// 的失败方向保持一致。
	//
	// 已存在时不重新激活：removed/exited 意味着组织已把这个人移除，注册流程
	// 不是推翻那个决定的地方。
	var existing model.OrganizationMember
	err = tx.Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).First(&existing).Error
	if err == nil {
		common.SysError(fmt.Sprintf("user %d already belongs to organization %d, join rule %d skipped", user.Id, organization.Id, rule.Id))
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	now := common.GetTimestamp()
	member := model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         user.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := tx.Create(&member).Error; err != nil {
		return err
	}
	return recordOrganizationAudit(tx, &organization, user.Id, organizationAuditOperatorRoleSystem, organizationAuditActionMemberAutoJoin, "member", member.Id, nil, member, "join_rule:"+rule.PatternNormalized)
}

const organizationJoinRuleMaxBatchSize = 500

const (
	organizationJoinRuleLineErrorInvalidPattern = "invalid_pattern"
	organizationJoinRuleLineErrorConflict       = "conflict"
)

const (
	organizationJoinRuleNoticeCoveredByOtherOrganization   = "covered_by_other_organization"
	organizationJoinRuleNoticeCoversOtherOrganizationEntry = "covers_other_organization_address"
	organizationJoinRuleNoticePublicMailboxProvider        = "public_mailbox_provider"
)

// ErrOrganizationJoinRuleEmailVerificationDisabled 表示当前没有开启邮箱验证：
// 规则永远不会生效，所以拒绝配置，而不是让管理员配完一堆死规则。
var ErrOrganizationJoinRuleEmailVerificationDisabled = errors.New("email verification is disabled")

type JoinRuleView struct {
	Id                 int    `json:"id"`
	OrganizationId     int    `json:"organization_id"`
	MatchType          string `json:"match_type"`
	Pattern            string `json:"pattern"`
	PatternNormalized  string `json:"pattern_normalized"`
	CreatedBy          int    `json:"created_by"`
	CreatorUsername    string `json:"creator_username"`
	CreatorDisplayName string `json:"creator_display_name"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
}

type CreateJoinRulesRequest struct {
	Patterns []string
	Reason   string
}

type JoinRuleLineError struct {
	Line             int    `json:"line"`
	Pattern          string `json:"pattern"`
	Kind             string `json:"kind"`
	Message          string `json:"message"`
	OrganizationName string `json:"organization_name,omitempty"`
}

type JoinRuleNotice struct {
	Pattern          string `json:"pattern"`
	Kind             string `json:"kind"`
	OrganizationName string `json:"organization_name"`
	PatternConflict  string `json:"pattern_conflict"`
}

type CreateJoinRulesResult struct {
	Rules      []JoinRuleView      `json:"rules"`
	LineErrors []JoinRuleLineError `json:"line_errors"`
	Notices    []JoinRuleNotice    `json:"notices"`
}

func ListOrganizationJoinRules(operatorUserId, organizationId int, accessMode string) ([]JoinRuleView, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, errors.New("invalid organization join rule request")
	}
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}
	if !actor.Capabilities.CanViewOrganization {
		return nil, errors.New("permission denied")
	}
	rules := make([]JoinRuleView, 0)
	if err := model.DB.Table("organization_join_rules").
		Select("organization_join_rules.*, users.username AS creator_username, users.display_name AS creator_display_name").
		Joins("LEFT JOIN users ON users.id = organization_join_rules.created_by").
		Where("organization_join_rules.organization_id = ?", organizationId).
		Order("organization_join_rules.id asc").
		Scan(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// CreateOrganizationJoinRules 批量写入规则。整批先校验再写：任何一行硬失败都不写库，
// 避免留下半批需要人工比对的脏数据。
func CreateOrganizationJoinRules(operatorUserId, organizationId int, accessMode string, req CreateJoinRulesRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*CreateJoinRulesResult, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, errors.New("invalid organization join rule request")
	}
	if !common.EmailVerificationEnabled {
		return nil, ErrOrganizationJoinRuleEmailVerificationDisabled
	}
	if len(req.Patterns) > organizationJoinRuleMaxBatchSize {
		return nil, errors.New("too many join rule patterns in one request")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("join rule reason is required")
	}
	result := &CreateJoinRulesResult{Rules: []JoinRuleView{}, LineErrors: []JoinRuleLineError{}, Notices: []JoinRuleNotice{}}
	now := common.GetTimestamp()

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityManageMembers)
		if err != nil {
			return err
		}
		if decision.Role != OrganizationPolicyRolePlatformAdmin && decision.Role != OrganizationPolicyRolePlatformRoot {
			return errors.New("permission denied")
		}

		// 规则总量是几十条量级：一次性读出来做互斥与提示，比逐行查库更省事也更一致。
		var existing []model.OrganizationJoinRule
		if err := tx.Find(&existing).Error; err != nil {
			return err
		}
		organizations := map[int]model.Organization{}
		var organizationRows []model.Organization
		if err := tx.Select("id", "name").Find(&organizationRows).Error; err != nil {
			return err
		}
		for _, row := range organizationRows {
			organizations[row.Id] = row
		}

		accepted := make([]model.OrganizationJoinRule, 0, len(req.Patterns))
		seen := map[string]struct{}{}
		for index, raw := range req.Patterns {
			line := index + 1
			pattern := strings.TrimSpace(raw)
			if pattern == "" {
				continue
			}
			matchType := joinRuleMatchTypeFor(pattern)
			normalized, err := NormalizeJoinRulePattern(matchType, pattern)
			if err != nil {
				result.LineErrors = append(result.LineErrors, JoinRuleLineError{
					Line: line, Pattern: pattern, Kind: organizationJoinRuleLineErrorInvalidPattern, Message: err.Error(),
				})
				continue
			}
			if _, duplicated := seen[normalized]; duplicated {
				continue // 同一次粘贴里的重复行不是错误
			}
			candidate := model.OrganizationJoinRule{
				OrganizationId:    organizationId,
				MatchType:         matchType,
				Pattern:           pattern,
				PatternNormalized: normalized,
				CreatedBy:         operatorUserId,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			conflict := false
			for i := range existing {
				if !JoinRulesOverlap(&candidate, &existing[i]) {
					continue
				}
				sameOrganization := existing[i].OrganizationId == organizationId
				patternTaken := existing[i].PatternNormalized == normalized
				bothDomains := existing[i].MatchType == model.OrganizationJoinRuleMatchTypeDomain && matchType == model.OrganizationJoinRuleMatchTypeDomain
				if sameOrganization && patternTaken {
					conflict = true // 本组织已有同一条：幂等跳过
					break
				}
				// 跨组织时同一 pattern 被占用（地址规则之间、域名规则之间），或任意
				// 两条域名规则互为父子域，都是硬冲突：必须报出行级冲突并指名占用者，
				// 否则管理员只会撞到唯一索引，看到的是一条原始数据库错误而不是"谁占着"。
				if (patternTaken && !sameOrganization) || bothDomains {
					result.LineErrors = append(result.LineErrors, JoinRuleLineError{
						Line:             line,
						Pattern:          pattern,
						Kind:             organizationJoinRuleLineErrorConflict,
						Message:          "pattern conflicts with an existing join rule",
						OrganizationName: organizations[existing[i].OrganizationId].Name,
					})
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}
			seen[normalized] = struct{}{}
			accepted = append(accepted, candidate)
		}
		if len(result.LineErrors) > 0 {
			return nil
		}

		// 提示：地址规则落在别的组织域名规则里（或反过来）时，两个组织的成员边界
		// 会在这几条地址上交叉，管理员需要在写死之前看到。
		for i := range accepted {
			for j := range existing {
				if existing[j].OrganizationId == organizationId || !JoinRulesOverlap(&accepted[i], &existing[j]) {
					continue
				}
				kind := organizationJoinRuleNoticeCoveredByOtherOrganization
				if accepted[i].MatchType == model.OrganizationJoinRuleMatchTypeDomain {
					kind = organizationJoinRuleNoticeCoversOtherOrganizationEntry
				}
				result.Notices = append(result.Notices, JoinRuleNotice{
					Pattern:          accepted[i].Pattern,
					Kind:             kind,
					OrganizationName: organizations[existing[j].OrganizationId].Name,
					PatternConflict:  existing[j].Pattern,
				})
				break
			}
		}

		// 提示：域名规则落在公共邮箱服务商上等于对所有人开放。配置仍然放行——只想
		// 放行具体的人时该写地址规则，地址规则在这里不产生提示——但管理员必须知道
		// 这条域名规则吸收的是"任何能注册 163.com 邮箱的人"，而不是某个组织。
		for i := range accepted {
			if accepted[i].MatchType != model.OrganizationJoinRuleMatchTypeDomain {
				continue
			}
			provider := joinRulePublicMailboxProvider(joinRuleBase(accepted[i].PatternNormalized))
			if provider == "" {
				continue
			}
			result.Notices = append(result.Notices, JoinRuleNotice{
				Pattern:         accepted[i].Pattern,
				Kind:            organizationJoinRuleNoticePublicMailboxProvider,
				PatternConflict: provider,
			})
		}

		for i := range accepted {
			if err := tx.Create(&accepted[i]).Error; err != nil {
				return err
			}
			view := JoinRuleView{
				Id:                accepted[i].Id,
				OrganizationId:    accepted[i].OrganizationId,
				MatchType:         accepted[i].MatchType,
				Pattern:           accepted[i].Pattern,
				PatternNormalized: accepted[i].PatternNormalized,
				CreatedBy:         accepted[i].CreatedBy,
				CreatedAt:         accepted[i].CreatedAt,
				UpdatedAt:         accepted[i].UpdatedAt,
			}
			var creator model.User
			if err := tx.Select("id", "username", "display_name").Where("id = ?", operatorUserId).First(&creator).Error; err == nil {
				view.CreatorUsername = creator.Username
				view.CreatorDisplayName = creator.DisplayName
			}
			result.Rules = append(result.Rules, view)
			if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionJoinRuleCreate, "join_rule", accepted[i].Id, nil, accepted[i], reason, auditMetadata...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 规则是跨组织的全局约束，冲突要能在组织审计之外被平台侧看到。
	common.SysLog(fmt.Sprintf("organization %d join rules created by user %d: %d rule(s), %d notice(s)", organizationId, operatorUserId, len(result.Rules), len(result.Notices)))
	return result, nil
}

func DeleteOrganizationJoinRule(operatorUserId, organizationId, ruleId int, accessMode string, reason string, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 || ruleId <= 0 {
		return errors.New("invalid organization join rule request")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("join rule reason is required")
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityManageMembers)
		if err != nil {
			return err
		}
		if decision.Role != OrganizationPolicyRolePlatformAdmin && decision.Role != OrganizationPolicyRolePlatformRoot {
			return errors.New("permission denied")
		}
		var rule model.OrganizationJoinRule
		if err := tx.Where("id = ? AND organization_id = ?", ruleId, organizationId).First(&rule).Error; err != nil {
			return err
		}
		if err := recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionJoinRuleDelete, "join_rule", rule.Id, rule, nil, reason, auditMetadata...); err != nil {
			return err
		}
		if err := tx.Delete(&model.OrganizationJoinRule{}, rule.Id).Error; err != nil {
			return err
		}
		common.SysLog(fmt.Sprintf("organization %d join rule %d (%s) deleted by user %d", organizationId, rule.Id, rule.PatternNormalized, operatorUserId))
		return nil
	})
}
