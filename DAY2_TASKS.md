# 📅 Day 2 — Control Protocol, Framing & Handshake Authentication (Odin Project Style)

> **Rule**: Hum code direct inject nahi karenge. Hum batayenge KYA likhna hai, KIS file mein, KAUNSA function, KAUNSA concept pehle samjho. Har ek function tum khud likhoge.
>
> **Apoorv** = 🔵 | **Gaurav** = 🟢 | **Both** = ⚪

---

## 🗺️ Day 2 Overview: Where We Are & Where We Are Going

Kal Day 1 mein humne ek solid foundation banayi:
- TCP Server (`net.Listen` + `Accept` loop)
- TCP Client (`net.Dial`)
- Echo with `io.Copy` & Goroutines
- Graceful Shutdown (`signal.NotifyContext` + `sync.WaitGroup`)
- Structured Logging (`log/slog`)

Lekin abhi humara server **"andha" (blind)** hai:
Koi bhi client connect karta hai, server ko nahi pata:
- Client kaun hai? (Authentication token kya hai?)
- Client ko kaunsa subdomain chahiye? (`rahul`? `apoorv`? `gaurav`?)
- Client ka CLI version kya hai? Kya ye purana version hai jo server se incompatible hai?

**Day 2 ka Mission:**
Hum ek **Application-Layer Control Protocol** design karenge jisme client aur server ek structured, type-safe JSON Handshake exchange karenge, timeouts aur security validation ke saath.

```
Client (Laptop)                                                Server (VPS :9000)
      │                                                               │
      ├────────────── 1. TCP 3-Way Handshake (OS Kernel) ────────────►│
      │                                                               │
      │   [Control Handshake Phase with 5-Second Deadline]           │
      ├────── 2. HELLO Frame (Token, Subdomain, Version) ─────────────►│
      │                                                               │ (Verify token,
      │                                                               │  check subdomain)
      │◄───── 3. AUTH_OK Frame (Assigned Public URL) ─────────────────┤
      │                                                               │
      ▼                                                               ▼
   Ready for Multiplexing (Day 3)                                Ready to Route Traffic
```

---

## ⚪ Step 0: Day 1 Code Sanity & Git Branch (Both — 10 min)

Before starting Day 2, dono confirm karo ki Day 1 ka code cleanly compile hota hai:

```bash
cd c:\Users\gaura\Downloads\gongrok-personal\go-ngrok
go build ./cmd/server
go build ./cmd/client
```

Agar dono bina error ke compile hote hain $\rightarrow$ Day 2 branch banao:
```bash
git checkout -b day2/control-protocol
```

---

## ⚪ Step 1: The Core Systems Concept — Stream vs Message Framing (Both — 20 min)

### 🧠 The Mental Model: Why TCP Has No Concept of "Messages"

Ye interviewers ka favorite question hai aur 90% junior backend developers isme confuse hote hain:

> **The Problem:**
> TCP ek **Byte-Stream Protocol** hai, **Message Protocol** nahi.
> OS kernel ke liye TCP connection ek continuous paani ke pipe (stream of bytes) jaisa hai. Isme packets ya messages ki koi boundary nahi hoti.

Imagine karo Client ne 2 messages back-to-back bheje:
```go
conn.Write([]byte(`{"type":"HELLO"}`))
conn.Write([]byte(`{"type":"PING"}`))
```

Network layer (Nagle's Algorithm, MTU packet sizes, router buffering) in bytes ko kisi bhi tarah deliver kar sakti hai:
1. **Packet Merging (Coalescing):** Server ko dono ek saath mil sakte hain:  
   `{"type":"HELLO"}{"type":"PING"}` $\rightarrow$ JSON parser crash ho jayega!
2. **Packet Fragmentation:** Server ko aadha message pehle mila aur aadha baad mein:  
   Chunk 1: `{"type":"HE`  
   Chunk 2: `LLO"}{"type":"PING"}`

### 🛠️ Three Ways to Fix This (Framing Strategies):

| Strategy | Kaise kaam karta hai | Pros | Cons |
|:---|:---|:---|:---|
| **1. Delimiter-Based** (Humara Choice) | Har message ke end mein special character (`\n`) lagao | Human-readable, simple, `json.Encoder` natively supports it | Payload ke andar delimiter escape karna padta hai |
| **2. Length-Prefixed** | Har message ke aage 4-byte integer bhejo jo bataye body kitne bytes ki hai | Raw binary payloads ke liye fastest | Hex inspection thodi mushkil hoti hai |
| **3. Fixed-Length** | Har message exactly 1024 bytes ka hoga | Simple parsing | Chhote messages mein bandwidth waste |

**Day 2 Decision:** Control plane ke liye hum **Newline-Delimited JSON** use karenge. Go ka standard library `json.NewEncoder` har JSON object ke baad automatically `\n` append karta hai, aur `json.NewDecoder` exactly ek valid JSON object stream se parse karta hai!

---

## 🟢 Step 2: Protocol Structs & Message Definitions (Gaurav — 30 min)

### Concept samjho pehle (10 min):
> **Why Typed Enums in Go?**
> JavaScript ya Python mein log string likh dete hain: `msg["type"] = "hello"`.
> Lekin production Go code mein agar kisi ne `"Hello"` ya `"HELLO "` likh diya, toh runtime pe bug aa jayega!
> Go mein hum `type MessageType string` declare karte hain aur constants banate hain. Compiler typo ko build-time pe hi pakad leta hai.

### File: `internal/protocol/messages.go`
### Package: `package protocol`

### Kya likhna hai:

```go
package protocol

// MessageType represents the type of control message being sent over the wire.
type MessageType string

const (
	// MsgHello is sent by the client immediately after connecting.
	MsgHello MessageType = "HELLO"

	// MsgAuthOK is returned by the server when authentication and subdomain reservation succeed.
	MsgAuthOK MessageType = "AUTH_OK"

	// MsgAuthErr is returned by the server if token or subdomain validation fails.
	MsgAuthErr MessageType = "AUTH_ERR"

	// MsgPing is sent periodically to keep the control connection alive.
	MsgPing MessageType = "PING"

	// MsgPong is the response to a MsgPing frame.
	MsgPong MessageType = "PONG"
)

// HelloMessage is the initial payload sent by the client.
type HelloMessage struct {
	Type             MessageType `json:"type"`
	Token            string      `json:"token"`
	DesiredSubdomain string      `json:"desired_subdomain"`
	ClientVersion    string      `json:"client_version"`
	OSPlatform       string      `json:"os_platform"`
}

// AuthResponseMessage is the server's handshake decision.
type AuthResponseMessage struct {
	Type         MessageType `json:"type"`
	Success      bool        `json:"success"`
	AssignedURL  string      `json:"assigned_url,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
	ServerTime   int64       `json:"server_time"`
}

// PingPongMessage represents heartbeat telemetry.
type PingPongMessage struct {
	Type      MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
}
```

### 🔍 Deep Line-by-Line Breakdown:
- **`type MessageType string`**: Custom type alias. Type safety ensure karta hai taaki koi arbitrary string pass na kar sake.
- **`json:"desired_subdomain"`**: JSON struct tags batate hain ki wire par serialize hote waqt field ka naam snake_case hoga.
- **`json:"assigned_url,omitempty"`**: Agar `Success` false hai aur koi URL assign nahi hua, toh `omitempty` ki wajah se JSON output mein ye field gayab ho jayegi (clean wire payload).

### Verify:
```bash
go build ./internal/protocol/...
```
- [ ] Compiles cleanly.

---

## 🔵 Step 3: Generic Streaming Codec (Apoorv — 35 min)

### Concept samjho pehle (10 min):
> **`json.Marshal` vs `json.Encoder`:**
> - `json.Marshal(v)` pura JSON pehle RAM mein ek bada byte slice (`[]byte`) allocate karta hai, phir socket par bhejta hai (Double memory allocation).
> - `json.NewEncoder(w).Encode(v)` zero intermediate slice allocation karta hai — directly `io.Writer` (humare TCP connection) mein stream kar deta hai!
>
> Aur hum modern **Go Generics (`[T any]`)** use karenge taaki jab hum message read karein, toh hume typecasting `msg.(*HelloMessage)` na karni pade — compiler automatically correct type return karega!

### File: `internal/protocol/codec.go`
### Package: `package protocol`

### Function 1: `WriteMessage`
```go
// WriteMessage encodes any message struct as a newline-delimited JSON frame to the writer.
func WriteMessage(w io.Writer, msg any) error {
	if w == nil {
		return errors.New("cannot write to nil writer")
	}

	encoder := json.NewEncoder(w)
	err := encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to encode protocol message: %w", err)
	}

	return nil
}
```

### Function 2: `ReadMessage`
```go
// ReadMessage decodes a single newline-delimited JSON frame from the reader into type T.
func ReadMessage[T any](r io.Reader) (*T, error) {
	if r == nil {
		return nil, errors.New("cannot read from nil reader")
	}

	var target T
	decoder := json.NewDecoder(r)
	err := decoder.Decode(&target)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to decode protocol message: %w", err)
	}

	return &target, nil
}
```

### 🔍 Deep Line-by-Line Breakdown:
- **`encoder.Encode(msg)`**: Encode function automatically trailing newline `\n` add karta hai, jo framing boundary ka kaam karti hai.
- **`errors.Is(err, io.EOF)`**: Jab doosri side connection band kar deti hai, toh `Decode` `io.EOF` return karta hai. Isko alag se preserve karna critical hai taaki caller ko pata chale connection normal close hua hai ya network fail hua hai.

---

## 🟢 Step 4: Unit Testing the Codec with `bytes.Buffer` (Gaurav — 25 min)

### Concept samjho pehle (10 min):
> Ek senior engineer kabhi real network sockets pe codec test nahi karta.
> Real network test slow hota hai aur ports occupy karta hai.
> Go standard library ka `bytes.Buffer` simultaneously **`io.Reader`** aur **`io.Writer`** dono implement karta hai!
> Hum in-memory buffer mein `WriteMessage` karenge aur wahi se `ReadMessage` karenge — 1 millisecond mein unit test run hoga!

### File: `internal/protocol/codec_test.go`
### Package: `package protocol`

### Kya likhna hai:

```go
package protocol_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/devGPP23/go-ngrok/internal/protocol"
)

func TestCodec_HelloMessage_Roundtrip(t *testing.T) {
	var buf bytes.Buffer

	original := protocol.HelloMessage{
		Type:             protocol.MsgHello,
		Token:            "test_secret_token_123",
		DesiredSubdomain: "gaurav-dev",
		ClientVersion:    "1.0.0",
		OSPlatform:       "windows",
	}

	// 1. Encode into buffer
	err := protocol.WriteMessage(&buf, original)
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	// 2. Decode from buffer
	decoded, err := protocol.ReadMessage[protocol.HelloMessage](&buf)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// 3. Verify assertions
	if decoded.Type != original.Type {
		t.Errorf("expected type %s, got %s", original.Type, decoded.Type)
	}
	if decoded.Token != original.Token {
		t.Errorf("expected token %s, got %s", original.Token, decoded.Token)
	}
	if decoded.DesiredSubdomain != original.DesiredSubdomain {
		t.Errorf("expected subdomain %s, got %s", original.DesiredSubdomain, decoded.DesiredSubdomain)
	}
}

func TestCodec_MultipleFrames(t *testing.T) {
	var buf bytes.Buffer

	msg1 := protocol.PingPongMessage{Type: protocol.MsgPing, Timestamp: time.Now().Unix()}
	msg2 := protocol.PingPongMessage{Type: protocol.MsgPong, Timestamp: time.Now().Unix() + 1}

	// Write two frames back to back into the same buffer
	if err := protocol.WriteMessage(&buf, msg1); err != nil {
		t.Fatalf("failed to write msg1: %v", err)
	}
	if err := protocol.WriteMessage(&buf, msg2); err != nil {
		t.Fatalf("failed to write msg2: %v", err)
	}

	// Read them back sequentially
	r1, err := protocol.ReadMessage[protocol.PingPongMessage](&buf)
	if err != nil || r1.Type != protocol.MsgPing {
		t.Fatalf("failed to read msg1 properly: %v", err)
	}

	r2, err := protocol.ReadMessage[protocol.PingPongMessage](&buf)
	if err != nil || r2.Type != protocol.MsgPong {
		t.Fatalf("failed to read msg2 properly: %v", err)
	}
}
```

### Verify:
```bash
go test -v ./internal/protocol/...
```
Expected output:
```text
=== RUN   TestCodec_HelloMessage_Roundtrip
--- PASS: TestCodec_HelloMessage_Roundtrip (0.00s)
=== RUN   TestCodec_MultipleFrames
--- PASS: TestCodec_MultipleFrames (0.00s)
PASS
ok      github.com/devGPP23/go-ngrok/internal/protocol  0.021s
```
- [ ] Both unit tests pass with zero warnings!

---

## 🔵 Step 5: Server Handshake Verifier & Deadlines (Apoorv — 40 min)

### Concept samjho pehle (15 min):
> **Slowloris & Connection Hanging Protection:**
> Agar koi client server se TCP connect kare aur chupchap baith jaye (ek byte bhi na bheje), toh server ka `ReadMessage` call **hamesha ke liye block ho jayega**.
> Agar 1,000 bots aisa karein, toh server ke saare file descriptors aur memory exhaust ho jayenge!
>
> **Solution:**
> `conn.SetReadDeadline(time.Now().Add(5 * time.Second))`
> Hum OS kernel ko bolte hain: *"Agar 5 second ke andar client ka complete HELLO frame nahi aaya, toh turant timeout error fenko aur connection kaat do!"*
> Handshake pass hone ke baad, hum deadline ko reset kar dete hain:
> `conn.SetReadDeadline(time.Time{})` (Zero time matlab infinite / normal streaming).

### File: `internal/server/handshake.go`
### Package: `package server`

### Function 1: Subdomain Validator (Helper)
```go
// isValidSubdomain enforces alphanumeric characters and hyphens between 3 and 30 chars.
func isValidSubdomain(sub string) bool {
	if len(sub) < 3 || len(sub) > 30 {
		return false
	}
	for _, ch := range sub {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return false
		}
	}
	return true
}
```

### Function 2: `PerformHandshake`
```go
// PerformHandshake validates the incoming client hello frame under a strict 5-second deadline.
func PerformHandshake(conn net.Conn, validToken string, domainRoot string) (*protocol.HelloMessage, error) {
	// 1. Enforce 5-second deadline for the handshake
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{}) // Reset on return

	// 2. Read HelloMessage from client
	hello, err := protocol.ReadMessage[protocol.HelloMessage](conn)
	if err != nil {
		return nil, fmt.Errorf("handshake read failed: %w", err)
	}

	// 3. Verify Message Type
	if hello.Type != protocol.MsgHello {
		errMsg := "invalid handshake: expected HELLO message"
		protocol.WriteMessage(conn, protocol.AuthResponseMessage{
			Type:         protocol.MsgAuthErr,
			Success:      false,
			ErrorMessage: errMsg,
			ServerTime:   time.Now().Unix(),
		})
		return nil, errors.New(errMsg)
	}

	// 4. Verify Authentication Token
	if hello.Token != validToken {
		errMsg := "authentication failed: invalid or expired token"
		protocol.WriteMessage(conn, protocol.AuthResponseMessage{
			Type:         protocol.MsgAuthErr,
			Success:      false,
			ErrorMessage: errMsg,
			ServerTime:   time.Now().Unix(),
		})
		return nil, errors.New(errMsg)
	}

	// 5. Verify Subdomain Format
	if !isValidSubdomain(hello.DesiredSubdomain) {
		errMsg := fmt.Sprintf("invalid subdomain '%s': must be 3-30 lowercase alphanumeric characters", hello.DesiredSubdomain)
		protocol.WriteMessage(conn, protocol.AuthResponseMessage{
			Type:         protocol.MsgAuthErr,
			Success:      false,
			ErrorMessage: errMsg,
			ServerTime:   time.Now().Unix(),
		})
		return nil, errors.New(errMsg)
	}

	// 6. Build Success Response
	assignedURL := fmt.Sprintf("https://%s.%s", hello.DesiredSubdomain, domainRoot)
	resp := protocol.AuthResponseMessage{
		Type:        protocol.MsgAuthOK,
		Success:     true,
		AssignedURL: assignedURL,
		ServerTime:  time.Now().Unix(),
	}

	if err := protocol.WriteMessage(conn, resp); err != nil {
		return nil, fmt.Errorf("failed to send handshake response: %w", err)
	}

	return hello, nil
}
```

### 🔍 Deep Line-by-Line Breakdown:
- **`defer conn.SetReadDeadline(time.Time{})`**: Handshake chahe pass ho ya fail ho, `defer` guarantee karega ki deadline reset ho jaye ya socket close ho jaye.
- **Failures return `MsgAuthErr` frame**: Client ko clean JSON error milta hai, raw socket disconnection nahi. This is how professional CLIs show friendly error messages.

---

## 🟢 Step 6: Client Handshake Initiator (Gaurav — 35 min)

### Concept samjho pehle (10 min):
> Client connect hote hi chup nahi baithega.
> Uska pehla kaam hai: Server ko apna `HelloMessage` bhejna, aur 5 second ke andar server ka response collect karna.
> Agar server ne auth reject kiya, toh client error log karke cleanly exit karega.

### File: `internal/client/handshake.go`
### Package: `package client`

### Function: `PerformClientHandshake`
```go
package client

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/devGPP23/go-ngrok/internal/protocol"
)

// PerformClientHandshake initiates authentication with the tunnel server and retrieves the public URL.
func PerformClientHandshake(conn net.Conn, token string, subdomain string) (*protocol.AuthResponseMessage, error) {
	// 1. Strict 5-second deadline for safety
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetDeadline(time.Time{})

	// 2. Prepare Hello Frame
	hello := protocol.HelloMessage{
		Type:             protocol.MsgHello,
		Token:            token,
		DesiredSubdomain: subdomain,
		ClientVersion:    "1.0.0",
		OSPlatform:       runtime.GOOS,
	}

	// 3. Send Hello Frame
	if err := protocol.WriteMessage(conn, hello); err != nil {
		return nil, fmt.Errorf("failed to send hello frame: %w", err)
	}

	// 4. Read Server Response
	resp, err := protocol.ReadMessage[protocol.AuthResponseMessage](conn)
	if err != nil {
		return nil, fmt.Errorf("failed to receive handshake response: %w", err)
	}

	// 5. Evaluate Decision
	if !resp.Success {
		return nil, fmt.Errorf("server rejected authentication: %s", resp.ErrorMessage)
	}

	return resp, nil
}
```

### 🔍 Deep Line-by-Line Breakdown:
- **`runtime.GOOS`**: Client automatically detect karta hai ki woh Windows, Linux ya macOS pe chal raha hai aur telemetry server ko bhejta hai.
- **`conn.SetDeadline`**: Sets BOTH read and write deadlines.

---

## 🔵 Step 7: Integrate Handshake into Server Listener (Apoorv — 25 min)

Ab `internal/server/listener.go` ko update karo taaki accept loop ke baad pehle handshake ho!

### File: `internal/server/listener.go`
In `handleConnection`:

```go
func handleConnection(conn net.Conn, validToken string, domainRoot string) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	slog.Info("new connection accepted", "remote_addr", remoteAddr)

	// Step 1: Perform Handshake
	hello, err := PerformHandshake(conn, validToken, domainRoot)
	if err != nil {
		slog.Warn("handshake aborted", "remote_addr", remoteAddr, "err", err)
		return
	}

	slog.Info("client authenticated successfully!",
		"remote_addr", remoteAddr,
		"subdomain", hello.DesiredSubdomain,
		"os", hello.OSPlatform,
		"version", hello.ClientVersion,
	)

	// Step 2: Keep alive / Echo (from Day 1)
	io.Copy(conn, conn)
	slog.Info("session terminated", "subdomain", hello.DesiredSubdomain)
}
```

Aur `cmd/server/main.go` mein pass karo:
```go
const AuthToken = "dev-secret-token"
const DomainRoot = "tunnel.dev"

// inside main:
server.StartListener(ctx, ":9000", AuthToken, DomainRoot)
```

---

## 🟢 Step 8: Integrate Handshake into Client CLI (Gaurav — 25 min)

Ab `cmd/client/main.go` ko update karo taaki connect hone ke baad handshake kare:

### File: `cmd/client/main.go`

```go
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/devGPP23/go-ngrok/internal/client"
)

func main() {
	token := "dev-secret-token"
	desiredSubdomain := "gaurav"

	slog.Info("GoNgrok Client starting...", "subdomain", desiredSubdomain)

	// 1. Dial Server
	conn, err := client.Connect("localhost:9000")
	if err != nil {
		slog.Error("connection failed", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 2. Perform Handshake
	authResp, err := client.PerformClientHandshake(conn, token, desiredSubdomain)
	if err != nil {
		slog.Error("handshake failed", "err", err)
		os.Exit(1)
	}

	// 3. Print Awesome Banner
	fmt.Println("==================================================")
	fmt.Printf("🎉 TUNNEL LIVE AT: %s\n", authResp.AssignedURL)
	fmt.Println("Forwarding traffic to: localhost:3000 (Day 5)")
	fmt.Println("Press Ctrl+C to quit.")
	fmt.Println("==================================================")

	// 4. Interactive Echo
	client.HandleEcho(conn)
}
```

---

## ⚪ Step 9: The Day 2 Verification Gauntlet (Both — 30 min)

Ye chaar experiments karo aur console outputs observe karo:

### 🧪 Experiment 1: Happy Path (Valid Token & Subdomain)
```bash
# Terminal 1:
go run ./cmd/server

# Terminal 2:
go run ./cmd/client
```
**Expected:**
```text
[Server] client authenticated successfully! subdomain=gaurav os=windows version=1.0.0
[Client] 🎉 TUNNEL LIVE AT: https://gaurav.tunnel.dev
```

---

### 🧪 Experiment 2: Wrong Token (Security Test)
Client mein token change karo to `"hacker-token"`.
Run client.
**Expected:**
- Client outputs: `handshake failed: server rejected authentication: authentication failed: invalid or expired token`
- Client exits with code 1.
- Server logs warning and closes connection. Zero goroutine leak!

---

### 🧪 Experiment 3: Illegal Subdomain Name
Client mein subdomain set karo to `"bad@sub!domain"`.
Run client.
**Expected:**
- Client outputs: `invalid subdomain 'bad@sub!domain': must be 3-30 lowercase alphanumeric characters`

---

### 🧪 Experiment 4: Slowloris Attack Simulation (Deadlines Test)
Ek raw TCP connection kholo bina koi data bheje:
```powershell
# PowerShell:
$tcp = New-Object System.Net.Sockets.TcpClient("localhost", 9000)
# Kuch mat type karo. 5 second wait karo!
```
**Expected:**
- Server par 5 second baad print hoga:
  `handshake aborted err="handshake read failed: read tcp 127.0.0.1:9000->...: i/o timeout"`
- Server socket band kar deta hai. You just defended against a DoS attack! 🛡️

---

## 🤝 Step 10: PR, Review & Merge (Both — 20 min)

### Gaurav:
```bash
git add .
git commit -m "day2: protocol framing, HelloMessage, client handshake initiator, codec unit tests"
git push origin day2/control-protocol
```

### Review checklist:
- [ ] Codec unit tests pass: `go test ./internal/protocol/...`
- [ ] `go test -race ./...` shows zero race conditions
- [ ] All 4 experiments verified successfully

---

## ✅ Day 2 Final Checklist

### Files Created/Modified:

| File | Kaun | Contains |
|:---|:---:|:---|
| `internal/protocol/messages.go` | 🟢 Gaurav | `MessageType`, `HelloMessage`, `AuthResponseMessage` |
| `internal/protocol/codec.go` | 🔵 Apoorv | Generic `WriteMessage` & `ReadMessage[T]` |
| `internal/protocol/codec_test.go` | 🟢 Gaurav | In-memory Buffer roundtrip tests |
| `internal/server/handshake.go` | 🔵 Apoorv | `PerformHandshake`, deadlines, subdomain validation |
| `internal/client/handshake.go` | 🟢 Gaurav | `PerformClientHandshake`, OS detection |
| `cmd/server/main.go` & `cmd/client/main.go` | ⚪ Both | Full handshake integration & CLI banner |

### Concepts Mastered:
- [ ] Stream vs Message protocols in TCP
- [ ] Newline-delimited JSON framing boundaries
- [ ] Zero-allocation streaming with `json.Encoder`/`json.Decoder`
- [ ] Go Generics (`[T any]`) for type-safe network codecs
- [ ] In-memory testing with `bytes.Buffer`
- [ ] Slowloris attack prevention using `SetReadDeadline`
- [ ] Structured error propagation over the wire

---

*Day 2 complete hone ke baad batao. Day 3 mein hum Yamux multiplexer introduce karenge — ek connection ke andar parallel streams!*
