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
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
