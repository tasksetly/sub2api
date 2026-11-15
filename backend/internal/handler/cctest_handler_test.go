//go:build unit

package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type cctestHandlerSettings struct{ service.SettingRepository }

func (*cctestHandlerSettings) GetValue(context.Context, string) (string, error) {
	return `{"enabled":true,"allow_users":true}`, nil
}

type cctestHandlerKeys struct {
	service.APIKeyRepository
	ownerID int64
}

func (r *cctestHandlerKeys) GetByID(context.Context, int64) (*service.APIKey, error) {
	id := int64(3)
	return &service.APIKey{ID: 3, UserID: r.ownerID, Key: "real-user-key-canary", Status: service.StatusActive, GroupID: &id, Group: &service.Group{Status: service.StatusActive}}, nil
}

type cctestHandlerRepo struct {
	service.CCTestRepository
	hash  string
	calls int
}

func (r *cctestHandlerRepo) ConsumeRelay(_ context.Context, hash, model string) (*service.CCTestRun, error) {
	r.calls++
	if hash != r.hash || model != "claude-sonnet-test" {
		return nil, infraerrors.Unauthorized("CCTEST_RELAY_INVALID", "invalid test capability")
	}
	id := int64(3)
	return &service.CCTestRun{OwnerID: 7, APIKeyID: &id}, nil
}

func TestCCTestUserRelayPreservesBillingChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := "cctrelay-" + strings.Repeat("a", 64)
	digest := sha256.Sum256([]byte(token))
	repo := &cctestHandlerRepo{hash: hex.EncodeToString(digest[:])}
	svc := service.NewCCTestService(repo, &cctestHandlerSettings{}, nil, nil, &cctestHandlerKeys{ownerID: 7}, nil)
	t.Cleanup(svc.Stop)
	h := NewCCTestHandler(svc, nil, nil)
	router := gin.New()
	visited := false
	body := `{"model":"claude-sonnet-test","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"probe"}]}`
	router.POST("/cctest/relay/v1/messages", h.Relay, func(c *gin.Context) {
		visited = true
		if c.GetHeader("Authorization") != "Bearer real-user-key-canary" || c.GetHeader("x-api-key") != "" {
			t.Error("scoped credential was not replaced server-side")
		}
		actual, err := io.ReadAll(c.Request.Body)
		if err != nil || string(actual) != body {
			t.Error("gateway request body was changed")
		}
		// The relay must preserve a downstream auth/billing rejection, not answer
		// directly or invoke the unbilled admin forwarding path for a user task.
		c.AbortWithStatus(http.StatusPaymentRequired)
	})
	req := httptest.NewRequest(http.MethodPost, "/cctest/relay/v1/messages", strings.NewReader(body))
	req.Header.Set("x-api-key", token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if !visited || w.Code != http.StatusPaymentRequired || repo.calls != 1 {
		t.Fatal("user test bypassed the downstream billing chain")
	}
	if strings.Contains(w.Body.String(), "real-user-key-canary") {
		t.Fatal("real key leaked to response")
	}
}

func TestCCTestRelayRejectsWrongModelOwnerAndTokenLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, model string
		ownerID     int64
		maxTokens   int
	}{
		{"wrong model", "claude-other", 7, 100},
		{"wrong owner", "claude-sonnet-test", 8, 100},
		{"oversized output", "claude-sonnet-test", 7, 100000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := "cctrelay-" + strings.Repeat("a", 64)
			digest := sha256.Sum256([]byte(token))
			repo := &cctestHandlerRepo{hash: hex.EncodeToString(digest[:])}
			svc := service.NewCCTestService(repo, &cctestHandlerSettings{}, nil, nil, &cctestHandlerKeys{ownerID: tc.ownerID}, nil)
			t.Cleanup(svc.Stop)
			h := NewCCTestHandler(svc, nil, nil)
			router := gin.New()
			router.POST("/cctest/relay/v1/messages", h.Relay, func(c *gin.Context) { t.Error("invalid capability reached billing/generation") })
			body := `{"model":"` + tc.model + `","max_tokens":100,"messages":[]}`
			if tc.maxTokens > 16384 {
				body = strings.Replace(body, ":100,", ":100000,", 1)
			}
			req := httptest.NewRequest(http.MethodPost, "/cctest/relay/v1/messages", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code < 400 {
				t.Fatal("invalid capability accepted")
			}
		})
	}
}
