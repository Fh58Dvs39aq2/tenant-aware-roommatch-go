package roommatch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientEnvelopeAndRetry(t *testing.T) {
	tests := []struct {
		name      string
		responses []string
		statuses  []int
		wantCode  string
		wantCalls int
	}{
		{name: "returns business rejection from envelope", responses: []string{`{"ok":false,"data":null,"error":{"code":"policy","message":"room denied"},"metadata":{}}`}, statuses: []int{422}, wantCode: "policy", wantCalls: 1},
		{name: "retries rate limit with the same write identity", responses: []string{`{"ok":false,"data":null,"error":{"code":"rate","message":"try later"},"metadata":{}}`, `{"ok":true,"data":{"id":"channel-1"},"error":null,"metadata":{}}`}, statuses: []int{429, 200}, wantCalls: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s", r.Method)
				}
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("authorization missing")
				}
				if r.Header.Get("Idempotency-Key") != "match-1:channel" {
					t.Errorf("idempotency key changed")
				}
				index := calls
				calls++
				w.WriteHeader(tt.statuses[index])
				_, _ = w.Write([]byte(tt.responses[index]))
			}))
			defer server.Close()
			client := NewClient("test-key")
			client.BaseURL = server.URL
			client.Sleep = func(context.Context, time.Duration) error { return nil }
			var output map[string]any
			err := client.call(context.Background(), http.MethodPost, "/v1/realtime/channel/create", map[string]any{"channel": "tenant-acme-match-1"}, "match-1:channel", &output)
			var apiErr *APIError
			gotCode := ""
			if errors.As(err, &apiErr) {
				gotCode = apiErr.Code
			}
			if gotCode != tt.wantCode {
				t.Fatalf("error code = %q, want %q", gotCode, tt.wantCode)
			}
			if calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, tt.wantCalls)
			}
		})
	}
}
