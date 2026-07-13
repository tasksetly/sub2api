package domain

import "time"

const (
	TicketStatusPending    = "pending"
	TicketStatusInProgress = "in_progress"
	TicketStatusResolved   = "resolved"
	TicketStatusClosed     = "closed"
)

const (
	TicketCategoryAccount = "account"
	TicketCategoryBilling = "billing"
	TicketCategoryAPI     = "api"
	TicketCategoryUsage   = "usage"
	TicketCategoryOther   = "other"
)

const (
	TicketPriorityLow    = "low"
	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"
)

type Ticket struct {
	ID             int64
	UserID         int64
	Subject        string
	Category       string
	Priority       string
	Status         string
	LastActivityAt time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TicketMessage struct {
	ID           int64
	TicketID     int64
	SenderUserID int64
	SenderRole   string
	Content      string
	CreatedAt    time.Time
}

func IsTicketStatus(status string) bool {
	switch status {
	case TicketStatusPending, TicketStatusInProgress, TicketStatusResolved, TicketStatusClosed:
		return true
	default:
		return false
	}
}

func IsTicketCategory(category string) bool {
	switch category {
	case TicketCategoryAccount, TicketCategoryBilling, TicketCategoryAPI, TicketCategoryUsage, TicketCategoryOther:
		return true
	default:
		return false
	}
}

func IsTicketPriority(priority string) bool {
	switch priority {
	case TicketPriorityLow, TicketPriorityNormal, TicketPriorityHigh, TicketPriorityUrgent:
		return true
	default:
		return false
	}
}

func CanUserReply(status string) bool {
	return status != TicketStatusClosed
}
