package lifecycle

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownHTTP_NilServer(t *testing.T) {
	// Should not panic or error if srv is nil
	err := ShutdownHTTP(nil, 1*time.Second)
	if err != nil {
		t.Fatalf("expected nil error for nil server, got %v", err)
	}
}

func TestShutdownHTTP_DrainsActiveRequests(t *testing.T) {
	// 1. Pick a free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on local port: %v", err)
	}

	// 2. Create server with slow handler
	handlerStarted := make(chan struct{})
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(handlerStarted)
			time.Sleep(100 * time.Millisecond) // simulates in-flight work
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("checkout receipt"))
		}),
	}

	go func() {
		_ = srv.Serve(l)
	}()

	// 3. Fire an HTTP request in the background
	clientDone := make(chan bool)
	go func() {
		resp, reqErr := http.Get("http://" + l.Addr().String())
		if reqErr == nil && resp.StatusCode == http.StatusOK {
			clientDone <- true
		} else {
			clientDone <- false
		}
	}()

	// Wait until handler is actively processing
	<-handlerStarted

	// 4. Trigger ShutdownHTTP with 1 second timeout while request is in-flight
	shutdownErr := ShutdownHTTP(srv, 1*time.Second)
	if shutdownErr != nil {
		t.Fatalf("ShutdownHTTP failed: %v", shutdownErr)
	}

	// 5. Verify the client was NOT interrupted and received its receipt!
	select {
	case ok := <-clientDone:
		if !ok {
			t.Errorf("expected client request to succeed during graceful drain")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for client request")
	}
}
