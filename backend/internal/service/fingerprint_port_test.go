package service

import "testing"

func TestDefaultFingerprintPlanUsesSpecDefaults(t *testing.T) {
	p := DefaultFingerprintPlan()
	if p.CronExpression != "0 */6 * * *" {
		t.Fatalf("cron = %q, want %q", p.CronExpression, "0 */6 * * *")
	}
	if p.ChallengeCount != 3 {
		t.Fatalf("challenge count = %d, want 3", p.ChallengeCount)
	}
	if p.MaxResults != 50 {
		t.Fatalf("max results = %d, want 50", p.MaxResults)
	}
	if p.MismatchActionThreshold != 2 {
		t.Fatalf("threshold = %d, want 2", p.MismatchActionThreshold)
	}
	if !p.Enabled {
		t.Fatal("plan should default to enabled")
	}
	if p.API != string(FingerprintAPIChatCompletions) {
		t.Fatalf("api = %q, want %q", p.API, FingerprintAPIChatCompletions)
	}
}

func TestDefaultFingerprintPlanKeepsFailureActionsOff(t *testing.T) {
	p := DefaultFingerprintPlan()
	if p.FailureDisable {
		t.Fatal("failure_disable must default to off")
	}
	if p.FailureTempUnschedulableMinutes != 0 {
		t.Fatalf("failure_temp_unschedulable_minutes = %d, want 0", p.FailureTempUnschedulableMinutes)
	}
	if len(p.FailureRemoveGroupIDs) != 0 {
		t.Fatalf("failure_remove_group_ids = %v, want empty", p.FailureRemoveGroupIDs)
	}
	if len(p.SuccessAddGroupIDs) != 0 {
		t.Fatalf("success_add_group_ids = %v, want empty", p.SuccessAddGroupIDs)
	}
}

func TestFingerprintVerdictConstantsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range []string{
		FingerprintVerdictMatch,
		FingerprintVerdictMismatch,
		FingerprintVerdictInconclusive,
		FingerprintVerdictError,
	} {
		if v == "" {
			t.Fatal("verdict constant must not be empty")
		}
		if seen[v] {
			t.Fatalf("duplicate verdict constant %q", v)
		}
		seen[v] = true
	}
}

func TestIsValidFingerprintAPI(t *testing.T) {
	for _, tt := range []struct {
		api  string
		want bool
	}{
		{"responses", true},
		{"chat_completions", true},
		{"messages", true},
		{"", false},
		{"anthropic", false},
		{"Chat_Completions", false},
	} {
		if got := IsValidFingerprintAPI(tt.api); got != tt.want {
			t.Fatalf("IsValidFingerprintAPI(%q) = %v, want %v", tt.api, got, tt.want)
		}
	}
}
