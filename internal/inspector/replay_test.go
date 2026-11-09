package inspector

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReplayRequest_Success(t *testing.T) {
	// Spin up a mock local app (simulating localhost:3000)
	var receivedMethod string
	var receivedPath string
	var receivedBody string
	var receivedHeader string
	var isReplayedHeader string

	mockLocalApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedHeader = r.Header.Get("X-Stripe-Signature")
		isReplayedHeader = r.Header.Get("X-GoNgrok-Replayed")

		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"payment_processed"}`))
	}))
	defer mockLocalApp.Close()

	// Captured record from Stripe
	record := &RequestRecord{
		ID:          "req-stripe-123",
		Method:      "POST",
		Path:        "/api/stripe-webhook",
		Headers:     map[string]string{"X-Stripe-Signature": "sig_secret_123"},
		RequestBody: `{"event":"payment_success","amount":5000}`,
	}

	// Execute Replay
	resp, duration, err := ReplayRequest(record, mockLocalApp.URL)
	if err != nil {
		t.Fatalf("unexpected replay error: %v", err)
	}
	defer resp.Body.Close()

	// Assertions
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if duration < 0 {
		t.Errorf("duration should be >= 0, got %d", duration)
	}
	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedPath != "/api/stripe-webhook" {
		t.Errorf("expected /api/stripe-webhook, got %s", receivedPath)
	}
	if receivedBody != `{"event":"payment_success","amount":5000}` {
		t.Errorf("expected body to match, got %s", receivedBody)
	}
	if receivedHeader != "sig_secret_123" {
		t.Errorf("expected custom header to match, got %s", receivedHeader)
	}
	if isReplayedHeader != "true" {
		t.Errorf("expected X-GoNgrok-Replayed header to be true, got %s", isReplayedHeader)
	}
}

func TestReplayRequest_TargetOffline(t *testing.T) {
	record := &RequestRecord{
		ID:     "req-offline",
		Method: "GET",
		Path:   "/test",
	}

	// Port 59999 is inactive
	_, _, err := ReplayRequest(record, "http://127.0.0.1:59999")
	if err == nil {
		t.Fatalf("expected error when local server is offline, got nil")
	}
}

func TestReplayRequest_NilRecord(t *testing.T) {
	_, _, err := ReplayRequest(nil, "http://localhost:3000")
	if err == nil {
		t.Fatalf("expected error for nil record, got nil")
	}
}

