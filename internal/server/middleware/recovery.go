package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer catches any panics in downstream HTTP handlers, logs the stack trace,
// and returns a clean 500 Internal Server Error without crashing the entire server process.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				slog.Error("panic recovered in HTTP handler",
					"error", rvr,
					"path", r.URL.Path,
					"method", r.Method,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

/*
================================================================================
 REAL-WORLD EXAMPLE: The Circus Safety Net (Panic Recovery Middleware)
================================================================================

1. THE PROBLEM (Unprotected Server):
   Imagine our tunnel server is hosting 500 active developer tunnels.
   A visitor sends an HTTP request with a malformed header to Alice's tunnel.
   Due to an unexpected edge case, Alice's handler triggers a nil pointer panic:
   "panic: runtime error: invalid memory address or nil pointer dereference"
   Without a recovery middleware, Go terminates the ENTIRE OS process!
   Alice, Rahul, Gaurav, and all 500 developers are instantly disconnected!

2. THE FIX (The Safety Net):
   The Recoverer middleware wraps every HTTP request with a deferred recover():
   - Visitor requests arrive -> Enters Recoverer -> Calls next.ServeHTTP(w, r).
   - If next.ServeHTTP panics:
     - recover() catches the crash like a circus safety net!
     - The process does NOT terminate.
     - A clean "500 Internal Server Error" is returned to that one bad visitor.
     - The other 499 developers continue running smoothly with zero downtime!

3. DIAGRAM:
   Incoming Visitor Request
             │
             ▼
   [ Recoverer Middleware ]  <─── (Sets up deferred recover() safety net)
             │
             ▼
   [ Downstream Handler / Proxy ]
             │
         Did it PANIC?
         ├── NO  ──► Normal HTTP 200 OK sent to visitor.
         │
         └── YES (Crash!) ──► Caught by recover()!
                              • Logs error + debug.Stack() line number
                              • Sends HTTP 500 to the visitor
                              • Server stays alive! 500 developers stay online!
================================================================================
*/

