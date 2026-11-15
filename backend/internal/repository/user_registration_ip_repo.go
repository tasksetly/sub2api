package repository

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *userRepository) recordRegistrationIP(ctx context.Context, client *dbent.Client, userID int64) error {
	clientIP := service.RegistrationIPFromContext(ctx)
	if clientIP == "" {
		return nil
	}
	exec := sqlExecutorFromEntClient(client)
	if exec == nil {
		return fmt.Errorf("registration IP SQL executor is not configured")
	}
	_, err := exec.ExecContext(ctx, `INSERT INTO user_registration_ips (user_id, ip_address)
		VALUES ($1, $2) ON CONFLICT (user_id) DO NOTHING`, userID, clientIP)
	return err
}

func (r *userRepository) GetRegistrationIP(ctx context.Context, userID int64) (string, error) {
	exec, err := r.userProfileIdentitySQL(ctx)
	if err != nil {
		return "", err
	}
	rows, err := exec.QueryContext(ctx, `SELECT ip_address FROM user_registration_ips WHERE user_id = $1`, userID)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	var clientIP string
	if rows.Next() {
		if err := rows.Scan(&clientIP); err != nil {
			return "", err
		}
	}
	return clientIP, rows.Err()
}
