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
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gorm.io/gorm"
)

func TestNormalizeJoinRulePattern(t *testing.T) {
	cases := []struct {
		name      string
		matchType string
		raw       string
		want      string
		wantErr   bool
	}{
		{name: "bare domain lowercased", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "  Enterprise.COM ", want: "enterprise.com"},
		{name: "trailing dot is one fqdn dot", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "enterprise.com.", want: "enterprise.com"},
		{name: "wildcard keeps its prefix", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "*.Enterprise.com", want: "*.enterprise.com"},
		{name: "bare single label rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "com", wantErr: true},
		{name: "wildcard on a bare tld rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "*.com", wantErr: true},
		{name: "wildcard in the middle rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "mail.*.enterprise.com", wantErr: true},
		{name: "bare star rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "*", wantErr: true},
		{name: "email in a domain rule rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "user@enterprise.com", wantErr: true},
		{name: "unicode domain rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "企業.com", wantErr: true},
		{name: "empty label rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "enterprise..com", wantErr: true},
		{name: "leading hyphen label rejected", matchType: model.OrganizationJoinRuleMatchTypeDomain, raw: "-enterprise.com", wantErr: true},
		{name: "address normalized", matchType: model.OrganizationJoinRuleMatchTypeEmail, raw: " User-A@Enterprise.com ", want: "user-a@enterprise.com"},
		{name: "invalid address rejected", matchType: model.OrganizationJoinRuleMatchTypeEmail, raw: "not-an-email", wantErr: true},
		{name: "unknown match type rejected", matchType: "wildcard", raw: "enterprise.com", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeJoinRulePattern(tc.matchType, tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNormalizeJoinRulePatternRejectsOverlongPattern(t *testing.T) {
	long := strings.Repeat("a", 190) + ".com" // 194 字节，超过唯一索引列宽
	_, err := NormalizeJoinRulePattern(model.OrganizationJoinRuleMatchTypeDomain, long)
	require.Error(t, err)
}

func TestJoinRuleMatchTypeFor(t *testing.T) {
	assert.Equal(t, model.OrganizationJoinRuleMatchTypeEmail, joinRuleMatchTypeFor("user-a@enterprise.com"))
	assert.Equal(t, model.OrganizationJoinRuleMatchTypeDomain, joinRuleMatchTypeFor("*.enterprise.com"))
	assert.Equal(t, model.OrganizationJoinRuleMatchTypeDomain, joinRuleMatchTypeFor("enterprise.com"))
}

func joinRuleForTest(t *testing.T, matchType, pattern string) *model.OrganizationJoinRule {
	t.Helper()
	normalized, err := NormalizeJoinRulePattern(matchType, pattern)
	require.NoError(t, err)
	return &model.OrganizationJoinRule{
		MatchType:         matchType,
		Pattern:           pattern,
		PatternNormalized: normalized,
	}
}

func TestMatchJoinRuleDomainBoundaries(t *testing.T) {
	wildcard := joinRuleForTest(t, model.OrganizationJoinRuleMatchTypeDomain, "*.enterprise.com")
	bare := joinRuleForTest(t, model.OrganizationJoinRuleMatchTypeDomain, "enterprise.com")
	cases := []struct {
		name  string
		rule  *model.OrganizationJoinRule
		email string
		want  bool
	}{
		{name: "wildcard matches apex", rule: wildcard, email: "a@enterprise.com", want: true},
		{name: "wildcard matches subdomain", rule: wildcard, email: "a@mail.enterprise.com", want: true},
		{name: "wildcard matches deep subdomain", rule: wildcard, email: "a@x.y.enterprise.com", want: true},
		{name: "wildcard does not match suffix spoof", rule: wildcard, email: "a@evil-enterprise.com", want: false},
		{name: "wildcard does not match reversed spoof", rule: wildcard, email: "a@enterprise.com.evil.com", want: false},
		{name: "wildcard does not match unrelated", rule: wildcard, email: "a@enterprise.org", want: false},
		{name: "bare matches apex", rule: bare, email: "a@enterprise.com", want: true},
		{name: "bare does not match subdomain", rule: bare, email: "a@mail.enterprise.com", want: false},
		{name: "empty email never matches", rule: wildcard, email: "", want: false},
		{name: "email without at never matches", rule: wildcard, email: "enterprise.com", want: false},
		{name: "trailing at never matches", rule: wildcard, email: "a@", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MatchJoinRule(tc.rule, tc.email))
		})
	}
}

func TestMatchJoinRuleAddressIsExact(t *testing.T) {
	rule := joinRuleForTest(t, model.OrganizationJoinRuleMatchTypeEmail, "user-a@enterprise.com")
	assert.True(t, MatchJoinRule(rule, "user-a@enterprise.com"))
	assert.False(t, MatchJoinRule(rule, "xuser-a@enterprise.com"))
	assert.False(t, MatchJoinRule(rule, "user-a@mail.enterprise.com"))
	assert.False(t, MatchJoinRule(rule, "user-a+1@enterprise.com"))
	assert.False(t, MatchJoinRule(rule, "someone-else@enterprise.com"))
}

func TestJoinRulesOverlap(t *testing.T) {
	domain := func(pattern string) *model.OrganizationJoinRule {
		return joinRuleForTest(t, model.OrganizationJoinRuleMatchTypeDomain, pattern)
	}
	address := func(pattern string) *model.OrganizationJoinRule {
		return joinRuleForTest(t, model.OrganizationJoinRuleMatchTypeEmail, pattern)
	}
	cases := []struct {
		name string
		a    *model.OrganizationJoinRule
		b    *model.OrganizationJoinRule
		want bool
	}{
		{name: "same bare domain", a: domain("enterprise.com"), b: domain("enterprise.com"), want: true},
		{name: "wildcard and its apex", a: domain("*.enterprise.com"), b: domain("enterprise.com"), want: true},
		{name: "parent covers child", a: domain("*.enterprise.com"), b: domain("mail.enterprise.com"), want: true},
		{name: "child covered by parent", a: domain("mail.enterprise.com"), b: domain("*.enterprise.com"), want: true},
		{name: "sibling subdomains do not overlap", a: domain("mail.enterprise.com"), b: domain("vpn.enterprise.com"), want: false},
		{name: "different domains do not overlap", a: domain("enterprise.com"), b: domain("partner.com"), want: false},
		{name: "suffix spoof does not overlap", a: domain("enterprise.com"), b: domain("evil-enterprise.com"), want: false},
		{name: "address inside a wildcard overlaps", a: address("user-a@enterprise.com"), b: domain("*.enterprise.com"), want: true},
		{name: "address on a bare domain overlaps", a: domain("enterprise.com"), b: address("user-a@enterprise.com"), want: true},
		{name: "address on a subdomain does not overlap a bare domain", a: domain("enterprise.com"), b: address("user-a@mail.enterprise.com"), want: false},
		{name: "same address overlaps", a: address("user-a@enterprise.com"), b: address("user-a@enterprise.com"), want: true},
		{name: "different addresses do not overlap", a: address("user-a@enterprise.com"), b: address("user-b@enterprise.com"), want: false},
		{name: "nil rule never overlaps", a: nil, b: domain("enterprise.com"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, JoinRulesOverlap(tc.a, tc.b))
		})
	}
}

func createJoinRuleForTest(t *testing.T, organizationId int, matchType, pattern string) *model.OrganizationJoinRule {
	t.Helper()
	normalized, err := NormalizeJoinRulePattern(matchType, pattern)
	require.NoError(t, err)
	rule := &model.OrganizationJoinRule{
		OrganizationId:    organizationId,
		MatchType:         matchType,
		Pattern:           pattern,
		PatternNormalized: normalized,
		CreatedBy:         1,
		CreatedAt:         common.GetTimestamp(),
		UpdatedAt:         common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(rule).Error)
	return rule
}

func TestResolveOrganizationJoinRulePrefersAddressRule(t *testing.T) {
	setupServiceTestDB(t)
	domainRule := createJoinRuleForTest(t, 1, model.OrganizationJoinRuleMatchTypeDomain, "*.enterprise.com")
	addressRule := createJoinRuleForTest(t, 2, model.OrganizationJoinRuleMatchTypeEmail, "user-a@enterprise.com")

	rule, err := ResolveOrganizationJoinRuleWithTx(model.DB, "user-a@enterprise.com")
	require.NoError(t, err)
	require.NotNil(t, rule)
	assert.Equal(t, addressRule.Id, rule.Id)

	// 域名规则仍然覆盖同一域里的其他人。
	rule, err = ResolveOrganizationJoinRuleWithTx(model.DB, "user-b@enterprise.com")
	require.NoError(t, err)
	require.NotNil(t, rule)
	assert.Equal(t, domainRule.Id, rule.Id)
}

func TestResolveOrganizationJoinRuleFindsWildcardAsAncestor(t *testing.T) {
	setupServiceTestDB(t)
	createJoinRuleForTest(t, 1, model.OrganizationJoinRuleMatchTypeDomain, "enterprise.com")
	other := createJoinRuleForTest(t, 2, model.OrganizationJoinRuleMatchTypeEmail, "user-a@enterprise.com")

	// 三级子域不在裸域名规则的范围内。
	resolved, err := ResolveOrganizationJoinRuleWithTx(model.DB, "user-c@deep.mail.enterprise.com")
	require.NoError(t, err)
	assert.Nil(t, resolved)

	resolved, err = ResolveOrganizationJoinRuleWithTx(model.DB, "user-a@enterprise.com")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, other.Id, resolved.Id)
}

func TestResolveOrganizationJoinRuleReportsAmbiguity(t *testing.T) {
	setupServiceTestDB(t)
	// 直接绕过配置期检查写入两条会同时命中的域名规则，模拟历史脏数据或检查被绕过。
	createJoinRuleForTest(t, 1, model.OrganizationJoinRuleMatchTypeDomain, "*.enterprise.com")
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    2,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "mail.enterprise.com",
		PatternNormalized: "mail.enterprise.com",
		CreatedBy:         1,
	}).Error)

	_, err := ResolveOrganizationJoinRuleWithTx(model.DB, "user-a@mail.enterprise.com")
	require.ErrorIs(t, err, ErrOrganizationJoinRuleAmbiguous)
}

func TestResolveOrganizationJoinRuleReturnsNilWhenNothingMatches(t *testing.T) {
	setupServiceTestDB(t)
	createJoinRuleForTest(t, 1, model.OrganizationJoinRuleMatchTypeDomain, "partner.com")

	rule, err := ResolveOrganizationJoinRuleWithTx(model.DB, "user@enterprise.com")
	require.NoError(t, err)
	assert.Nil(t, rule)

	rule, err = ResolveOrganizationJoinRuleWithTx(model.DB, "not-an-email")
	require.NoError(t, err)
	assert.Nil(t, rule)
}

func TestJoinRuleDomainCandidatesCoverBareAndWildcardForms(t *testing.T) {
	got := joinRuleDomainCandidates("a.b.enterprise.com")
	assert.Equal(t, []string{
		"a.b.enterprise.com", "*.a.b.enterprise.com",
		"b.enterprise.com", "*.b.enterprise.com",
		"enterprise.com", "*.enterprise.com",
	}, got)
}

func createJoinTestOrganization(t *testing.T, status string) model.Organization {
	t.Helper()
	owner := createServiceTestUser(t, "join-owner-"+common.GetUUID(), common.RoleCommonUser)
	organization := model.Organization{
		Name:        "Join Org " + common.GetUUID(),
		Slug:        "join-org-" + common.GetUUID(),
		Status:      status,
		OwnerUserId: owner.Id,
		CreatedBy:   owner.Id,
	}
	require.NoError(t, model.DB.Create(&organization).Error)
	return organization
}

func createJoinTestUser(t *testing.T, email string) model.User {
	t.Helper()
	user := model.User{
		Username:    "join-user-" + common.GetUUID(),
		DisplayName: "join user",
		Email:       email,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     common.GetUUID(),
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func TestJoinOrganizationByJoinRuleAddsVerifiedMember(t *testing.T) {
	setupServiceTestDB(t)
	organization := createJoinTestOrganization(t, model.OrganizationStatusActive)
	rule := createJoinRuleForTest(t, organization.Id, model.OrganizationJoinRuleMatchTypeDomain, "*.enterprise.com")
	user := createJoinTestUser(t, "newcomer@mail.enterprise.com")

	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		return JoinOrganizationByJoinRuleWithTx(tx, &user, user.Email)
	}))

	var member model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).First(&member).Error)
	assert.Equal(t, model.OrganizationRoleMember, member.Role)
	assert.Equal(t, model.OrganizationMemberStatusActive, member.Status)
	assert.Equal(t, 0, member.InvitedBy)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberAutoJoin).First(&audit).Error)
	assert.Equal(t, user.Id, audit.OperatorUserId)
	assert.Equal(t, "join_rule:"+rule.PatternNormalized, audit.Reason)
}

func TestJoinOrganizationByJoinRuleSkipsDisabledOrganization(t *testing.T) {
	setupServiceTestDB(t)
	organization := createJoinTestOrganization(t, model.OrganizationStatusDisabled)
	createJoinRuleForTest(t, organization.Id, model.OrganizationJoinRuleMatchTypeDomain, "*.enterprise.com")
	user := createJoinTestUser(t, "newcomer@enterprise.com")

	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		return JoinOrganizationByJoinRuleWithTx(tx, &user, user.Email)
	}))

	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).Count(&count).Error)
	assert.Zero(t, count, "停用的组织不吸收新成员")
}

func TestJoinOrganizationByJoinRuleSkipsAmbiguousRules(t *testing.T) {
	setupServiceTestDB(t)
	organization := createJoinTestOrganization(t, model.OrganizationStatusActive)
	createJoinRuleForTest(t, organization.Id, model.OrganizationJoinRuleMatchTypeDomain, "*.enterprise.com")
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "mail.enterprise.com",
		PatternNormalized: "mail.enterprise.com",
		CreatedBy:         1,
	}).Error)
	user := createJoinTestUser(t, "newcomer@mail.enterprise.com")

	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		return JoinOrganizationByJoinRuleWithTx(tx, &user, user.Email)
	}), "脏数据只跳过加入，不阻断注册")

	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
}

// 已存在的成员关系（哪怕是被移除的状态）不会被自动加入重新激活：
// 移除是组织的决定，注册流程不是推翻它的地方。
func TestJoinOrganizationByJoinRuleLeavesExistingMembershipAlone(t *testing.T) {
	setupServiceTestDB(t)
	organization := createJoinTestOrganization(t, model.OrganizationStatusActive)
	createJoinRuleForTest(t, organization.Id, model.OrganizationJoinRuleMatchTypeDomain, "*.enterprise.com")
	user := createJoinTestUser(t, "returning@enterprise.com")
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         user.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusRemoved,
	}).Error)

	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		return JoinOrganizationByJoinRuleWithTx(tx, &user, user.Email)
	}))

	var members []model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).Find(&members).Error)
	require.Len(t, members, 1, "重复插入必须被挡住，不能撞唯一索引")
	assert.Equal(t, model.OrganizationMemberStatusRemoved, members[0].Status, "被移除的成员不会被注册流程重新激活")
}

func TestJoinOrganizationByJoinRuleIgnoresUnmatchedEmail(t *testing.T) {
	setupServiceTestDB(t)
	organization := createJoinTestOrganization(t, model.OrganizationStatusActive)
	createJoinRuleForTest(t, organization.Id, model.OrganizationJoinRuleMatchTypeDomain, "partner.com")
	user := createJoinTestUser(t, "outsider@enterprise.com")

	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		return JoinOrganizationByJoinRuleWithTx(tx, &user, user.Email)
	}))

	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func createJoinRuleFixtureOrg(t *testing.T, slug string) (model.User, model.Organization) {
	t.Helper()
	// 规则管理的三个入口都以 common.EmailVerificationEnabled 为前置开关（关着直接返回
	// ErrOrganizationJoinRuleEmailVerificationDisabled），而 service 包的测试进程里该全局
	// 默认是 false（options 只在 main.go 的启动路径里加载）。所有规则管理用例都从这里取
	// 组织，所以开关在这里打开；关闭态由 TestCreateOrganizationJoinRulesRequiresEmailVerification
	// 自己覆盖。
	originalEmailVerification := common.EmailVerificationEnabled
	common.EmailVerificationEnabled = true
	t.Cleanup(func() { common.EmailVerificationEnabled = originalEmailVerification })

	root := createServiceTestUser(t, "join-rule-root-"+common.GetUUID(), common.RoleRootUser)
	organization := model.Organization{Name: "Rule Org " + common.GetUUID(), Slug: slug + "-" + common.GetUUID(), Status: model.OrganizationStatusActive, OwnerUserId: root.Id, CreatedBy: root.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	return root, organization
}

func TestCreateOrganizationJoinRulesRejectsOverlappingDomains(t *testing.T) {
	setupServiceTestDB(t)
	root, organization := createJoinRuleFixtureOrg(t, "rule-first")
	_, other := createJoinRuleFixtureOrg(t, "rule-second")

	first, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"*.enterprise.com"}, Reason: "onboarding"})
	require.NoError(t, err)
	require.Empty(t, first.LineErrors)
	require.Len(t, first.Rules, 1)

	second, err := CreateOrganizationJoinRules(root.Id, other.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"mail.enterprise.com"}, Reason: "onboarding"})
	require.NoError(t, err)
	require.Len(t, second.Rules, 0)
	require.Len(t, second.LineErrors, 1)
	assert.Equal(t, "conflict", second.LineErrors[0].Kind)
	assert.Equal(t, 1, second.LineErrors[0].Line)
	assert.Equal(t, organization.Name, second.LineErrors[0].OrganizationName)
}

// 同一个地址被别的组织占用时必须是行级冲突并指名占用者 ——
// 放它过去只会撞唯一索引，管理员看到的是原始数据库错误。
func TestCreateOrganizationJoinRulesRejectsForeignAddressPattern(t *testing.T) {
	setupServiceTestDB(t)
	_, firstOrg := createJoinRuleFixtureOrg(t, "rule-addr-first")
	root, secondOrg := createJoinRuleFixtureOrg(t, "rule-addr-second")

	_, err := CreateOrganizationJoinRules(root.Id, firstOrg.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"user-a@enterprise.com"}, Reason: "onboarding"})
	require.NoError(t, err)

	result, err := CreateOrganizationJoinRules(root.Id, secondOrg.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"user-a@enterprise.com"}, Reason: "onboarding"})
	require.NoError(t, err, "必须报出行级冲突，而不是把唯一索引的原始错误抛出来")
	require.Empty(t, result.Rules)
	require.Len(t, result.LineErrors, 1)
	assert.Equal(t, organizationJoinRuleLineErrorConflict, result.LineErrors[0].Kind)
	assert.Equal(t, firstOrg.Name, result.LineErrors[0].OrganizationName)
}

func TestCreateOrganizationJoinRulesIsIdempotentForOwnRules(t *testing.T) {
	setupServiceTestDB(t)
	root, organization := createJoinRuleFixtureOrg(t, "rule-idempotent")

	_, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"enterprise.com"}, Reason: "onboarding"})
	require.NoError(t, err)

	// 改完一处再整批重贴是常规动作：本组织已存在的规则静默跳过，不报错。
	again, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"enterprise.com", "partner.com"}, Reason: "onboarding"})
	require.NoError(t, err)
	require.Empty(t, again.LineErrors)
	require.Len(t, again.Rules, 1)
	assert.Equal(t, "partner.com", again.Rules[0].Pattern)
}

// 方向一：地址规则落在别的组织域名规则范围内——允许，但必须提示。
func TestCreateOrganizationJoinRulesNoticesAddressCoveredByForeignDomain(t *testing.T) {
	setupServiceTestDB(t)
	_, domainOrg := createJoinRuleFixtureOrg(t, "rule-domain-org")
	root, addressOrg := createJoinRuleFixtureOrg(t, "rule-address-org")

	_, err := CreateOrganizationJoinRules(root.Id, domainOrg.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"*.enterprise.com"}, Reason: "onboarding"})
	require.NoError(t, err)

	result, err := CreateOrganizationJoinRules(root.Id, addressOrg.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"user-a@enterprise.com"}, Reason: "contractor"})
	require.NoError(t, err)
	require.Empty(t, result.LineErrors, "地址规则落在别的组织域名规则里是允许的：地址更具体，例外正是这么表达的")
	require.Len(t, result.Rules, 1)
	require.Len(t, result.Notices, 1)
	assert.Equal(t, organizationJoinRuleNoticeCoveredByOtherOrganization, result.Notices[0].Kind)
	assert.Equal(t, domainOrg.Name, result.Notices[0].OrganizationName)
	assert.Equal(t, "*.enterprise.com", result.Notices[0].PatternConflict)
}

// 方向二：后加的域名规则覆盖别的组织已有地址规则——同样允许并提示，
// 因为地址规则更具体，那条地址仍然进原组织。
func TestCreateOrganizationJoinRulesNoticesDomainCoveringForeignAddress(t *testing.T) {
	setupServiceTestDB(t)
	_, addressOrg := createJoinRuleFixtureOrg(t, "rule-cover-address")
	root, domainOrg := createJoinRuleFixtureOrg(t, "rule-cover-domain")

	_, err := CreateOrganizationJoinRules(root.Id, addressOrg.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"user-a@partner.com"}, Reason: "contractor"})
	require.NoError(t, err)

	result, err := CreateOrganizationJoinRules(root.Id, domainOrg.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"*.partner.com"}, Reason: "onboarding"})
	require.NoError(t, err)
	require.Empty(t, result.LineErrors)
	require.Len(t, result.Rules, 1)
	require.Len(t, result.Notices, 1)
	assert.Equal(t, organizationJoinRuleNoticeCoversOtherOrganizationEntry, result.Notices[0].Kind)
	assert.Equal(t, addressOrg.Name, result.Notices[0].OrganizationName)
	assert.Equal(t, "user-a@partner.com", result.Notices[0].PatternConflict)
}

// 公共邮箱服务商上的域名规则吸收的是"任何能注册这个邮箱的人"，而不是某个组织，
// 必须在写入前说清楚；地址规则（user-a@163.com）是这条功能的正当用法，不提示。
func TestCreateOrganizationJoinRulesNoticesPublicMailboxProviderDomains(t *testing.T) {
	cases := []struct {
		name    string
		slug    string
		pattern string
		// 提示里该指名的服务商域；空表示这条规则不该产生提示。
		wantProvider string
	}{
		{name: "domain on a public provider", slug: "rule-public-domain", pattern: "163.com", wantProvider: "163.com"},
		{name: "wildcard on a public provider", slug: "rule-public-wildcard", pattern: "*.gmail.com", wantProvider: "gmail.com"},
		{name: "address on a public provider is the legitimate use", slug: "rule-public-address", pattern: "user-a@163.com"},
		{name: "corporate domain is not a public provider", slug: "rule-public-corporate", pattern: "*.enterprise.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupServiceTestDB(t)
			root, organization := createJoinRuleFixtureOrg(t, tc.slug)

			result, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{tc.pattern}, Reason: "onboarding"})
			require.NoError(t, err)
			require.Empty(t, result.LineErrors)
			require.Len(t, result.Rules, 1)

			if tc.wantProvider == "" {
				assert.Empty(t, result.Notices)
				return
			}
			require.Len(t, result.Notices, 1)
			assert.Equal(t, organizationJoinRuleNoticePublicMailboxProvider, result.Notices[0].Kind)
			assert.Equal(t, tc.pattern, result.Notices[0].Pattern)
			assert.Equal(t, tc.wantProvider, result.Notices[0].PatternConflict)
			assert.Empty(t, result.Notices[0].OrganizationName, "提示背后没有另一个组织")
		})
	}
}

func TestCreateOrganizationJoinRulesRejectsInvalidLinesAtomically(t *testing.T) {
	setupServiceTestDB(t)
	root, organization := createJoinRuleFixtureOrg(t, "rule-invalid")

	result, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"enterprise.com", "*"}, Reason: "onboarding"})
	require.NoError(t, err)
	require.Len(t, result.LineErrors, 1)
	assert.Equal(t, 2, result.LineErrors[0].Line)
	require.Empty(t, result.Rules)

	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationJoinRule{}).Where("organization_id = ?", organization.Id).Count(&count).Error)
	assert.Zero(t, count, "整批拒绝，不留半批脏数据")
}

func TestCreateOrganizationJoinRulesRejectsOversizedBatch(t *testing.T) {
	setupServiceTestDB(t)
	root, organization := createJoinRuleFixtureOrg(t, "rule-oversized")

	patterns := make([]string, organizationJoinRuleMaxBatchSize+1)
	for i := range patterns {
		patterns[i] = fmt.Sprintf("host%d.example.com", i)
	}
	_, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: patterns, Reason: "onboarding"})
	require.Error(t, err)
}

func TestCreateOrganizationJoinRulesRequiresEmailVerification(t *testing.T) {
	setupServiceTestDB(t)
	root, organization := createJoinRuleFixtureOrg(t, "rule-no-verify")
	original := common.EmailVerificationEnabled
	common.EmailVerificationEnabled = false
	t.Cleanup(func() { common.EmailVerificationEnabled = original })

	_, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"enterprise.com"}, Reason: "onboarding"})
	require.ErrorIs(t, err, ErrOrganizationJoinRuleEmailVerificationDisabled)
}

func TestDeleteOrganizationJoinRuleRequiresReasonAndScope(t *testing.T) {
	setupServiceTestDB(t)
	root, organization := createJoinRuleFixtureOrg(t, "rule-delete")
	_, other := createJoinRuleFixtureOrg(t, "rule-delete-other")

	created, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"enterprise.com"}, Reason: "onboarding"})
	require.NoError(t, err)
	require.Len(t, created.Rules, 1)

	require.Error(t, DeleteOrganizationJoinRule(root.Id, organization.Id, created.Rules[0].Id, OrganizationAccessModeAdmin, ""))
	require.Error(t, DeleteOrganizationJoinRule(root.Id, other.Id, created.Rules[0].Id, OrganizationAccessModeAdmin, "wrong org"))

	var stored model.OrganizationJoinRule
	require.NoError(t, model.DB.First(&stored, created.Rules[0].Id).Error)

	require.NoError(t, DeleteOrganizationJoinRule(root.Id, organization.Id, created.Rules[0].Id, OrganizationAccessModeAdmin, "offboarding"))
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationJoinRule{}).Where("id = ?", stored.Id).Count(&count).Error)
	assert.Zero(t, count)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("action_type = ?", organizationAuditActionJoinRuleDelete).First(&audit).Error)
	assert.Equal(t, "offboarding", audit.Reason)
}

func TestListOrganizationJoinRulesReturnsCreator(t *testing.T) {
	setupServiceTestDB(t)
	root, organization := createJoinRuleFixtureOrg(t, "rule-list")

	_, err := CreateOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin, CreateJoinRulesRequest{Patterns: []string{"enterprise.com"}, Reason: "onboarding"})
	require.NoError(t, err)

	rules, err := ListOrganizationJoinRules(root.Id, organization.Id, OrganizationAccessModeAdmin)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, root.Username, rules[0].CreatorUsername)
}
