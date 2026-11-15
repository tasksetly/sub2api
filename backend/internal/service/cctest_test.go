//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type cctestSettingsStub struct{ SettingRepository }

func (*cctestSettingsStub) GetValue(context.Context, string) (string, error) {
	return `{"enabled":true,"allow_users":true,"secret":"encrypted:service-key","public_url":"https://site.example"}`, nil
}

type cctestEncryptorStub struct{}

func (cctestEncryptorStub) Encrypt(value string) (string, error) { return "encrypted:" + value, nil }
func (cctestEncryptorStub) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, "encrypted:") {
		return "", errors.New("invalid ciphertext")
	}
	return strings.TrimPrefix(value, "encrypted:"), nil
}

type cctestRepoStub struct {
	CCTestRepository
	saved *CCTestRun
}

func (r *cctestRepoStub) Update(_ context.Context, run *CCTestRun) error {
	copy := *run
	r.saved = &copy
	return nil
}

type cctestKeyStub struct {
	APIKeyRepository
	key *APIKey
}

func (r *cctestKeyStub) GetByID(context.Context, int64) (*APIKey, error) { return r.key, nil }

type cctestRoundTripper func(*http.Request) (*http.Response, error)

func (f cctestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func cctestWorkerFixture(t *testing.T, reply string) (*CCTestService, *cctestRepoStub, *CCTestRun) {
	t.Helper()
	repo := &cctestRepoStub{}
	svc := NewCCTestService(repo, &cctestSettingsStub{}, cctestEncryptorStub{}, nil, nil, nil)
	t.Cleanup(svc.Stop)
	svc.client.Transport = cctestRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(reply)), Header: make(http.Header)}, nil
	})
	secret, _ := json.Marshal(cctestCredentials{APIKey: "cct-service-canary", RelayToken: "cctrelay-canary"})
	cipher, _ := svc.encryptor.Encrypt(string(secret))
	run := &CCTestRun{ID: "test-run", Status: "running", TaskID: "task-1", Model: "claude-sonnet-test", Credentials: cipher, RelayURL: "https://site.example/cctest/relay", Result: json.RawMessage(`{}`)}
	return svc, repo, run
}

func TestCCTestOfficialSubmissionUsesScopedRelayCredential(t *testing.T) {
	svc, repo, run := cctestWorkerFixture(t, "")
	run.Status = "starting"
	calls := 0
	svc.client.Transport = cctestRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != "https://cctest.ai/api/v1/check" || req.Method != http.MethodPost || req.Header.Get("Authorization") != "Bearer cct-service-canary" {
			t.Fatalf("incorrect official request: %s %s", req.Method, req.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["provider"] != "claude" || body["apiKey"] != "cctrelay-canary" || body["url"] != run.RelayURL || body["model"] != run.Model || body["concurrency"] != float64(1) {
			t.Fatalf("wrong CCTest protocol: %v", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"taskId":"task-2"}`)), Header: make(http.Header)}, nil
	})
	svc.advance(context.Background(), run)
	if calls != 1 || repo.saved == nil || repo.saved.Status != "running" || repo.saved.TaskID != "task-2" {
		t.Fatal("task was not started once")
	}
}

func TestCCTestAmbiguousSubmissionIsTerminal(t *testing.T) {
	svc, repo, run := cctestWorkerFixture(t, "")
	run.Status = "starting"
	calls := 0
	svc.client.Transport = cctestRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("timeout after submission")
	})
	svc.advance(context.Background(), run)
	if calls != 1 || repo.saved == nil || repo.saved.Status != "error" || !strings.Contains(repo.saved.Error, "No automatic resubmission") {
		t.Fatal("ambiguous POST must not be retried")
	}
}

func TestCCTestMissingOrInvalidScoresAreFailures(t *testing.T) {
	for _, reply := range []string{`{"status":"done"}`, `{"status":"done","total":null}`, `{"status":"done","total":-1}`, `{"status":"done","total":101}`, `{"status":"done","total":"100"}`} {
		t.Run(reply, func(t *testing.T) {
			svc, repo, run := cctestWorkerFixture(t, reply)
			svc.advance(context.Background(), run)
			if repo.saved == nil || repo.saved.Status != "error" {
				t.Fatal("invalid score must not be reported as a completed zero score")
			}
		})
	}
}

func TestCCTestZeroScoreIsAValidCompletedReport(t *testing.T) {
	svc, repo, run := cctestWorkerFixture(t, `{"status":"done","total":0,"verdictKey":"not_claude"}`)
	svc.advance(context.Background(), run)
	if repo.saved == nil || repo.saved.Status != "done" {
		t.Fatal("a real zero score must be preserved")
	}
}

func TestCCTestReportsExcludeCredentialsAndRawAuditRows(t *testing.T) {
	svc, repo, run := cctestWorkerFixture(t, `{"status":"done","total":75,"url":"private-url-canary","apiKey":"private-key-canary","responseModel":"cct-service-canary","error":"cctrelay-canary","scores":{"tag_check":10},"tokenAudit":{"overallRatio":1.2,"rows":["raw-audit-canary"]}}`)
	svc.advance(context.Background(), run)
	if repo.saved == nil || repo.saved.Status != "done" {
		t.Fatal("report missing")
	}
	for _, secret := range []string{"private-url-canary", "private-key-canary", "cct-service-canary", "cctrelay-canary", "raw-audit-canary"} {
		if strings.Contains(string(repo.saved.Result), secret) {
			t.Fatalf("report leaked %s", secret)
		}
	}
	if !strings.Contains(string(repo.saved.Result), `"overallRatio":1.2`) {
		t.Fatal("numeric audit summary missing")
	}
}

func TestCCTestUserCannotSubmitAdminTargets(t *testing.T) {
	svc, _, _ := cctestWorkerFixture(t, "")
	_, err := svc.Create(context.Background(), 7, false, CCTestInput{AccountIDs: []int64{9}, APIKeyID: 8, Model: "claude-sonnet-test"})
	if err == nil {
		t.Fatal("user must not target an upstream account")
	}
}

func TestCCTestUserKeyOwnershipAndEligibility(t *testing.T) {
	groupID := int64(1)
	valid := APIKey{ID: 1, UserID: 7, Status: StatusActive, GroupID: &groupID, Group: &Group{Status: StatusActive}}
	for _, tc := range []struct {
		name   string
		mutate func(*APIKey)
	}{
		{"another owner", func(k *APIKey) { k.UserID = 8 }},
		{"disabled key", func(k *APIKey) { k.Status = "disabled" }},
		{"no group", func(k *APIKey) { k.Group = nil }},
		{"exhausted", func(k *APIKey) { k.Quota = 1; k.QuotaUsed = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := valid
			tc.mutate(&key)
			svc := &CCTestService{keys: &cctestKeyStub{key: &key}}
			if _, err := svc.userKey(context.Background(), 1, 7); err == nil {
				t.Fatal("ineligible user key accepted")
			}
		})
	}
}
