package inspector

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ReplayRequest refires a previously captured request against the local application.
// It reconstructs the identical HTTP method, path, headers, and body, adds the
// "X-GoNgrok-Replayed: true" telemetry header, and returns the response and duration.
func ReplayRequest(record *RequestRecord, localTargetAddr string) (*http.Response, int64, error) {
	if record == nil {
		return nil, 0, fmt.Errorf("cannot replay a nil record")
	}

	// 1. Ensure target URL scheme exists (e.g. "localhost:3000" -> "http://localhost:3000")
	if !strings.HasPrefix(localTargetAddr, "http://") && !strings.HasPrefix(localTargetAddr, "https://") {
		localTargetAddr = "http://" + localTargetAddr
	}

	// Clean trailing slash on target address and leading slash on path to avoid double slashes
	localTargetAddr = strings.TrimRight(localTargetAddr, "/")
	path := record.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	targetURL := fmt.Sprintf("%s%s", localTargetAddr, path)

	// 2. Build body stream from saved request body
	// NOTE (Solving Trap 2 on Replay): Since original network stream was consumed,
	// we turn the saved RequestBody string into a brand-new, re-readable stream
	// using bytes.NewReader!
	bodyReader := bytes.NewReader([]byte(record.RequestBody))

	// 3. Create clone HTTP request
	req, err := http.NewRequest(record.Method, targetURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to reconstruct request: %w", err)
	}

	// 4. Copy original headers (skip Host header to allow Go client to set target host)
	for k, v := range record.Headers {
		if strings.ToLower(k) != "host" {
			req.Header.Set(k, v)
		}
	}

	// Mark request as a replay for local app debugging
	req.Header.Set("X-GoNgrok-Replayed", "true")

	// 5. Measure round-trip execution latency and execute
	startTime := time.Now()
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return nil, duration, fmt.Errorf("replay request failed against %s: %w", targetURL, err)
	}

	return resp, duration, nil
}

/*
================================================================================
 REAL-WORLD EXAMPLE: The 1-Click Webhook Replay
================================================================================

1. THE PROBLEM:
   A customer named John pays $50 on Stripe.
   Stripe sends a webhook to Rahul's tunnel:
   POST /api/stripe-webhook with body {"user":"john@gmail.com", "paid":5000}.
   Rahul's local app on localhost:3000 has a database typo:
   "INSERT INTO paymnet ..." (spelled payment wrong!).
   Rahul's local app crashes with 500 Internal Server Error.

2. RAHUL FIXES THE BUG:
   Rahul fixes the typo in his code and restarts his local app.
   How does he test if the fix works?
   - WITHOUT REPLAY: He has to make another test payment on Stripe or use Postman.
   - WITH REPLAY: Rahul visits http://localhost:4040, sees the red failed
     request, and clicks "REPLAY".

3. WHAT ReplayRequest DOES:
   - Reconstructs the exact clone HTTP request.
   - Restores the original JSON payload and Stripe signature headers.
   - Appends "X-GoNgrok-Replayed: true" marker header.
   - Shoots the request directly to http://localhost:3000/api/stripe-webhook.
   - Rahul's local app processes the payment, returns 200 OK (12ms), and John
     gets his purchase!

4. DIAGRAM:
   [ Dashboard: localhost:4040 ]
                │
                │ (Rahul clicks "Replay" on #req-stripe-123)
                ▼
   [ ReplayRequest() Engine ]
   ├── Stitches Target: http://localhost:3000/api/stripe-webhook
   ├── Copies Body: {"user":"john@gmail.com", "paid":5000}
   ├── Copies Headers: Stripe-Signature, Content-Type
   └── Adds: X-GoNgrok-Replayed: true
                │
                ▼ (Direct HTTP Post)
   [ Rahul's Local App: localhost:3000 ]
   Processes payment and returns 200 OK!
================================================================================
*/


