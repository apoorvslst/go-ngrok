# 🌐 Server-Side Architecture: Day-by-Day Guide (Rahul's Example)

> **For Gaurav:**
> This document explains our **Server** architecture from **Day 1 to Day 6** using one single, real-world developer example: **Rahul running a web app on his laptop (`localhost:3000`) and using our server to expose it to the internet.**
> 
> - No fluff or unrelated analogies.
> - Technical terms are defined in brackets `(...)` right when they appear.
> - Flowcharts use clean ASCII text boxes.

---

## 📑 Table of Contents
1. [The Real-World Setup (Who is Rahul?)](#the-real-world-setup-who-is-rahul)
2. [Day 1: Rahul Dials the Server (TCP Listening & Concurrency)](#day-1-rahul-dials-the-server-tcp-listening--concurrency)
3. [Day 2: Rahul Authenticates (Framing, Generics & Handshake)](#day-2-rahul-authenticates-framing-generics--handshake)
4. [Day 3: Multiplexing Rahul's Connection (Yamux Streams)](#day-3-multiplexing-rahuls-connection-yamux-streams)
5. [Day 4: Storing Rahul in the Registry & The Two-Door Server](#day-4-storing-rahul-in-the-registry--the-two-door-server)
6. [Day 5: Visitors Hit Rahul's App (HTTP Reverse Proxy over Yamux)](#day-5-visitors-hit-rahuls-app-http-reverse-proxy-over-yamux)
7. [Day 6: Rahul's Wi-Fi Drops (Grace Period, Hijack Protection & Cleaner)](#day-6-rahuls-wi-fi-drops-grace-period-hijack-protection--cleaner)
8. [Quick Reference Cheat Sheet](#quick-reference-cheat-sheet)

---

## The Real-World Setup (Who is Rahul?)

- **Rahul's Laptop:** Rahul is running a React/Node web server locally on `http://localhost:3000`.
- **The Problem:** Rahul's laptop is on his home Wi-Fi. It has a private IP (`192.168.1.15`). People on the internet (friends, clients, Stripe webhooks) cannot reach his laptop directly.
- **Our Tunnel Server:** A Go server running on a public cloud VPS (like AWS or DigitalOcean) with a public IP.
- **The Goal:** When someone visits `http://rahul.tunnel.dev:8080`, our server routes that request directly to Rahul's `localhost:3000` and displays his app.

---

# Day 1: Rahul Dials the Server (TCP Listening & Concurrency)

📁 **Server Files:** `internal/server/listener.go`, `cmd/server/main.go`

### What the Server Does:
1. The server opens port `:9000` using `net.Listen("tcp", ":9000")` and starts an infinite accept loop (`listener.Accept`).
2. Rahul runs `./client` on his laptop, which dials `net.Dial("tcp", "server:9000")`.
3. As soon as Rahul connects, the server accepts his TCP socket (`net.Conn`).
4. Instead of freezing the server to only talk to Rahul, the server spins off a new goroutine (background thread in Go): `go handleConnection(conn)`.
5. The main loop immediately goes back to waiting at port `:9000` so another developer (e.g. Gaurav) can connect at the same time.
6. If someone stops the server with `Ctrl+C`, the server catches the signal using `context.Context` and waits for Rahul's active connection to close cleanly using `sync.WaitGroup` before shutting down.

---

### Flowchart: Day 1 Server Flow

```
+-------------------------------------------------------------------+
|                     RAHUL'S LAPTOP (CLI)                          |
|  net.Dial("tcp", "server:9000")                                   |
+-------------------------------------------------------------------+
                               |
                               | (TCP connection request)
                               v
+-------------------------------------------------------------------+
|                     SERVER PORT :9000                             |
|  net.Listen("tcp", ":9000")                                       |
+-------------------------------------------------------------------+
                               |
                               v
+-------------------------------------------------------------------+
|                     ACCEPT LOOP (listener.Accept)                 |
|  Unblocks when Rahul connects.                                    |
+-------------------------------------------------------------------+
                               |
                               | Spawns dedicated goroutine:
                               v
+-------------------------------------------------------------------+
|               go handleConnection(conn net.Conn)                  |
|                                                                   |
|  * Dedicated goroutine (super-lightweight Go thread) for Rahul.   |
|  * Main loop is already free to accept Gaurav's connection.       |
|  * defer conn.Close() guarantees socket is freed on disconnect.   |
+-------------------------------------------------------------------+
                               ^
                               |
+-------------------------------------------------------------------+
|                  SHUTDOWN WATCHER (context.Context)               |
|  On Ctrl+C, stops accepting new connections and uses              |
|  sync.WaitGroup to let Rahul finish before exiting.               |
+-------------------------------------------------------------------+
```

---

### Technical Terms Explained:
- **TCP (Transmission Control Protocol):** A reliable, two-way internet connection standard (like a phone call) where data bytes are guaranteed to arrive in order without corruption.
- **Port:** A numbered network door on a computer (e.g. `:9000`). It tells the OS which program should receive the incoming data.
- **`net.Conn` (Network Connection):** The live two-way data pipe between Rahul's laptop and the server.
- **`net.Listener`:** A Go object that sits on a port and waits for incoming connections.
- **Goroutine (Super-lightweight Go thread):** A mini-thread managed by the Go runtime taking ~2KB of RAM, allowing thousands of developers to connect simultaneously.
- **`sync.WaitGroup`:** A counter that tracks active connections so the server doesn't shut down while Rahul is still using it.
- **Graceful Shutdown:** Stopping a server safely by closing ports and letting active connections finish, instead of crashing them.

---

### Server Code Snippet (Day 1):

```go
// Open Port :9000 for client connections
listener, err := net.Listen("tcp", ":9000")
if err != nil {
    log.Fatal(err)
}
defer listener.Close()

for {
    // Blocks until Rahul's laptop connects
    conn, err := listener.Accept()
    if err != nil {
        break // Server shutting down
    }

    wg.Add(1) // Increment active connection counter
    // Handle Rahul in a separate goroutine so other clients can connect
    go func(c net.Conn) {
        defer wg.Done()
        defer c.Close()
        handleConnection(c)
    }(conn)
}
```

---

# Day 2: Rahul Authenticates (Framing, Generics & Handshake)

📁 **Server Files:** `internal/protocol/messages.go`, `internal/protocol/codec.go`, `internal/server/listener.go`

### What the Server Does:
1. **The Problem:** TCP is a continuous byte stream without message boundaries. If Rahul sends two JSON messages fast, the server might read them merged together: `{"token":"secret"}{"type":"ping"}`. Passing this to JSON decoding causes a crash.
2. **The Framing Fix:** Every message ends with a newline byte (`\n`). The server reads byte-by-byte until it sees `\n`, separating messages cleanly.
3. **The 5-Second Handshake Deadline:** The moment Rahul connects, the server starts a 5-second countdown timer (`conn.SetDeadline`). If Rahul takes longer than 5 seconds to send his credentials, the server disconnects him (protects against slow attackers).
4. **Validating Credentials:** Rahul sends a `HelloMessage` containing `Token: "secret123"` and `DesiredSubdomain: "rahul"`. The server verifies the token against its configured secret.
5. **Generic Codec (`ReadMessage[T]`):** The server uses Go generics (`[T any]`) to decode the message into a strongly typed Go struct without duplicate code.
6. **Response:** If valid, the server returns an `AuthResponseMessage` with `Success: true` and `AssignedURL: "https://rahul.tunnel.dev"`. The deadline is cleared and the connection is ready for Day 3.

---

### Flowchart: Day 2 Handshake & Framing

```
RAHUL'S LAPTOP                                              SERVER (:9000)
      |                                                            |
      | -------- 1. TCP Connect ---------------------------------> |
      |                                                            |
      |                                                 [Set 5-Second Deadline]
      |                                                 (Kills connection if Rahul lags)
      |                                                            |
      | -------- 2. HelloMessage + '\n' -------------------------> |
      |          {"token":"secret123", "desired_subdomain":"rahul"}|
      |                                                            |
      |                                                 [Read bytes until '\n']
      |                                                 [Generic Unmarshal to HelloMessage]
      |                                                 [Validate: token == "secret123"?]
      |                                                            |
      |                                                 TOKEN VALID:
      |                                                 AssignedURL = "https://rahul.tunnel.dev"
      |                                                            |
      | <------- 3. AuthResponseMessage + '\n' ------------------- |
      |          {"success":true, "assigned_url":"https://rahul..."}
      |                                                            |
      |                                                 [Clear Deadline]
      v                                                            v
[HANDSHAKE SUCCESSFUL: Upgraded to Yamux in Day 3]
```

---

### Technical Terms Explained:
- **Framing:** Adding boundary markers (like `\n`) around network messages so the receiver knows where one message ends and the next begins.
- **Delimiter:** The specific boundary character (we use `\n` newline).
- **Serialization / Marshaling:** Converting a Go struct into JSON text bytes.
- **Deserialization / Unmarshaling:** Parsing JSON text bytes back into a Go struct.
- **Generics (`[T any]`):** Writing a function once so it works with any Go struct type (`HelloMessage`, `AuthResponseMessage`, etc.).
- **Handshake:** The initial authentication exchange where credentials and requested settings are verified.
- **Deadline (`conn.SetDeadline`):** A strict network timeout preventing slow clients from holding open server connections indefinitely.

---

### Server Code Snippet (Day 2):

```go
func PerformHandshake(conn net.Conn, validToken string, domainRoot string) (*protocol.HelloMessage, error) {
    // 1. Enforce strict 5-second deadline for the handshake
    conn.SetDeadline(time.Now().Add(5 * time.Second))
    defer conn.SetDeadline(time.Time{}) // Clear deadline on function exit

    // 2. Read framed HelloMessage using generic codec
    hello, err := protocol.ReadMessage[protocol.HelloMessage](conn)
    if err != nil {
        return nil, fmt.Errorf("failed to read hello: %w", err)
    }

    // 3. Verify Rahul's secret token
    if hello.Token != validToken {
        resp := protocol.AuthResponseMessage{Success: false, ErrorMessage: "invalid token"}
        protocol.WriteMessage(conn, resp)
        return nil, errors.New("unauthorized token")
    }

    // 4. Send success response with Rahul's assigned URL
    resp := protocol.AuthResponseMessage{
        Success:     true,
        AssignedURL: fmt.Sprintf("https://%s.%s", hello.DesiredSubdomain, domainRoot),
    }
    protocol.WriteMessage(conn, resp)
    return hello, nil
}
```

---

# Day 3: Multiplexing Rahul's Connection (Yamux Streams)

📁 **Server Files:** `internal/server/listener.go`

### What the Server Does:
1. **The Problem:** Rahul's laptop only opened **one single TCP connection** to our server on port `:9000`. But when his website goes live, 20 different web visitors might click his site at the same second.
2. The server cannot dial back to Rahul's home laptop because of his home router/firewall.
3. **The Yamux Fix:** The server elevates Rahul's authenticated TCP socket into a **Yamux session** (`yamux.Server(conn, cfg)`).
4. Yamux turns that single physical TCP connection into a multi-lane highway:
   - When Visitor A arrives, the server calls `session.OpenStream()` (Lane 1).
   - When Visitor B arrives, the server calls `session.OpenStream()` (Lane 2).
   - Both streams travel through Rahul's single connection simultaneously without interfering.
5. **Keepalive Pings:** The server sends small ping packets across the connection every 15 seconds so Rahul's home router doesn't terminate the idle connection.

---

### Flowchart: Day 3 Yamux Highway

```
+------------------------------------------------------------------------+
|                             SERVER (:9000)                             |
|                                                                        |
|  Visitor 1 Request ---> session.OpenStream() [Stream ID: 1]  \         |
|  Visitor 2 Request ---> session.OpenStream() [Stream ID: 3]  -- [YAMUX]|
|  Visitor 3 Request ---> session.OpenStream() [Stream ID: 5]  /         |
+------------------------------------------------------------------------+
                                    ||
                                    || ONE SINGLE TCP SOCKET
                                    || (Kept alive by 15s heartbeats)
                                    ||
+------------------------------------------------------------------------+
|                         RAHUL'S LAPTOP                                 |
|                                                                        |
|  [YAMUX] -- session.AcceptStream() -> Stream 1 -> send to localhost:3000|
|          -- session.AcceptStream() -> Stream 3 -> send to localhost:3000|
|          -- session.AcceptStream() -> Stream 5 -> send to localhost:3000|
+------------------------------------------------------------------------+
```

---

### Technical Terms Explained:
- **Multiplexing:** Combining multiple data streams into a single physical connection so they can be transmitted together.
- **Yamux (Yet Another Multiplexer):** A Go library built by HashiCorp that splits one TCP connection into many virtual streams.
- **Virtual Stream (`yamux.Stream`):** A logical channel inside a Yamux session. It behaves just like a standalone network socket (`net.Conn`).
- **Keepalive Interval:** Background ping packets sent periodically (every 15s) so routers and firewalls don't drop the connection due to inactivity.

---

### Server Code Snippet (Day 3):

```go
// Elevate Rahul's raw TCP socket into a multiplexed Yamux server session
func NewYamuxServerSession(conn net.Conn) (*yamux.Session, error) {
    cfg := yamux.DefaultConfig()
    cfg.EnableKeepAlive = true
    cfg.KeepAliveInterval = 15 * time.Second // Ping Rahul's laptop every 15s

    session, err := yamux.Server(conn, cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to create yamux server session: %w", err)
    }

    // Server can now open virtual streams to Rahul at any time!
    return session, nil
}
```

---

# Day 4: Storing Rahul in the Registry & The Two-Door Server

📁 **Server Files:** `internal/server/registry/registry.go`, `internal/server/router/router.go`, `internal/server/httpserver/server.go`, `cmd/server/main.go`

### What the Server Does:
1. **The Two Doors (Two Ports):**
   - **Port `:9000` (Control Door):** Where Rahul's laptop connects, authenticates, and maintains its Yamux session.
   - **Port `:8080` (Public Front Door):** Where public internet visitors send HTTP requests (e.g. `http://rahul.tunnel.dev:8080/about`).
2. **The In-Memory Registry (`registry.go`):** A thread-safe phonebook (`map[string]*Tunnel`) protected by a read-write lock (`sync.RWMutex`).
   - When Rahul authenticates on `:9000`, the server saves:
     `"rahul"` -> `Rahul's Yamux Session`.
3. **The Virtual Host Router (`router.go`):**
   - A visitor sends a request to `http://rahul.tunnel.dev:8080/about`.
   - The server inspects the HTTP `Host` header: `"rahul.tunnel.dev:8080"`.
   - The `ExtractSubdomain` function strips the port (`:8080`) and root domain (`.tunnel.dev`), leaving the subdomain `"rahul"`.
4. **Registry Lookup:** Port `:8080` looks up `"rahul"` in the Registry:
   - Found: The server confirms Rahul is connected.
   - Not found: The server returns `404 Not Found`.

---

### Flowchart: Day 4 Two-Door Server Routing

```
=============================================================================
                              SERVER MACHINE
=============================================================================

 [PORT :9000 - CONTROL DOOR]                       [PORT :8080 - PUBLIC DOOR]
  Rahul connects his laptop                         Visitor visits website
           |                                                   |
           v                                                   v
   Authenticates & creates                             Visitor requests:
   Yamux Session                                       "rahul.tunnel.dev:8080"
           |                                                   |
           v                                                   v
   reg.Register("rahul", session)                      router.ExtractSubdomain(...)
           |                                           Result: "rahul"
           |                                                   |
           +------------------> [REGISTRY PHONEBOOK] <---------+
                                 "rahul" -> Session
                                         |
                                         v
                                Found Rahul's Session!
```

---

### Technical Terms Explained:
- **In-Memory Registry:** A dictionary stored in server RAM (`map[string]*Tunnel`) linking subdomains to active client sessions.
- **`sync.RWMutex` (Read-Write Lock):** A concurrency lock. Allows 10,000 visitors to search the map at the same millisecond (`RLock`), but prevents race condition crashes when adding or removing entries (`Lock`).
- **Host Header:** The domain name included in every HTTP request (e.g., `Host: rahul.tunnel.dev:8080`).
- **Virtual Host Routing:** Directing traffic to different backends based on the domain name in the HTTP request.

---

### Server Code Snippet (Day 4):

```go
// 1. Router extracts "rahul" from "rahul.tunnel.dev:8080"
func ExtractSubdomain(hostHeader string, rootDomain string) (string, error) {
    host, _, err := net.SplitHostPort(hostHeader)
    if err != nil {
        host = hostHeader // No port in header
    }
    suffix := "." + strings.ToLower(rootDomain)
    if !strings.HasSuffix(strings.ToLower(host), suffix) {
        return "", fmt.Errorf("host does not match domain")
    }
    return strings.TrimSuffix(strings.ToLower(host), suffix), nil
}

// 2. Thread-safe Registry Lookup
func (r *Registry) Get(subdomain string) (*Tunnel, bool) {
    r.mu.RLock() // Multiple visitors can read concurrently
    defer r.mu.RUnlock()
    tunnel, ok := r.tunnels[subdomain]
    return tunnel, ok
}
```

---

# Day 5: Visitors Hit Rahul's App (HTTP Reverse Proxy over Yamux)

📁 **Server Files:** `internal/server/proxy/proxy.go`, `internal/server/httpserver/server.go`

### What the Server Does:
1. A visitor types `http://rahul.tunnel.dev:8080/api/users` into their browser.
2. The public listener on `:8080` extracts subdomain `"rahul"` and fetches Rahul's `yamux.Session` from the Registry.
3. The server uses Go's production reverse proxy (`httputil.ReverseProxy`) to forward the visitor's request.
4. **The Custom `DialContext` Trick:**
   - Normally, Go's reverse proxy connects to an internet IP address via `net.Dial`.
   - We replace `DialContext` with a custom function that calls `tunnel.Session.OpenStream()`.
   - This opens a new virtual stream inside Rahul's existing persistent TCP connection.
5. The HTTP request bytes are streamed down to Rahul's laptop.
6. Rahul's client reads the request, forwards it to his local app (`http://localhost:3000/api/users`), receives the response, and streams it back through the Yamux stream.
7. The server streams the response back to the visitor's browser.

---

### Flowchart: Day 5 Reverse Proxy Flow

```
VISITOR BROWSER                                   SERVER (:8080)                               RAHUL'S LAPTOP
       |                                                 |                                            |
       | --- 1. GET rahul.tunnel.dev:8080/api ---------> |                                            |
       |                                                 |                                            |
       |                                       [Extract "rahul"]                                      |
       |                                       [Fetch Rahul's Session]                                |
       |                                       [ReverseProxy runs DialContext]                        |
       |                                       [Calls session.OpenStream()]                           |
       |                                                 |                                            |
       |                                                 | === 2. Stream HTTP Request over Yamux ===> |
       |                                                 |                                            |
       |                                                 |                                  [Forwards to local app]
       |                                                 |                                  [localhost:3000/api]
       |                                                 |                                            |
       |                                                 | <== 3. Stream HTTP Response (HTML/JSON) ===|
       |                                                 |                                            |
       | <-- 4. Displays webpage to visitor -------------|                                            |
       v                                                 v                                            v
```

---

### Technical Terms Explained:
- **Reverse Proxy:** A server that sits between public users and internal private servers, forwarding requests and returning responses.
- **`httputil.ReverseProxy`:** Go's built-in reverse proxy implementation. Handles HTTP headers, streaming bodies, status codes, and chunked transfer encoding automatically.
- **`http.Transport`:** The network transport engine inside Go that manages connections.
- **`DialContext`:** The hook function inside `http.Transport` that creates network connections. By replacing it, we route traffic through our Yamux tunnel instead of the public internet.

---

### Server Code Snippet (Day 5):

```go
func NewTunnelProxy(reg *registry.Registry, rootDomain string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        subdomain, _ := router.ExtractSubdomain(r.Host, rootDomain)
        tunnel, ok := reg.Get(subdomain)
        if !ok {
            http.Error(w, "Tunnel offline", http.StatusNotFound)
            return
        }

        // Configure ReverseProxy with our Yamux stream dialer
        proxy := &httputil.ReverseProxy{
            Director: func(req *http.Request) {
                req.URL.Scheme = "http"
                req.URL.Host = r.Host
                req.Host = r.Host
            },
            Transport: &http.Transport{
                // OVERRIDE: Open a Yamux stream to Rahul's laptop instead of dialing an IP!
                DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
                    return tunnel.Session.OpenStream()
                },
            },
        }

        proxy.ServeHTTP(w, r)
    })
}
```

---

# Day 6: Rahul's Wi-Fi Drops (Grace Period, Hijack Protection & Cleaner)

📁 **Server Files:** `internal/server/registry/registry.go`, `internal/server/listener.go`, `cmd/server/main.go`

### What the Server Does:
1. **The Problem:** Rahul closes his laptop lid or his home Wi-Fi flickers for 10 seconds.
   - If the server immediately deletes `"rahul"` from the Registry, another user (e.g. Bob) could connect and steal the subdomain `rahul.tunnel.dev`.
   - Also, active web visitors would immediately get `404 Not Found`.
2. **The Grace Period Solution:**
   - When Rahul's connection drops, the server runs `defer reg.MarkGracePeriod("rahul", 45*time.Second)`.
   - The tunnel state switches from `ACTIVE` to `GRACE_PERIOD`.
   - The dead Yamux session is cleared (`nil`), but the subdomain `"rahul"` remains locked for 45 seconds.
3. **Subdomain Hijack Protection:**
   - While in `GRACE_PERIOD`, if Bob tries to register `"rahul"`, the server compares tokens. Bob's token does not match Rahul's saved token, so the server rejects Bob.
4. **Reattachment:**
   - Within 45 seconds, Rahul's laptop reconnects, presents his token (`"secret123"`), and asks for `"rahul"`.
   - The server verifies the token matches, attaches the new Yamux session, switches the state back to `ACTIVE`, and clears the timer.
5. **The Background Cleaner:**
   - A background goroutine (`StartCleaner`) runs every 5 seconds.
   - If Rahul does not reconnect within 45 seconds (`now.After(tunnel.GraceExpiry)`), the cleaner deletes `"rahul"` from the map, releasing it for others.

---

### Flowchart: Day 6 Grace Period & Reconnection

```
[RAHUL CONNECTED] ---> State = ACTIVE (Proxying traffic)
       |
       | (Rahul's Wi-Fi drops for 10 seconds)
       v
[DISCONNECT DETECTED]
       |
       v
[reg.MarkGracePeriod] -> State = GRACE_PERIOD
                         GraceExpiry = Now + 45s
                         Session = nil (Dead socket cleared)
                         Rahul's Token is SAVED to protect ownership
       |
       +---------------------------------------------+
       |                                             |
 (Rahul reconnects within 45s)                 (45s pass without Rahul)
       |                                             |
       v                                             v
[reg.Register Checks Token]                    [StartCleaner Runs]
  Matches: State = ACTIVE                        Time > GraceExpiry:
           New session attached!                 Delete "rahul" from registry!
  Mismatch (Bob): Rejected!
```

---

### Technical Terms Explained:
- **Grace Period:** A temporary reservation window (45 seconds) holding a disconnected client's subdomain.
- **Subdomain Hijacking:** An exploit where someone steals a recently disconnected subdomain name.
- **Tunnel State (`ACTIVE` vs `GRACE_PERIOD`):**
  - `ACTIVE`: Client is online and handling visitor requests.
  - `GRACE_PERIOD`: Client disconnected temporarily; subdomain is reserved.
- **Reattachment:** Restoring an existing tunnel record with a newly connected Yamux session after a disconnect.
- **Background Cleaner (`time.Ticker`):** A background worker loop that runs every 5 seconds to remove abandoned reservations whose grace period expired.

---

### Server Code Snippet (Day 6):

```go
// 1. Reclaiming from Grace Period during Handshake
func (r *Registry) Register(subdomain string, session *yamux.Session, addr string, token string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    existing, ok := r.tunnels[subdomain]
    if ok {
        // Token matches -> Rahul reconnecting! Reclaim tunnel!
        if existing.State == StateGracePeriod && existing.Token == token {
            existing.State = StateActive
            existing.Session = session
            existing.ClientAddr = addr
            existing.GraceExpiry = time.Time{} // Clear countdown
            slog.Info("tunnel reclaimed from grace period", "subdomain", subdomain)
            return nil
        }
        // Subdomain reserved or taken by someone else
        return fmt.Errorf("subdomain '%s' is unavailable", subdomain)
    }

    // New registration
    r.tunnels[subdomain] = &Tunnel{
        Subdomain: subdomain, Session: session, ClientAddr: addr,
        Token: token, State: StateActive, CreatedAt: time.Now(),
    }
    return nil
}

// 2. Background cleaner runs every 5 seconds
func (r *Registry) StartCleaner(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.mu.Lock()
            now := time.Now()
            for sub, tunnel := range r.tunnels {
                if tunnel.State == StateGracePeriod && now.After(tunnel.GraceExpiry) {
                    delete(r.tunnels, sub)
                    slog.Info("grace period expired, released subdomain", "subdomain", sub)
                }
            }
            r.mu.Unlock()
        }
    }
}
```

---

# Quick Reference Cheat Sheet

| Day | Feature | What Rahul Does | What the Server Does |
| :---: | :--- | :--- | :--- |
| **Day 1** | **TCP Listening** | Dials port `:9000`. | Accepts TCP socket and gives Rahul a dedicated goroutine. |
| **Day 2** | **Framing & Handshake** | Sends token & `"rahul"` + `\n`. | Parses with `\n` framing under a 5s deadline, verifies token, returns public URL. |
| **Day 3** | **Yamux Multiplexing** | Wraps TCP in Yamux client. | Wraps TCP in Yamux server, sends keepalive pings every 15s to keep connection alive. |
| **Day 4** | **Registry & Router** | Stays connected on `:9000`. | Saves `"rahul"` in Registry; extracts `"rahul"` from incoming `Host: rahul.tunnel.dev:8080`. |
| **Day 5** | **HTTP Reverse Proxy** | Runs web app on `localhost:3000`. | Visitor hits `:8080` -> Server opens Yamux stream to Rahul's laptop -> Returns response. |
| **Day 6** | **Grace Period & Cleaner**| Wi-Fi drops for 10 seconds. | Holds `"rahul"` for 45s, prevents Bob from stealing it, and reattaches Rahul when he returns. |

---
