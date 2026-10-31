# 🚇 GoNgrok — Everything You Need to Know Before Writing a Single Line of Code

> **Authors**: Apoorv & Gaurav
> **Timeline**: 15 days
> **Language**: Go
> **What**: An ngrok-style reverse tunneling service — expose `localhost` to the internet

---

## Table of Contents

1. [The Story (Tell This in Interviews)](#1-the-story-tell-this-in-interviews)
2. [Why This Project is a Beast on Your Resume](#2-why-this-project-is-a-beast-on-your-resume)
3. [The Real Problem — Explained Simply](#3-the-real-problem--explained-simply)
4. [The Solution in One Picture](#4-the-solution-in-one-picture)
5. [Tech Stack & Why Each Choice](#5-tech-stack--why-each-choice)
6. [Design Decisions & Tradeoffs](#6-design-decisions--tradeoffs)
7. [Core Concepts to Study (Surface Level)](#7-core-concepts-to-study-surface-level)
8. [Database Design & Why Postgres](#8-database-design--why-postgres)
9. [What We Will NOT Build (And Why That's Smart)](#9-what-we-will-not-build-and-why-thats-smart)
10. [Glossary — Words You'll Hear a Lot](#10-glossary--words-youll-hear-a-lot)
11. [Resources & Reading](#11-resources--reading)

---

## 1. The Story (Tell This in Interviews)

> *"During development, every time we wanted to test Razorpay/Stripe webhooks, we had to deploy our entire backend to a cloud server, wait for the build, check logs remotely — it was slow and painful. We knew tools like ngrok exist, but we asked ourselves: **what's actually happening under the hood?** How does a single TCP connection carry 50 parallel HTTP requests? How do you handle a laptop that just closes its lid?*
>
> *So we decided to build our own version from scratch in Go. Not to replace ngrok — nobody needs another ngrok — but because the problem space is a goldmine for learning real systems programming: TCP, multiplexing, concurrency, failure handling, graceful shutdown, observability. Every design decision forced us to think about tradeoffs.*
>
> *We built the server, the CLI client, an inspector UI, token auth, subdomain routing, heartbeats, reconnection with backoff, and even Prometheus metrics. It was our deepest Go project."*

### Why this story works:
- ❌ "I followed a tutorial" — boring
- ❌ "I wanted to learn Go" — vague
- ✅ "I had a real pain point, dug into how the existing solution works, and rebuilt it to truly understand networking and concurrency" — **this lands**

---

## 2. Why This Project is a Beast on Your Resume

| Skill Area | What This Project Proves |
|---|---|
| **Networking** | You understand TCP, HTTP, TLS, NAT traversal, the `Host` header, reverse proxies |
| **Concurrency** | Goroutines, channels, `sync.RWMutex`, `context` cancellation, `io.Copy` pipelines |
| **Multiplexing** | Many logical streams over one physical connection (same idea as HTTP/2, gRPC) |
| **Failure Handling** | Heartbeats, dead peer detection, reconnects, backoff, graceful shutdown, `defer`/`recover` |
| **System Design** | You can whiteboard this in an interview: actors, data flow, tradeoffs, what you chose and what you left out |
| **Database** | Schema design, hashed tokens, session tracking |
| **Observability** | Structured logging (`slog`), Prometheus metrics, `pprof` profiling |
| **DevOps** | Docker, Docker Compose, wildcard DNS, TLS certs |
| **CLI Tooling** | Building a real CLI with Cobra |

An interviewer hears "I built a tunneling service" and immediately thinks:
- *"This person doesn't just know how to call an API — they know what happens underneath."*

---

## 3. The Real Problem — Explained Simply

### Situation
Your laptop is behind a **NAT (Network Address Translation)** router. That means:
- ✅ Your laptop can **reach out** to the internet (make outbound connections)
- ❌ The internet **cannot reach in** to your laptop (no inbound connections)
- ❌ You have no public IP address

### Why this hurts
- Stripe sends a webhook → needs a URL → your laptop has no URL → **dead end**
- You want to demo to a client → they can't open `localhost:3000` → **dead end**
- You're testing a mobile app → phone can't reach your laptop → **dead end**

### The Trick: Reverse Connection
Instead of the internet connecting TO your laptop, your **laptop connects OUT to a public server**, and that outbound connection becomes a two-way tunnel.

```
Your laptop dials out ──────────────► Public server (has a domain)
                         ◄──────────
                    This TCP connection
                    now carries traffic
                    in BOTH directions
```

The server acts as a **middleman**. When Stripe sends a request to `rahul.tunnel.dev`, the server forwards it through the existing connection to Rahul's laptop.

### The Sub-Problems
Once you understand the trick, five hard problems appear:

| # | Problem | In Plain Hindi |
|---|---------|----------------|
| 1 | **NAT Traversal** | Laptop bahar se connect karega, andar se nahi aa sakta |
| 2 | **Routing** | Server ko pata hona chahiye ki `rahul.tunnel.dev` ka request kisko bhejna hai |
| 3 | **Multiplexing** | Ek connection pe 50 requests ek saath — kaise mix up na ho? |
| 4 | **Lifecycle** | Client disconnect ho gaya? WiFi gaya? Goroutines leak nahi hone chahiye |
| 5 | **Security** | Koi bhi kisi ka subdomain na chheen le, rate limits, auth |

---

## 4. The Solution in One Picture

```
┌──────────┐         HTTPS          ┌───────────────────┐       yamux session       ┌──────────────┐         HTTP        ┌────────────┐
│          │ ───────────────────────►│                   │◄═══════════════════════════│              │──────────────────── │            │
│ VISITOR  │   rahul.tunnel.dev     │     SERVER        │   (one outbound TCP conn  │   CLIENT     │  localhost:3000     │ LOCAL APP  │
│ (Stripe) │ ◄─────────────────────-│   (Public VPS)    │════════════════════════════►│  (Laptop)    │◄────────────────── │ (Your App) │
│          │        response        │                   │       from client)         │              │     response        │            │
└──────────┘                        │  ┌─────────────┐  │                            │ ┌──────────┐ │                     └────────────┘
                                    │  │  Registry    │  │                            │ │Inspector │ │
                                    │  │  subdomain → │  │                            │ │ :4040    │ │
                                    │  │  yamux sess  │  │                            │ └──────────┘ │
                                    │  └─────────────┘  │                            └──────────────┘
                                    │  ┌─────────────┐  │
                                    │  │  Postgres    │  │
                                    │  │  tokens/subs │  │
                                    │  └─────────────┘  │
                                    └───────────────────┘
```

### Four actors:
1. **Visitor** — Stripe, browser, anyone hitting the public URL
2. **Server** — your VPS with a public IP, runs Go, has Postgres
3. **Client** — CLI on your laptop (`tunnel http 3000`)
4. **Local App** — whatever is running on `localhost:3000`

---

## 5. Tech Stack & Why Each Choice

### 5.1 Go (the language)

| Why Go? | Detail |
|---------|--------|
| **Built for networking** | `net`, `net/http`, `net/http/httputil` — first-class TCP, HTTP, reverse proxy support |
| **Goroutines are cheap** | Each request gets its own goroutine. 10,000 concurrent tunnels? No problem. Threads would choke. |
| **`io.Copy` is king** | The core of proxying: copy bytes from A to B. Go's `io` package does this beautifully with minimal allocations |
| **`context.Context`** | Cancellation propagation — when a visitor disconnects, context cancels, goroutines die cleanly |
| **Single binary** | `go build` gives you one file. No runtime, no `node_modules`, easy to deploy |
| **`sync` package** | `RWMutex`, `WaitGroup`, `Once` — battle-tested concurrency primitives |
| **`slog`** | Structured logging built into the standard library (Go 1.21+) |

### 5.2 hashicorp/yamux — The Multiplexer

**What is it?**
A library that takes one TCP connection and creates many independent "streams" inside it. Each stream behaves like its own `net.Conn`.

**Why yamux and not alternatives?**

| Option | Verdict |
|--------|---------|
| **One TCP connection per request** | Too many TCP handshakes, TLS handshakes. Slow. Connection limits. |
| **HTTP/2** | Go's `x/net/http2` can do this, but it's designed for browser↔server, not tunnel↔tunnel. Harder to control. |
| **QUIC** | Solves head-of-line blocking (which yamux has), but much more complex. Out of scope. |
| **Custom framing** | Writing our own protocol = bugs, no community review. Why reinvent? |
| **yamux** ✅ | HashiCorp uses it in Consul, Nomad, Vault. Battle-tested. Simple API. Wraps `net.Conn`, gives back `net.Conn`-like streams. Built-in keepalive. |

**Key insight**: yamux gives you a `Session` object. The session has `Open()` to create a new stream (server side for each request) and `Accept()` to receive streams (client side). Each stream is an independent bidirectional byte pipe.

### 5.3 PostgreSQL + pgx

**Why a database at all?**
- Store user tokens (hashed) — so auth survives server restart
- Store subdomain reservations — so `rahul` always belongs to Rahul's token
- Store tunnel session logs — for auditing and metrics

**Why Postgres over alternatives?**

| Option | Why Not |
|--------|---------|
| SQLite | Great for single-user apps, but no concurrent writes from multiple goroutines safely under load |
| MySQL | Would work fine. Postgres is just more common in Go ecosystem and has better JSON support |
| Redis | Perfect for the registry (subdomain → session), but not for durable user/token storage. We might add it later for multi-server scaling |
| No DB, just a JSON file | Doesn't survive concurrent access. Not serious. |

**Why `pgx` over `database/sql`?**
`pgx` is the best Postgres driver for Go. It's faster (binary protocol), has connection pooling (`pgxpool`), and supports Postgres-specific features. `database/sql` is fine but `pgx` is the Go community standard for Postgres.

### 5.4 Cobra (CLI Framework)

We need a CLI like:
```bash
tunnel http 3000 --name rahul --token abc123
tunnel server --port 8080 --control-port 9090
```

**Cobra** is the de-facto Go CLI library (used by `kubectl`, `docker`, `hugo`). It gives us:
- Subcommands (`http`, `server`)
- Flags (`--name`, `--token`, `--port`)
- Help text auto-generated
- Environment variable binding

### 5.5 Caddy / autocert (TLS)

Visitors hit `https://rahul.tunnel.dev`. Someone must terminate TLS (decrypt HTTPS).

| Option | Detail |
|--------|--------|
| **Caddy** ✅ | Automatic HTTPS, wildcard certs via Let's Encrypt + DNS challenge. Sits in front of our Go server as a reverse proxy. Zero-config TLS. |
| **autocert** (Go library) | We could do it in our Go code directly. More control but more work. Good alternative. |
| **Nginx** | Can do it, but Caddy is simpler for wildcard certs. |

**Decision**: Start with Caddy in Docker Compose. It handles `*.tunnel.dev` → our Go server. We focus on the tunnel logic, not certificate management.

### 5.6 Vanilla HTML/JS (Inspector UI)

The client runs a web UI on `localhost:4040` where you see every request live.

**Why vanilla and not React?**
- It's a simple list of requests with details. React is overkill.
- No build step. No `npm install`. Just serve an HTML file from Go's `embed` package.
- SSE (Server-Sent Events) for live updates. Dead simple.

### 5.7 Docker & Docker Compose

For deploying the server to a VPS:
```yaml
services:
  server:    # our Go tunnel server
  postgres:  # database
  caddy:     # TLS termination + wildcard HTTPS
```

One `docker compose up` and the entire server infrastructure is running.

### 5.8 Prometheus (Metrics)

Expose a `/metrics` endpoint with:
- `tunnel_active_tunnels` — how many clients are connected
- `tunnel_active_streams` — how many requests are in-flight
- `tunnel_requests_total` — counter
- `tunnel_request_duration_seconds` — histogram
- `tunnel_errors_total` — counter by type

This is standard industry practice. Any Go HTTP service that's "production-quality" has Prometheus metrics.

---

## 6. Design Decisions & Tradeoffs

This section is **interview gold**. For every decision, know: *what we picked, what we rejected, and why*.

### T1. One Connection, Many Requests (Multiplexing)

```
Option A: New TCP connection per request
   Visitor → Server → [new TCP to client] → Client → Local App
   ❌ Too many handshakes. Slow. What if 50 requests arrive in 1 second?

Option B: One long-lived connection, many logical streams  ✅
   Visitor → Server → [new yamux STREAM on existing connection] → Client → Local App
   ✅ One TCP handshake. Streams are cheap. Parallel by default.
   ⚠️ Cost: Head-of-line blocking (one lost TCP packet stalls ALL streams)
   ⚠️ Cost: Must manage stream lifecycle (open, close, reset, clean up)
```

**Interview line**: *"We chose yamux multiplexing over one TCP connection. It's the same principle as HTTP/2 — many logical streams over one physical connection. The tradeoff is TCP head-of-line blocking: if one packet is lost, all streams stall until retransmission. QUIC solves this but was out of scope."*

### T2. Routing: Subdomain vs Path

```
Option A: Path-based routing
   tunnel.dev/rahul/webhook  →  Rahul's laptop
   ❌ Breaks absolute URLs. If the local app serves /static/app.js,
      the visitor would need /rahul/static/app.js. Path rewriting is messy.

Option B: Subdomain-based routing  ✅
   rahul.tunnel.dev/webhook  →  Rahul's laptop
   ✅ Clean. The local app doesn't need to know about the tunnel.
   ⚠️ Cost: Need wildcard DNS (*.tunnel.dev → server IP)
   ⚠️ Cost: Need wildcard TLS certificate
```

### T3. TLS Termination: Server vs Pass-Through

```
Option A: TLS pass-through (SNI routing)
   Server reads SNI header, forwards encrypted bytes to client.
   ✅ Server never sees plaintext — more private.
   ❌ Server can't read Host header, can't inspect traffic, can't add headers.
   ❌ Client needs its own TLS cert.

Option B: Terminate at server  ✅
   Caddy/autocert decrypts HTTPS at the server. Server sees plaintext HTTP.
   ✅ Server can read Host header for routing.
   ✅ Server can add X-Forwarded-For, X-Forwarded-Proto.
   ✅ Client doesn't need certificates.
   ⚠️ Cost: Users must trust the server operator (they can see the traffic).
```

**Interview line**: *"We terminate TLS at the server because we need to read the Host header for subdomain routing. The tradeoff is that the server sees plaintext — for end-to-end encryption you'd need SNI passthrough, but then the client needs its own certificate infrastructure."*

### T4. Registry: In-Memory vs Redis

```
Option A: Redis
   ✅ Survives server restarts. Enables multi-server scaling.
   ❌ Extra dependency. Extra latency on every request lookup.
   ❌ We don't need multi-server yet.

Option B: In-memory map with sync.RWMutex  ✅
   ✅ Blazing fast lookups (nanoseconds).
   ✅ Zero external dependencies.
   ⚠️ Cost: Lost on server restart (clients reconnect anyway).
   ⚠️ Cost: Can't scale to multiple servers (each has its own map).
```

**Interview line**: *"The registry is an in-memory map guarded by sync.RWMutex. Reads happen on every single visitor request — they must be fast. Writes only happen on client connect/disconnect, so RWMutex is perfect. If we needed horizontal scaling, we'd move to Redis with pub/sub for cross-node routing."*

### T5. The `httputil.ReverseProxy` Trick

This is the **cleverest part** of the design:

Go's `httputil.ReverseProxy` is designed to proxy HTTP requests. It normally dials a TCP connection to a backend. But you can override `Transport.DialContext` to open a **yamux stream** instead.

```go
proxy := &httputil.ReverseProxy{
    Transport: &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            // Instead of TCP dial, open a yamux stream!
            return yamuxSession.Open()
        },
    },
}
```

Now `ReverseProxy` handles:
- ✅ HTTP parsing
- ✅ Streaming bodies
- ✅ WebSocket upgrades (with `Upgrade` header handling)
- ✅ Hop-by-hop header removal
- ✅ `X-Forwarded-*` headers

We get all of this **for free**. Zero hand-written HTTP parsing.

### T6. Heartbeats vs TCP Keepalive

```
Option A: Rely on TCP keepalive
   ❌ Default is 2 hours on Linux. That's way too slow.
   ❌ Even if tuned, NAT routers can silently drop idle connections.

Option B: Application-level heartbeats (yamux keepalive)  ✅
   ✅ yamux sends keepalive pings every ~10 seconds.
   ✅ If 3 pings fail, session is declared dead.
   ✅ Combined with read/write deadlines for extra safety.
   ⚠️ Cost: Tiny bandwidth overhead (negligible).
```

### T7. Reconnection Strategy

When the client loses connection (WiFi drops, laptop sleeps):

```
1. Client detects disconnect
2. Waits:  1s → 2s → 4s → 8s → ... (exponential backoff)
3. Adds random jitter (so 1000 clients don't all reconnect at once)
4. Tries to reclaim the same subdomain
5. Server holds the subdomain for 30-60 seconds (grace period)
6. If within grace period → same URL still works
7. If expired → subdomain is released
```

**Interview line**: *"We use exponential backoff with jitter for reconnects — the jitter is critical because without it, a server restart causes a thundering herd of simultaneous reconnection attempts."*

### T8. Inspector Capture: Tee, Don't Buffer

For the inspector to show request/response bodies:

```
Option A: Buffer entire body in memory
   ❌ A 500MB file upload = 500MB memory = OOM

Option B: Tee with a cap  ✅
   ✅ Use io.TeeReader to copy first ~1MB to the inspector buffer
   ✅ Rest of the body streams through without capture
   ✅ Constant, predictable memory usage
   ⚠️ Cost: Inspector shows "(truncated)" for large bodies
```

---

## 7. Core Concepts to Study (Surface Level)

Study each of these **briefly** before we start coding. You don't need to be an expert — just understand what they are and why we need them.

### 7.1 Networking Fundamentals

| Concept | What to Study | Why We Need It |
|---------|---------------|----------------|
| **TCP** | Connection-oriented, reliable, ordered byte stream. 3-way handshake. | Our tunnel is a TCP connection |
| **NAT** | Your router maps `192.168.x.x:random → public_ip:random`. Inbound doesn't work because the router doesn't know which device to send to | The whole reason this project exists |
| **HTTP** | Request-response over TCP. Methods, headers, body, status codes | We proxy HTTP requests |
| **Host header** | `Host: rahul.tunnel.dev` — tells the server which website is being requested | We use this for subdomain routing |
| **TLS/HTTPS** | Encryption layer on top of TCP. Certificates. Let's Encrypt | Visitors connect via HTTPS |
| **WebSocket** | Starts as HTTP, then "upgrades" to a persistent bidirectional connection | We need to support WebSocket tunneling |
| **Reverse Proxy** | A server that receives a request and forwards it to a backend | Our server is literally a reverse proxy |

### 7.2 Go Fundamentals

| Concept | What to Study | Why We Need It |
|---------|---------------|----------------|
| **`net.Conn`** | Interface: `Read([]byte)`, `Write([]byte)`, `Close()`. Every TCP connection is a `net.Conn` | yamux streams implement `net.Conn` |
| **`net.Listener`** | Interface: `Accept() (net.Conn, error)`. Waits for incoming connections | yamux session implements `net.Listener` on the client side |
| **`net/http`** | Go's HTTP server & client. `http.Handler`, `http.ListenAndServe` | Server's public listener, client's inspector |
| **`httputil.ReverseProxy`** | Built-in reverse proxy. Handles HTTP proxying, streaming, WebSocket | The core of our server-side proxying |
| **`io.Copy`** | Copies bytes from a Reader to a Writer. Blocks until EOF or error | Core of bidirectional proxying |
| **`io.TeeReader`** | Reads from one source, writes to two destinations | Inspector capture |
| **Goroutines** | `go func(){}()` — lightweight concurrent function | One per request, one per stream direction, one per heartbeat |
| **Channels** | `chan` — typed communication between goroutines | Inspector event broadcasting, semaphores |
| **`context.Context`** | Carries cancellation, deadlines, and values across API boundaries | Visitor disconnect → cancel stream → cancel local request |
| **`sync.RWMutex`** | Reader-writer lock. Many readers OR one writer | Registry: many reads (every request), rare writes (connect/disconnect) |
| **`sync.WaitGroup`** | Wait for N goroutines to finish | Graceful shutdown: wait for in-flight requests |
| **`defer`** | Run a function when the enclosing function returns | Close connections, unlock mutexes, cleanup |
| **`embed`** | Embed files into the Go binary at compile time | Inspector HTML/JS served from the binary |

### 7.3 yamux Specifically

```go
// Server side: wrap a net.Conn into a yamux session
session, _ := yamux.Server(conn, yamux.DefaultConfig())

// Open a new stream (server → client, for each visitor request)
stream, _ := session.Open()

// Client side: wrap a net.Conn into a yamux session
session, _ := yamux.Client(conn, yamux.DefaultConfig())

// Accept incoming streams (client accepts what the server opens)
stream, _ := session.Accept()

// Each stream is a net.Conn — you can Read, Write, Close it
```

**Study**: Read the [yamux README](https://github.com/hashicorp/yamux) — it's short.

### 7.4 The `httputil.ReverseProxy` Pattern

```go
// Normal reverse proxy: dials a TCP connection to a backend
proxy := httputil.NewSingleHostReverseProxy(targetURL)

// Our trick: override DialContext to open a yamux stream instead
proxy := &httputil.ReverseProxy{
    Director: func(req *http.Request) {
        req.URL.Scheme = "http"
        req.URL.Host = "localhost" // doesn't matter, we override Dial
    },
    Transport: &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            return yamuxSession.Open() // <-- the magic
        },
    },
}
```

**Study**: Read the [`httputil.ReverseProxy` docs](https://pkg.go.dev/net/http/httputil#ReverseProxy).

### 7.5 Structured Logging with `slog`

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
logger.Info("tunnel connected",
    "tunnel_id", tunnelID,
    "subdomain", subdomain,
    "remote_addr", conn.RemoteAddr(),
)
```

Every log line has structured fields — easy to search, filter, and parse.

### 7.6 Graceful Shutdown Pattern

```go
// Listen for Ctrl+C
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

// Start server in goroutine
go server.ListenAndServe()

// Block until signal
<-ctx.Done()

// Drain: stop accepting new connections, wait for in-flight to finish
server.Shutdown(context.Background())
```

---

## 8. Database Design & Why Postgres

### Schema

```sql
-- Who uses the service
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Auth tokens (one user can have many tokens)
CREATE TABLE tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    token_hash  TEXT NOT NULL,           -- SHA-256 hash, NEVER store plaintext
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ            -- NULL means active
);

-- Reserved subdomains
CREATE TABLE subdomains (
    name        TEXT PRIMARY KEY,        -- "rahul", "apoorv"
    user_id     UUID NOT NULL REFERENCES users(id),
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tunnel session history (for metrics/auditing)
CREATE TABLE tunnel_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id        UUID NOT NULL REFERENCES tokens(id),
    subdomain       TEXT NOT NULL,
    connected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    disconnected_at TIMESTAMPTZ,
    bytes_in        BIGINT DEFAULT 0,
    bytes_out       BIGINT DEFAULT 0
);
```

### Why Hash Tokens?
- We store `SHA-256(token)` in the database, not the raw token
- If the database leaks, the attacker can't use the tokens
- Client sends raw token → server hashes it → looks up the hash in DB
- Same principle as password hashing (but SHA-256 is fine for API tokens since they're random and high-entropy, unlike passwords)

### Why UUIDs?
- No sequential IDs that reveal information (user #5 → "they only have 5 users")
- Safe for distributed systems if we ever scale
- `gen_random_uuid()` is built into Postgres

---

## 9. What We Will NOT Build (And Why That's Smart)

In an interview, saying "we deliberately chose not to build X because Y" shows more maturity than claiming everything is perfect.

| Not Building | Why |
|---|---|
| **Raw TCP/UDP tunnels** | HTTP covers 95% of use cases. TCP tunneling needs port allocation and has no Host header for routing. |
| **Multi-server horizontal scaling** | Requires shared registry (Redis), cross-node stream routing, consistent hashing. Big scope increase. We know HOW we'd do it. |
| **Custom user domains** | Needs per-domain TLS certs, DNS verification. Caddy can do it but it's a product feature, not a systems learning. |
| **End-to-end encryption** | Would need SNI passthrough. Server can't read Host header. Routing breaks. |
| **Full account system** | OAuth, email verification, password reset — product work, not systems work. Static tokens are enough. |
| **DDoS protection** | Real services need Cloudflare, IP reputation, CAPTCHA. Out of scope. We do basic rate limiting. |
| **Persistent inspector history** | Inspector is per-session, in-memory. Persisting to a DB is a feature, not a learning. |

---

## 10. Glossary — Words You'll Hear a Lot

| Term | Meaning |
|------|---------|
| **NAT** | Network Address Translation — your router maps private IPs to one public IP |
| **Tunnel** | The logical connection between a client and the server. One tunnel = one subdomain |
| **Stream** | A single logical channel inside a yamux session. One stream = one HTTP request |
| **Session** | A yamux session wrapping a TCP connection. Contains many streams |
| **Registry** | The in-memory map on the server: `subdomain → yamux session` |
| **Control listener** | The TCP port where clients connect and authenticate |
| **Public listener** | The HTTP(S) port where visitors send requests |
| **Handshake** | The initial message exchange: client sends `{token, subdomain}`, server responds `{ok, url}` |
| **Keepalive** | Periodic pings to check if the other side is still alive |
| **Backoff** | Increasing wait time between retries: 1s, 2s, 4s, 8s... |
| **Jitter** | Random variation added to backoff to prevent thundering herd |
| **Grace period** | Time the server holds a subdomain after client disconnects, waiting for reconnect |
| **Thundering herd** | When many clients reconnect at the exact same time, overwhelming the server |
| **Head-of-line blocking** | In yamux/TCP, one lost packet blocks ALL streams until it's retransmitted |
| **io.Copy** | Go function that copies bytes from a Reader to a Writer until EOF |
| **TeeReader** | Reads from source, writes to two places (like a T-shaped pipe) |
| **RWMutex** | A lock that allows many simultaneous readers OR one exclusive writer |
| **Context** | Go's mechanism for carrying deadlines, cancellation, and values across goroutines |
| **502 Bad Gateway** | HTTP status: "I'm a proxy and the backend failed" |
| **Reverse Proxy** | A server that receives client requests and forwards them to backends |
| **Wildcard DNS** | `*.tunnel.dev → 1.2.3.4` — any subdomain resolves to the same IP |
| **Wildcard TLS** | One certificate that covers `*.tunnel.dev` |
| **SNI** | Server Name Indication — TLS extension that tells the server which hostname the client wants |
| **SSE** | Server-Sent Events — server pushes updates to the browser over HTTP |

---

## 11. Resources & Reading

### Must Read (Before Starting)
- [ ] [yamux README](https://github.com/hashicorp/yamux) — understand the API
- [ ] [Go `net/http/httputil.ReverseProxy` docs](https://pkg.go.dev/net/http/httputil#ReverseProxy) — how we proxy
- [ ] [Go `io.Copy` docs](https://pkg.go.dev/io#Copy) — core of byte forwarding
- [ ] [Go `context` package](https://pkg.go.dev/context) — cancellation and timeouts
- [ ] [Go `sync.RWMutex` docs](https://pkg.go.dev/sync#RWMutex) — registry locking

### Good to Skim
- [ ] [Go `net` package](https://pkg.go.dev/net) — `Conn`, `Listener`, `Dial`
- [ ] [Go `slog` package](https://pkg.go.dev/log/slog) — structured logging
- [ ] [Cobra CLI library](https://github.com/spf13/cobra) — CLI framework
- [ ] [pgx README](https://github.com/jackc/pgx) — Postgres driver
- [ ] [How ngrok works](https://ngrok.com/docs) — inspiration

### Concepts to Google
- [ ] "TCP NAT traversal explained"
- [ ] "HTTP Host header routing"
- [ ] "multiplexing vs multiple connections"
- [ ] "exponential backoff with jitter"
- [ ] "Go goroutine leak detection"
- [ ] "Go graceful shutdown pattern"
- [ ] "io.Copy vs io.CopyBuffer Go"

---

## ⚡ Summary: What You Must Be Able to Explain Before Day 1 of Coding

1. **Why does this project exist?** → NAT/firewall prevents inbound connections. We reverse the direction.
2. **What is yamux?** → Many logical streams over one TCP connection. Like HTTP/2 but at the library level.
3. **How does routing work?** → Visitor's `Host` header → extract subdomain → look up registry → find yamux session.
4. **Why RWMutex?** → Registry reads happen on EVERY request (high frequency). Writes happen only on connect/disconnect (rare).
5. **What if the client dies?** → Heartbeat fails → session closed → registry cleaned → visitor gets 502.
6. **Why not Redis for the registry?** → Unnecessary complexity for a single-server setup. In-memory is nanoseconds vs milliseconds.
7. **Why Postgres for tokens?** → Tokens must survive server restarts. Subdomains must be durable.
8. **Why hash tokens?** → If DB leaks, attacker can't use the tokens.
9. **What's the ReverseProxy trick?** → Override `DialContext` to open a yamux stream. Get HTTP proxying for free.
10. **What tradeoffs did you leave unsolved?** → Single server (no HA), TCP HOL blocking, server sees plaintext, no DDoS protection.

---

*This document is your pre-flight checklist. Read it, discuss it together, ask each other these questions. When you can both explain every section to each other, you're ready to code.*
