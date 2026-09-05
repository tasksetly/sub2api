//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostRawJSONMeasuresFirstTextDelta(t *testing.T) {
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"4\"}}]}\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(120 * time.Millisecond)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(srv.Close)

	_, _, firstTokenLatency, err := postRawJSON(context.Background(), srv.URL, []byte(`{}`), nil, func(line string) bool {
		return strings.Contains(line, `"content":"4"`)
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstTokenLatency == nil {
		t.Fatal("expected a first-token latency")
	}
	if *firstTokenLatency >= 100*time.Millisecond {
		t.Fatalf("first-token latency = %s; should not include the delayed stream tail", *firstTokenLatency)
	}
}

func TestExtractMonitorStreamText(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		apiMode  string
		adapter  providerAdapter
		body     string
		want     string
	}{
		{
			name:     "openai chat deltas",
			provider: MonitorProviderOpenAI,
			apiMode:  MonitorAPIModeChatCompletions,
			adapter:  providerOpenAIChatAdapter,
			body: "data: {\"choices\":[{\"delta\":{\"content\":\"4\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"2\"}}]}\n\n" +
				"data: [DONE]\n\n",
			want: "42",
		},
		{
			name:     "responses only accepts output text deltas",
			provider: MonitorProviderOpenAI,
			apiMode:  MonitorAPIModeResponses,
			adapter:  providerOpenAIResponsesAdapter,
			body: "event: response.reasoning_summary_text.delta\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"ignore\"}\n\n" +
				"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"42\"}\n\n",
			want: "42",
		},
		{
			name:     "anthropic text delta",
			provider: MonitorProviderAnthropic,
			adapter:  providerAdapters[MonitorProviderAnthropic],
			body:     "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"42\"}}\n\n",
			want:     "42",
		},
		{
			name:     "gemini stream candidate",
			provider: MonitorProviderGemini,
			adapter:  providerAdapters[MonitorProviderGemini],
			body:     "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"42\"}]}}]}\n\n",
			want:     "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractMonitorStreamText(tt.provider, tt.apiMode, tt.adapter, []byte(tt.body)); got != tt.want {
				t.Fatalf("extractMonitorStreamText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildRequestBody_ForcesStreaming(t *testing.T) {
	body, err := buildRequestBody(providerOpenAIChatAdapter, MonitorProviderOpenAI, MonitorAPIModeChatCompletions, "gpt-test", "hello", &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride: map[string]any{
			"messages": []any{map[string]any{"role": "user", "content": "hello"}},
			"stream":   false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"stream":true`) {
		t.Fatalf("replace request must force stream=true, got %s", body)
	}
}
