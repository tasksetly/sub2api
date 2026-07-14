package repository

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type supportTicketS3Store struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
}

func NewSupportTicketAttachmentStore(cfg *config.Config) (service.SupportTicketAttachmentStore, error) {
	if cfg == nil || !cfg.SupportTicket.Attachments.Enabled {
		slog.Info("support_ticket_attachment storage_disabled")
		return nil, nil
	}
	storageCfg := cfg.SupportTicket.Attachments
	if strings.TrimSpace(storageCfg.Bucket) == "" || strings.TrimSpace(storageCfg.AccessKeyID) == "" || strings.TrimSpace(storageCfg.SecretAccessKey) == "" {
		return nil, fmt.Errorf("support ticket attachment storage requires bucket, access_key_id, and secret_access_key")
	}
	region := strings.TrimSpace(storageCfg.Region)
	if region == "" {
		region = "auto"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			storageCfg.AccessKeyID,
			storageCfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load support ticket S3 config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if endpoint := strings.TrimSpace(storageCfg.Endpoint); endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
		options.UsePathStyle = storageCfg.ForcePathStyle
		options.APIOptions = append(options.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	slog.Info("support_ticket_attachment storage_initialized", "endpoint", storageCfg.Endpoint, "bucket", storageCfg.Bucket, "region", region, "force_path_style", storageCfg.ForcePathStyle)
	return &supportTicketS3Store{client: client, presignClient: s3.NewPresignClient(client), bucket: storageCfg.Bucket}, nil
}

func (s *supportTicketS3Store) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), ContentLength: aws.Int64(size), Key: aws.String(key), Body: body, ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("S3 PutObject: %w", err)
	}
	return nil
}

func (s *supportTicketS3Store) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("S3 GetObject: %w", err)
	}
	return result.Body, nil
}

func (s *supportTicketS3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}

func (s *supportTicketS3Store) PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	result, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign support ticket attachment: %w", err)
	}
	return result.URL, nil
}
