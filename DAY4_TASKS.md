# Day 4 — In-Memory Registry & HTTP Virtual Host Routing (Odin Project Style)

> **Rule**: Hum code direct inject nahi karenge. Hum batayenge KYA likhna hai, KIS file mein, KAUNSA function, KAUNSA concept pehle samjho. Har ek function tum khud likhoge.
>
> **Apoorv** = Apoorv | **Gaurav** = Gaurav | **Both** = Both

---

## Day 4 Overview: Where We Are & Where We Are Going

Day 3 mein humne Yamux multiplexing set up kiya:
- Server ek `net.Conn` ko `yamux.Server` session mein wrap karta hai.
- Client ek `net.Conn` ko `yamux.Client` session mein wrap karta hai.
- Server `session.OpenStream()` se virtual channels khol sakta hai, client `session.AcceptStream()` se unhe receive karta hai.
- Sab parallel chalta hai bina Head-of-Line blocking ke.

Lekin abhi tak ek critical problem hai:

> **Problem:** Jab koi visitor `https://gaurav.tunnel.dev/api/data` pe request bhejta hai, server ko kaise pata chalega ki "gaurav" subdomain kaunse client ka hai aur kaunsi yamux session se iska data forward karna hai?

**Day 4 ka Mission:**
1. **In-Memory Tunnel Registry** banayenge — ek thread-safe `map[string]*Tunnel` jo subdomain ko yamux session se link karega.
2. **HTTP Virtual Host Router** banayenge — jo visitor ke HTTP request se `Host` header padhega, subdomain extract karega, aur registry mein lookup karega.

```
Visitor hits:  https://gaurav.tunnel.dev/webhook
                         |
                         v
              +---------------------+
              |   HTTP Listener     |
              |     (:8080)         |
              +----------+----------+
                         |
                   Host header parse:
                   "gaurav.tunnel.dev"
                         |
                   Extract subdomain:
                   "gaurav"
                         |
                         v
              +---------------------+
              |     REGISTRY        |
              |  "gaurav" -> Tunnel  |
              |  "apoorv" -> Tunnel  |
              |  "rahul"  -> Tunnel  |
              +----------+----------+
                         |
                  Tunnel found!
                  Get yamux session
                         |
                         v
              session.OpenStream()
              Forward request to Client
```

---

## Both — Step 0: Day 3 Code Sanity & Git Branch (10 min)

```bash
cd go-ngrok
go build ./cmd/server
go build ./cmd/client
```

Agar dono bina error ke compile hote hain:
```bash
git checkout -b day4/registry-routing
```

---

## Apoorv — Step 1: Thread-Safe Tunnel Registry (40 min)

### Concept samjho pehle (15 min):

**The Concurrency Challenge:**
Registry ko do situations mein access hota hai:
1. **Read (har visitor request pe):** Jab koi `gaurav.tunnel.dev` kholega, server ko registry mein `"gaurav"` dhoondhna padega. Yeh **hazaro baar per second** ho sakta hai.
2. **Write (sirf connect/disconnect pe):** Jab client connect hota hai (`Register`) ya disconnect hota hai (`Delete`). Yeh **bohot rare** event hai (din mein 10-50 baar).

**The Wrong Solution: `sync.Mutex`**
Agar hum normal Mutex use karein, toh har ek reader ko bhi lock lena padega. 1,000 concurrent visitors aayenge, sab ek dusre ka wait karenge (serialized). Performance destroy!

**The Right Solution: `sync.RWMutex` (Reader-Writer Lock)**
- **Multiple Readers Simultaneously:** 1,000 goroutines ek saath `RLock()` le sakte hain aur map padhte hain. Koi kisi ka wait nahi karta.
- **Exclusive Writer:** Jab ek writer `Lock()` leta hai, toh sab readers aur writers wait karte hain jab tak writer kaam khatam na kar de. Yeh safe hai kyunki writes bohot rare hote hain.

```
Normal Mutex:        Reader1 -> wait -> Reader2 -> wait -> Reader3 -> SLOW!
RWMutex (Reads):     Reader1, Reader2, Reader3 -> ALL READ AT SAME TIME -> FAST!
RWMutex (Write):     Writer1 -> (all readers wait) -> done -> readers resume
```

### File: `internal/server/registry/registry.go`
### Package: `package registry`

### Struct Definition:
```go
package registry

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// Tunnel represents a single registered client tunnel.
type Tunnel struct {
	Subdomain  string
	Session    *yamux.Session
	ClientAddr string
	CreatedAt  time.Time
}

// Registry holds the in-memory mapping of subdomain -> active tunnel.
type Registry struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel
}
```

### Function 1: `NewRegistry() *Registry`
```go
// NewRegistry creates an empty tunnel registry.
func NewRegistry() *Registry {
	return &Registry{
		tunnels: make(map[string]*Tunnel),
	}
}
```

**Why a constructor function?**
Go mein maps ka zero value `nil` hota hai. Agar hum seedha `Registry{}` banayein bina `make()` ke, toh pehli baar `tunnels["gaurav"] = ...` karte hi runtime **nil pointer panic** aa jayega. Constructor `make()` guarantee karta hai ki map initialized hai.

---

### Function 2: `Register(subdomain string, session *yamux.Session, addr string) error`
```go
// Register adds a tunnel to the registry. Returns error if subdomain already taken.
func (r *Registry) Register(subdomain string, session *yamux.Session, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if subdomain already occupied by another client
	if existing, ok := r.tunnels[subdomain]; ok {
		return fmt.Errorf("subdomain '%s' already registered by client %s", subdomain, existing.ClientAddr)
	}

	r.tunnels[subdomain] = &Tunnel{
		Subdomain:  subdomain,
		Session:    session,
		ClientAddr: addr,
		CreatedAt:  time.Now(),
	}

	slog.Info("tunnel registered", "subdomain", subdomain, "client_addr", addr)
	return nil
}
```

**Line-by-line:**
- `r.mu.Lock()`: Exclusive write lock liya. Koi aur reader ya writer access nahi kar sakta jab tak hum kaam khatam na karein.
- `defer r.mu.Unlock()`: Function chahe normally exit kare ya error return kare, lock release hona guaranteed hai.
- `if existing, ok := r.tunnels[subdomain]; ok`: Go ka **comma-ok idiom**. Agar key already map mein hai toh `ok = true` aur `existing` mein purana Tunnel aa jayega. Isse hum subdomain collision detect karte hain.

---

### Function 3: `Get(subdomain string) (*Tunnel, bool)`
```go
// Get retrieves a tunnel by subdomain. Thread-safe for concurrent reads.
func (r *Registry) Get(subdomain string) (*Tunnel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tunnel, ok := r.tunnels[subdomain]
	return tunnel, ok
}
```

**Critical Observation:** Yahan `RLock()` hai, `Lock()` nahi!
`RLock` = Read Lock. 10,000 goroutines simultaneously `Get()` call kar sakti hain bina kisi wait ke. Yahi `sync.RWMutex` ki power hai.

---

### Function 4: `Delete(subdomain string)`
```go
// Delete removes a tunnel from the registry (on client disconnect).
func (r *Registry) Delete(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tunnels[subdomain]; ok {
		delete(r.tunnels, subdomain)
		slog.Info("tunnel deregistered", "subdomain", subdomain)
	}
}
```

---

### Function 5: `ActiveCount() int`
```go
// ActiveCount returns how many tunnels are currently connected.
func (r *Registry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tunnels)
}
```

### Verify:
```bash
go build ./internal/server/registry/...
```
- [ ] Compiles with zero errors.

---

## Apoorv — Step 2: Registry Unit Test with Race Detector (30 min)

### Concept samjho pehle:
> Go ka **Race Detector** (`go test -race`) ek built-in tool hai jo runtime pe concurrent memory access bugs dhoondhta hai. Production Go projects mein yeh CI/CD pipeline mein mandatory hota hai.

### File: `internal/server/registry/registry_test.go`
### Package: `package registry`

### Test 1: `TestRegistry_RegisterAndGet(t *testing.T)`
1. `NewRegistry()` se registry banao.
2. Ek dummy `yamux.Session` ke liye `nil` pass karo (hum sirf map logic test kar rahe hain, yamux actual stream test Day 3 mein hua).
3. `Register("gaurav", nil, "127.0.0.1:5000")` call karo.
4. `Get("gaurav")` call karo — check karo `ok == true` aur `tunnel.ClientAddr == "127.0.0.1:5000"`.
5. `Get("nonexistent")` call karo — check karo `ok == false`.

### Test 2: `TestRegistry_DuplicateSubdomain(t *testing.T)`
1. `Register("gaurav", nil, "addr1")` karo — should succeed.
2. `Register("gaurav", nil, "addr2")` karo — should return error (subdomain already taken).

### Test 3: `TestRegistry_DeleteAndReuse(t *testing.T)`
1. `Register("gaurav", ...)` karo.
2. `Delete("gaurav")` karo.
3. `Get("gaurav")` karo — `ok == false` hona chahiye.
4. `Register("gaurav", ..., "new_addr")` karo — should succeed again (subdomain free ho gaya).

### Test 4: `TestRegistry_ConcurrentAccess(t *testing.T)`
Sabse important test:
1. 100 goroutines launch karo jo simultaneously `Register`, `Get`, aur `Delete` call karein.
2. `sync.WaitGroup` se sab goroutines ko wait karo.
3. Koi panic ya data race nahi hona chahiye.

### Verify:
```bash
go test -v -race ./internal/server/registry/...
```
- [ ] All tests pass.
- [ ] `WARNING: DATA RACE` kabhi nahi dikhna chahiye.

---

## Gaurav — Step 3: Host Header Parser & Subdomain Extractor (35 min)

### Concept samjho pehle (10 min):

**HTTP Host Header kya hai?**
Jab browser `https://gaurav.tunnel.dev/api/users` kholata hai, browser server ko yeh HTTP request bhejta hai:
```http
GET /api/users HTTP/1.1
Host: gaurav.tunnel.dev
Accept: text/html
```

`Host` header batata hai ki client kis domain pe request bhej raha hai. Ek hi server (ek hi IP address) 50 alag-alag subdomains serve kar sakta hai agar woh `Host` header padhe. Isi technique ko **Virtual Hosting** kehte hain (Apache, Nginx sab yahi karte hain).

**Humara Kaam:**
`"gaurav.tunnel.dev"` string se `"gaurav"` extract karna.

**Edge Cases (Corner Cases) jo handle karne hain:**
1. Port number ke saath: `"gaurav.tunnel.dev:8080"` -> `"gaurav"` (port ignore karo)
2. Root domain directly: `"tunnel.dev"` -> Error! (koi subdomain nahi hai)
3. IP address: `"192.168.1.1"` -> Error! (IP pe subdomain nahi hota)
4. Multi-level: `"api.gaurav.tunnel.dev"` -> `"api.gaurav"` ya `"api"` ya error? (Hum sirf first level support karenge)
5. Empty string: `""` -> Error!

### File: `internal/server/router/router.go`
### Package: `package router`

### Function: `ExtractSubdomain(hostHeader string, rootDomain string) (string, error)`

```go
package router

import (
	"fmt"
	"net"
	"strings"
)

// ExtractSubdomain parses the Host header and extracts the subdomain prefix.
// Example: hostHeader="gaurav.tunnel.dev:8080", rootDomain="tunnel.dev" -> "gaurav"
func ExtractSubdomain(hostHeader string, rootDomain string) (string, error) {
	// 1. Strip port number if present (e.g. "gaurav.tunnel.dev:8080" -> "gaurav.tunnel.dev")
	host, _, err := net.SplitHostPort(hostHeader)
	if err != nil {
		// SplitHostPort fails when there is no port — that's fine, use as-is
		host = hostHeader
	}

	// 2. Lowercase for case-insensitive comparison
	host = strings.ToLower(host)
	rootDomain = strings.ToLower(rootDomain)

	// 3. Check if host ends with our root domain
	suffix := "." + rootDomain
	if !strings.HasSuffix(host, suffix) {
		return "", fmt.Errorf("host '%s' is not a subdomain of '%s'", host, rootDomain)
	}

	// 4. Extract subdomain by trimming the suffix
	subdomain := strings.TrimSuffix(host, suffix)

	// 5. Validate non-empty
	if subdomain == "" {
		return "", fmt.Errorf("no subdomain found in host '%s'", hostHeader)
	}

	return subdomain, nil
}
```

**Line-by-line breakdown:**
- `net.SplitHostPort(hostHeader)`: Go standard library ka function jo `"host:port"` ko alag-alag karta hai. Agar port nahi hai toh error return karta hai, lekin humein woh error ignore karna hai (kyunki port optional hai).
- `strings.ToLower(host)`: DNS case-insensitive hota hai. `"GAURAV.Tunnel.Dev"` aur `"gaurav.tunnel.dev"` same hai.
- `strings.HasSuffix(host, suffix)`: Check karta hai ki host humare root domain se end hota hai ya nahi. Agar koi random domain (`"google.com"`) pe hit kare toh reject.
- `strings.TrimSuffix(host, suffix)`: Root domain portion hata ke sirf subdomain bacha lena. `"gaurav.tunnel.dev"` minus `".tunnel.dev"` = `"gaurav"`.

---

## Gaurav — Step 4: Table-Driven Unit Tests for Router (25 min)

### Concept samjho pehle:
> **Table-Driven Tests** Go mein testing ka gold standard pattern hai. Ek test function ke andar tum ek slice of test cases define karte ho aur loop mein sabko run karte ho. Agar baad mein naya edge case aaye, toh sirf table mein ek aur entry add karni hoti hai, naya test function nahi likhna padta.

### File: `internal/server/router/router_test.go`
### Package: `package router`

```go
package router_test

import (
	"testing"

	"github.com/apoorvslst/go-ngrok/internal/server/router"
)

func TestExtractSubdomain(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		rootDomain string
		want       string
		wantErr    bool
	}{
		// Happy path cases
		{name: "simple subdomain", host: "gaurav.tunnel.dev", rootDomain: "tunnel.dev", want: "gaurav", wantErr: false},
		{name: "with port", host: "gaurav.tunnel.dev:8080", rootDomain: "tunnel.dev", want: "gaurav", wantErr: false},
		{name: "uppercase mixed", host: "GAURAV.Tunnel.Dev", rootDomain: "tunnel.dev", want: "gaurav", wantErr: false},

		// Error cases
		{name: "root domain only", host: "tunnel.dev", rootDomain: "tunnel.dev", want: "", wantErr: true},
		{name: "wrong domain", host: "gaurav.google.com", rootDomain: "tunnel.dev", want: "", wantErr: true},
		{name: "empty host", host: "", rootDomain: "tunnel.dev", want: "", wantErr: true},
		{name: "IP address", host: "192.168.1.1", rootDomain: "tunnel.dev", want: "", wantErr: true},

		// Edge cases
		{name: "hyphenated subdomain", host: "my-app.tunnel.dev", rootDomain: "tunnel.dev", want: "my-app", wantErr: false},
		{name: "numeric subdomain", host: "123.tunnel.dev", rootDomain: "tunnel.dev", want: "123", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := router.ExtractSubdomain(tt.host, tt.rootDomain)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

**Key pattern:**
- `t.Run(tt.name, func(t *testing.T) { ... })`: Sub-tests banata hai. Output mein har test case ka naam dikhega: `TestExtractSubdomain/simple_subdomain`, `TestExtractSubdomain/with_port`, etc.
- Naya edge case mila? Bas table mein ek aur `{}` add karo. Clean!

### Verify:
```bash
go test -v ./internal/server/router/...
```
- [ ] All 9 test cases pass.

---

## Apoorv — Step 5: Public HTTP Listener (30 min)

### Concept samjho pehle:
> Server ke paas **do listeners** hone chahiye:
> 1. **Control Listener (`:9000`)** — Clients (laptops) yahan connect hote hain, handshake karte hain, yamux session start hoti hai. Yeh Day 1-3 mein bana.
> 2. **Public HTTP Listener (`:8080`)** — Internet se visitors (browsers, Stripe webhooks) yahan hit karte hain. Yeh hum abhi banayenge.
>
> Go ka `net/http` package ek built-in high-performance HTTP server deta hai jo internally goroutine-per-request model follow karta hai.

### File: `internal/server/httpserver/server.go`
### Package: `package httpserver`

### Function: `StartPublicListener(addr string, reg *registry.Registry, rootDomain string) error`

```go
package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/apoorvslst/go-ngrok/internal/server/registry"
	"github.com/apoorvslst/go-ngrok/internal/server/router"
)

// StartPublicListener starts the HTTP server that receives visitor requests.
func StartPublicListener(addr string, reg *registry.Registry, rootDomain string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract subdomain from Host header
		subdomain, err := router.ExtractSubdomain(r.Host, rootDomain)
		if err != nil {
			slog.Warn("invalid host", "host", r.Host, "err", err)
			http.Error(w, "Invalid tunnel URL", http.StatusBadRequest)
			return
		}

		// 2. Look up tunnel in registry
		tunnel, ok := reg.Get(subdomain)
		if !ok {
			slog.Info("tunnel not found", "subdomain", subdomain)
			http.Error(w, "Tunnel not found or offline", http.StatusNotFound)
			return
		}

		// 3. For now, just confirm the lookup works
		slog.Info("request routed",
			"subdomain", subdomain,
			"client_addr", tunnel.ClientAddr,
			"method", r.Method,
			"path", r.URL.Path,
		)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Tunnel found! Proxying will be implemented on Day 5.\n"))
	})

	slog.Info("public HTTP listener starting", "addr", addr)
	return http.ListenAndServe(addr, mux)
}
```

**Line-by-line breakdown:**
- `http.NewServeMux()`: Go ka HTTP request multiplexer (router). URL patterns ko handler functions se map karta hai.
- `mux.HandleFunc("/", ...)`: Catch-all handler. Koi bhi URL aaye (`/webhook`, `/api/users`, `/favicon.ico`), yahi function chalega.
- `r.Host`: HTTP request se `Host` header nikalta hai (e.g. `"gaurav.tunnel.dev"`).
- `reg.Get(subdomain)`: Registry mein lookup. Thread-safe `RLock` ke saath.
- `http.Error(w, msg, statusCode)`: Standard HTTP error response bhejta hai.
- `http.ListenAndServe(addr, mux)`: Server start karta hai. Yeh blocking call hai.

---

## Both — Step 6: Integration Wiring & Full Test (30 min)

### Update `cmd/server/main.go`:
Server ko ab do goroutines mein chalna chahiye:
1. Goroutine 1: Control Listener (`:9000`) — clients connect hote hain.
2. Goroutine 2: Public HTTP Listener (`:8080`) — visitors hit karte hain.

```go
// Create registry (shared between both listeners)
reg := registry.NewRegistry()

// Start public HTTP listener in background goroutine
go func() {
	if err := httpserver.StartPublicListener(":8080", reg, "tunnel.dev"); err != nil {
		slog.Error("public listener failed", "err", err)
	}
}()

// After successful handshake + yamux session, register the tunnel:
err := reg.Register(hello.DesiredSubdomain, yamuxSession, conn.RemoteAddr().String())
if err != nil {
	slog.Error("failed to register tunnel", "err", err)
}
```

### Verify — End-to-End Test:
```bash
# Terminal 1: Start server
go run ./cmd/server
# Output: public HTTP listener starting addr=:8080
# Output: control listener starting addr=:9000

# Terminal 2: Start client (connects, handshakes, gets "gaurav" subdomain)
go run ./cmd/client

# Terminal 3: Simulate visitor (pretend to be gaurav.tunnel.dev)
curl -H "Host: gaurav.tunnel.dev" http://localhost:8080/webhook
# Expected: "Tunnel found! Proxying will be implemented on Day 5."

curl -H "Host: nonexistent.tunnel.dev" http://localhost:8080/webhook
# Expected: "Tunnel not found or offline" (404)
```

---

## Both — Step 7: Experiments (15 min)

| Experiment | Kya karo | Observe karo |
|---|---|---|
| **E1** | Client connect karo, visitor request bhejo, phir client disconnect karo. Phir visitor request bhejo. | Pehli baar "Tunnel found", doosri baar "Tunnel not found" aana chahiye. |
| **E2** | 2 clients connect karo alag-alag subdomains ke saath (`gaurav` aur `apoorv`). Dono ko separately curl karo. | Dono independently route hone chahiye. |
| **E3** | Same subdomain se 2 clients connect karne ki koshish karo. | Doosre client ko "subdomain already registered" error aana chahiye. |
| **E4** | `go test -race ./internal/server/registry/...` run karo with 1000 goroutines. | Zero data races. |

---

## Day 4 Checklist

### Files Created/Modified:

| File | Kaun | Contains |
|:---|:---:|:---|
| `internal/server/registry/registry.go` | Apoorv | `Tunnel` struct, `Registry` with `sync.RWMutex`, Register/Get/Delete/ActiveCount |
| `internal/server/registry/registry_test.go` | Apoorv | Concurrent access tests with race detector |
| `internal/server/router/router.go` | Gaurav | `ExtractSubdomain(host, rootDomain)` — Host header parser |
| `internal/server/router/router_test.go` | Gaurav | Table-driven tests for 9 URL edge cases |
| `internal/server/httpserver/server.go` | Apoorv | Public HTTP listener with registry lookup |
| `cmd/server/main.go` | Both | Two listeners (control + HTTP) with shared registry |

### Concepts Mastered:
- [ ] `sync.RWMutex` — Many readers, exclusive writer, zero contention for reads
- [ ] Go's comma-ok idiom for map lookups
- [ ] Constructor pattern (`NewRegistry()`) to avoid nil map panics
- [ ] HTTP `Host` header and Virtual Hosting
- [ ] `net.SplitHostPort` for port-agnostic parsing
- [ ] Table-driven testing pattern
- [ ] Race detector (`go test -race`)
- [ ] Two-listener architecture (Control + Public)

---

*Day 4 khatam hone pe batao. Day 5 mein ReverseProxy trick aur client-side local forwarding implement karenge — visitor ka request seedha tumhare `localhost:3000` pe pahunchega!*

