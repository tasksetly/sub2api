package ticketupload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const multipartMemoryBytes = 1 << 20

func Parse(c *gin.Context, policy service.SupportTicketAttachmentPolicy) ([]service.SupportTicketAttachmentUpload, error) {
	maxBodyBytes := policy.MaxFileSizeBytes*int64(policy.MaxAttachmentsPerMessage) + multipartMemoryBytes
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	if err := c.Request.ParseMultipartForm(multipartMemoryBytes); err != nil {
		return nil, fmt.Errorf("parse multipart form: %w", err)
	}
	files := c.Request.MultipartForm.File["attachments"]
	if len(files) > policy.MaxAttachmentsPerMessage {
		return nil, service.ErrSupportTicketTooManyAttachments
	}
	uploads := make([]service.SupportTicketAttachmentUpload, 0, len(files))
	for _, fileHeader := range files {
		upload, err := readFile(fileHeader, policy.MaxFileSizeBytes)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, upload)
	}
	return uploads, nil
}

func readFile(fileHeader *multipart.FileHeader, maxFileSizeBytes int64) (service.SupportTicketAttachmentUpload, error) {
	if fileHeader.Size <= 0 || fileHeader.Size > maxFileSizeBytes {
		return service.SupportTicketAttachmentUpload{}, service.ErrSupportTicketAttachmentTooLarge
	}
	file, err := fileHeader.Open()
	if err != nil {
		return service.SupportTicketAttachmentUpload{}, fmt.Errorf("open support ticket attachment: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFileSizeBytes+1))
	if err != nil {
		return service.SupportTicketAttachmentUpload{}, fmt.Errorf("read support ticket attachment: %w", err)
	}
	if int64(len(data)) == 0 || int64(len(data)) > maxFileSizeBytes {
		return service.SupportTicketAttachmentUpload{}, service.ErrSupportTicketAttachmentTooLarge
	}
	return service.SupportTicketAttachmentUpload{FileName: fileHeader.Filename, Data: data}, nil
}
