package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoverer_CatchesPanic(t *testing.T) {
	// A buggy handler that triggers a panic
	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated fatal nil-pointer dereference in proxy")
	})

	// Wrap with our Recoverer middleware
	protectedHandler := Recoverer(panickingHandler)

	// Fire a test request
	req := httptest.NewRequest("GET", "/cause-crash", nil)
	rec := httptest.NewRecorder()

	// This call would kill the test process if Recoverer didn't catch it!
	protectedHandler.ServeHTTP(rec, req)

	// Verify that the visitor receives a clean 500 status code
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 Internal Server Error, got %d", rec.Code)
	}

	// Verify error message in body
	body := rec.Body.String()
	if body != "500 Internal Server Error\n" {
		t.Errorf("expected 500 error body, got %q", body)
	}
}

func TestRecoverer_PassesNormalRequests(t *testing.T) {
	// A normal healthy handler
	healthyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("healthy response"))
	})

	protectedHandler := Recoverer(healthyHandler)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	protectedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "healthy response" {
		t.Errorf("expected 'healthy response', got %q", rec.Body.String())
	}
}

