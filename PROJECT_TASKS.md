# 🚇 GoNgrok — Master Step-by-Step Task Roadmap (Odin Project Style)

> **Authors**: Apoorv (🔵) & Gaurav (🟢) | Both (⚪)  
> **Philosophy**: Master-level backend engineering built line by line. Every single function is understood, typed by hand, tested for race conditions, and designed for real-world production.  
> **Rule**: Concepts first $\rightarrow$ Mental model $\rightarrow$ Exact blueprints & signatures $\rightarrow$ Line-by-line logic $\rightarrow$ Verification & Edge cases.

---

## 🗺️ Project Architecture & Component Division

```
                  ┌───────────────────────────────────────────────┐
                  │                 PUBLIC INTERNET               │
                  └───────────────────────┬───────────────────────┘
                                          │ Visitor Request (e.g. Host: rahul.tunnel.dev)
                                          ▼
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ SERVER (VPS)                                                                           │
│                                                                                        │
│   🔵 [Step 14] Public HTTP Listener (:8080)                                            │
│        │                                                                               │
│        ▼                                                                               │
│   🔵 [Step 13] In-Memory Registry (subdomain -> *yamux.Session, sync.RWMutex)           │
│        │                                                                               │
│        ▼                                                                               │
│   🔵 [Step 15] ReverseProxy with Yamux DialContext Trick                               │
│        │                                                                               │
│        │ Opens new Yamux Stream per request                                            │
│        ▼                                                                               │
│   🟢 [Step 7-10] Control Server (:9000)                                                │
│        ▲   (TCP + Handshake Frame + Yamux Session Server + Heartbeat)                  │
└────────┼───────────────────────────────────────────────────────────────────────────────┘
         │
         │ Single Outbound TCP Connection (TLS / Yamux Multiplexed)
         │
┌────────┼───────────────────────────────────────────────────────────────────────────────┐
│ CLIENT (Laptop)                                                                        │
│                                                                                        │
│   🟢 [Step 8-11] Client Connector & Reconnect Loop (Exponential Backoff + Jitter)      │
│        │                                                                               │
│        ▼                                                                               │
│   🟢 [Step 16] Yamux Stream Acceptor (Accepts streams from Server)                     │
│        │                                                                               │
│        ▼                                                                               │
│   🟢 [Step 17] Capture Middleware (TeeReader / Capped Buffer for Inspector)            │
│        │                                                                               │
│        ▼                                                                               │
│   🔵 [Step 18] Local Reverse Proxy (Forwards to localhost:3000)                        │
│        │                                                                               │
│        ▼                                                                               │
│   🔵 [Step 19-20] Inspector Server & UI (:4040) + Replay Engine                        │
│                                                                                        │
│   ⚪ [Step 21-25] Postgres Auth, Rate Limiter, Metrics, Docker & Load Tests            │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 📅 Phase 1: Networking & Concurrency Foundations (Days 1–2)

### ⚪ Step 0–4: Skeleton & Hello Worlds *(Completed)*
- `go.mod` module path: `github.com/devGPP23/go-ngrok`
- `cmd/server/main.go` and `cmd/client/main.go` executables verified.

---

### 🔵 Step 5: TCP Listener (`internal/server/listener.go`) *(Completed)*
- **Concept**: OS Socket binding, TCP backlog queue, `net.Listen("tcp", addr)`, blocking `Accept()` loop.
- **Function**: `StartListener(addr string) error`

---

### 🟢 Step 6: TCP Connector (`internal/client/connector.go`) *(Completed)*
- **Concept**: Outbound dialing through NAT, `net.Dial("tcp", addr)`, ephemeral client ports.
- **Function**: `Connect(serverAddr string) (net.Conn, error)`

---

### 🔵 Step 7: Server Echo & Goroutine Dispatch (`internal/server/listener.go`)
- **Concept**: Why synchronous socket reading blocks all clients. The M:N Go scheduler. Spinning off 2KB goroutines per connection.
- **Function**: `handleConnection(conn net.Conn)`
- **Key Logic**:
  ```go
  defer conn.Close()
  io.Copy(conn, conn) // Read from socket and write directly back
  ```
- **In Loop**: Replace `conn.Close()` with `go handleConnection(conn)`.
- **Verify**: Server handles multiple simultaneous clients without freezing.

---

### 🟢 Step 8: Client Bidirectional Echo (`internal/client/connector.go`)
- **Concept**: Full-duplex I/O. Decoupling stdin (keyboard) and stdout (screen) with two concurrent goroutines.
- **Function**: `HandleEcho(conn net.Conn)`
- **Key Logic**:
  - Goroutine 1: `go io.Copy(conn, os.Stdin)`
  - Main thread: `io.Copy(os.Stdout, conn)`
- **Verify**: Type in client terminal $\rightarrow$ Server echoes back instantaneously.

---

### 🔵 Step 9: Graceful Shutdown (`internal/server/listener.go` & `cmd/server/main.go`)
- **Concept**: Eliminating brutal connection cuts (`SIGINT`/`SIGTERM`). Draining in-flight requests.
- **Key Tools**:
  1. `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`
  2. `sync.WaitGroup` to track active connections.
  3. Closing listener to unblock `Accept()` while waiting on `wg.Wait()`.
- **Signature**: `StartListener(ctx context.Context, addr string) error`
- **Verify**: Press `Ctrl+C` on server while client is actively chatting. Server halts new connects, finishes active client, and exits cleanly.

---

### 🟢 Step 10: Structured Logging with `log/slog`
- **Concept**: Why `fmt.Println` is banned in modern backend production. Key-value pairs, zero-allocation log handlers, JSON formatters for Datadog/ELK.
- **Files**: `internal/server/listener.go` & `internal/client/connector.go`
- **Key Logic**:
  ```go
  slog.Info("connection accepted", "remote_addr", conn.RemoteAddr().String())
  slog.Error("failed to accept", "err", err)
  ```
- **Verify**: All logs output structured format with timestamp and severity level.

---

## 📅 Phase 2: The Control Handshake & Framing (Days 3–4)

### 🧠 The Core Problem:
Before streaming HTTP, how does the client identify itself? How does the server know what subdomain the client wants (`rahul`? `apoorv`?) and whether their token is valid? We need an **application-layer handshake protocol**.

```
Client ──────────────────── TCP ───────────────────► Server
       ─── { Type: "HELLO", Token: "...", Subdomain: "rahul" } ──►
       ◄── { Type: "AUTH_OK", URL: "rahul.tunnel.dev" } ──────────
```

---

### 🟢 Step 11: Protocol Framing & Message Types (`internal/protocol/protocol.go`)
- **Concept**: Stream protocols vs message protocols. TCP has no message boundaries (packets can merge or fragment). We define JSON-based control frames with explicit types.
- **Package**: `package protocol`
- **Structs to Define**:
  ```go
  type MessageType string
  const (
      MsgHello   MessageType = "HELLO"
      MsgAuthOK  MessageType = "AUTH_OK"
      MsgAuthErr MessageType = "AUTH_ERR"
      MsgPing    MessageType = "PING"
      MsgPong    MessageType = "PONG"
  )

  type HelloMessage struct {
      Type             MessageType `json:"type"`
      Token            string      `json:"token"`
      DesiredSubdomain string      `json:"desired_subdomain"`
      Version          string      `json:"version"`
  }

  type AuthResponseMessage struct {
      Type         MessageType `json:"type"`
      Success      bool        `json:"success"`
      AssignedURL  string      `json:"assigned_url,omitempty"`
      ErrorMessage string      `json:"error_message,omitempty"`
  }
  ```
- **Functions**:
  1. `WriteMsg(conn net.Conn, msg any) error` — Encodes JSON and writes with newline delimiter (`json.NewEncoder`).
  2. `ReadMsg(conn net.Conn, target any) error` — Decodes JSON frame (`json.NewDecoder`).
- **Verify**: Write unit test `internal/protocol/protocol_test.go` verifying encode $\rightarrow$ decode roundtrip.

---

### 🔵 Step 12: Server Handshake Verifier (`internal/server/handshake.go`)
- **Concept**: Synchronous handshake validation before elevating socket to a tunnel session.
- **Package**: `package server`
- **Function**: `PerformHandshake(conn net.Conn) (*protocol.HelloMessage, error)`
- **Logic**:
  1. Set read deadline on connection: `conn.SetReadDeadline(time.Now().Add(5 * time.Second))` (Prevents slowloris attacks hanging server threads).
  2. Read `HelloMessage` via `protocol.ReadMsg`.
  3. Validate token & desired subdomain (sanity regex, non-empty).
  4. Reset deadline: `conn.SetReadDeadline(time.Time{})`.
  5. Respond with `AuthResponseMessage{Success: true, AssignedURL: ...}`.
- **Verify**: Client sends malformed JSON or times out $\rightarrow$ Server safely aborts without hanging.

---

## 📅 Phase 3: Multiplexing with Yamux (Days 5–6)

### 🧠 The Core Problem:
In a naive tunnel, 1 visitor request = 1 new TCP connection from laptop to server. But establishing TCP connections takes 3 RTTs, and laptops behind NAT cannot receive inbound TCP connections!  
**Solution: Yamux (Yet Another Multiplexer)**. One long-lived TCP pipe carries 10,000 independent virtual streams (similar to HTTP/2 multiplexing).

---

### 🔵 Step 13: Server Yamux Session Manager (`internal/server/session.go`)
- **Concept**: Turning a raw `net.Conn` into a multiplexed session server.
- **Package**: `package server`
- **Dependency**: `github.com/hashicorp/yamux`
- **Function**: `InitYamuxServer(conn net.Conn) (*yamux.Session, error)`
- **Key Logic**:
  ```go
  cfg := yamux.DefaultConfig()
  cfg.EnableKeepAlive = true
  cfg.KeepAliveInterval = 15 * time.Second
  session, err := yamux.Server(conn, cfg)
  ```
- **Verify**: Inspect yamux keep-alive packets flowing in background.

---

### 🟢 Step 14: Client Yamux Session & Stream Loop (`internal/client/session.go`)
- **Concept**: The client operates as a yamux client (`yamux.Client`), waiting to accept incoming streams initiated by the server.
- **Package**: `package client`
- **Function**: `InitYamuxClient(conn net.Conn) (*yamux.Session, error)`
- **Function**: `ListenStreams(session *yamux.Session)`
- **Key Logic**:
  ```go
  for {
      stream, err := session.AcceptStream() // Blocks until server forwards a request
      if err != nil {
          slog.Error("session closed", "err", err)
          return
      }
      go handleIncomingStream(stream)
  }
  ```
- **Verify**: Server opens 50 concurrent streams (`session.OpenStream()`), client accepts all 50 without drops.

---

## 📅 Phase 4: In-Memory Registry & Routing (Days 7–8)

### 🧠 The Core Problem:
When `visitor` hits `https://rahul.tunnel.dev/api/data`, the server must know:
1. Which laptop owns subdomain `rahul`?
2. How to quickly look up the active `yamux.Session`?
3. How to handle concurrent reads (millions of requests) vs writes (connect/disconnect)?

---

### 🔵 Step 15: Thread-Safe Tunnel Registry (`internal/server/registry/registry.go`)
- **Concept**: High-concurrency reads with `sync.RWMutex`.
- **Package**: `package registry`
- **Struct Blueprint**:
  ```go
  type Tunnel struct {
      Subdomain   string
      Session     *yamux.Session
      CreatedAt   time.Time
      ClientAddr  string
  }

  type Registry struct {
      mu      sync.RWMutex
      tunnels map[string]*Tunnel
  }
  ```
- **Methods**:
  1. `Register(subdomain string, session *yamux.Session, addr string) error`
  2. `Get(subdomain string) (*Tunnel, bool)`
  3. `Delete(subdomain string)`
- **Verify**: Run `go test -race ./internal/server/registry/...` with 1,000 parallel goroutines doing reads and writes.

---

### 🟢 Step 16: Host Header Parser & Subdomain Extractor (`internal/server/router/router.go`)
- **Concept**: Extracting virtual host names from HTTP `Host` header (`rahul.tunnel.dev:8080` $\rightarrow$ `rahul`).
- **Package**: `package router`
- **Function**: `ExtractSubdomain(hostHeader string, rootDomain string) (string, error)`
- **Corner Cases to Handle**:
  - Port numbers in host (`localhost:8080` vs `rahul.domain.com:443`).
  - IP address hits (no subdomain $\rightarrow$ show 404 landing page).
  - Multi-level subdomains.
- **Verify**: Comprehensive table-driven unit tests covering all URL formats.

---

## 📅 Phase 5: End-to-End Reverse Proxying (Days 9–10)

### 🧠 The Core Problem:
How does a visitor's standard HTTP request travel into the server, across a Yamux stream, through the client, and into `localhost:3000`?

---

### 🔵 Step 17: The Server ReverseProxy Yamux Trick (`internal/server/proxy/proxy.go`)
- **Concept**: Standard library `httputil.ReverseProxy` is designed to dial normal TCP sockets. We override its `Transport.DialContext` to return a `yamux.Stream`!
- **Package**: `package proxy`
- **Function**: `NewTunnelReverseProxy(reg *registry.Registry, rootDomain string) http.Handler`
- **Key Logic**:
  ```go
  director := func(req *http.Request) {
      req.URL.Scheme = "http"
      req.URL.Host = req.Host // Preserve Host for local application
  }

  transport := &http.Transport{
      DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
          subdomain, _ := router.ExtractSubdomain(addr, rootDomain)
          tunnel, ok := reg.Get(subdomain)
          if !ok {
              return nil, errors.New("tunnel not found or offline")
          }
          return tunnel.Session.OpenStream() // Opening yamux stream!
      },
  }
  ```
- **Verify**: Calling the HTTP listener triggers an automatic stream creation on the connected client.

---

### 🟢 Step 18: Client Local Forwarder (`internal/client/forwarder/forwarder.go`)
- **Concept**: The client receives a yamux stream (which implements `net.Conn`), dials the local web app (`localhost:3000`), and connects them with bidirectional `io.Copy`.
- **Package**: `package forwarder`
- **Function**: `ForwardStream(stream net.Conn, localTargetAddr string)`
- **Key Logic**:
  ```go
  localConn, err := net.Dial("tcp", localTargetAddr)
  if err != nil {
      // Local app is not running! Return clean 502 Bad Gateway to visitor
      stream.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\nLocal application unreachable."))
      stream.Close()
      return
  }
  defer localConn.Close()
  defer stream.Close()

  // Full duplex pipe
  errChan := make(chan error, 2)
  go func() { _, err := io.Copy(localConn, stream); errChan <- err }()
  go func() { _, err := io.Copy(stream, localConn); errChan <- err }()
  <-errChan
  ```
- **Verify**: Run a dummy Python/Node server on `localhost:3000`. Send request to public server $\rightarrow$ Python server responds with 200 OK!

---

## 📅 Phase 6: Reliability & Inspector UI (Days 11–13)

### 🟢 Step 19: Exponential Backoff Reconnection (`internal/client/reconnect.go`)
- **Concept**: If WiFi drops or laptop closes lid, client must reconnect automatically without hammering the server (Thundering Herd prevention).
- **Package**: `package client`
- **Formula**: $\text{Backoff} = \min(\text{MaxWait}, \text{Base} \times 2^{\text{attempt}}) \pm \text{RandomJitter}$
- **Function**: `WithRetry(ctx context.Context, fn func() error) error`
- **Verify**: Kill server and restart it $\rightarrow$ Client reconnects smoothly within seconds.

---

### 🔵 Step 20: Request Capture & Inspector Middleware (`internal/inspector/capture.go`)
- **Concept**: Non-blocking request interception using `io.TeeReader`. Capturing headers, status code, latency, and bodies up to 1MB without exhausting memory.
- **Package**: `package inspector`
- **Struct**:
  ```go
  type RequestRecord struct {
      ID          string            `json:"id"`
      Method      string            `json:"method"`
      Path        string            `json:"path"`
      Headers     map[string]string `json:"headers"`
      Body        []byte            `json:"body"`
      StatusCode  int               `json:"status_code"`
      DurationMs  int64             `json:"duration_ms"`
      Timestamp   time.Time         `json:"timestamp"`
  }
  ```
- **Storage**: In-memory Ring Buffer (last 100 requests).

---

### ⚪ Step 21: Inspector UI & Replay API (`internal/inspector/server.go`)
- **Concept**: Embedding a live web UI in the Go binary using `//go:embed`.
- **Endpoints**:
  - `GET /` $\rightarrow$ Embedded dashboard HTML/JS.
  - `GET /events` $\rightarrow$ Server-Sent Events (SSE) streaming live requests to browser.
  - `POST /replay/:id` $\rightarrow$ Re-fires captured request to `localhost:3000` with same headers/body!
- **Verify**: Open `http://localhost:4040` in browser. Click replay $\rightarrow$ Local backend receives identical webhook.

---

## 📅 Phase 7: Production Readiness & Deployment (Days 14–15)

### 🔵 Step 22: PostgreSQL Auth & Durable Subdomains (`internal/store/postgres.go`)
- **Concept**: Hashed API token verification (`SHA-256`) and checking registered subdomains.
- **Table Schema**: `tokens`, `subdomains`, `tunnel_sessions`.

---

### 🟢 Step 23: Per-Tunnel Stream Rate Limiter & Concurrency Capping
- **Concept**: Preventing one client or abuser from opening 50,000 streams and starving the server.
- **Pattern**: Buffered channel as a counting semaphore: `chan struct{}` of capacity `N` (e.g. 500).

---

### ⚪ Step 24: Prometheus Metrics & `pprof` Profiling
- **Endpoints**: `/metrics` & `/debug/pprof`
- **Tracked Metrics**:
  - `active_tunnels_count` (Gauge)
  - `total_requests_proxied` (Counter)
  - `stream_duration_seconds` (Histogram)

---

### ⚪ Step 25: Docker Compose, Caddy Wildcard TLS & Load Testing
- **Docker Compose**: Spins up Server + Postgres + Caddy Reverse Proxy.
- **Load Test**: Run `k6` or `vegeta` targeting 1,000 concurrent requests across tunnels.
- **Race Detection**: Verify `go test -race ./...` produces 0 data races.

---

## 🏆 Summary Checklist: Who Owns What

| Step # | Module / Feature | Owner | Core Learning |
|:---:|---|:---:|---|
| **0–4** | Skeleton, Mod, Entrypoints | ⚪ Both | Modules, Project Layout |
| **5** | TCP Server Listener | 🔵 Apoorv | Sockets, `net.Listen`, `Accept()` |
| **6** | TCP Client Connector | 🟢 Gaurav | Sockets, `net.Dial`, ephemeral ports |
| **7** | Server Echo & Goroutines | 🔵 Apoorv | M:N Scheduler, `io.Copy`, goroutines |
| **8** | Client Full-Duplex Echo | 🟢 Gaurav | `os.Stdin`, `os.Stdout`, Goroutines |
| **9** | Graceful Shutdown | 🔵 Apoorv | `signal.NotifyContext`, `sync.WaitGroup` |
| **10** | Structured Logging | 🟢 Gaurav | `log/slog`, JSON Handler, Context |
| **11** | Protocol Framing & Types | 🟢 Gaurav | TCP framing, JSON encoding |
| **12** | Handshake Verifier | 🔵 Apoorv | Deadlines, Slowloris prevention |
| **13** | Yamux Server Session | 🔵 Apoorv | Multiplexing, Keep-alives |
| **14** | Yamux Client Stream Loop | 🟢 Gaurav | Virtual streams, stream acceptance |
| **15** | In-Memory Tunnel Registry | 🔵 Apoorv | `sync.RWMutex`, Concurrency safety |
| **16** | Host Header Subdomain Router | 🟢 Gaurav | Virtual hosts, URL parsing |
| **17** | ReverseProxy Yamux Trick | 🔵 Apoorv | `httputil.ReverseProxy`, `DialContext` |
| **18** | Client Local Port Forwarder | 🟢 Gaurav | Localhost tunneling, 502 handling |
| **19** | Reconnect with Jitter | 🟢 Gaurav | Exponential backoff, Thundering herd |
| **20** | Inspector Capture Middleware | 🔵 Apoorv | `io.TeeReader`, Ring buffers |
| **21** | Inspector UI & Webhook Replay | ⚪ Both | `//go:embed`, SSE, Replay engine |
| **22** | Postgres Auth Store | 🔵 Apoorv | SHA-256 tokens, SQL schema |
| **23** | Rate Limiting & Semaphores | 🟢 Gaurav | Token bucket, Channel semaphores |
| **24** | Metrics & Profiling | ⚪ Both | Prometheus, `pprof` |
| **25** | Docker Compose & Load Tests | ⚪ Both | Wildcard TLS, Vegeta benchmark |

---

*Keep this document open as your single source of truth. As we build each step, you will write the exact function in your IDE, verify it, and check it off!*
