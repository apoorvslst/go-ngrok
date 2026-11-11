package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestResponseWriterWrapper_CapturesStatusCode verifies that WriteHeader()
// saves the status code into our wrapper instead of losing it.
func TestResponseWriterWrapper_CapturesStatusCode(t *testing.T) {
	// httptest.NewRecorder() gives us a fake ResponseWriter for testing
	// (we can't use a real one without a live HTTP server)
	rec := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(rec)

	rw.WriteHeader(http.StatusNotFound) // send a 404

	if rw.StatusCode != http.StatusNotFound {
		t.Errorf("expected StatusCode 404, got %d", rw.StatusCode)
	}
}

// TestResponseWriterWrapper_CountsBytes verifies that Write()
// correctly accumulates the byte count.
func TestResponseWriterWrapper_CountsBytes(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(rec)

	body := []byte("hello world") // 11 bytes
	rw.Write(body)

	if rw.BytesWritten != 11 {
		t.Errorf("expected BytesWritten 11, got %d", rw.BytesWritten)
	}
}

// TestInstrumentHandler_NoError is a smoke test — just confirms
// InstrumentHandler doesn't panic when called with valid data.
func TestInstrumentHandler_NoError(t *testing.T) {
	InstrumentHandler("rahul", "GET", 45*time.Millisecond, 200, 312, 8420)
	// If we reach this line without panicking, the test passes.
}
