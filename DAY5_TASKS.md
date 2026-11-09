# Day 5 — End-to-End Reverse Proxying (The Yamux DialContext Trick) (Odin Project Style)

> **Rule**: Hum code direct inject nahi karenge. Hum batayenge KYA likhna hai, KIS file mein, KAUNSA function, KAUNSA concept pehle samjho. Har ek function tum khud likhoge.
>
> **Apoorv** = Apoorv | **Gaurav** = Gaurav | **Both** = Both

---

## Day 5 Overview: Where We Are & Where We Are Going

Day 4 mein humne:
- **Registry** banayi (`subdomain -> yamux.Session` mapping)
- **Router** banaya (Host header se subdomain extract)
- **Public HTTP Listener** (`:8080`) banaya jo visitor ke request pe registry lookup karta hai

Lekin abhi tak visitor ke request ka response sirf `"Tunnel found!"` tha — actual data `localhost:3000` tak nahi gaya!

**Day 5 ka Mission: The Complete Data Path.**
Aaj hum visitor ka HTTP request server se yamux stream ke zariye client tak pahunchaayenge, aur client se `localhost:3000` pe chal rahi local app tak forward karenge.

Yeh hai pura end-to-end flow jo hum aaj banayenge:

```
 VISITOR (Browser/Stripe)
       |
       | HTTP Request: POST /webhook
       | Host: gaurav.tunnel.dev
       v
+------+------+
|   SERVER    |
|  :8080      |
|             |   1. Host header -> Extract "gaurav"
|             |   2. Registry.Get("gaurav") -> yamux.Session
|             |   3. ReverseProxy.DialContext -> session.OpenStream()
|             |   4. HTTP Request flows over yamux stream
+------+------+
       |
       | ======= yamux stream (inside single TCP conn) =======
       |
+------+------+
|   CLIENT    |
|  (Laptop)   |
|             |   5. session.AcceptStream() receives the stream
|             |   6. net.Dial("tcp", "localhost:3000")
|             |   7. io.Copy bidirectional: stream <-> local conn
+------+------+
       |
       | Local TCP connection
       v
+------+------+
|  LOCAL APP  |
|  :3000      |
|  (Express,  |
|   Flask,    |
|   Go etc)   |
+-------------+
       |
       | HTTP Response: 200 OK { "status": "payment_received" }
       |
       v (Response travels back the same path in reverse)
```

---

## Apoorv — Step 1: The ReverseProxy Yamux Trick (45 min)

### Concept samjho pehle (20 min):

Yeh Day 5 ka **sabse important concept** hai aur interview mein agar tum yeh explain kar sake toh interviewer impress ho jayega.

**Normal Reverse Proxy (Nginx/Caddy):**
Jab Nginx ko visitor ka request milta hai, woh internally ek naya TCP connection kholta hai backend server se:
```
Visitor -> Nginx -> net.Dial("tcp", "backend:3000") -> Backend
```

**Humara Problem:**
Humara backend client ke laptop pe hai jo NAT ke peeche hai. Server laptop ko `net.Dial` nahi kar sakta!
Lekin humare paas pehle se ek yamux session hai. Us session ke andar hum `session.OpenStream()` kar sakte hain — aur `yamux.Stream` implement karta hai `net.Conn` interface!

**The Trick: Override `DialContext`**
Go ka `httputil.ReverseProxy` internally `http.Transport` use karta hai, jo `DialContext` function call karta hai server se connect karne ke liye. Hum is function ko **override** kar dete hain taaki yeh TCP dial karne ke bajaye **yamux stream open kare**:

```
Normal ReverseProxy:
  Transport.DialContext -> net.Dial("tcp", "backend:3000") -> TCP connection

OUR ReverseProxy:
  Transport.DialContext -> yamuxSession.OpenStream() -> Virtual stream over existing TCP
```

Ek line mein poora HTTP reverse proxying free mein mil gaya — headers, body, streaming, chunked encoding, WebSocket upgrade — sab `httputil.ReverseProxy` handle karta hai. Hume sirf `DialContext` override karna tha!

### File: `internal/server/proxy/proxy.go`
### Package: `package proxy`

### Function: `NewTunnelProxy(reg *registry.Registry, rootDomain string) http.Handler`

```go
package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/apoorvslst/go-ngrok/internal/server/registry"
	"github.com/apoorvslst/go-ngrok/internal/server/router"
)

// NewTunnelProxy creates an HTTP handler that proxies visitor requests through yamux streams.
func NewTunnelProxy(reg *registry.Registry, rootDomain string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract subdomain from Host header
		subdomain, err := router.ExtractSubdomain(r.Host, rootDomain)
		if err != nil {
			http.Error(w, "Invalid tunnel URL", http.StatusBadRequest)
			return
		}

		// 2. Lookup tunnel in registry
		tunnel, ok := reg.Get(subdomain)
		if !ok {
			http.Error(w, "Tunnel not found or offline", http.StatusNotFound)
			return
		}

		// 3. Check if yamux session is still alive
		if tunnel.Session.IsClosed() {
			reg.Delete(subdomain)
			http.Error(w, "Tunnel client disconnected", http.StatusBadGateway)
			return
		}

		// 4. Build ReverseProxy with custom DialContext
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = "http"
				req.URL.Host = r.Host
				req.Host = r.Host
			},
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// THE TRICK: Open a yamux stream instead of a TCP connection!
					stream, err := tunnel.Session.OpenStream()
					if err != nil {
						slog.Error("failed to open yamux stream", "subdomain", subdomain, "err", err)
						return nil, err
					}
					slog.Info("yamux stream opened for visitor request",
						"subdomain", subdomain,
						"stream_id", stream.StreamID(),
						"method", r.Method,
						"path", r.URL.Path,
					)
					return stream, nil
				},
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				slog.Error("proxy error", "subdomain", subdomain, "err", err)
				http.Error(w, "Bad Gateway: tunnel client failed to respond", http.StatusBadGateway)
			},
		}

		proxy.ServeHTTP(w, r)
	})
}
```

**Line-by-line breakdown of the critical parts:**

- **`Director` function:**
  - `httputil.ReverseProxy` incoming request ko modify karne ke liye `Director` call karta hai before forwarding. Hum URL scheme aur host set karte hain.
  - `req.Host = r.Host`: Original Host header preserve karte hain taaki client-side local app ko pata chale ki request kaunse domain pe aayi thi.

- **`Transport.DialContext` (THE TRICK):**
  - Normally `http.Transport` yahan `net.Dial` call karta hai.
  - Hum ise override karke `tunnel.Session.OpenStream()` call karte hain.
  - `OpenStream()` ek `*yamux.Stream` return karta hai jo `net.Conn` implement karta hai.
  - `httputil.ReverseProxy` ko pata hi nahi chalta ki yeh ek virtual stream hai — uske liye yeh sirf ek normal `net.Conn` hai!
  - **Yahi hai "The Trick" jo poore project ko possible banata hai.**

- **`ErrorHandler`:**
  - Agar client disconnect ho gaya hai ya stream fail ho gayi, toh visitor ko clean `502 Bad Gateway` milta hai, cryptic stack trace nahi.

- **`tunnel.Session.IsClosed()`:**
  - Yamux session already close ho gayi hai lekin registry mein abhi bhi entry bachi hai — toh pehle check karte hain aur stale entry clean karte hain.

---

## Gaurav — Step 2: Client-Side Local Forwarder (45 min)

### Concept samjho pehle (15 min):

**Client Side Pe Kya Hona Chahiye?**
Server ne yamux stream open kiya aur HTTP request push kar diya. Client side pe:
1. `session.AcceptStream()` naya stream accept karega.
2. Client ko us stream ko `localhost:3000` se connect karna hoga.
3. Dono ke beech bidirectional data pipe banana hoga (`io.Copy` dono directions mein).

**The Bidirectional Pipe Pattern:**
```
yamux.Stream         |          local net.Conn
(from server)        |       (to localhost:3000)
                     |
   Read bytes -----> | -----> Write bytes        (Request flow: server to local app)
                     |
   Write bytes <---- | <----- Read bytes          (Response flow: local app to server)
```

Iske liye do goroutines chahiye jo simultaneously dono directions mein data copy karein. Jaise hi ek direction EOF de, dono ko terminate hona chahiye.

### File: `internal/client/forwarder/forwarder.go`
### Package: `package forwarder`

### Function: `ForwardToLocal(stream net.Conn, localAddr string)`

```go
package forwarder

import (
	"io"
	"log/slog"
	"net"
)

// ForwardToLocal connects an incoming yamux stream to the local application.
func ForwardToLocal(stream net.Conn, localAddr string) {
	defer stream.Close()

	// 1. Dial the local application
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		slog.Error("local app unreachable — is it running?",
			"local_addr", localAddr,
			"err", err,
		)
		// Send a raw HTTP 502 response back through the stream to the visitor
		stream.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\n\r\nLocal application at " + localAddr + " is not reachable. Is it running?\n"))
		return
	}
	defer localConn.Close()

	slog.Info("forwarding stream to local app",
		"local_addr", localAddr,
		"stream_remote", stream.RemoteAddr(),
	)

	// 2. Bidirectional copy: stream <-> localConn
	errChan := make(chan error, 2)

	// Direction 1: stream -> localConn (visitor request flows to local app)
	go func() {
		_, err := io.Copy(localConn, stream)
		errChan <- err
	}()

	// Direction 2: localConn -> stream (local app response flows back to visitor)
	go func() {
		_, err := io.Copy(stream, localConn)
		errChan <- err
	}()

	// Wait for either direction to finish (EOF or error)
	err = <-errChan
	if err != nil {
		slog.Debug("forwarding completed with error", "err", err)
	}
}
```

**Line-by-line breakdown:**

- **`defer stream.Close()`**: Stream har haal mein close hona chahiye. Warna server side pe resource leak.
- **`net.Dial("tcp", localAddr)`**: Client apne hi laptop ke `localhost:3000` pe connect karta hai. Agar local app band hai toh `connection refused` aata hai.
- **Raw HTTP 502 Response:**
  ```go
  stream.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n..."))
  ```
  Agar local app nahi chal raha, toh hum yamux stream mein ek raw HTTP response likh dete hain. Server ki `ReverseProxy` isse visitor ko bhej degi. Visitor ko clean "502 Bad Gateway" error milega.
  **Why raw bytes?** Kyunki is point pe humara code HTTP server ke andar nahi hai — hum raw TCP bytes handle kar rahe hain.
- **`errChan := make(chan error, 2)`**: Buffered channel of capacity 2. Dono goroutines apna result yahan push karenge.
- **Two `io.Copy` goroutines:** Day 1 mein tum `HandleEcho` mein do goroutines use kiye the (stdin/stdout). Same pattern hai, sirf sources alag hain:
  - Goroutine 1: Stream se padho, local app ko likho (visitor request aata hai).
  - Goroutine 2: Local app se padho, stream mein likho (response wapas jaata hai).
- **`<-errChan`**: Jaise hi koi ek direction EOF ya error de, hum function exit karte hain. `defer` se dono connections close ho jayenge.

---

## Gaurav — Step 3: Wire Forwarder into Client Stream Loop (25 min)

### File: `internal/client/stream_worker.go` (Day 3 mein banaya tha)

Day 3 mein tumne `handleIncomingStream(stream)` mein dummy echo dala tha. Ab usse replace karo forwarder se:

```go
package client

import (
	"log/slog"

	"github.com/hashicorp/yamux"

	"github.com/apoorvslst/go-ngrok/internal/client/forwarder"
)

// ListenForServerStreams accepts yamux streams initiated by the server and forwards them.
func ListenForServerStreams(session *yamux.Session, localTargetAddr string) {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			slog.Info("yamux session closed, stopping stream listener", "err", err)
			return
		}

		slog.Info("accepted incoming stream from server",
			"stream_id", stream.StreamID(),
		)

		// Forward each stream to the local app in a separate goroutine
		go forwarder.ForwardToLocal(stream, localTargetAddr)
	}
}
```

**Key change from Day 3:**
- Dummy echo hataya, `forwarder.ForwardToLocal(stream, localTargetAddr)` daala.
- `localTargetAddr` parameter add kiya (e.g. `"localhost:3000"`) taaki client ko pata chale ki local app kahan chal raha hai.

---

## Apoorv — Step 4: Update Public HTTP Listener to Use Proxy (15 min)

### File: `internal/server/httpserver/server.go`

Day 4 mein jo simple handler tha ("Tunnel found! Proxying will be implemented..."), usse replace karo `proxy.NewTunnelProxy` se:

```go
package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/apoorvslst/go-ngrok/internal/server/proxy"
	"github.com/apoorvslst/go-ngrok/internal/server/registry"
)

// StartPublicListener starts the HTTP server that proxies visitor traffic through tunnels.
func StartPublicListener(addr string, reg *registry.Registry, rootDomain string) error {
	// Use the proxy handler instead of the placeholder
	handler := proxy.NewTunnelProxy(reg, rootDomain)

	slog.Info("public HTTP listener starting", "addr", addr)
	return http.ListenAndServe(addr, handler)
}
```

---

## Both — Step 5: The Grand End-to-End Test (30 min)

### Setup:

#### Terminal 1: Start a dummy local web app
Hume test karne ke liye koi bhi local HTTP server chahiye. Python ka built-in HTTP server use karo:
```bash
# Python 3
python -m http.server 3000
```
Ya agar Go se banana ho (ekdum simple):
```bash
# Ek alag folder mein test-app/main.go banao
# Nahi banana chahte toh Python server se kaam chala lo
```

#### Terminal 2: Start the tunnel server
```bash
go run ./cmd/server
# Output:
# level=INFO msg="control listener starting" addr=:9000
# level=INFO msg="public HTTP listener starting" addr=:8080
```

#### Terminal 3: Start the tunnel client
```bash
go run ./cmd/client
# Output:
# level=INFO msg="connected to server"
# level=INFO msg="handshake successful" url=https://gaurav.tunnel.dev
# level=INFO msg="listening for incoming streams"
```

#### Terminal 4: Send a visitor request
```bash
curl -v -H "Host: gaurav.tunnel.dev" http://localhost:8080/
```

### Expected Data Flow:
```
curl -> Server :8080
        |-> Extract "gaurav" from Host header
        |-> Registry.Get("gaurav") -> yamux session found
        |-> ReverseProxy opens yamux stream (stream_id=1)
        |-> HTTP request sent over stream
        |
        Client receives stream_id=1
        |-> net.Dial("tcp", "localhost:3000") -> connects to Python HTTP server
        |-> io.Copy: stream -> local conn (request forward)
        |-> io.Copy: local conn -> stream (response forward)
        |
        Python server responds with directory listing HTML
        |
        Response travels back: local -> client -> stream -> server -> curl
```

### Expected `curl` output:
```html
<!DOCTYPE HTML>
<html>
<head>...
(Python HTTP server ka directory listing ya index page)
```

### Expected Server logs:
```text
level=INFO msg="request routed" subdomain=gaurav method=GET path=/
level=INFO msg="yamux stream opened for visitor request" subdomain=gaurav stream_id=1
```

### Expected Client logs:
```text
level=INFO msg="accepted incoming stream from server" stream_id=1
level=INFO msg="forwarding stream to local app" local_addr=localhost:3000
```

---

## Both — Step 6: Failure Scenario Testing (20 min)

### Test 1: Local App Not Running
Client chal raha hai lekin Python server band hai (port 3000 pe kuch nahi chal raha):
```bash
curl -H "Host: gaurav.tunnel.dev" http://localhost:8080/
```
**Expected:** Visitor ko `502 Bad Gateway: Local application at localhost:3000 is not reachable` milna chahiye.

### Test 2: Client Disconnects Mid-Request
1. Server aur client dono chalu karo.
2. Ek slow request bhejo (jaise large file download):
   ```bash
   curl -H "Host: gaurav.tunnel.dev" http://localhost:8080/largefile
   ```
3. Beech mein client terminal pe `Ctrl+C` dabao.
**Expected:** curl ko `502 Bad Gateway` milna chahiye. Server cleanly stream close kar deta hai.

### Test 3: Multiple Simultaneous Visitors
10 concurrent requests bhejo ek saath:
```bash
# Windows PowerShell:
1..10 | ForEach-Object { Start-Job { curl -H "Host: gaurav.tunnel.dev" http://localhost:8080/ } }
Get-Job | Wait-Job | Receive-Job
```
**Expected:** Saare 10 requests independently resolve hone chahiye, koi blocking nahi.

### Test 4: Wrong Subdomain
```bash
curl -H "Host: hackerboy.tunnel.dev" http://localhost:8080/
```
**Expected:** `404 Not Found: Tunnel not found or offline`.

---

## Both — Step 7: PR, Review & Celebrate (20 min)

### Gaurav:
```bash
git checkout -b day5/gaurav
git add .
git commit -m "day5: client local forwarder with bidirectional io.Copy and 502 fallback"
git push origin day5/gaurav
```

### Apoorv:
```bash
git checkout -b day5/apoorv
git add .
git commit -m "day5: ReverseProxy yamux DialContext trick, end-to-end proxying"
git push origin day5/apoorv
```

### Celebrate!
Agar Step 5 ka test pass ho gaya — toh congratulations!
Tumne **ek working ngrok clone** bana liya hai!
Visitor ka HTTP request internet se tumhare laptop ke `localhost:3000` pe arrive ho raha hai through a single multiplexed TCP connection.

Baaki ke din (Day 6-15) mein hum reliability, inspector UI, auth, rate limiting, aur deployment add karenge.

---

## Day 5 Checklist

### Files Created/Modified:

| File | Kaun | Contains |
|:---|:---:|:---|
| `internal/server/proxy/proxy.go` | Apoorv | `NewTunnelProxy` — ReverseProxy with yamux DialContext override |
| `internal/client/forwarder/forwarder.go` | Gaurav | `ForwardToLocal` — bidirectional stream-to-localhost pipe |
| `internal/client/stream_worker.go` | Gaurav | Updated to call forwarder instead of dummy echo |
| `internal/server/httpserver/server.go` | Apoorv | Updated to use proxy handler |

### Concepts Mastered:
- [ ] `httputil.ReverseProxy` — Go's standard library reverse proxy
- [ ] `Transport.DialContext` override — The core trick that makes tunneling possible
- [ ] `yamux.Stream` implements `net.Conn` — Virtual streams look like real sockets
- [ ] Bidirectional `io.Copy` with channel-based completion (`errChan`)
- [ ] Raw HTTP response writing for 502 fallback when local app is down
- [ ] Full end-to-end request lifecycle: Visitor -> Server -> Yamux -> Client -> Local App -> Response back

### Interview Gold:
Agar koi interviewer poochhe "How does ngrok work internally?", toh tum ab bol sakte ho:
> "Client laptop se server ko ek outbound TCP connection kholta hai (NAT traversal). Dono sides yamux session banate hain jo ek pipe ke andar multiple virtual streams chalati hai. Server pe jab visitor request aata hai, server `httputil.ReverseProxy` use karta hai lekin `DialContext` override karke yamux stream open karta hai instead of TCP dial. Client stream accept karke local app se connect karta hai aur bidirectional `io.Copy` se data forward karta hai."

---

*Day 5 khatam hone pe batao. Day 6 mein client auto-reconnection with exponential backoff + jitter implement karenge taaki WiFi drop hone pe client automatically wapas connect ho jaye!*

