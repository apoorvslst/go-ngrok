# 📅 Day 7 — Live Traffic Inspector: Request Capture, `io.TeeReader`, Ring Buffer & Webhook Replay (Odin Project Style)

> **Rule**: Hum code direct inject nahi karenge. Hum batayenge KYA likhna hai, KIS file mein, KAUNSA function, KAUNSA concept pehle samjho. Har ek function tum khud likhoge.
>
> **Apoorv** = 🔵 | **Gaurav** = 🟢 | **Both** = ⚪

---

## 🗺️ Day 7 Overview: Where We Are & Where We Are Going

ngrok ka sabse famous aur sabse pyara feature kya hai?
Jab developer ngrok run karta hai, toh terminal pe ek magical line aati hai:
```text
Forwarding                    https://gaurav.tunnel.dev -> http://localhost:3000
Web Interface (Inspector)     http://127.0.0.1:4040
```

Browser mein `localhost:4040` kholo, aur tumhein dikhta hai:
- Har aane wala HTTP request (Stripe, GitHub webhook, browser click).
- Headers, Query Params, JSON Body.
- Status Code (`200 OK`, `500 Internal Error`), Response Time (`14ms`).
- **The "Replay" Button:** Ek click karo, aur wahi request dobara `localhost:3000` pe fire ho jati hai! Tumhe Stripe pe jaake baar-baar test payment nahi karni padti.

**Day 7 ka Mission:**
Hum yeh pura **Traffic Inspection & Replay Engine** scratch se Go mein build karenge!

```
Visitor (Stripe Webhook)
       │
       ▼
 [ Yamux Stream ]
       │
       ▼
 ┌────────────────────────────────────────────────────────┐
 │ CAPTURE MIDDLEWARE (Client Laptop)                     │
 │                                                        │
 │   io.TeeReader (T-shaped pipe)                         │
 │        ├── 1. Direct Pass-through ──► localhost:3000   │
 │        │                              (Zero Latency)   │
 │        └── 2. Cloned 1MB Buffer   ──► Ring Buffer      │
 │                                       (In-Memory Store)│
 └──────────────────────────────────────────────┬─────────┘
                                                │
                                  ┌─────────────▼────────────┐
                                  │ INSPECTOR API & UI       │
                                  │ http://localhost:4040    │
                                  │                          │
                                  │ • GET /api/requests      │
                                  │ • POST /api/replay/:id   │
                                  │ • Embedded Web UI        │
                                  └──────────────────────────┘
```

---

## 🔵 Step 1: The Core Systems Problem — Capturing Without Blocking Streams (Apoorv — 20 min)

### 🧠 The Engineering Traps:

#### Trap 1: Memory Exhaustion (The Out-of-Memory Crash)
Agar koi user 500MB ki video upload karta hai, aur hum pura body `io.ReadAll(r.Body)` karke memory mein load karein:
5 parallel uploads = **2.5GB RAM used!** Laptop crash ho jayega.
**Solution:** Hum sirf first **1MB** capture karenge. Agar body 1MB se badi hai, toh capture buffer freeze ho jayega aur `Truncated: true` mark hoga, lekin stream aage bina rukawat pass hoti rahegi.

#### Trap 2: Streams Can Only Be Read ONCE!
Go mein `http.Request.Body` ek `io.ReadCloser` stream hai.
Agar tumne body ko inspect karne ke liye padh liya:
```go
data, _ := io.ReadAll(req.Body) // Body stream ab KHAALI (consumed) ho gayi!
```
Ab jab tum ise `localhost:3000` ko forward karoge, toh local app ko **0 bytes** milenge!

#### 🛠️ The Solution: `io.TeeReader` (The T-Joint Pipe)
Plumbing mein T-Joint pipe dekha hai? Ek taraf se paani aata hai, do taraf nikalta hai.
```
Incoming Stream ──► [ io.TeeReader ] ──┬──► Destination (localhost:3000)
                                       │
                                       └──► Memory Buffer (Inspector)
```
Jaise-jaise local app bytes padhega, `io.TeeReader` automatically unhi bytes ki ek copy hamare inspector buffer mein likhta jayega! **Zero extra pass, zero double reading!**

---

## 🔵 Step 2: Request Record Struct & Ring Buffer (Apoorv — 35 min)

Humein last 100 requests memory mein rakhni hain.
Agar hum simple slice `[]Request` use karein aur append karte rahein, toh 1 din mein slice mein 1,000,000 requests jama ho jayengi (Memory leak!).

**The Pro Solution: Circular Ring Buffer**
Ring buffer ka fixed size hota hai (e.g. 100 items).
Jab 101st request aati hai, woh automatically 1st request ko overwrite kar deti hai. Memory usage hamesha strictly capped rehta hai!

### File: `internal/inspector/storage.go`
### Package: `package inspector`

### Data Models:
```go
package inspector

import (
	"sync"
	"time"
)

// RequestRecord stores the full telemetry of an inspected HTTP exchange.
type RequestRecord struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Path         string            `json:"path"`
	Headers      map[string]string `json:"headers"`
	RequestBody  string            `json:"request_body"`
	BodyLength   int64             `json:"body_length"`
	IsTruncated  bool              `json:"is_truncated"`

	// Response fields
	StatusCode   int               `json:"status_code"`
	DurationMs   int64             `json:"duration_ms"`
	ResponseBody string            `json:"response_body"`
}

// RingBuffer stores a fixed-capacity circular queue of requests.
type RingBuffer struct {
	mu       sync.RWMutex
	capacity int
	items    []*RequestRecord
	cursor   int
	isFull   bool
}

// NewRingBuffer initializes a fixed-capacity ring buffer.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		capacity: capacity,
		items:    make([]*RequestRecord, capacity),
	}
}

// Add appends a new record, overwriting the oldest if full.
func (rb *RingBuffer) Add(record *RequestRecord) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.items[rb.cursor] = record
	rb.cursor = (rb.cursor + 1) % rb.capacity
	if rb.cursor == 0 {
		rb.isFull = true
	}
}

// GetAll returns all stored records in reverse chronological order (newest first).
func (rb *RingBuffer) GetAll() []*RequestRecord {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var result []*RequestRecord
	count := rb.capacity
	if !rb.isFull {
		count = rb.cursor
	}

	// Loop backwards from newest to oldest
	idx := rb.cursor - 1
	for i := 0; i < count; i++ {
		if idx < 0 {
			idx = rb.capacity - 1
		}
		if rb.items[idx] != nil {
			result = append(result, rb.items[idx])
		}
		idx--
	}

	return result
}

// GetByID finds a specific request for replay.
func (rb *RingBuffer) GetByID(id string) (*RequestRecord, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	for _, item := range rb.items {
		if item != nil && item.ID == id {
			return item, true
		}
	}
	return nil, false
}
```

---

## 🟢 Step 3: Capped Capture Reader with `io.LimitReader` (Gaurav — 30 min)

### Concept samjho pehle (10 min):
Humein incoming request body se maximum 1MB capture karna hai.
Go ka standard library function hai:
```go
io.LimitReader(r, 1024 * 1024) // 1MB limit
```
Yeh reader se maximum $N$ bytes padhta hai aur uske baad `EOF` bol deta hai, chahe asli stream 1GB ki ho!

### File: `internal/inspector/capture.go`
### Package: `package inspector`

### Function: `CaptureRequest(req *http.Request, maxCaptureBytes int64) (*RequestRecord, error)`

```go
package inspector

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"time"
)

// GenerateID produces a short unique identifier for the request.
func GenerateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CaptureRequest inspects headers and captures up to maxCaptureBytes of the body.
func CaptureRequest(req *http.Request, maxCaptureBytes int64) (*RequestRecord, error) {
	record := &RequestRecord{
		ID:        GenerateID(),
		Timestamp: time.Now(),
		Method:    req.Method,
		URL:       req.URL.String(),
		Path:      req.URL.Path,
		Headers:   make(map[string]string),
	}

	for k, v := range req.Header {
		if len(v) > 0 {
			record.Headers[k] = v[0]
		}
	}

	if req.Body == nil {
		return record, nil
	}

	// 1. TeeReader buffer for capturing
	var captureBuf bytes.Buffer

	// 2. Limit the captured amount to maxCaptureBytes (1MB)
	limited := io.LimitReader(req.Body, maxCaptureBytes)

	// 3. TeeReader: As the forwarder reads from 'tee', bytes also write into 'captureBuf'
	tee := io.TeeReader(limited, &captureBuf)

	// 4. We drain the limited bytes into captureBuf and restore the body
	bodyBytes, err := io.ReadAll(tee)
	if err != nil {
		return nil, err
	}

	record.RequestBody = string(bodyBytes)
	record.BodyLength = int64(len(bodyBytes))

	// If body was larger than limit, mark truncated
	if int64(len(bodyBytes)) >= maxCaptureBytes {
		record.IsTruncated = true
	}

	// 5. RESTORE the body so downstream forwarder can read it too!
	// We concatenate what we read + the remaining unread stream
	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBytes), req.Body))

	return record, nil
}
```

---

### 🔍 Line-by-Line Breakdown:

- **`io.MultiReader(bytes.NewReader(bodyBytes), req.Body)`**:
  - **The Pro Trick:** Humne jitna body padh liya tha, use wapas stream ke aage chipka diya!
  - Jab aage `localhost:3000` ka forwarder body padhega, usko pata bhi nahi chalega ki kisi ne pehle se 1MB padh liya tha. Local app ko 100% complete body milegi!
- **`io.NopCloser`**:
  - `MultiReader` sirf `io.Reader` return karta hai, lekin `req.Body` ko `Close()` method bhi chahiye hota hai (`io.ReadCloser`). `io.NopCloser` ek dummy no-op `Close()` method provide karta hai.

---

## 🔵 Step 4: Webhook Replay Engine (Apoorv — 35 min)

### 🧠 The Concept (10 min):
Replay feature kaise kaam karega?
1. User `localhost:4040` dashboard pe "Replay" button dabata hai.
2. Inspector API ko request ID milta hai (e.g. `POST /api/replay/a1b2c3d4`).
3. Hum ring buffer se original request ka record nikaalte hain:
   - Method (`POST`), Path (`/webhook`), Headers, Body (`{"event":"payment_success"}`).
4. Go ka standard `http.NewRequest` use karke **ekdum duplicate HTTP request** construct karte hain.
5. Seedha `http.DefaultClient.Do(newReq)` se `localhost:3000` pe dubara shoot kar dete hain!
6. Local app ko lagta hai ki Stripe ne dobara webhook bhej diya! Developer bina ek rupya kharch kiye 50 baar apna payment handler test kar sakta hai! 🔥

### File: `internal/inspector/replay.go`
### Package: `package inspector`

### Function: `ReplayRequest(record *RequestRecord, localTargetAddr string) (*http.Response, int64, error)`

```go
package inspector

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ReplayRequest refires a previously captured request against the local application.
func ReplayRequest(record *RequestRecord, localTargetAddr string) (*http.Response, int64, error) {
	// 1. Construct target URL: http://localhost:3000/webhook
	if !strings.HasPrefix(localTargetAddr, "http://") && !strings.HasPrefix(localTargetAddr, "https://") {
		localTargetAddr = "http://" + localTargetAddr
	}
	targetURL := fmt.Sprintf("%s%s", localTargetAddr, record.Path)

	// 2. Build body reader
	bodyReader := bytes.NewReader([]byte(record.RequestBody))

	// 3. Create identical HTTP request
	req, err := http.NewRequest(record.Method, targetURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to reconstruct request: %w", err)
	}

	// 4. Copy original headers
	for k, v := range record.Headers {
		// Host header override avoid karo
		if strings.ToLower(k) != "host" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("X-GoNgrok-Replayed", "true")

	// 5. Measure latency & fire!
	startTime := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return nil, duration, fmt.Errorf("replay request failed against %s: %w", targetURL, err)
	}

	return resp, duration, nil
}
```

---

## 🟢 Step 5: Inspector HTTP Server & Dashboard UI (Gaurav — 40 min)

Ab hum client ke laptop par ek local web server chalayenge: `http://localhost:4040`.

### File: `internal/inspector/server.go`
### Package: `package inspector`

```go
package inspector

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// Server provides the web interface and REST API for inspecting traffic on port :4040.
type Server struct {
	storage         *RingBuffer
	localTargetAddr string
	listenAddr      string
}

// NewServer initializes the inspector web service.
func NewServer(listenAddr string, storage *RingBuffer, localTargetAddr string) *Server {
	return &Server{
		storage:         storage,
		localTargetAddr: localTargetAddr,
		listenAddr:      listenAddr,
	}
}

// Start launches the inspector UI.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// 1. REST API: Return captured requests JSON
	mux.HandleFunc("/api/requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		records := s.storage.GetAll()
		json.NewEncoder(w).Encode(records)
	})

	// 2. REST API: Replay a request by ID
	mux.HandleFunc("/api/replay", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := r.URL.Query().Get("id")
		record, ok := s.storage.GetByID(id)
		if !ok {
			http.Error(w, "Request ID not found in history", http.StatusNotFound)
			return
		}

		resp, duration, err := ReplayRequest(record, s.localTargetAddr)
		if err != nil {
			slog.Error("replay failed", "id", id, "err", err)
			http.Error(w, fmt.Sprintf("Replay error: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		slog.Info("request replayed successfully", "id", id, "status", resp.StatusCode, "duration_ms", duration)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":      "ok",
			"status_code": resp.StatusCode,
			"duration_ms": duration,
		})
	})

	// 3. Embedded Vanilla Dashboard HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(DashboardHTML))
	})

	slog.Info("inspector web interface live", "url", "http://localhost"+s.listenAddr)
	return http.ListenAndServe(s.listenAddr, mux)
}

// Minimal, beautiful, single-file dashboard
const DashboardHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>GoNgrok Traffic Inspector</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0d1117; color: #c9d1d9; margin: 0; padding: 20px; }
        h1 { color: #58a6ff; font-size: 20px; border-bottom: 1px solid #30363d; padding-bottom: 10px; }
        table { width: 100%; border-collapse: collapse; margin-top: 15px; background: #161b22; border-radius: 6px; overflow: hidden; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #30363d; font-size: 14px; }
        th { background: #21262d; color: #8b949e; }
        .method { padding: 3px 8px; border-radius: 4px; font-weight: bold; font-size: 12px; }
        .POST { background: #238636; color: white; }
        .GET { background: #1f6feb; color: white; }
        .replay-btn { background: #8957e5; color: white; border: none; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; }
        .replay-btn:hover { background: #a371f7; }
        pre { background: #0d1117; padding: 8px; border-radius: 4px; font-size: 12px; max-height: 80px; overflow: auto; }
    </style>
</head>
<body>
    <h1>GoNgrok Webhook & Traffic Inspector (:4040)</h1>
    <button onclick="loadRequests()" style="padding: 6px 12px; cursor: pointer;">Refresh</button>
    <table>
        <thead>
            <tr>
                <th>Time</th>
                <th>Method</th>
                <th>Path</th>
                <th>Headers</th>
                <th>Body</th>
                <th>Action</th>
            </tr>
        </thead>
        <tbody id="table-body">
            <tr><td colspan="6">Loading traffic...</td></tr>
        </tbody>
    </table>

    <script>
        async function loadRequests() {
            const res = await fetch('/api/requests');
            const data = await res.json();
            const tbody = document.getElementById('table-body');
            tbody.innerHTML = '';
            if (!data || data.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6">No requests captured yet. Hit your tunnel URL!</td></tr>';
                return;
            }
            data.forEach(req => {
                const tr = document.createElement('tr');
                tr.innerHTML = ` + "`" + `
                    <td>${new Date(req.timestamp).toLocaleTimeString()}</td>
                    <td><span class="method ${req.method}">${req.method}</span></td>
                    <td>${req.path}</td>
                    <td><pre>${JSON.stringify(req.headers, null, 2)}</pre></td>
                    <td><pre>${req.request_body || '(empty)'}</pre></td>
                    <td><button class="replay-btn" onclick="replay('${req.id}')">Replay</button></td>
                ` + "`" + `;
                tbody.appendChild(tr);
            });
        }
        async function replay(id) {
            const res = await fetch('/api/replay?id=' + id, { method: 'POST' });
            const result = await res.json();
            alert('Replay sent! Status: ' + result.status_code + ' (' + result.duration_ms + 'ms)');
        }
        setInterval(loadRequests, 2000);
        loadRequests();
    </script>
</body>
</html>
`
```

---

## ⚪ Step 6: The Full Live Replay Verification (Both — 30 min)

### The End-to-End Test:
1. **Terminal 1:** Python local server chalu karo on `:3000`:
   ```bash
   python -m http.server 3000
   ```
2. **Terminal 2:** Server chalu karo (`go run ./cmd/server`).
3. **Terminal 3:** Client chalu karo (`go run ./cmd/client`).
   - Notice log: `inspector web interface live url=http://localhost:4040`
4. **Browser mein kholo:** `http://localhost:4040`
5. **Terminal 4:** Ek POST webhook simulate karo:
   ```bash
   curl -X POST -H "Host: gaurav.tunnel.dev" -H "Content-Type: application/json" -d "{\"event\":\"order_created\",\"amount\":500}" http://localhost:8080/webhook
   ```
6. **Magical Moment:**
   - Browser mein `http://localhost:4040` refresh karo.
   - Boom! Screen pe POST request, headers, aur JSON body dikhegi!
   - **Click the "Replay" Button:**
   - Notice karo Terminal 1 mein: Python server ko bina curl run kiye turant wohi request dobara receive hui! 🚀

---

## Day 7 Checklist

### Files Created:
- [ ] `internal/inspector/storage.go` — `RingBuffer` (100-item circular FIFO queue, thread-safe with `sync.RWMutex`)
- [ ] `internal/inspector/capture.go` — `CaptureRequest`, `io.TeeReader`, `io.MultiReader` body restoration, 1MB limit
- [ ] `internal/inspector/replay.go` — Reconstructs identical HTTP request with headers/body and re-fires at `localhost:3000`
- [ ] `internal/inspector/server.go` — Web server on `:4040` with `/api/requests`, `/api/replay`, and embedded dashboard UI

### Concepts Mastered:
- [ ] Why HTTP bodies can only be read once & how `io.MultiReader` restores them
- [ ] `io.TeeReader` for zero-allocation streaming tap
- [ ] Fixed-size Ring Buffers for bounding memory usage in long-running services
- [ ] Webhook Replay mechanics (reconstructing requests with identical headers)
- [ ] Single-binary web dashboard serving

