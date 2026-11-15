//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGlobalIPBlacklistTransactionLockWaitHonorsCancellation(t *testing.T) {
	client := testEntClient(t)
	repo := &settingRepository{client: client}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	defer finish()
	first := make(chan error, 1)
	go func() {
		first <- repo.WithGlobalIPBlacklistUpdate(ctx, func(opCtx context.Context) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-opCtx.Done():
				return opCtx.Err()
			}
		})
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("first transaction did not acquire the blacklist lock")
	}
	waitCtx, cancelWait := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancelWait()
	second := make(chan error, 1)
	called := make(chan struct{}, 1)
	go func() {
		second <- repo.WithGlobalIPBlacklistUpdate(waitCtx, func(context.Context) error {
			called <- struct{}{}
			return nil
		})
	}()
	// The first transaction remains open until the queued writer has returned.
	// A process-wide sync.Mutex before the SQL lock would ignore this deadline.
	select {
	case err := <-second:
		require.Error(t, err)
		require.ErrorIs(t, waitCtx.Err(), context.DeadlineExceeded)
	case <-time.After(3 * time.Second):
		t.Fatal("blacklist transaction lock ignored cancellation")
	}
	require.Empty(t, called, "A canceled writer must not execute its callback")
	finish()
	select {
	case err := <-first:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("first transaction did not finish")
	}
	// Cancellation of a waiter must leave the next transaction able to proceed.
	require.NoError(t, repo.WithGlobalIPBlacklistUpdate(ctx, func(context.Context) error { return nil }))
}
