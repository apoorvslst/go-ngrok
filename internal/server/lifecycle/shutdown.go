package lifecycle

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// WaitForSignal blocks until the process receives an interrupt signal (SIGINT or SIGTERM).
// If a second signal is received while shutting down, it forces immediate termination.
func WaitForSignal() {
	sigChan := make(chan os.Signal, 1)
	// Catch Ctrl+C (Interrupt) or container shutdown (SIGTERM)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Block until the first signal arrives
	sig := <-sigChan
	slog.Info("shutdown signal received, initiating graceful teardown...", "signal", sig.String())

	// Handle a second Ctrl+C for impatient users (force quit immediately)
	go func() {
		secondSig := <-sigChan
		slog.Warn("forced termination requested by second signal", "signal", secondSig.String())
		os.Exit(1)
	}()
}

// ShutdownHTTP cleanly shuts down an active http.Server within the given timeout deadline.
// It stops accepting new incoming TCP connections and waits for active in-flight requests to complete.
func ShutdownHTTP(srv *http.Server, timeout time.Duration) error {
	if srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	slog.Info("draining in-flight HTTP requests...", "timeout", timeout.String())
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful HTTP shutdown failed or timed out", "err", err)
		return err
	}

	slog.Info("HTTP server cleanly stopped with zero dropped requests")
	return nil
}

/*
================================================================================
 REAL-WORLD EXAMPLE: The Polite Restaurant Closing (Graceful Shutdown)
================================================================================

1. THE PROBLEM (Abrupt Crash / kill -9 / unhandled Ctrl+C):
   Rahul's local store app is serving a customer checking out on localhost:3000.
   At that exact second, the server admin hits Ctrl+C to reboot the server.
   Without graceful shutdown, Go immediately terminates the entire OS process!
   The customer's browser crashes with "ERR_CONNECTION_RESET", the payment receipt
   is lost, and Rahul's database is left in a corrupted state!

2. THE FIX (Graceful Teardown Flow):
   WaitForSignal() intercepts the Ctrl+C signal:
   - Step 1: Front door is locked (no NEW visitors can enter).
   - Step 2: ShutdownHTTP gives currently dining customers (in-flight requests)
     a 5-second countdown timer to finish receiving their responses.
   - Step 3: Once in-flight requests finish (or 5s expires), all sockets and
     listeners are politely closed.
   - Step 4: The server exits with code 0. Zero customer errors!

3. ASCII WORKFLOW:
   Admin hits Ctrl+C (SIGINT)
            │
            ▼
   [ WaitForSignal() ] ──► Catches signal! (Prevents instant process crash)
            │
            ├── (If user hits Ctrl+C again) ──► os.Exit(1) [Immediate Emergency Stop]
            │
            ▼
   [ ShutdownHTTP(srv, 5s) ]
            │
            ├── 1. Stops listener from accepting NEW requests
            ├── 2. Waits for active in-flight requests to finish
            └── 3. Deadline hits (5s) or requests complete
                    │
                    ▼
          [ Clean Server Teardown (Code 0) ]
================================================================================
*/

