# 📅 Day 10 — Production CLI (Cobra), Graceful Shutdown & Multi-Platform Packaging (Odin Project Style)

> **Rule**: Hum code direct inject nahi karenge. Hum batayenge KYA likhna hai, KIS file mein, KAUNSA function, KAUNSA concept pehle samjho. Har ek function tum khud likhoge.
>
> **Apoorv** = 🔵 | **Gaurav** = 🟢 | **Both** = ⚪

---

## 🗺️ Day 10 Overview: Where We Are & Where We Are Going

Hamaare paas reverse tunnel, grace period, yamux multiplexing, request inspector, abuse controls (semaphore + recovery), aur Prometheus metrics sab ready hain!

Lekin abhi client aur server chalane ke liye hardcoded flags ya `go run cmd/client/main.go` karna padta hai.
**Real world mein developer aise command chalata hai:**
```bash
# Developer laptop pe (Rahul):
tunnel http 3000 --subdomain rahul --server tunnel.myserver.com:9000 --token secret123

# Cloud VPS pe:
tunneld --control-port 9000 --public-port 8080 --metrics-port 9090
```

### Day 10 ka Grand Capstone Mission:
1. **Production CLI with Cobra & Viper (Gaurav 🟢):**
   - Industry-standard CLI interface (`tunnel http <port>`, `tunnel version`, `tunnel status`).
   - Flags, Environment Variables, and config file (`~/.tunnel.yaml`) support.
2. **OS Signal Handling & Zero-Downtime Graceful Shutdown (Apoorv 🔵):**
   - Server ya Client pe `Ctrl+C` (SIGINT/SIGTERM) dabane pe connection abruptly freeze nahi hoga.
   - Server active connections ko drain karega, registry close karega, aur clean 502/reconnect triggers fire karega.
3. **Cross-Compilation & Multi-Arch Binary Packaging (Both ⚪):**
   - Windows (`.exe`), Linux (`amd64`/`arm64`), aur macOS (`darwin-arm64` Apple Silicon) ke standalone binaries build karna.
   - Production Dockerfile & `docker-compose.yml` for VPS deployment.

```
                           DEVELOPER WORKFLOW
                                   │
               ┌───────────────────┴───────────────────┐
               ▼                                       ▼
    [ tunnel http 3000 ]                     [ tunneld (Server) ]
    • Cobra CLI Flags Parsing               • OS SIGINT/SIGTERM Trap
    • Validates target & auth               • Stops new listeners
    • Starts Inspector + TUI                • Drains in-flight requests (5s timeout)
    • Yamux Multiplex Loop                  • Yamux Sessions Closed Cleanly
               │                                       │
               └───────────────► ◄─────────────────────┘
                             (Clean Exit)
```

---

## 🔵 Step 1: The Core Systems Concept — Graceful Shutdown & OS Signals (Apoorv — 20 min)

### 🧠 The Real-World Disaster Without Graceful Shutdown:
Imagine Rahul ek heavy 5MB image upload kar raha hai tunnel ke through.
Server admin maintenance ke liye server restart karta hai (`kill -9` ya unhandled `SIGTERM`):
- Socket bina warning ke forcefully band ho jata hai (`RST` packet).
- Rahul ka upload 98% pe fail ho jata hai!
- Visitor ko browser pe ugly `ERR_CONNECTION_RESET` dikhta hai.

### 🛡️ The Graceful Shutdown Protocol:
1. OS signal catch karo (`os.Interrupt`, `syscall.SIGTERM`).
2. Server ko bolo: **"Naye connections lena band karo"** (`listener.Close()`).
3. Ek deadline context set karo (e.g. `ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)`).
4. Pehle se chal rahe active requests ko finish hone do (`http.Server.Shutdown(ctx)`).
5. Connected Yamux sessions ko gracefully close karo.
6. Exit code `0` ke saath clean exit!

---

## 🔵 Step 2: Graceful Server & Client Lifecycle Manager (Apoorv — 45 min)

### File: `internal/server/lifecycle/shutdown.go`
### Package: `package lifecycle`

### Key Responsibilities:
- `SetupSignalHandler(cancel context.CancelFunc)`:
  Goroutine jo `os.Signal` channel listen karegi. Pehla signal aate hi context cancel hoga, doosra signal aane pe force exit karegi (`os.Exit(1)`).
- `GracefulDrain(ctx context.Context, server *http.Server, reg *registry.Registry)`:
  In-flight streams drain hone ka wait karega within timeout.

### Skeleton for Apoorv to write:
```go
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

// WaitForShutdown blocks until an interrupt signal is received,
// then executes graceful shutdown of listeners and active connections.
func WaitForShutdown(srv *http.Server, timeout time.Duration) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	sig := <-stop
	slog.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server gracefully stopped")
}
```

---

## 🟢 Step 3: Professional Cobra CLI Framework (Gaurav — 45 min)

### File: `cmd/client/cli/root.go` & `cmd/client/cli/http.go`
### Dependency:
```bash
go get github.com/spf13/cobra
```

### CLI Command Design:
```text
tunnel [command]

Available Commands:
  http        Start an HTTP tunnel to a local web server (e.g. tunnel http 3000)
  version     Print the version and build commit of tunnel
  help        Help about any command

Flags:
  -s, --server string      Tunnel server control address (default "localhost:9000")
  -d, --subdomain string   Custom subdomain request (e.g. "rahul")
  -t, --token string       Authentication token
      --inspector int      Local inspector port (default 4040)
```

### Core Execution Flow (`http.go`):
1. User types `tunnel http 3000 --subdomain rahul`.
2. Cobra parses `args[0]` as `"3000"`, validates port is positive integer.
3. Sets up local target URL: `http://localhost:3000`.
4. Connects to server, starts inspector on `:4040`, prints live dashboard, and begins multiplexing loop.

---

## ⚪ Step 4: Docker & Docker-Compose Deployment Bundle (Both — 30 min)

### 1. Multi-Stage Production Dockerfile (`Dockerfile.server`):
```dockerfile
# Stage 1: Build the Go binary statically
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tunneld ./cmd/server

# Stage 2: Minimal scratch / distroless runtime container
FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /tunneld .
EXPOSE 8080 9000 9090
ENTRYPOINT ["./tunneld"]
```

### 2. `docker-compose.yml`:
Tunnel server + Prometheus setup out-of-the-box:
```yaml
version: '3.8'

services:
  tunneld:
    build:
      context: .
      dockerfile: Dockerfile.server
    ports:
      - "8080:8080" # Visitor HTTP Port
      - "9000:9000" # Yamux Control Port
      - "9090:9090" # Prometheus Metrics Port
    restart: unless-stopped

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./deploy/prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9091:9090"
    restart: unless-stopped
```

---

## ⚪ Step 5: Multi-Platform Cross-Compilation Script (Both — 20 min)

### Script: `scripts/build_all.ps1` (or `Makefile`)
Compile binaries for every major OS without needing virtual machines:
```powershell
# Windows
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -o bin/tunnel-windows-amd64.exe ./cmd/client
# Linux AMD64
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o bin/tunnel-linux-amd64 ./cmd/client
# macOS (Apple Silicon M1/M2/M3)
$env:GOOS="darwin"; $env:GOARCH="arm64"; go build -o bin/tunnel-darwin-arm64 ./cmd/client
```

---

## 🧪 Verification & Acceptance Criteria:

1. **Clean Shutdown Test:**
   Start server, connect client, send long request, press `Ctrl+C`. Verify:
   - Server logs `"shutdown signal received"`.
   - In-flight request completes successfully.
   - Port `:8080` and `:9000` released immediately without lingering sockets.
2. **CLI Test:**
   `./bin/tunnel http 3000 --subdomain rahul` starts properly and gives clear error if arguments are missing.
3. **Docker Test:**
   `docker compose up` starts server and scrapes Prometheus metrics cleanly.

