# TUNNEL: A Reverse Tunneling Service in Go
(ngrok-style: expose a local app to the internet via a public URL)

## 1. One-Line Summary
A service that gives an app running on a developer's laptop (e.g. `localhost:3000`) a public HTTPS URL (e.g. `https://rahul.tunnel.dev`), by relaying internet traffic through a public server and a long-lived outbound connection opened by the laptop. It also provides a live request inspector with replay.

---

## 2. Background
- A laptop on home/college WiFi sits behind NAT and a firewall. It can make outbound connections but cannot receive inbound ones, and has no stable public IP or domain.
- During development, many things need to reach the local app:
  - **Webhooks** (Stripe, Razorpay, GitHub, Twilio) require a public URL to deliver events.
  - **Demos** for clients, teammates, hackathon judges.
  - **Mobile apps** testing against a backend on the laptop.
  - **Remote access** to machines behind a router (Pi, home server).
- Alternative of deploying every code change to the cloud is slow (build, push, wait, remote logs, repeat).
- Existing tools (ngrok, Cloudflare Tunnel, frp, localtunnel) solve this. Our goal is to build our own version to learn and demonstrate networking, concurrency, and failure handling in Go.

---

## 3. Problem Statement
Design and build a system where:
- **a)** A developer runs one CLI command: `tunnel http 3000`
- **b)** They receive a public URL that forwards all HTTP/HTTPS and WebSocket traffic to their local port
- **c)** Many visitors can use the URL at the same time
- **d)** The developer can see every request/response live and replay any request
- **e)** The system survives disconnects, slow clients, and abuse without leaking resources

### Core Sub-problems:
1. **NAT traversal**: how to reach a machine that accepts no inbound connections.
2. **Routing**: how one public server maps incoming requests to the right laptop among many (`subdomain -> tunnel`).
3. **Multiplexing**: how to carry many simultaneous requests over one connection without mixing up responses.
4. **Lifecycle**: how to detect dead clients, clean up goroutines and connections, and let clients reconnect.
5. **Security and fairness**: authentication, subdomain ownership, limits so one client cannot hurt others.
6. **Observability**: capturing traffic without breaking streaming or exhausting memory.

---

## 4. Goals
- Public HTTPS URL for any local HTTP app
- Random or custom subdomain per tunnel
- Token-based authentication
- Support for WebSocket and streaming responses
- Many concurrent visitors per tunnel (target: 1000+ concurrent streams per tunnel in load tests)
- Automatic client reconnect with same subdomain
- Local inspector UI (`localhost:4040`): live list of requests, headers, bodies, status, latency, and one-click replay
- Graceful shutdown on both server and client
- Metrics (active tunnels, active streams, bytes, errors)
- Dockerized deployment on a VPS with wildcard domain + TLS

---

## 5. Non-Goals (Explicitly out of scope)
- Raw TCP or UDP tunnels
- Multi-server / horizontally scaled deployment
- Custom user domains
- End-to-end encryption (server terminates TLS)
- Billing, teams, full account system
- DDoS protection and abuse moderation at production level
- Persistent, shared request history

---

## 6. Actors
- **SERVER**: Go program on a VPS with public IP + wildcard domain. The middleman.
- **CLIENT**: Go CLI on the developer's laptop. Runs next to the local app and the inspector.
- **VISITOR**: Anyone (browser, Stripe, Razorpay, etc.) calling the public URL.
- **LOCAL APP**: The developer's app on `localhost:PORT`.

---

## 7. High-Level Architecture

```
 Visitor --HTTPS--> [ SERVER ] <== yamux session ==> [ CLIENT ] --> Local App
                       |        (one outbound TCP         |
                    Registry     connection from client)   Inspector :4040
                    Postgres
```

### Server Components:
- **Control listener**: accepts client connections, performs handshake (`token + requested subdomain`), starts a yamux session.
- **Public HTTP(S) listener**: receives visitor traffic, reads `Host` header, finds the tunnel, proxies through a new stream.
- **Registry**: map `subdomain -> tunnel` (session, owner, stats), protected by `sync.RWMutex`.
- **Auth store**: Postgres table of hashed tokens and reserved subdomains.
- **Metrics endpoint**.

### Client Components:
- **Connector**: dials server, handshake, reconnect loop with backoff.
- **Stream handler**: serves HTTP on the yamux session (the session implements `net.Listener`) with a reverse-proxy handler that forwards to `localhost:PORT`.
- **Capture middleware**: records request/response pairs for the inspector (size-capped).
- **Inspector server + UI**: on `:4040` with replay.

---

## 8. Detailed Flow (Example: Stripe Webhook)
1. Rahul runs `tunnel http 3000 --name rahul --token XYZ`.
2. Client dials server control port over TCP (TLS), sends hello: `{ token, requested_subdomain, version }`.
3. Server validates token, checks subdomain is free or owned by this token, replies `{ ok, public_url }`.
4. Both sides wrap the connection in a yamux session (server = yamux server, client = yamux client).
5. Server registers `rahul -> session` in the registry. Client prints: `Live at https://rahul.tunnel.dev`.
6. Rahul sets Stripe webhook URL to `https://rahul.tunnel.dev/webhook`.
7. A test payment happens. Stripe sends `POST /webhook` to the server.
8. Server reads `Host = rahul.tunnel.dev`, extracts `"rahul"`, looks up the registry (`RLock`).
9. Server's `ReverseProxy` calls `DialContext`, which opens a **NEW** yamux stream on Rahul's session.
10. Request bytes flow over the stream to the client.
11. Client's HTTP handler receives it, capture middleware records it, reverse proxy forwards to `localhost:3000`.
12. Local app responds. Response flows back: local -> client -> stream -> server -> Stripe.
13. Inspector shows the request live; Rahul can replay it with one click without making another payment.
14. If the client disconnects, heartbeats fail, the server closes the session, removes the registry entry (after a short grace period), and visitors get 502/404.

---

## 9. Concurrency Design
- One goroutine per visitor request (`net/http` server model).
- One yamux stream per request; streams are independent so slow requests do not block fast ones.
- Per stream: bidirectional copy (`io.Copy` goroutines or `ReverseProxy` internals). Both must exit when either side closes.
- Per tunnel: keepalive / heartbeat watcher goroutine.
- Registry: `sync.RWMutex` (many reads on every request, writes only on connect/disconnect).
- `context.Context` propagated from visitor request into the stream so visitor cancels/timeouts close the stream.
- Per-tunnel semaphore (buffered channel) to cap concurrent streams.
- Inspector: capture events pushed over a buffered channel to a single broadcaster goroutine, fanned out over SSE/WebSocket. Slow browser clients dropped/skipped to avoid blocking data path.
- Graceful shutdown: stop accepting new streams, drain in-flight streams up to deadline, close sessions.
- Clean with `go test -race` and profileable with `pprof`.

---

## 10. Tradeoffs & Decisions
- **T1. Multiplexing**: yamux over single long-lived TCP connection.
- **T2. Subdomain routing**: Wildcard DNS + TLS for `*.tunnel.dev`.
- **T3. TLS termination**: Terminate TLS at server (Caddy / autocert).
- **T4. Streaming**: Stream with `io.Copy` / `ReverseProxy`; inspector tees first ~1 MB.
- **T5. Registry**: In-memory with `sync.RWMutex`.
- **T6. Heartbeats**: yamux keepalive + read/write deadlines (~10-30s).
- **T7. Auth**: Static hashed API token (Postgres).
- **T8. Reconnection**: Exponential backoff + jitter; 30-60s grace period for subdomain retention.
- **T9. Abuse controls**: Per-tunnel stream semaphore, max body sizes, token bucket rate limiting.
- **T10. Protocol**: HTTP + WebSocket.
- **T11. Inspector capture**: Client-side capture for privacy and zero server-side body retention.
- **T12. Framing**: HashiCorp yamux.

---

## 11. Failure Scenarios & Handling
- Client dies mid-request -> yamux stream reset, context cancelled, 502 Bad Gateway to visitor.
- Local app down -> client returns 502 with friendly error page.
- Visitor disconnects -> context cancelled, stream and local request closed.
- Network drop -> keepalive timeout, session closed, grace period, auto reconnect.
- Subdomain collision / taken -> clear handshake error.
- Invalid token -> handshake rejected.
- Server restart -> clients reconnect with exponential backoff.
- Upstream stalls -> per-request / response header / idle timeouts.
- Semaphore full -> 503 / 429 response.
- Panics -> deferred recover middleware per request.

---

## 12. Security Considerations
- Control connection over TLS; tokens never logged in plaintext.
- Tokens stored as hashes (SHA-256 / bcrypt).
- Subdomain sanitization (`^[a-z0-9-]{3,30}$`, reserved keywords blocked).
- Explicit `X-Forwarded-For` / `X-Forwarded-Proto` handling.
- Body & header size limits; IP & token rate limiting.

---

## 13. Observability
- Structured logging (`slog`) with `tunnel_id` and `request_id`.
- Prometheus metrics: active tunnels, active streams, total requests, latency histograms, error rates.
- Protected `pprof` endpoint.
- Client inspector on `:4040`.

---

## 14. Data Model (Postgres)
```sql
users(id, email, created_at)
tokens(id, user_id, token_hash, created_at, revoked_at)
subdomains(name, user_id, reserved_at)
tunnel_sessions(id, token_id, subdomain, connected_at, disconnected_at, bytes_in, bytes_out)
```

---

## 15. Tech Stack
- **Go** (`net`, `net/http`, `httputil`, `context`, `sync`, `log/slog`)
- **hashicorp/yamux**
- **PostgreSQL** + `pgx` / `database/sql`
- **Cobra** for CLI
- **Caddy** / **autocert**
- **HTML/JS** (vanilla/clean) for `:4040` Inspector UI
- **Docker & Docker Compose**
- **Prometheus**
