package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const supportTicketNotificationExcerptRunes = 240

type SupportTicketNotificationService struct {
	settingRepo SettingRepository
	userRepo    UserRepository
	emailQueue  *EmailQueueService
}

func NewSupportTicketNotificationService(settingRepo SettingRepository, userRepo UserRepository, emailQueue *EmailQueueService) *SupportTicketNotificationService {
	return &SupportTicketNotificationService{settingRepo: settingRepo, userRepo: userRepo, emailQueue: emailQueue}
}

func (s *SupportTicketNotificationService) TicketCreated(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage) {
	s.notifyAdmins(ctx, NotificationEmailEventSupportTicketCreated, ticket, message)
}

func (s *SupportTicketNotificationService) UserReplied(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage) {
	s.notifyAdmins(ctx, NotificationEmailEventSupportTicketUserReply, ticket, message)
}

func (s *SupportTicketNotificationService) AdminReplied(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage) {
	s.notifyUser(ctx, NotificationEmailEventSupportTicketAdminReply, ticket, message, "")
}

func (s *SupportTicketNotificationService) StatusChanged(ctx context.Context, ticket *SupportTicket, oldStatus string, adminID int64) {
	actor := s.userDisplayName(ctx, adminID)
	s.notifyUser(ctx, NotificationEmailEventSupportTicketStatusChanged, ticket, nil, oldStatus, actor)
}

func (s *SupportTicketNotificationService) notifyAdmins(ctx context.Context, event string, ticket *SupportTicket, message *SupportTicketMessage) {
	if !s.enabled(ctx) || ticket == nil || s.emailQueue == nil {
		return
	}
	recipients := s.adminRecipients(ctx)
	actor := s.userDisplayName(ctx, ticket.UserID)
	variables := supportTicketNotificationVariables(ticket, message, actor, "", s.ticketURL(ctx, true))
	for _, recipient := range recipients {
		s.enqueue(NotificationEmailSendInput{
			Event: event, RecipientEmail: recipient, RecipientName: emailRecipientName(recipient),
			SourceType: "support_ticket", SourceID: strconv.FormatInt(ticket.ID, 10),
			ReminderKey: supportTicketNotificationReminderKey(message, ticket), Variables: variables,
		})
	}
}

func (s *SupportTicketNotificationService) notifyUser(ctx context.Context, event string, ticket *SupportTicket, message *SupportTicketMessage, oldStatus string, actorOverride ...string) {
	if !s.enabled(ctx) || ticket == nil || s.emailQueue == nil {
		return
	}
	user, err := s.userRepo.GetByID(ctx, ticket.UserID)
	if err != nil || user == nil {
		slog.Warn("support ticket email recipient lookup failed", "ticket_id", ticket.ID, "err", err)
		return
	}
	actor := "Support"
	if len(actorOverride) > 0 && strings.TrimSpace(actorOverride[0]) != "" {
		actor = actorOverride[0]
	} else if message != nil {
		actor = s.userDisplayName(ctx, message.SenderID)
	}
	variables := supportTicketNotificationVariables(ticket, message, actor, oldStatus, s.ticketURL(ctx, false))
	for _, recipient := range supportTicketUserRecipients(user) {
		s.enqueue(NotificationEmailSendInput{
			Event: event, RecipientEmail: recipient, RecipientName: supportTicketUserName(user), UserID: user.ID,
			SourceType: "support_ticket", SourceID: strconv.FormatInt(ticket.ID, 10),
			ReminderKey: supportTicketNotificationReminderKey(message, ticket), Variables: variables,
		})
	}
}

func (s *SupportTicketNotificationService) enabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeySupportTicketNotifyEnabled)
	return err == nil && value == "true"
}

func (s *SupportTicketNotificationService) adminRecipients(ctx context.Context) []string {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeySupportTicketNotifyEmails)
	if err == nil && strings.TrimSpace(raw) != "" && strings.TrimSpace(raw) != "[]" {
		entries := ParseNotifyEmails(raw)
		if len(entries) > 0 {
			return filterVerifiedEmails(entries)
		}
	}
	admin, err := s.userRepo.GetFirstAdmin(ctx)
	if err != nil || admin == nil || strings.TrimSpace(admin.Email) == "" {
		return nil
	}
	return []string{strings.TrimSpace(admin.Email)}
}

func (s *SupportTicketNotificationService) userDisplayName(ctx context.Context, userID int64) string {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Sprintf("#%d", userID)
	}
	return supportTicketUserName(user)
}

func (s *SupportTicketNotificationService) ticketURL(ctx context.Context, admin bool) string {
	base := ""
	for _, key := range []string{SettingKeyFrontendURL, SettingKeyAPIBaseURL} {
		value, err := s.settingRepo.GetValue(ctx, key)
		if err == nil && strings.TrimSpace(value) != "" {
			base = strings.TrimRight(strings.TrimSpace(value), "/")
			break
		}
	}
	base = strings.TrimSuffix(base, "/api/v1")
	path := "/tickets"
	if admin {
		path = "/admin/tickets"
	}
	return base + path
}

func (s *SupportTicketNotificationService) enqueue(input NotificationEmailSendInput) {
	if err := s.emailQueue.EnqueueNotification(input); err != nil {
		slog.Warn("support ticket notification email queue rejected task", "event", input.Event, "source_id", input.SourceID, "err", err)
	}
}

func supportTicketNotificationVariables(ticket *SupportTicket, message *SupportTicketMessage, actor, oldStatus, ticketURL string) map[string]string {
	messageExcerpt := ""
	if message != nil {
		messageExcerpt = strings.TrimSpace(message.Content)
		if messageExcerpt == "" && len(message.Attachments) > 0 {
			messageExcerpt = "Image attachment / 图片附件"
		}
		runes := []rune(messageExcerpt)
		if len(runes) > supportTicketNotificationExcerptRunes {
			messageExcerpt = string(runes[:supportTicketNotificationExcerptRunes]) + "..."
		}
	}
	return map[string]string{
		"ticket_id": strconv.FormatInt(ticket.ID, 10), "ticket_subject": ticket.Subject,
		"ticket_category": ticket.Category, "ticket_priority": ticket.Priority, "ticket_status": ticket.Status,
		"ticket_old_status": oldStatus, "ticket_new_status": ticket.Status, "ticket_actor": actor,
		"ticket_message": messageExcerpt, "ticket_url": ticketURL,
	}
}

func supportTicketNotificationReminderKey(message *SupportTicketMessage, ticket *SupportTicket) string {
	if message != nil && message.ID > 0 {
		return "message:" + strconv.FormatInt(message.ID, 10)
	}
	updatedAt := ticket.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	return "status:" + ticket.Status + ":" + strconv.FormatInt(updatedAt.UnixNano(), 10)
}

func supportTicketUserRecipients(user *User) []string {
	if user == nil {
		return nil
	}
	candidates := append([]string{user.Email}, filterVerifiedEmails(user.BalanceNotifyExtraEmails)...)
	seen := make(map[string]struct{}, len(candidates))
	recipients := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		email := strings.TrimSpace(candidate)
		key := strings.ToLower(email)
		if email == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		recipients = append(recipients, email)
	}
	return recipients
}

func supportTicketUserName(user *User) string {
	if user == nil {
		return "User"
	}
	if name := strings.TrimSpace(user.Username); name != "" {
		return name
	}
	if email := strings.TrimSpace(user.Email); email != "" {
		return email
	}
	return fmt.Sprintf("#%d", user.ID)
}
