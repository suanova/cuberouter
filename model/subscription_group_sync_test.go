package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Subscription-driven group changes (upgrade / downgrade / expiration) must
// keep the "inviter and direct invitees share one group" invariant that
// registration inheritance establishes: UpdateUserGroupWithInviteesTx moves
// the direct invitees together with the inviter.

func seedSubscriptionSyncUser(t *testing.T, username string, group string, inviterId int, affCode string) *User {
	t.Helper()
	user := &User{
		Username: username, Password: "unused-password-hash",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Group: group, InviterId: inviterId, AffCode: affCode,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

// Upgrade: purchasing a plan that elevates the group also moves direct invitees.
func TestSubscriptionUpgradeSyncsInviteeGroup(t *testing.T) {
	truncateTables(t)
	inviter := seedSubscriptionSyncUser(t, "sub-sync-inviter", "default", 0, "SUBSYNC-A")
	invitee := seedSubscriptionSyncUser(t, "sub-sync-invitee", "default", inviter.Id, "SUBSYNC-B")

	plan := &SubscriptionPlan{
		Title: "sync-pro", DurationUnit: SubscriptionDurationMonth,
		DurationValue: 1, TotalAmount: 100, UpgradeGroup: "pro", Enabled: true,
	}
	require.NoError(t, DB.Create(plan).Error)

	subscription, err := CreateUserSubscriptionFromPlanTx(DB, inviter.Id, plan, "test")
	require.NoError(t, err)
	require.Equal(t, "default", subscription.PrevUserGroup)

	var afterInviter, afterInvitee User
	require.NoError(t, DB.First(&afterInviter, inviter.Id).Error)
	require.NoError(t, DB.First(&afterInvitee, invitee.Id).Error)
	assert.Equal(t, "pro", afterInviter.Group)
	assert.Equal(t, "pro", afterInvitee.Group, "invitee must follow the inviter's upgrade")
}

// Downgrade/cancellation: reverting the group also reverts direct invitees.
func TestSubscriptionDowngradeSyncsInviteeGroup(t *testing.T) {
	truncateTables(t)
	inviter := seedSubscriptionSyncUser(t, "sub-sync-inviter-2", "default", 0, "SUBSYNC-C")
	invitee := seedSubscriptionSyncUser(t, "sub-sync-invitee-2", "default", inviter.Id, "SUBSYNC-D")

	plan := &SubscriptionPlan{
		Title: "sync-pro-2", DurationUnit: SubscriptionDurationMonth,
		DurationValue: 1, TotalAmount: 100, UpgradeGroup: "pro", Enabled: true,
	}
	require.NoError(t, DB.Create(plan).Error)
	subscription, err := CreateUserSubscriptionFromPlanTx(DB, inviter.Id, plan, "test")
	require.NoError(t, err)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		target, err := downgradeUserGroupForSubscriptionTx(tx, subscription, time.Now().Unix()+1)
		assert.Equal(t, "default", target)
		return err
	}))

	var afterInviter, afterInvitee User
	require.NoError(t, DB.First(&afterInviter, inviter.Id).Error)
	require.NoError(t, DB.First(&afterInvitee, invitee.Id).Error)
	assert.Equal(t, "default", afterInviter.Group)
	assert.Equal(t, "default", afterInvitee.Group, "invitee must follow the inviter's downgrade")
}

// Expiration: the batch expiry task reverting the group also reverts invitees.
func TestSubscriptionExpirationSyncsInviteeGroup(t *testing.T) {
	truncateTables(t)
	inviter := seedSubscriptionSyncUser(t, "sub-sync-inviter-3", "pro", 0, "SUBSYNC-E")
	invitee := seedSubscriptionSyncUser(t, "sub-sync-invitee-3", "pro", inviter.Id, "SUBSYNC-F")

	sub := &UserSubscription{
		UserId: inviter.Id, PlanId: 1, Status: "active",
		UpgradeGroup: "pro", PrevUserGroup: "default", DowngradeGroup: "",
		StartTime: 1000, EndTime: 2000, // already due
		CreatedAt: 1000, UpdatedAt: 1000,
	}
	require.NoError(t, DB.Create(sub).Error)

	count, err := ExpireDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var afterInviter, afterInvitee User
	require.NoError(t, DB.First(&afterInviter, inviter.Id).Error)
	require.NoError(t, DB.First(&afterInvitee, invitee.Id).Error)
	assert.Equal(t, "default", afterInviter.Group)
	assert.Equal(t, "default", afterInvitee.Group, "invitee must follow the inviter's expiration revert")
}
