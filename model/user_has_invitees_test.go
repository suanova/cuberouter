package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// User without invitees -> HasInvitees returns false.
func TestHasInvitees_NoInvitees(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "has-invitees-inviter", 0)
	insertOpsUser(t, "has-invitees-invitee", inviter.Id)

	has, err := HasInvitees(200)
	require.NoError(t, err)
	assert.False(t, has, "user with no invitees must return false")
}

// User with invitees -> HasInvitees returns true.
func TestHasInvitees_HasInvitees(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "has-invitees-inviter-2", 0)
	insertOpsUser(t, "has-invitees-invitee-a", inviter.Id)
	insertOpsUser(t, "has-invitees-invitee-b", inviter.Id)

	has, err := HasInvitees(inviter.Id)
	require.NoError(t, err)
	assert.True(t, has, "user with invitees must return true")
}

// Inviter isolation: A has invitees, B does not.
func TestHasInvitees_InviterIsolation(t *testing.T) {
	truncateTables(t)
	a := insertOpsUser(t, "has-invitees-a", 0)
	b := insertOpsUser(t, "has-invitees-b", 0)
	insertOpsUser(t, "has-invitees-of-a", a.Id)

	hasA, err := HasInvitees(a.Id)
	require.NoError(t, err)
	assert.True(t, hasA)

	hasB, err := HasInvitees(b.Id)
	require.NoError(t, err)
	assert.False(t, hasB, "inviter b has no invitees")
}

// Non-existent user -> HasInvitees returns false, no error.
func TestHasInvitees_NonExistentUser(t *testing.T) {
	truncateTables(t)

	has, err := HasInvitees(99999)
	require.NoError(t, err)
	assert.False(t, has, "non-existent user must return false")
}
