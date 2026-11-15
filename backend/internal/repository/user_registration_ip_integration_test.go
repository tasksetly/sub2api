//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRegistrationIPIsCreatedWithUserAndSurvivesLogin(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newUserRepositoryWithSQL(client, integrationDB)
	user := &service.User{
		Email: fmt.Sprintf("registration-ip-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "test-hash", Role: service.RoleUser, Status: service.StatusActive,
	}
	require.NoError(t, repo.Create(service.WithRegistrationIP(ctx, "::ffff:203.0.113.10"), user))
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM users WHERE id = $1`, user.ID) })
	address, err := repo.GetRegistrationIP(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.10", address)
	// Attempting to record a later login cannot overwrite the original address.
	require.NoError(t, repo.recordRegistrationIP(service.WithRegistrationIP(ctx, "192.0.2.1"), client, user.ID))
	address, err = repo.GetRegistrationIP(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.10", address)

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	opCtx := service.WithRegistrationIP(dbent.NewTxContext(ctx, tx), "198.51.100.1")
	rolledBack := &service.User{
		Email: fmt.Sprintf("registration-ip-rollback-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "test-hash", Role: service.RoleUser, Status: service.StatusActive,
	}
	require.NoError(t, repo.Create(opCtx, rolledBack))
	require.NoError(t, tx.Rollback())
	address, err = repo.GetRegistrationIP(ctx, rolledBack.ID)
	require.NoError(t, err)
	require.Empty(t, address, "The registration IP must roll back with the user")
}

func TestBlacklistAndUserDeletionShareTransaction(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	users := newUserRepositoryWithSQL(client, integrationDB)
	settings := &settingRepository{client: client}
	user := mustCreateUser(t, client, &service.User{})
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM users WHERE id = $1`, user.ID) })
	before, beforeErr := settings.GetValue(ctx, service.SettingKeyGlobalIPBlacklist)
	if beforeErr != nil {
		require.ErrorIs(t, beforeErr, service.ErrSettingNotFound)
	}
	t.Cleanup(func() {
		if beforeErr == nil {
			_ = settings.Set(ctx, service.SettingKeyGlobalIPBlacklist, before)
		} else {
			_ = settings.Delete(ctx, service.SettingKeyGlobalIPBlacklist)
		}
	})
	require.NoError(t, settings.Set(ctx, service.SettingKeyGlobalIPBlacklist, `{"enabled":false,"rules":[]}`))
	failure := errors.New("force rollback after both writes")
	err := settings.WithGlobalIPBlacklistUpdate(ctx, func(opCtx context.Context) error {
		if err := users.Delete(opCtx, user.ID); err != nil {
			return err
		}
		if err := settings.Set(opCtx, service.SettingKeyGlobalIPBlacklist, `{"enabled":true,"rules":["203.0.113.10"]}`); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	_, err = users.GetByID(ctx, user.ID)
	require.NoError(t, err, "The user deletion must roll back")
	value, err := settings.GetValue(ctx, service.SettingKeyGlobalIPBlacklist)
	require.NoError(t, err)
	require.JSONEq(t, `{"enabled":false,"rules":[]}`, value, "The ban must roll back too")
}
