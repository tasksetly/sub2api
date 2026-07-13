package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTicketStateHelpers(t *testing.T) {
	t.Parallel()

	statusCases := []struct {
		status       string
		canUserReply bool
	}{
		{TicketStatusPending, true},
		{TicketStatusInProgress, true},
		{TicketStatusResolved, true},
		{TicketStatusClosed, false},
	}
	for _, tc := range statusCases {
		t.Run(tc.status, func(t *testing.T) {
			assert.True(t, IsTicketStatus(tc.status))
			assert.Equal(t, tc.canUserReply, CanUserReply(tc.status))
		})
	}

	for _, category := range []string{
		TicketCategoryAccount,
		TicketCategoryBilling,
		TicketCategoryAPI,
		TicketCategoryUsage,
		TicketCategoryOther,
	} {
		t.Run("category_"+category, func(t *testing.T) {
			assert.True(t, IsTicketCategory(category))
		})
	}

	for _, priority := range []string{
		TicketPriorityLow,
		TicketPriorityNormal,
		TicketPriorityHigh,
		TicketPriorityUrgent,
	} {
		t.Run("priority_"+priority, func(t *testing.T) {
			assert.True(t, IsTicketPriority(priority))
		})
	}

	assert.False(t, IsTicketStatus("reopened"))
	assert.False(t, IsTicketCategory("support"))
	assert.False(t, IsTicketPriority("critical"))
	assert.False(t, CanUserReply("reopened"))
}
