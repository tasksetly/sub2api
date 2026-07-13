package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTicketStateHelpers(t *testing.T) {
	assert.True(t, IsTicketStatus(TicketStatusPending))
	assert.True(t, IsTicketCategory("billing"))
	assert.True(t, IsTicketPriority("urgent"))
	assert.False(t, IsTicketStatus("reopened"))
	assert.False(t, CanUserReply(TicketStatusClosed))
	assert.True(t, CanUserReply(TicketStatusResolved))
}
