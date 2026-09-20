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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func createOrganizationInviteTestOrg(t *testing.T) (model.User, *model.Organization) {
	t.Helper()
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "invite-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Invite Org"})
	require.NoError(t, err)
	return admin, organization
}

func TestRevokeInviteLocksOrganizationThenOperatorMemberAndInvitation(t *testing.T) {
	owner, organization := createOrganizationInviteTestOrg(t)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "revoke-lock@example.com", Role: model.OrganizationRoleMember, Token: "revoke-lock-token", Status: model.OrganizationInviteStatusPending, InviterUserId: owner.Id}
	require.NoError(t, model.DB.Create(&invite).Error)
	lockOrder := make([]string, 0, 3)
	callbackName := "test:organization-invite-revoke-lock-order"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		if !ok || locking.Strength != "UPDATE" {
			return
		}
		switch tx.Statement.Table {
		case "organizations":
			lockOrder = append(lockOrder, "organization")
		case "organization_members":
			lockOrder = append(lockOrder, "member")
		case "organization_invitations":
			lockOrder = append(lockOrder, "invitation")
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove(callbackName) })

	err := RevokeInvite(owner.Id, organization.Id, OrganizationAccessModeWorkspace, invite.Id, "cleanup")

	require.NoError(t, err)
	requireOrganizationRowLockOrder(t, lockOrder, []string{"organization", "member", "invitation"})
}

func withInviteEmailSender(t *testing.T, fn func(subject string, receiver string, content string) error) {
	t.Helper()
	old := sendOrganizationInviteEmail
	sendOrganizationInviteEmail = fn
	t.Cleanup(func() { sendOrganizationInviteEmail = old })
}

func expireOrganizationInviteIdempotencyRecord(t *testing.T, recordId int) {
	t.Helper()
	var record model.OrganizationIdempotencyRecord
	require.NoError(t, model.DB.First(&record, recordId).Error)
	record.ExpiresAt = common.GetTimestamp() - 1
	require.NoError(t, model.DB.Model(&record).Select("expires_at").Updates(&record).Error)
}

func TestNormalizeEmailTrimsAndLowercasesOnly(t *testing.T) {
	t.Parallel()

	require.Equal(t, "target@example.com", NormalizeEmail("  Target@Example.COM  "))
	require.Equal(t, "first last@example.com", NormalizeEmail(" First Last@Example.COM "))
}

func TestOrganizationInviteSMTPTimeoutUsesBoundedConfiguration(t *testing.T) {
	testCases := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{name: "missing", expected: 30 * time.Second},
		{name: "invalid", value: "invalid", expected: 30 * time.Second},
		{name: "zero", value: "0", expected: 30 * time.Second},
		{name: "minimum", value: "1", expected: time.Second},
		{name: "maximum", value: "240", expected: 240 * time.Second},
		{name: "above maximum", value: "241", expected: 30 * time.Second},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("SMTP_TIMEOUT_SECONDS", testCase.value)

			require.Equal(t, testCase.expected, organizationInviteSMTPTimeout())
			require.Less(t, organizationInviteSMTPTimeout(), time.Duration(organizationInviteDeliveryLeaseSeconds)*time.Second)
		})
	}
}

func TestOrganizationInviteDeliveryErrorClassification(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "recipient permanent rejection",
			err:      &common.EmailDeliveryError{Stage: common.EmailDeliveryStageRecipient, SMTPCode: 550, Err: errors.New("user not found")},
			expected: model.OrganizationInviteDeliveryRejected,
		},
		{
			name:     "recipient temporary rejection",
			err:      &common.EmailDeliveryError{Stage: common.EmailDeliveryStageRecipient, SMTPCode: 450, Err: errors.New("mailbox busy")},
			expected: model.OrganizationInviteDeliveryFailed,
		},
		{
			name:     "authentication failure",
			err:      &common.EmailDeliveryError{Stage: common.EmailDeliveryStageAuth, SMTPCode: 535, Err: errors.New("bad credentials")},
			expected: model.OrganizationInviteDeliveryFailed,
		},
		{
			name:     "content result unknown",
			err:      &common.EmailDeliveryError{Stage: common.EmailDeliveryStageContent, Err: errors.New("timeout")},
			expected: model.OrganizationInviteDeliveryUnknown,
		},
		{
			name:     "unclassified result unknown",
			err:      errors.New("unexpected smtp failure"),
			expected: model.OrganizationInviteDeliveryUnknown,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expected, organizationInviteDeliveryStatusForError(testCase.err))
		})
	}
}

func TestOrganizationInviteLookupSurvivesCryptoSecretChange(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	oldSecret := common.CryptoSecret
	common.CryptoSecret = "invite-secret-before"
	t.Cleanup(func() { common.CryptoSecret = oldSecret })
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "stable-lookup-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	common.CryptoSecret = "invite-secret-after"
	view, err := GetInviteByToken(invite.Token, 0)

	require.NoError(t, err)
	require.Equal(t, invite.Id, view.Id)
}

func TestOrganizationInviteLookupSupportsLegacyHMACTokenHash(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	oldSecret := common.CryptoSecret
	common.CryptoSecret = "legacy-invite-secret"
	t.Cleanup(func() { common.CryptoSecret = oldSecret })
	token := "legacy-lookup-token"
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, TokenHash: model.LegacyHashOrganizationInviteToken(token), Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	view, err := GetInviteByToken(token, 0)

	require.NoError(t, err)
	require.Equal(t, invite.Id, view.Id)
}

func TestOrganizationInviteMemberCannotCreateInvite(t *testing.T) {
	_, organization := createOrganizationInviteTestOrg(t)
	member := createServiceTestUser(t, "invite-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

	invite, err := CreateEmailInvite(member.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleMember})
	require.Error(t, err)
	require.Nil(t, invite)
}

func TestPlatformAdminCanListInvitesButCannotCreateInvite(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "platform-list-invite-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

	testCases := []struct {
		name  string
		role  int
		email string
	}{
		{name: "platform admin", role: common.RoleAdminUser, email: "platform-admin-target@example.com"},
		{name: "platform root", role: common.RoleRootUser, email: "platform-root-target@example.com"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actor := createServiceTestUser(t, "invite-"+tc.name, tc.role)
			invites, total, err := ListOrganizationInvitesPaged(actor.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationInviteListRequest{Limit: 10})
			require.NoError(t, err)
			require.EqualValues(t, 1, total)
			require.Len(t, invites, 1)

			created, err := CreateEmailInvite(actor.Id, organization.Id, OrganizationAccessModeAdmin, CreateInviteRequest{Email: tc.email, Role: model.OrganizationRoleMember})
			require.ErrorContains(t, err, "permission denied")
			require.Nil(t, created)
		})
	}
}

func TestOrganizationInviteEmailConfigMissingPersistsFailedDelivery(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		return common.SendEmailWithTimeout(subject, receiver, content, time.Second)
	})
	oldServer, oldAccount, oldFrom := common.SMTPServer, common.SMTPAccount, common.SMTPFrom
	common.SMTPServer, common.SMTPAccount, common.SMTPFrom = "", "", ""
	t.Cleanup(func() { common.SMTPServer, common.SMTPAccount, common.SMTPFrom = oldServer, oldAccount, oldFrom })

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})
	var deliveryErr *OrganizationInviteDeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, model.OrganizationInviteDeliveryFailed, deliveryErr.DeliveryStatus)
	require.Nil(t, invite)

	var stored model.OrganizationInvite
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationInviteStatusPending, stored.Status)
	require.Equal(t, model.OrganizationInviteDeliveryFailed, stored.DeliveryStatus)
	require.Equal(t, 1, stored.DeliveryAttempts)
	require.Zero(t, stored.DeliveredAt)
}

func TestOrganizationInviteEmailSendFailurePersistsFailedDelivery(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		return &common.EmailDeliveryError{Stage: common.EmailDeliveryStageConnect, Err: errors.New("smtp down")}
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})
	var deliveryErr *OrganizationInviteDeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, model.OrganizationInviteDeliveryFailed, deliveryErr.DeliveryStatus)
	require.NotContains(t, err.Error(), "smtp down")
	require.Nil(t, invite)

	var stored model.OrganizationInvite
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationInviteStatusPending, stored.Status)
	require.Equal(t, model.OrganizationInviteDeliveryFailed, stored.DeliveryStatus)
	require.Equal(t, 1, stored.DeliveryAttempts)
	require.Equal(t, "connect:failed", stored.LastDeliveryError)
	require.Zero(t, stored.DeliveredAt)
}

func TestOrganizationInviteRecipientRejectionPersistsRejectedDelivery(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		return &common.EmailDeliveryError{Stage: common.EmailDeliveryStageRecipient, SMTPCode: 550, Err: errors.New("550 User not found: target@example.com")}
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.Nil(t, invite)
	var deliveryErr *OrganizationInviteDeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, model.OrganizationInviteDeliveryRejected, deliveryErr.DeliveryStatus)
	require.NotZero(t, deliveryErr.InviteID)
	require.NotContains(t, deliveryErr.Error(), "User not found")
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, deliveryErr.InviteID).Error)
	require.Equal(t, model.OrganizationInviteDeliveryRejected, stored.DeliveryStatus)
	require.Equal(t, 1, stored.DeliveryAttempts)
	require.Equal(t, "recipient:550:rejected", stored.LastDeliveryError)
}

func TestOrganizationInviteUnclassifiedFailurePersistsUnknownDelivery(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		return errors.New("ambiguous smtp failure")
	})

	_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	var deliveryErr *OrganizationInviteDeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, model.OrganizationInviteDeliveryUnknown, deliveryErr.DeliveryStatus)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, deliveryErr.InviteID).Error)
	require.Equal(t, model.OrganizationInviteDeliveryUnknown, stored.DeliveryStatus)
	require.Equal(t, "unknown:unknown", stored.LastDeliveryError)
}

func TestOrganizationInvitePersistenceFailureNeverSendsEmail(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	sentCount := 0
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentCount++
		return nil
	})
	callbackName := "test:organization-invite-audit-failure"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "organization_audit_logs" {
			tx.AddError(errors.New("forced invite audit persistence failure"))
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove(callbackName) })

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.ErrorContains(t, err, "forced invite audit persistence failure")
	require.Nil(t, invite)
	require.Zero(t, sentCount)
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationInvite{}).Where("organization_id = ?", organization.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestOrganizationInviteFailedDeliveryRetryRotatesTokenAndSends(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		return &common.EmailDeliveryError{Stage: common.EmailDeliveryStageConnect, Err: errors.New("smtp down")}
	})

	_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})
	var deliveryErr *OrganizationInviteDeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, model.OrganizationInviteDeliveryFailed, deliveryErr.DeliveryStatus)
	var failed model.OrganizationInvite
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).First(&failed).Error)
	oldHash := failed.TokenHash

	var sentContent string
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentContent = content
		return nil
	})
	retried, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.NoError(t, err)
	require.Equal(t, failed.Id, retried.Id)
	require.NotEmpty(t, retried.Token)
	require.NotEqual(t, oldHash, model.HashOrganizationInviteToken(retried.Token))
	require.Contains(t, sentContent, "/organization/invite/"+retried.Token)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, failed.Id).Error)
	require.Equal(t, model.OrganizationInviteDeliverySent, stored.DeliveryStatus)
	require.Equal(t, 2, stored.DeliveryAttempts)
	require.Equal(t, model.HashOrganizationInviteToken(retried.Token), stored.TokenHash)
	require.NotZero(t, stored.DeliveredAt)
}

func TestOrganizationInviteDissolvedBetweenPersistenceAndDeliveryRevokesWithoutEmail(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	persisted, err := persistOrganizationEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})
	require.NoError(t, err)
	require.Equal(t, model.OrganizationInviteDeliveryPending, persisted.Invite.DeliveryStatus)
	require.Equal(t, 1, persisted.Invite.DeliveryAttempts)
	sentCount := 0
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentCount++
		return nil
	})
	require.NoError(t, dissolveOrganizationForTest(t, admin.Id, organization.Id, "closed before delivery"))

	err = deliverOrganizationEmailInvite(persisted)

	require.ErrorContains(t, err, "organization dissolved")
	require.Zero(t, sentCount)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, persisted.Invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusRevoked, stored.Status)
	require.NotZero(t, stored.RevokedAt)
	require.NotEqual(t, model.OrganizationInviteDeliverySent, stored.DeliveryStatus)
}

func TestOrganizationInviteRecentPendingDeliveryBlocksDuplicateSend(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	persisted, err := persistOrganizationEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})
	require.NoError(t, err)
	oldHash := persisted.Invite.TokenHash
	sentCount := 0
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentCount++
		return nil
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.Nil(t, invite)
	require.ErrorIs(t, err, ErrOrganizationInviteDeliveryInProgress)
	require.Zero(t, sentCount)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, persisted.Invite.Id).Error)
	require.Equal(t, oldHash, stored.TokenHash)
	require.Equal(t, 1, stored.DeliveryAttempts)
}

func TestOrganizationInviteConcurrentRetryOnlyOneEntersSMTP(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	now := common.GetTimestamp()
	existing := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "retry-concurrent-old", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliveryFailed, DeliveryAttempts: 1, InviterUserId: admin.Id, CreatedAt: now - 60, UpdatedAt: now - 30, ExpiredAt: now + 3600}
	require.NoError(t, model.DB.Create(&existing).Error)
	enteredSMTP := make(chan struct{}, 2)
	releaseSMTP := make(chan struct{})
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		enteredSMTP <- struct{}{}
		<-releaseSMTP
		return nil
	})
	firstResult := make(chan error, 1)
	go func() {
		_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: existing.TargetEmail, Role: existing.Role})
		firstResult <- err
	}()
	<-enteredSMTP

	second, secondErr := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: existing.TargetEmail, Role: existing.Role})

	require.Nil(t, second)
	require.ErrorIs(t, secondErr, ErrOrganizationInviteDeliveryInProgress)
	select {
	case <-enteredSMTP:
		t.Fatal("second retry entered SMTP")
	default:
	}
	close(releaseSMTP)
	require.NoError(t, <-firstResult)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, existing.Id).Error)
	require.Equal(t, model.OrganizationInviteDeliverySent, stored.DeliveryStatus)
	require.Equal(t, 2, stored.DeliveryAttempts)
}

func TestOrganizationInviteDeliveryFinalizationPreservesAcceptedLifecycle(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	target := createServiceTestUser(t, "invite-accepted-during-smtp", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&target).Update("email", "target@example.com").Error)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		parts := strings.SplitN(content, "/organization/invite/", 2)
		require.Len(t, parts, 2)
		rawToken := strings.SplitN(parts[1], `"`, 2)[0]
		return AcceptInvite(rawToken, target.Id)
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.NoError(t, err)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusAccepted, stored.Status)
	require.Equal(t, model.OrganizationInviteDeliverySent, stored.DeliveryStatus)
}

func TestOrganizationInviteDeliveryFinalizationPreservesRevokedLifecycle(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		var invite model.OrganizationInvite
		if err := model.DB.Where("organization_id = ? AND target_email = ?", organization.Id, receiver).First(&invite).Error; err != nil {
			return err
		}
		return RevokeInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, invite.Id, "revoked during smtp")
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.NoError(t, err)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusRevoked, stored.Status)
	require.Equal(t, model.OrganizationInviteDeliverySent, stored.DeliveryStatus)
}

func TestOrganizationInviteDeliveryFinalizationPreservesDissolvedLifecycle(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		return dissolveOrganizationForTest(t, admin.Id, organization.Id, "dissolved during smtp")
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.NoError(t, err)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusRevoked, stored.Status)
	require.Equal(t, model.OrganizationInviteDeliverySent, stored.DeliveryStatus)
}

func TestOrganizationInviteOldDeliveryResultCannotOverwriteNewToken(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	persisted, err := persistOrganizationEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})
	require.NoError(t, err)
	newTokenHash := model.HashOrganizationInviteToken("newer-attempt-token")
	require.NoError(t, model.DB.Model(&model.OrganizationInvite{}).Where("id = ?", persisted.Invite.Id).Updates(map[string]any{"token_hash": newTokenHash, "delivery_status": model.OrganizationInviteDeliveryPending}).Error)

	err = finalizeOrganizationInviteDelivery(persisted, model.OrganizationInviteDeliverySent, "", common.GetTimestamp())

	require.ErrorContains(t, err, "organization idempotency conflict")
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, persisted.Invite.Id).Error)
	require.Equal(t, newTokenHash, stored.TokenHash)
	require.Equal(t, model.OrganizationInviteDeliveryPending, stored.DeliveryStatus)
}

func TestOrganizationInviteListRepairsOnlyStalePendingDelivery(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	now := common.GetTimestamp()
	stale := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "stale@example.com", Role: model.OrganizationRoleMember, Token: "stale-token", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliveryPending, DeliveryAttempts: 1, InviterUserId: admin.Id, CreatedAt: now - 600, UpdatedAt: now - organizationInviteDeliveryLeaseSeconds - 1}
	accepted := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "accepted@example.com", Role: model.OrganizationRoleMember, Token: "accepted-token", Status: model.OrganizationInviteStatusAccepted, DeliveryStatus: model.OrganizationInviteDeliveryPending, DeliveryAttempts: 1, InviterUserId: admin.Id, CreatedAt: now - 600, UpdatedAt: now - organizationInviteDeliveryLeaseSeconds - 1}
	require.NoError(t, model.DB.Create(&stale).Error)
	require.NoError(t, model.DB.Create(&accepted).Error)

	invites, _, err := ListOrganizationInvitesPaged(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationInviteListRequest{Limit: 10})

	require.NoError(t, err)
	require.Len(t, invites, 2)
	require.NoError(t, model.DB.First(&stale, stale.Id).Error)
	require.Equal(t, model.OrganizationInviteDeliveryUnknown, stale.DeliveryStatus)
	require.Equal(t, "lease_expired:unknown", stale.LastDeliveryError)
	require.NoError(t, model.DB.First(&accepted, accepted.Id).Error)
	require.Equal(t, model.OrganizationInviteDeliveryPending, accepted.DeliveryStatus)
}

func TestOrganizationInviteForceRotateFailedDeliveryResumesSameIdempotency(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	now := common.GetTimestamp()
	existing := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "failed-resume-original", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id, CreatedAt: now, UpdatedAt: now, ExpiredAt: now + 3600}
	require.NoError(t, model.DB.Create(&existing).Error)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		return &common.EmailDeliveryError{Stage: common.EmailDeliveryStageConnect, Err: errors.New("smtp down")}
	})
	request := CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleAdmin, ForceRotate: true, IdempotencyKey: "force-rotate-failed-resume"}

	_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, request)
	var deliveryErr *OrganizationInviteDeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, model.OrganizationInviteDeliveryFailed, deliveryErr.DeliveryStatus)
	var failedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&failedInvite, existing.Id).Error)
	require.Equal(t, model.OrganizationInviteDeliveryFailed, failedInvite.DeliveryStatus)
	failedHash := failedInvite.TokenHash
	var failedRecord model.OrganizationIdempotencyRecord
	require.NoError(t, model.DB.Where("idempotency_key = ?", request.IdempotencyKey).First(&failedRecord).Error)
	require.Equal(t, model.OrganizationIdempotencyStatusFailed, failedRecord.Status)
	require.Contains(t, failedRecord.ResultJson, `"invite_id":`)

	sentCount := 0
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentCount++
		return nil
	})
	resumed, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, request)

	require.NoError(t, err)
	require.Equal(t, existing.Id, resumed.Id)
	require.NotEmpty(t, resumed.Token)
	require.NotEqual(t, failedHash, model.HashOrganizationInviteToken(resumed.Token))
	require.Equal(t, 1, sentCount)
	var succeededRecord model.OrganizationIdempotencyRecord
	require.NoError(t, model.DB.First(&succeededRecord, failedRecord.Id).Error)
	require.Equal(t, model.OrganizationIdempotencyStatusSucceeded, succeededRecord.Status)
	require.Zero(t, succeededRecord.ExpiresAt)
	require.NotContains(t, succeededRecord.ResultJson, resumed.Token)
}

func TestOrganizationInviteForceRotateProcessingResumesOnlyAfterExpiry(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	request := CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleMember, ForceRotate: true, IdempotencyKey: "force-rotate-processing-expiry"}
	persisted, err := persistOrganizationEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, request)
	require.NoError(t, err)
	firstHash := persisted.Invite.TokenHash
	sentCount := 0
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentCount++
		return nil
	})

	_, err = CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, request)
	require.ErrorIs(t, err, ErrOrganizationInviteDeliveryInProgress)
	require.Zero(t, sentCount)
	expireOrganizationInviteIdempotencyRecord(t, persisted.IdempotencyRecordId)
	require.NoError(t, model.DB.Model(&model.OrganizationInvite{}).Where("id = ?", persisted.Invite.Id).Update("updated_at", common.GetTimestamp()-organizationInviteDeliveryLeaseSeconds-1).Error)

	resumed, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, request)
	require.NoError(t, err)
	require.Equal(t, persisted.Invite.Id, resumed.Id)
	require.NotEqual(t, firstHash, model.HashOrganizationInviteToken(resumed.Token))
	require.Equal(t, 1, sentCount)
}

func TestExpiredOrganizationInviteDeliveryProcessingDoesNotBlockDissolutionForever(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	persisted, err := persistOrganizationEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", ForceRotate: true, IdempotencyKey: "invite-delivery-dissolve-expiry"})
	require.NoError(t, err)

	err = dissolveOrganizationForTest(t, admin.Id, organization.Id, "processing blocks")
	var blockedErr *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blockedErr)
	require.Contains(t, blockedErr.Blockers, "processing_idempotency")
	expireOrganizationInviteIdempotencyRecord(t, persisted.IdempotencyRecordId)

	err = dissolveOrganizationForTest(t, admin.Id, organization.Id, "expired processing ignored")
	require.NoError(t, err)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.First(&stored, persisted.Invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusRevoked, stored.Status)
}

func TestOrganizationInviteCreateLocksOrganizationBeforePersistence(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })
	organizationLocked := false
	inviteMutatedBeforeLock := false
	queryCallback := "test:organization-invite-lock"
	createCallback := "test:organization-invite-create-order"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		if tx.Statement.Table != "organizations" {
			return
		}
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		if ok && locking.Strength == "UPDATE" {
			organizationLocked = true
		}
	}))
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(createCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "organization_invitations" && !organizationLocked {
			inviteMutatedBeforeLock = true
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(queryCallback)
		_ = model.DB.Callback().Create().Remove(createCallback)
	})

	_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.NoError(t, err)
	requireOrganizationRowLock(t, organizationLocked, "creating an invite must lock the organization row before writing the invitation")
	requireOrganizationLockOrderingHolds(t, inviteMutatedBeforeLock, "creating an invite must lock the organization row before writing the invitation")
}

func TestOrganizationInviteDeliveryLocksOrganizationBeforeInvitation(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	persisted, err := persistOrganizationEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})
	require.NoError(t, err)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })
	organizationLocked := false
	invitationLockedBeforeOrganization := false
	callbackName := "test:organization-invite-delivery-lock-order"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		if !ok || locking.Strength != "UPDATE" {
			return
		}
		switch tx.Statement.Table {
		case "organizations":
			organizationLocked = true
		case "organization_invitations":
			if !organizationLocked {
				invitationLockedBeforeOrganization = true
			}
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove(callbackName) })

	err = deliverOrganizationEmailInvite(persisted)

	require.NoError(t, err)
	requireOrganizationRowLock(t, organizationLocked, "invite delivery must lock the organization row")
	requireOrganizationLockOrderingHolds(t, invitationLockedBeforeOrganization, "the organization row must be locked before the invitation row")
}

func TestOrganizationInviteDeliveryFailureReturnsFinalizationErrorWhenFailureMarkFails(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		return errors.New("original smtp failure")
	})
	callbackName := "test:organization-invite-delivery-mark-failure"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "organization_invitations" {
			tx.AddError(errors.New("forced delivery mark failure"))
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.ErrorContains(t, err, "forced delivery mark failure")
	require.NotContains(t, err.Error(), "original smtp failure")
}

func TestOrganizationInviteDeliveryRedactsRawTokenFromSMTPErrorAndStoredState(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	rawToken := ""
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		parts := strings.SplitN(content, "/organization/invite/", 2)
		require.Len(t, parts, 2)
		rawToken = strings.SplitN(parts[1], `"`, 2)[0]
		return &common.EmailDeliveryError{Stage: common.EmailDeliveryStageContent, Err: errors.New("smtp rejected content: " + content)}
	})

	_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com"})

	require.Error(t, err)
	require.NotEmpty(t, rawToken)
	require.NotContains(t, err.Error(), rawToken)
	var stored model.OrganizationInvite
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationInviteDeliveryUnknown, stored.DeliveryStatus)
	require.NotContains(t, stored.LastDeliveryError, rawToken)
}

func TestOrganizationInviteForceRotateLocksInvitationBeforeIdempotency(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	now := common.GetTimestamp()
	existing := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "force-lock-order-existing", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id, CreatedAt: now, UpdatedAt: now, ExpiredAt: now + 3600}
	require.NoError(t, model.DB.Create(&existing).Error)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })
	organizationLocked := false
	invitationLocked := false
	idempotencyBeforeInvitation := false
	queryCallback := "test:organization-invite-force-lock-query"
	createCallback := "test:organization-invite-force-lock-create"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		if !ok || locking.Strength != "UPDATE" {
			return
		}
		switch tx.Statement.Table {
		case "organizations":
			organizationLocked = true
		case "organization_invitations":
			invitationLocked = organizationLocked
		case "organization_idempotency_records":
			if !invitationLocked {
				idempotencyBeforeInvitation = true
			}
		}
	}))
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(createCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "organization_idempotency_records" && !invitationLocked {
			idempotencyBeforeInvitation = true
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(queryCallback)
		_ = model.DB.Callback().Create().Remove(createCallback)
	})

	_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleAdmin, ForceRotate: true, IdempotencyKey: "force-lock-order"})

	require.NoError(t, err)
	requireOrganizationRowLock(t, organizationLocked, "force rotate must lock the organization row first")
	requireOrganizationRowLock(t, invitationLocked, "force rotate must lock the invitation row after the organization row")
	requireOrganizationLockOrderingHolds(t, idempotencyBeforeInvitation, "force rotate must lock the invitation row before touching idempotency records")
}

func TestOrganizationInviteCreateSuccessCreatesPendingInvite(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	var sentReceiver string
	var sentContent string
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentReceiver = receiver
		sentContent = content
		return nil
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "Target@Example.com", Role: model.OrganizationRoleMember})
	require.NoError(t, err)
	require.NotNil(t, invite)
	require.Equal(t, model.OrganizationInviteStatusPending, invite.Status)
	require.Equal(t, model.OrganizationInviteDeliverySent, invite.DeliveryStatus)
	require.Equal(t, 1, invite.DeliveryAttempts)
	require.NotZero(t, invite.DeliveredAt)
	require.Equal(t, "target@example.com", invite.TargetEmail)
	require.Len(t, invite.Token, 43)
	require.Greater(t, invite.ExpiredAt, common.GetTimestamp())
	require.Equal(t, "target@example.com", sentReceiver)
	require.True(t, strings.Contains(sentContent, "/organization/invite/"+invite.Token))

	var storedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedInvite, invite.Id).Error)
	require.Empty(t, storedInvite.Token)
	require.Equal(t, model.HashOrganizationInviteToken(invite.Token), storedInvite.TokenHash)
	require.NotEqual(t, invite.Token, storedInvite.TokenHash)
}

func TestOrganizationInviteCreateRejectsExistingActiveMemberEmail(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	member := createServiceTestUser(t, "invite-existing-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&member).Update("email", "member@example.com").Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "Member@Example.com", Role: model.OrganizationRoleMember})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already joined organization")
	require.Nil(t, invite)
}

func TestOrganizationInviteCreateRejectsExistingDisabledMemberEmail(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	member := createServiceTestUser(t, "invite-existing-disabled", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&member).Update("email", "disabled@example.com").Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled}).Error)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "disabled@example.com", Role: model.OrganizationRoleMember})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already joined organization")
	require.Nil(t, invite)
}

func TestOrganizationInviteCreateAllowsRelationshipEndedMemberEmail(t *testing.T) {
	testCases := []struct {
		name   string
		status string
	}{
		{name: "removed", status: model.OrganizationMemberStatusRemoved},
		{name: "exited", status: model.OrganizationMemberStatusExited},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			admin, organization := createOrganizationInviteTestOrg(t)
			member := createServiceTestUser(t, "invite-ended-"+tc.name, common.RoleCommonUser)
			targetEmail := tc.name + "@example.com"
			require.NoError(t, model.DB.Model(&member).Update("email", targetEmail).Error)
			require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: tc.status}).Error)
			withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

			invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: targetEmail, Role: model.OrganizationRoleMember})
			require.NoError(t, err)
			require.NotNil(t, invite)
			require.Equal(t, model.OrganizationInviteStatusPending, invite.Status)
			require.Equal(t, targetEmail, invite.TargetEmail)
		})
	}
}

func TestOrganizationInviteCreateRejectsExistingPendingInvite(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	now := common.GetTimestamp()
	existing := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "existing-pending-token", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id, CreatedAt: now, UpdatedAt: now, ExpiredAt: now + 3600}
	require.NoError(t, model.DB.Create(&existing).Error)
	var sentCount int
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentCount++
		return nil
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleMember})
	require.Error(t, err)
	require.Contains(t, err.Error(), "pending invite already sent")
	require.Nil(t, invite)
	require.Zero(t, sentCount)

	var storedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedInvite, existing.Id).Error)
	require.Empty(t, storedInvite.Token)
	require.Equal(t, model.HashOrganizationInviteToken(existing.Token), storedInvite.TokenHash)
	require.Equal(t, existing.Role, storedInvite.Role)
	require.Equal(t, existing.UpdatedAt, storedInvite.UpdatedAt)

	var pendingCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationInvite{}).Where("organization_id = ? AND target_email = ? AND status = ?", organization.Id, "target@example.com", model.OrganizationInviteStatusPending).Count(&pendingCount).Error)
	require.EqualValues(t, 1, pendingCount)
}

func TestOrganizationInviteForceRotateUpdatesExistingPendingInvite(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	now := common.GetTimestamp()
	existing := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "force-resend-existing-token", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id, AcceptedUserId: 123, CreatedAt: now, UpdatedAt: now - 100, ExpiredAt: now + 3600, AcceptedAt: now - 90, RevokedAt: now - 80, Reason: "old"}
	require.NoError(t, model.DB.Create(&existing).Error)
	var sentReceiver string
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentReceiver = receiver
		return nil
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleAdmin, ForceRotate: true, IdempotencyKey: "force-rotate-existing-invite"})
	require.NoError(t, err)
	require.NotNil(t, invite)
	require.Equal(t, "target@example.com", sentReceiver)
	require.Equal(t, existing.Id, invite.Id)
	require.Equal(t, model.OrganizationRoleAdmin, invite.Role)
	require.Equal(t, model.OrganizationInviteStatusPending, invite.Status)
	require.NotEqual(t, existing.Token, invite.Token)
	require.Greater(t, invite.UpdatedAt, existing.UpdatedAt)
	require.Greater(t, invite.ExpiredAt, now)
	require.Zero(t, invite.AcceptedUserId)
	require.Zero(t, invite.AcceptedAt)
	require.Zero(t, invite.RevokedAt)
	require.Empty(t, invite.Reason)

	var storedExisting model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedExisting, existing.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusPending, storedExisting.Status)
	require.Empty(t, storedExisting.Token)
	require.Equal(t, model.HashOrganizationInviteToken(invite.Token), storedExisting.TokenHash)
	require.NotEqual(t, model.HashOrganizationInviteToken(existing.Token), storedExisting.TokenHash)
	require.Empty(t, storedExisting.Reason)
	require.Zero(t, storedExisting.RevokedAt)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, existing.Id, organizationAuditActionInviteResend).First(&audit).Error)
	require.Equal(t, "resend", audit.Reason)

	var inviteCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationInvite{}).Where("organization_id = ? AND target_email = ?", organization.Id, "target@example.com").Count(&inviteCount).Error)
	require.EqualValues(t, 1, inviteCount)
}

func TestOrganizationInviteForceRotateWithIdempotencyKeyReplaysWithoutRawToken(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	now := common.GetTimestamp()
	existing := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "idempotent-existing-token", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id, CreatedAt: now, UpdatedAt: now - 100, ExpiredAt: now + 3600}
	require.NoError(t, model.DB.Create(&existing).Error)
	var sentCount int
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentCount++
		return nil
	})

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: " Target@Example.com ", Role: model.OrganizationRoleAdmin, ForceRotate: true, IdempotencyKey: "invite-rotate-key"})
	require.NoError(t, err)
	require.NotEmpty(t, invite.Token)
	require.Equal(t, existing.Id, invite.Id)
	require.Equal(t, model.OrganizationRoleAdmin, invite.Role)
	require.Equal(t, 1, sentCount)
	_, err = GetInviteByToken(existing.Token, 0)
	require.Error(t, err)

	var record model.OrganizationIdempotencyRecord
	require.NoError(t, model.DB.Where("idempotency_key = ?", "invite-rotate-key").First(&record).Error)
	require.Equal(t, model.OrganizationIdempotencyStatusSucceeded, record.Status)
	require.NotContains(t, record.ResultJson, invite.Token)
	require.NotContains(t, record.ResultJson, "token")

	replayed, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleAdmin, ForceRotate: true, IdempotencyKey: "invite-rotate-key"})
	require.NoError(t, err)
	require.Equal(t, invite.Id, replayed.Id)
	require.Empty(t, replayed.Token)
	require.Equal(t, 1, sentCount)
}

func TestOrganizationInviteForceRotateReplaySucceedsAfterInviteAccepted(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	target := createServiceTestUser(t, "invite-force-replay-accepted", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&target).Update("email", "target@example.com").Error)
	var sentCount int
	withInviteEmailSender(t, func(subject string, receiver string, content string) error {
		sentCount++
		return nil
	})
	req := CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleMember, ForceRotate: true, IdempotencyKey: "invite-rotate-accepted-replay"}

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)
	require.NoError(t, AcceptInvite(invite.Token, target.Id))

	replayed, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)
	require.Equal(t, invite.Id, replayed.Id)
	require.Equal(t, model.OrganizationInviteStatusAccepted, replayed.Status)
	require.Empty(t, replayed.Token)
	require.Equal(t, 1, sentCount)
}

func TestOrganizationInviteForceRotateIdempotencyConflict(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	existing := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "conflict-existing-token", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id, CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp(), ExpiredAt: common.GetTimestamp() + 3600}
	require.NoError(t, model.DB.Create(&existing).Error)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

	_, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleMember, ForceRotate: true, IdempotencyKey: "invite-conflict-key"})
	require.NoError(t, err)
	_, err = CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleAdmin, ForceRotate: true, IdempotencyKey: "invite-conflict-key"})

	require.ErrorContains(t, err, "organization idempotency conflict")
}

func TestOrganizationInviteForceRotateRequiresIdempotencyKey(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleMember, ForceRotate: true})

	require.ErrorContains(t, err, "idempotency key is required")
	require.Nil(t, invite)
}

func TestOrganizationInviteReusesNonPendingInviteRecords(t *testing.T) {
	testCases := []struct {
		name   string
		status string
	}{
		{name: "expired", status: model.OrganizationInviteStatusExpired},
		{name: "revoked", status: model.OrganizationInviteStatusRevoked},
		{name: "accepted", status: model.OrganizationInviteStatusAccepted},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			admin, organization := createOrganizationInviteTestOrg(t)
			now := common.GetTimestamp()
			targetEmail := tc.name + "@example.com"
			existing := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: targetEmail, Role: model.OrganizationRoleMember, Token: tc.name + "-existing-token", Status: tc.status, InviterUserId: admin.Id, AcceptedUserId: 123, CreatedAt: now - 7200, UpdatedAt: now - 3600, ExpiredAt: now - 1, AcceptedAt: now - 3000, RevokedAt: now - 2000, Reason: "old"}
			require.NoError(t, model.DB.Create(&existing).Error)
			withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

			invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: targetEmail, Role: model.OrganizationRoleAdmin})
			require.NoError(t, err)
			require.NotNil(t, invite)
			require.Equal(t, existing.Id, invite.Id)
			require.Equal(t, model.OrganizationInviteStatusPending, invite.Status)
			require.Equal(t, model.OrganizationRoleAdmin, invite.Role)
			require.NotEqual(t, existing.Token, invite.Token)
			require.Greater(t, invite.ExpiredAt, now)
			require.Zero(t, invite.AcceptedUserId)
			require.Zero(t, invite.AcceptedAt)
			require.Zero(t, invite.RevokedAt)
			require.Empty(t, invite.Reason)

			var storedExisting model.OrganizationInvite
			require.NoError(t, model.DB.First(&storedExisting, existing.Id).Error)
			require.Empty(t, storedExisting.Token)
			require.Equal(t, model.HashOrganizationInviteToken(invite.Token), storedExisting.TokenHash)
			require.NotEqual(t, model.HashOrganizationInviteToken(existing.Token), storedExisting.TokenHash)

			var audit model.OrganizationAuditLog
			require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, existing.Id, organizationAuditActionInviteReopen).First(&audit).Error)
			require.Equal(t, "reopen from "+tc.status, audit.Reason)

			var inviteCount int64
			require.NoError(t, model.DB.Model(&model.OrganizationInvite{}).Where("organization_id = ? AND target_email = ?", organization.Id, targetEmail).Count(&inviteCount).Error)
			require.EqualValues(t, 1, inviteCount)
		})
	}
}

func TestOrganizationInviteEmailMismatchCannotAccept(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-email-mismatch", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "actual@example.com").Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "mismatch-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	err := AcceptInvite(invite.Token, user.Id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "email mismatch")
}

func TestOrganizationInvitePublicViewNormalizesTargetEmailForMatch(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-public-match", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "target@example.com").Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: " TARGET@Example.COM ", Role: model.OrganizationRoleMember, Token: "public-match-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	view, err := GetInviteByToken(invite.Token, user.Id)

	require.NoError(t, err)
	require.True(t, view.EmailMatched)
}

func TestOrganizationInviteLookupRejectsRevokedInvite(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "revoked-lookup-token", Status: model.OrganizationInviteStatusRevoked, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	_, err := GetInviteByToken(invite.Token, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invite is not pending")
}

func TestOrganizationInviteLookupRejectsExpiredInviteAndMarksExpired(t *testing.T) {
	_, organization := createOrganizationInviteTestOrg(t)
	now := common.GetTimestamp()
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "expired-lookup-token", Status: model.OrganizationInviteStatusPending, InviterUserId: organization.CreatedBy, ExpiredAt: now - 1}
	require.NoError(t, model.DB.Create(&invite).Error)

	_, err := GetInviteByToken(invite.Token, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invite expired")

	var storedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedInvite, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusExpired, storedInvite.Status)
}

func TestOrganizationInviteAcceptCreatesActiveMemberAndUsesWorkspaceAuditRole(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-accept", common.RoleAdminUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "target@example.com").Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "TARGET@example.com", Role: model.OrganizationRoleMember, Token: "accept-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	err := AcceptInvite(invite.Token, user.Id)
	require.NoError(t, err)

	var member model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).First(&member).Error)
	require.Equal(t, model.OrganizationMemberStatusActive, member.Status)
	require.Equal(t, model.OrganizationRoleMember, member.Role)

	var storedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedInvite, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusAccepted, storedInvite.Status)
	require.Equal(t, user.Id, storedInvite.AcceptedUserId)
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, invite.Id, organizationAuditActionInviteAccept).First(&audit).Error)
	require.Equal(t, model.OrganizationRoleMember, audit.OperatorRole)
}

func TestOrganizationInviteCannotReactivateDisabledMember(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-disabled-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "disabled@example.com").Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: user.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled}).Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "disabled@example.com", Role: model.OrganizationRoleMember, Token: "accept-disabled-member-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	err := AcceptInvite(invite.Token, user.Id)

	require.ErrorContains(t, err, "organization member disabled")
	var storedMember model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).First(&storedMember).Error)
	require.Equal(t, model.OrganizationMemberStatusDisabled, storedMember.Status)
	var storedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedInvite, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusPending, storedInvite.Status)
}

func TestAcceptInviteLocksOrganizationBeforeInvitation(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-accept-lock-order", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "target@example.com").Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "accept-lock-order-token", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)
	organizationLocked := false
	invitationLockedBeforeOrganization := false
	callbackName := "test:accept-invite-lock-order"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		if !ok || locking.Strength != "UPDATE" {
			return
		}
		switch tx.Statement.Table {
		case "organizations":
			organizationLocked = true
		case "organization_invitations":
			if !organizationLocked {
				invitationLockedBeforeOrganization = true
			}
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove(callbackName) })

	err := AcceptInvite(invite.Token, user.Id)

	require.NoError(t, err)
	requireOrganizationRowLock(t, organizationLocked, "invite delivery must lock the organization row")
	requireOrganizationLockOrderingHolds(t, invitationLockedBeforeOrganization, "the organization row must be locked before the invitation row")
}

func TestAcceptInviteUsesConditionalPendingTransition(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-accept-conditional", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "target@example.com").Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "accept-conditional-token", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)
	conditionalTransition := false
	callbackName := "test:accept-invite-conditional-transition"
	require.NoError(t, model.DB.Callback().Update().After("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "organization_invitations" {
			return
		}
		sql := strings.ToLower(tx.Statement.SQL.String())
		whereIndex := strings.Index(sql, " where ")
		if whereIndex >= 0 && strings.Contains(sql[whereIndex:], "status") {
			conditionalTransition = true
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	err := AcceptInvite(invite.Token, user.Id)

	require.NoError(t, err)
	require.True(t, conditionalTransition, "invite acceptance must condition its state transition on pending status")
}

func TestConcurrentAcceptInviteSQLite(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-accept-concurrent", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "target@example.com").Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "accept-concurrent-token", Status: model.OrganizationInviteStatusPending, DeliveryStatus: model.OrganizationInviteDeliverySent, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- AcceptInvite(invite.Token, user.Id)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	notPending := 0
	for resultErr := range results {
		if resultErr == nil {
			successes++
		} else if strings.Contains(resultErr.Error(), "invite is not pending") {
			notPending++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, notPending)
	var memberCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).Count(&memberCount).Error)
	require.EqualValues(t, 1, memberCount)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, invite.Id, organizationAuditActionInviteAccept).Count(&auditCount).Error)
	require.EqualValues(t, 1, auditCount)
}

func TestOrganizationInviteAcceptRejectsDisabledOrganization(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-disabled-accept", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "target@example.com").Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "disabled-accept-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)
	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	err := AcceptInvite(invite.Token, user.Id)
	require.ErrorContains(t, err, "organization disabled")

	var memberCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).Count(&memberCount).Error)
	require.EqualValues(t, 0, memberCount)
	var storedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedInvite, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusPending, storedInvite.Status)
}

func TestOrganizationInviteAcceptReusesRemovedMemberRecord(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-reactivate", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "target@example.com").Error)
	existing := model.OrganizationMember{OrganizationId: organization.Id, UserId: user.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusRemoved, RemovedAt: common.GetTimestamp()}
	require.NoError(t, model.DB.Create(&existing).Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleAdmin, Token: "reactivate-invite-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	err := AcceptInvite(invite.Token, user.Id)
	require.NoError(t, err)

	var members []model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).Find(&members).Error)
	require.Len(t, members, 1)
	require.Equal(t, existing.Id, members[0].Id)
	require.Equal(t, model.OrganizationMemberStatusActive, members[0].Status)
	require.Equal(t, model.OrganizationRoleAdmin, members[0].Role)
}

func TestOrganizationInviteRevokedInviteCannotAccept(t *testing.T) {
	admin, organization := createOrganizationInviteTestOrg(t)
	user := createServiceTestUser(t, "invite-revoked", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&user).Update("email", "target@example.com").Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "revoked-token", Status: model.OrganizationInviteStatusRevoked, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	err := AcceptInvite(invite.Token, user.Id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not pending")
}
