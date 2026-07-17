package dto

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const ticketMultipartLimit int64 = 256 << 20

type TicketAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
}

type TicketMessage struct {
	ID          int64              `json:"id"`
	AuthorID    int64              `json:"author_id"`
	AuthorRole  string             `json:"author_role"`
	AuthorName  string             `json:"author_name"`
	Content     string             `json:"content"`
	Attachments []TicketAttachment `json:"attachments"`
	CreatedAt   time.Time          `json:"created_at"`
}

type Ticket struct {
	ID            int64           `json:"id"`
	UserID        int64           `json:"user_id"`
	UserEmail     string          `json:"user_email,omitempty"`
	Username      string          `json:"username,omitempty"`
	Subject       string          `json:"subject"`
	Category      string          `json:"category"`
	Status        string          `json:"status"`
	Priority      string          `json:"priority"`
	LastMessageAt time.Time       `json:"last_message_at"`
	ClosedAt      *time.Time      `json:"closed_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	Messages      []TicketMessage `json:"messages,omitempty"`
}

func TicketFromService(item *service.Ticket, admin bool) *Ticket {
	if item == nil {
		return nil
	}
	out := &Ticket{
		ID: item.ID, UserID: item.UserID, UserEmail: item.UserEmail, Username: item.Username,
		Subject: item.Subject, Category: item.Category, Status: item.Status, Priority: item.Priority,
		LastMessageAt: item.LastMessageAt, ClosedAt: item.ClosedAt, CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt, Messages: make([]TicketMessage, 0, len(item.Messages)),
	}
	for _, message := range item.Messages {
		converted := TicketMessage{
			ID: message.ID, AuthorID: message.AuthorID, AuthorRole: message.AuthorRole,
			AuthorName: message.AuthorName, Content: message.Content, CreatedAt: message.CreatedAt,
			Attachments: make([]TicketAttachment, 0, len(message.Attachments)),
		}
		for index, attachment := range message.Attachments {
			path := fmt.Sprintf("/api/v1/tickets/%d/messages/%d/attachments/%d", item.ID, message.ID, index)
			if admin {
				path = fmt.Sprintf("/api/v1/admin/tickets/%d/messages/%d/attachments/%d", item.ID, message.ID, index)
			}
			converted.Attachments = append(converted.Attachments, TicketAttachment{
				Name: attachment.Name, ContentType: attachment.ContentType, Size: attachment.Size,
				URL: path,
			})
		}
		out.Messages = append(out.Messages, converted)
	}
	return out
}

func TicketsFromService(items []service.Ticket, admin bool) []Ticket {
	out := make([]Ticket, 0, len(items))
	for i := range items {
		out = append(out, *TicketFromService(&items[i], admin))
	}
	return out
}

// ParseTicketMultipart reads text fields and image files from a ticket form.
func ParseTicketMultipart(c *gin.Context) (subject, category, content string, uploads []service.TicketUpload, err error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ticketMultipartLimit)
	if err = c.Request.ParseMultipartForm(8 << 20); err != nil {
		return "", "", "", nil, fmt.Errorf("invalid multipart form: %w", err)
	}
	subject = c.PostForm("subject")
	category = c.PostForm("category")
	content = c.PostForm("content")
	form := c.Request.MultipartForm
	if form == nil {
		return subject, category, content, []service.TicketUpload{}, nil
	}
	files := form.File["images"]
	uploads = make([]service.TicketUpload, 0, len(files))
	for _, header := range files {
		file, openErr := header.Open()
		if openErr != nil {
			return "", "", "", nil, fmt.Errorf("open image: %w", openErr)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, 51<<20))
		_ = file.Close()
		if readErr != nil {
			return "", "", "", nil, fmt.Errorf("read image: %w", readErr)
		}
		uploads = append(uploads, service.TicketUpload{Name: header.Filename, Data: data})
	}
	return subject, category, content, uploads, nil
}
