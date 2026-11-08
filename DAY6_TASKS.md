# 📅 Day 6 — Reliability, Auto-Reconnect with Exponential Backoff & Server Grace Period (Odin Project Style)

> **Rule**: Hum code direct inject nahi karenge. Hum batayenge KYA likhna hai, KIS file mein, KAUNSA function, KAUNSA concept pehle samjho. Har ek function tum khud likhoge.
>
> **Apoorv** = 🔵 | **Gaurav** = 🟢 | **Both** = ⚪

---

## 🗺️ Day 6 Overview: Where We Are & Where We Are Going

Day 5 mein humne ek working reverse tunnel bana liya:
Internet se request aati hai -> Server Yamux stream kholta hai -> Client use `localhost:3000` pe forward karta hai.

Lekin socho: **Real World mein Network stable nahi hota!**
1. Developer college WiFi pe hai — har 15 minute mein network flicker hota hai.
2. Laptop ka dhakkan 2 minute ke liye band kiya (Lid Close).
3. Server restart hua deployment ke liye.

**Current Behavior (Disaster):**
- Client crash ho jata hai ya exit ho jata hai.
- Developer ko baar-baar terminal mein jaake command rerun karni padti hai.
- Aur sabse bura: Server turant subdomain kisi aur ko de deta hai ya delete kar deta hai!

**Day 6 ka Mission:**
Hum apne system ko **production-grade bulletproof** banayenge:
1. **Client Auto-Reconnect Engine** — Exponential Backoff aur Full Jitter ke saath.
2. **Server Subdomain Grace Period** — Agar client disconnect ho, toh 45 seconds tak uska subdomain sambhal kar rakho taaki reconnect hone pe wahi URL wapas mile!

```
[WiFi Drops] ──> Client detects disconnect
                      │
                      ├── Attempt 1: Wait 1.2s  ──> Fail
                      ├── Attempt 2: Wait 2.8s  ──> Fail
                      ├── Attempt 3: Wait 4.1s  ──> Connected!
                      │
                      └── Server Handshake: "Welcome back! Subdomain 'gaurav' was held for you."
```

---

## 🟢 Step 1: The Systems Concept — Exponential Backoff, Jitter & Thundering Herd (Gaurav — 20 min)

### 🧠 The Mental Model:

#### 1. Why NOT a simple retry loop?
```go
// ❌ THE ROOKIE MISTAKE:
for {
    err := Connect()
    if err != nil {
        time.Sleep(1 * time.Second) // Har 1 sec mein retry
    }
}
```
Agar 10,000 developers ngrok use kar rahe hain aur server 2 second ke liye restart hota hai:
10,000 clients **har ek second mein server pe attack karenge**! Server jaise hi start hoga, 10,000 requests ek saath aayengi aur server dobara crash ho jayega. Isko bolte hain **Thundering Herd Problem**.

#### 2. The Solution: Exponential Backoff
Har failure ke baad wait time double hota jata hai:
$$\text{Wait} = \text{Base} \times 2^{\text{attempt}}$$
- Attempt 1: $1 \times 2^0 = 1\text{s}$
- Attempt 2: $1 \times 2^1 = 2\text{s}$
- Attempt 3: $1 \times 2^2 = 4\text{s}$
- Attempt 4: $1 \times 2^3 = 8\text{s}$
- Max Cap: $30\text{s}$ (taaki wait time infinite na ho jaye).

#### 3. Full Jitter (The Secret Sauce by AWS):
Agar 1,000 clients ek saath disconnect hue, toh sabka wait time 1s, 2s, 4s same rahega — matlab sab fir bhi ek saath hit karenge!
**Jitter** random noise add karta hai:
$$\text{SleepTime} = \text{rand}(0, \text{Backoff})$$
Isse 1,000 clients ka retry time poore timeline pe fail jata hai. Server pe zero load spike!

---

## 🟢 Step 2: Exponential Backoff Retry Utility (Gaurav — 35 min)

### File: `internal/client/reconnect.go`
### Package: `package client`

### Struct & Function: `Backoff`

```go
package client

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"time"
)

// BackoffConfig controls retry parameters.
type BackoffConfig struct {
	BaseDelay time.Duration // e.g. 1 second
	MaxDelay  time.Duration // e.g. 30 seconds
	MaxRetries int          // 0 means retry forever
}

// RetryWithBackoff executes an operation repeatedly with exponential backoff and jitter.
func RetryWithBackoff(ctx context.Context, cfg BackoffConfig, opName string, op func() error) error {
	attempt := 0

	for {
		attempt++
		err := op()
		if err == nil {
			return nil // Operation succeeded!
		}

		// Context cancellation check (e.g. user pressed Ctrl+C)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if cfg.MaxRetries > 0 && attempt >= cfg.MaxRetries {
			return err
		}

		// 1. Calculate Exponential Backoff: Base * 2^(attempt-1)
		multiplier := math.Pow(2, float64(attempt-1))
		calculated := float64(cfg.BaseDelay) * multiplier

		// 2. Cap at MaxDelay
		if calculated > float64(cfg.MaxDelay) {
			calculated = float64(cfg.MaxDelay)
		}

		// 3. Add Full Jitter: random duration between 0 and calculated
		jittered := time.Duration(rand.Float64() * calculated)

		slog.Warn("operation failed, backing off before retry",
			"operation", opName,
			"attempt", attempt,
			"retry_in", jittered.Round(time.Millisecond).String(),
			"err", err,
		)

		// 4. Sleep with context awareness
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jittered):
		}
	}
}
```

---

### 🔍 Line-by-Line Breakdown:

- **`select { case <-ctx.Done(): ... case <-time.After(jittered): }`**:
  - **Super Important:** Agar client 15 second ke backoff sleep mein hai aur developer terminal mein `Ctrl+C` daba deta hai, toh normal `time.Sleep` program ko 15 second tak freeze rakhta hai.
  - Lekin `select` with `ctx.Done()` turant cancel signal pakad leta hai aur bina wait kiye instant exit karta hai!
- **`rand.Float64() * calculated`**:
  - Uniform random distribution jo AWS Architecture blog ki recommended "Full Jitter" strategy follow karti hai.

---

## 🟢 Step 3: Unit Testing Backoff & Jitter (Gaurav — 20 min)

### File: `internal/client/reconnect_test.go`
### Package: `package client_test`

### Test 1: Success on First Try
- Pass karo ek function jo turant `nil` return kare.
- Assert karo ki `RetryWithBackoff` zero delay ke sath 1 attempt mein `nil` return kare.

### Test 2: Eventual Success
- Ek counter variable rakho. Function pehle 2 baar error dega, 3rd time pe `nil` dega.
- Assert karo ki total 3 attempts hue aur final error `nil` aaya.

### Test 3: Context Cancellation
- Ek cancelled context pass karo.
- Verify karo ki function turant `context.Canceled` return kare, bina sleep kiye.

### Verify:
```bash
go test -v ./internal/client/...
```
- [ ] All backoff unit tests pass.

---

## 🔵 Step 4: Server-Side Subdomain Grace Period (Apoorv — 40 min)

### 🧠 The Problem on Server Side:
Abhi tak humne Day 4 mein `Registry.Delete(subdomain)` banaya tha jo connection drop hote hi subdomain turant delete kar deta tha.
Agar Gaurav ka laptop 5 second ke liye WiFi disconnect kare, aur koi aur banda usi second connect karke `"gaurav"` maang le, toh Gaurav ka subdomain chhin jayega!

**The Solution: 45-Second Grace Period**
Jab client disconnect hota hai:
1. Subdomain ko turant delete mat karo.
2. Uspe ek `ExpiresAt` timer lagao (e.g. `Now + 45s`).
3. Status set karo: `StateGracePeriod`.
4. Agar wahi client (matching auth token) 45s ke andar wapas connect kare, toh uska session instantly re-attach kar do!
5. Agar 45s tak koi na aaye, tab background cleaner goroutine use permanently delete karegi.

---

### File: `internal/server/registry/registry.go` (Update existing file)

### Struct Update:
```go
type TunnelState string

const (
	StateActive      TunnelState = "ACTIVE"
	StateGracePeriod TunnelState = "GRACE_PERIOD"
)

type Tunnel struct {
	Subdomain   string
	Session     *yamux.Session
	ClientAddr  string
	Token       string
	State       TunnelState
	CreatedAt   time.Time
	GraceExpiry time.Time
}
```

### Function Update: `MarkGracePeriod(subdomain string, graceDuration time.Duration)`
```go
// MarkGracePeriod marks a tunnel as disconnected but holds the subdomain reservation.
func (r *Registry) MarkGracePeriod(subdomain string, graceDuration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tunnel, ok := r.tunnels[subdomain]
	if !ok {
		return
	}

	tunnel.State = StateGracePeriod
	tunnel.GraceExpiry = time.Now().Add(graceDuration)
	tunnel.Session = nil // Clear dead session

	slog.Info("tunnel entered grace period",
		"subdomain", subdomain,
		"hold_duration", graceDuration.String(),
	)
}
```

### Function Update: `Reattach(subdomain string, token string, newSession *yamux.Session, addr string) error`
```go
// Reattach reclaims a tunnel in grace period if the token matches.
func (r *Registry) Reattach(subdomain string, token string, newSession *yamux.Session, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tunnel, ok := r.tunnels[subdomain]
	if !ok {
		return errors.New("tunnel not found in registry")
	}

	if tunnel.Token != token {
		return errors.New("unauthorized: token does not match original tunnel owner")
	}

	tunnel.State = StateActive
	tunnel.Session = newSession
	tunnel.ClientAddr = addr
	tunnel.GraceExpiry = time.Time{} // Clear expiry

	slog.Info("tunnel successfully reattached from grace period!",
		"subdomain", subdomain,
		"client_addr", addr,
	)
	return nil
}
```

### Background Cleaner: `StartCleaner(ctx context.Context, interval time.Duration)`
```go
// StartCleaner periodically removes expired tunnels from grace period.
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
					slog.Info("grace period expired, subdomain permanently released", "subdomain", sub)
				}
			}
			r.mu.Unlock()
		}
	}
}
```

---

## 🟢 Step 5: Wire Reconnect Loop into Client (`cmd/client/main.go`) (Gaurav — 25 min)

Ab client ke `main()` ko retry loop ke andar daal do:

```go
func main() {
	config.SetupLogger(true)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	token := "secret123"
	desiredSubdomain := "gaurav"
	localTarget := "localhost:3000"

	bCfg := client.BackoffConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  15 * time.Second,
	}

	err := client.RetryWithBackoff(ctx, bCfg, "TunnelSession", func() error {
		// 1. Dial Server
		conn, err := client.Connect("localhost:9000")
		if err != nil {
			return err
		}
		defer conn.Close()

		// 2. Handshake
		authResp, err := client.PerformClientHandshake(conn, token, desiredSubdomain)
		if err != nil {
			return err
		}

		slog.Info("tunnel connected", "url", authResp.AssignedURL)

		// 3. Yamux Session
		session, err := client.NewYamuxClientSession(conn)
		if err != nil {
			return err
		}
		defer session.Close()

		// 4. Accept Streams (Blocks until disconnect)
		client.ListenForServerStreams(session, localTarget)

		return errors.New("server closed session unexpectedly")
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("fatal tunnel error", "err", err)
	}
}
```

---

## ⚪ Step 6: The Ultimate WiFi-Kill Simulation Test (Both — 25 min)

### 🧪 Experiment 1: The Server Restart Test
1. Server start karo (`go run ./cmd/server`).
2. Client start karo (`go run ./cmd/client`).
   - Client connected: `https://gaurav.tunnel.dev`
3. **Server terminal mein `Ctrl+C` dabao (Simulate Server Crash)!**
   - Client terminal dekho:
     `operation failed, backing off before retry attempt=1 retry_in=840ms`
     `operation failed, backing off before retry attempt=2 retry_in=1.8s`
4. 4 second baad **Server wapas start karo!**
   - Client automatically reconnect ho jayega!
   - Console: `tunnel successfully reattached! url=https://gaurav.tunnel.dev`
   - Zero human intervention! 🤖

### 🧪 Experiment 2: The Grace Period Subdomain Hijack Test
1. Client 1 (`token=secret123`, `subdomain=gaurav`) connect karo aur disconnect karo.
2. Within 20 seconds, Client 2 (`token=hacker`, `subdomain=gaurav`) se connect karne ki koshish karo.
3. Server should reject Client 2: `"subdomain 'gaurav' is reserved in grace period"`.
4. Client 1 reconnect karega toh allow ho jayega!

---

## Day 6 Checklist

### Files Created/Modified:
- [ ] `internal/client/reconnect.go` — Exponential Backoff + Full Jitter with context awareness
- [ ] `internal/client/reconnect_test.go` — Unit tests for backoff math & cancellation
- [ ] `internal/server/registry/registry.go` — Grace period states (`ACTIVE`, `GRACE_PERIOD`), `Reattach`, and background cleaner ticker
- [ ] `cmd/client/main.go` — Reconnect loop wrapping session lifecycle

### Concepts Mastered:
- [ ] Thundering Herd Problem & why fixed delays are banned in distributed systems
- [ ] Full Jitter math: `rand(0, min(Max, Base * 2^n))`
- [ ] Context-aware sleeping with `select` and `time.After`
- [ ] State machines in registries (`ACTIVE` -> `GRACE_PERIOD` -> `EXPIRED`)
- [ ] Background cleanup loops with Go's `time.NewTicker`

