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
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Subject        string    `json:"subject"`
	Category       string    `json:"category"`
	Priority       string    `json:"priority"`
	Status         string    `json:"status"`
	LastActivityAt time.Time `json:"last_activity_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TicketMessage struct {
	ID           int64     `json:"id"`
	TicketID     int64     `json:"ticket_id"`
	SenderUserID int64     `json:"sender_user_id"`
	SenderRole   string    `json:"sender_role"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
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
	return IsTicketStatus(status) && status != TicketStatusClosed
}
