package service

import "github.com/Wei-Shaw/sub2api/internal/config"

// supportTicketAttachmentConfigProvider is implemented by legacy attachment
// stores that can reload their static configuration at runtime.
type supportTicketAttachmentConfigProvider interface {
	CurrentConfig() config.SupportTicketAttachmentConfig
}
