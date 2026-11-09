# 🚀 Go-Ngrok: Complete Client & Server Guide (Days 1 to 4)

> **For Gaurav & the Team:**
> This document explains both the **Client** and the **Server** from **Day 1 to Day 4** using one continuous, real-world developer scenario:
> **Rahul running a web application on his laptop (`localhost:3000`) and exposing it to the internet through our Go reverse tunnel service.**
>
> - Covers **both** sides of the wire: what Rahul's client does, what the server does, and how they communicate.
> - Every technical term is explained in brackets `(...)` right when it first appears.
> - All flowcharts are drawn in clean ASCII text boxes.

---

## 📑 Table of Contents
1. [The Real-World Setup (Who is Rahul?)](#the-real-world-setup-who-is-rahul)
2. [Day 1: Establishing the Raw TCP Connection](#day-1-establishing-the-raw-tcp-connection)
3. [Day 2: Protocol Framing, Generic Codec & Handshake Auth](#day-2-protocol-framing-generic-codec--handshake-auth)
4. [Day 3: Connection Multiplexing with Yamux](#day-3-connection-multiplexing-with-yamux)
5. [Day 4: Two-Door Server Architecture, Registry & Router](#day-4-two-door-server-architecture-registry--router)
6. [Summary Comparison Cheat Sheet (Days 1 to 4)](#summary-comparison-cheat-sheet-days-1-to-4)

---

## The Real-World Setup (Who is Rahul?)

```
+-----------------------------------------------------------------------------+
| RAHUL'S LAPTOP (Home Wi-Fi)                                                 |
| - Private IP: 192.168.1.15 (Hidden behind home router NAT / Firewall)       |
| - Web App: Running locally on http://localhost:3000                         |
| - Tunnel Client CLI: ./client (Connects OUTBOUND to Server)                 |
+-----------------------------------------------------------------------------+
                                      |
                                      | Outbound TCP Connection to Port :9000
                                      v
+-----------------------------------------------------------------------------+
| OUR TUNNEL SERVER (Public Cloud VPS / AWS EC2)                              |
| - Public IP: 54.210.10.20 (Accessible to the entire internet)               |
| - Port :9000 (Control Door): Listens for Rahul's CLI connection             |
| - Port :8080 (Public Door): Listens for internet visitors / web browsers    |
+-----------------------------------------------------------------------------+
```

---

# Day 1: Establishing the Raw TCP Connection

📁 **Client File:** `internal/client/connector.go`, `cmd/client/main.go`  
📁 **Server File:** `internal/server/listener.go`, `cmd/server/main.go`

### 1. What We Built & Why

Before we can build any fancy reverse tunnel features, we need a reliable, raw two-way pipe between Rahul's laptop and the server.

- **What Rahul's Client Does:**
  Rahul opens his terminal and runs `./client`. The client calls `net.Dial("tcp", "localhost:9000")`. This initiates an **outbound** TCP connection through his home Wi-Fi router to the server. Once connected, Rahul's client tests the pipe by sending keyboard input (`os.Stdin`) to the server and printing whatever the server replies back (`os.Stdout`).

- **What the Server Does:**
  The server binds to port `:9000` with `net.Listen("tcp", ":9000")` and enters an infinite loop waiting for connections (`listener.Accept`). When Rahul connects, the server accepts the socket (`net.Conn`) and immediately spawns a dedicated **goroutine** (super-lightweight background thread in Go) to talk to Rahul: `go handleConnection(conn)`.
  This keeps the main loop free so another developer (e.g. Gaurav) can connect at the same exact time.
  If the server operator presses `Ctrl+C`, the server catches the shutdown signal using `context.Context` (a Go tool for cancellation signals) and waits for active client connections to finish safely using `sync.WaitGroup` (a thread-safe counter) before exiting.

---

### 2. Flowchart: Day 1 Connection Lifecycle

```
RAHUL'S CLIENT LAPTOP                                        TUNNEL SERVER
+-------------------------------+             +-----------------------------------------+
| net.Dial("tcp", ":9000")     |             | net.Listen("tcp", ":9000")              |
| Dials outbound to server      |             | Binds port and enters infinite loop     |
+-------------------------------+             +-----------------------------------------+
                |                                                  |
                | ===== 1. TCP 3-Way Handshake (SYN/ACK) ========> |
                |                                                  v
                |                             +-----------------------------------------+
                |                             | listener.Accept()                       |
                |                             | Unblocks! Accepts Rahul's connection    |
                |                             +-----------------------------------------+
                |                                                  |
                |                                                  | Spawns independent goroutine:
                |                                                  v
+-------------------------------+             +-----------------------------------------+
| conn.Write(data)              |             | go handleConnection(conn net.Conn)      |
| Rahul types in terminal       |             | Server reads data from Rahul            |
+-------------------------------+             +-----------------------------------------+
                |                                                  |
                | ===== 2. Data Transfer over TCP Socket ========> |
                | <==== 3. Echoed Data back to Rahul ============= |
                |                                                  |
+-------------------------------+             +-----------------------------------------+
| conn.Close()                  |             | defer conn.Close()                      |
| Rahul presses Ctrl+C          | === FIN ==> | Socket freed, sync.WaitGroup decrements |
+-------------------------------+             +-----------------------------------------+
```

---

### 3. Technical Words Explained (Day 1)
- **TCP (Transmission Control Protocol):** A reliable, two-way internet connection standard (like a phone call) where data bytes are guaranteed to arrive in exact order without loss.
- **Port:** A numbered network door on a computer (e.g. `:9000`). It tells the operating system which program should handle incoming traffic.
- **`net.Conn` (Network Connection):** The live, two-way communication socket between Rahul's laptop and the server.
- **`net.Listener`:** A Go object that sits on a port and waits for incoming network calls.
- **Goroutine (Super-lightweight Go thread):** A mini-thread managed by Go taking ~2KB of RAM (unlike OS threads which take ~1MB), allowing thousands of developers to connect simultaneously.
- **`sync.WaitGroup`:** A concurrency counter (`Add`, `Done`, `Wait`) ensuring the server doesn't shut down while active connections are still talking.
- **Graceful Shutdown:** Stopping a server cleanly by letting current jobs finish, instead of brutally terminating connections.

---

### 4. Code Walkthrough (Day 1)

#### Rahul's Client (`internal/client/connector.go`):
```go
package client

import (
    "fmt"
    "net"
)

// Connect dials the tunnel server over raw TCP
func Connect(addr string) (net.Conn, error) {
    // Dials outbound connection to server port :9000
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        return nil, fmt.Errorf("connection failed: %w", err)
    }
    fmt.Printf("Connected to server at %s successfully!\n", addr)
    return conn, nil
}
```

#### The Server Listener (`internal/server/listener.go`):
```go
package server

import (
    "net"
    "sync"
)

func StartListener(addr string) error {
    // 1. Open port door :9000
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }
    defer listener.Close()

    var wg sync.WaitGroup

    // 2. Loop forever, accepting incoming connections
    for {
        conn, err := listener.Accept() // Blocks until a client dials in
        if err != nil {
            break // Triggered on server shutdown
        }

        // 3. Spawns an independent goroutine for each client!
        wg.Add(1)
        go func(c net.Conn) {
            defer wg.Done()
            defer c.Close() // Guarantees socket is closed on disconnect

            // Echo loop: reads from client, writes back to client
            buf := make([]byte, 1024)
            for {
                n, err := c.Read(buf)
                if err != nil {
                    return // Client disconnected
                }
                c.Write(buf[:n]) // Echo back
            }
        }(conn)
    }

    wg.Wait() // Wait for active connections to finish before exiting
    return nil
}
```

---

# Day 2: Protocol Framing, Generic Codec & Handshake Auth

📁 **Shared Protocol:** `internal/protocol/messages.go`, `internal/protocol/codec.go`  
📁 **Client File:** `internal/client/handshake.go`  
📁 **Server File:** `internal/server/listener.go` (`PerformHandshake`)

### 1. What We Built & Why

Raw TCP has a big problem: **TCP is a stream of bytes, NOT a stream of messages.**  
If Rahul sends two separate JSON objects back-to-back:
1. `{"token":"secret123"}`
2. `{"action":"ping"}`

Because of network chunking and operating system buffers, the server might receive them merged together as:  
`{"token":"secret123"}{"action":"ping"}`  
If you pass this merged data directly to Go's `json.Unmarshal`, it crashes with a syntax error.

**Our Solutions:**
1. **Framing with Newline Delimiter (`\n`):** Every message packet ends with a newline byte (`\n`). When reading, the receiver reads byte-by-byte from the wire until it hits `\n`. It cuts the frame there and deserializes that exact single message.
2. **Generic Codec (`ReadMessage[T]` / `WriteMessage[T]`):** Using Go Generics (`[T any]`), we wrote one set of functions that serializes and deserializes *any* Go struct safely without duplicating code.
3. **The 5-Second Handshake:**
   - Rahul's client sends a `HelloMessage` containing his secret authentication token (`"dev-secret-token"`) and his requested subdomain (`"rahul"`).
   - The server enforces a strict **5-second deadline** (`conn.SetDeadline`). If a malicious client connects and stays silent to waste server resources (a Slowloris attack), the server kills the connection after 5 seconds.
   - If the token matches, the server returns an `AuthResponseMessage` with `Success: true` and `AssignedURL: "https://rahul.tunnel.dev"`.

---

### 2. Flowchart: Day 2 Handshake & Message Framing

```
RAHUL'S CLIENT LAPTOP                                        TUNNEL SERVER (:9000)
+-----------------------------------+             +-----------------------------------------+
| 1. Starts 5-Second Deadline       |             | 1. Starts 5-Second Deadline             |
| conn.SetDeadline(now + 5s)        |             | conn.SetDeadline(now + 5s)              |
+-----------------------------------+             +-----------------------------------------+
                  |                                                  |
                  | --- 2. WriteMessage(HelloMessage + '\n') ------> |
                  |     {"token":"dev-secret-token",                 |
                  |      "desired_subdomain":"rahul"}                |
                  |                                                  v
                  |                               +-----------------------------------------+
                  |                               | 3. ReadMessage[HelloMessage]            |
                  |                               | - Reads bytes until '\n' delimiter      |
                  |                               | - Deserializes JSON into struct         |
                  |                               | - Validates: token == "dev-secret-token"|
                  |                               +-----------------------------------------+
                  |                                                  |
                  |                                                  | Valid! Prepares response:
                  |                                                  | "https://rahul.tunnel.dev"
                  |                                                  v
                  | <--- 4. WriteMessage(AuthResponse + '\n') ------ |
                  |      {"success":true,                            |
                  |       "assigned_url":"https://rahul..."}         |
                  v                                                  v
+-----------------------------------+             +-----------------------------------------+
| 5. ReadMessage[AuthResponse]      |             | 5. Handshake Complete                   |
| - Verifies resp.Success == true   |             | - Clears 5-second deadline              |
| - Prints: "TUNNEL LIVE AT: ..."   |             | - Ready to elevate to Yamux (Day 3)     |
| - Clears 5-second deadline        |             |                                         |
+-----------------------------------+             +-----------------------------------------+
```

---

### 3. Technical Words Explained (Day 2)
- **Framing:** Adding clear start and end markers (we use `\n` newline) around network data so the receiver knows where one message ends and the next begins.
- **Delimiter:** The specific boundary marker character (e.g. `\n`).
- **Serialization / Marshaling:** Converting an in-memory Go struct into JSON text bytes to send across the network wire.
- **Deserialization / Unmarshaling:** Converting raw JSON text bytes received from the wire back into a strongly typed Go struct.
- **Generics (`[T any]`):** A Go feature that lets you write one function that works with any data type instead of writing separate functions for each struct.
- **Handshake:** The initial authentication exchange where credentials and requested settings are verified before real data transfer starts.
- **Deadline (`conn.SetDeadline`):** A strict network timeout clock. If data is not sent or received within the deadline, the socket aborts, protecting against slow-reading attacks.

---

### 4. Code Walkthrough (Day 2)

#### Shared Message Structs (`internal/protocol/messages.go`):
```go
package protocol

const MsgHello = "hello"

// HelloMessage: Sent by Rahul's client to authenticate
type HelloMessage struct {
    Type             string `json:"type"`              // "hello"
    Token            string `json:"token"`             // Auth token (e.g. "dev-secret-token")
    DesiredSubdomain string `json:"desired_subdomain"` // Requested subdomain (e.g. "rahul")
    ClientVersion    string `json:"client_version"`    // e.g. "1.0.0"
    OSPlatform       string `json:"os_platform"`       // "windows", "linux", "darwin"
}

// AuthResponseMessage: Sent by Server back to Rahul
type AuthResponseMessage struct {
    Type         string `json:"type"`          // "auth_response"
    Success      bool   `json:"success"`       // true if authenticated
    AssignedURL  string `json:"assigned_url"`  // "https://rahul.tunnel.dev"
    ErrorMessage string `json:"error_message"` // Reason if success == false
}
```

#### Shared Generic Codec (`internal/protocol/codec.go`):
```go
package protocol

import (
    "encoding/json"
    "io"
)

// WriteMessage serializes any struct to JSON and appends '\n' framing delimiter
func WriteMessage[T any](w io.Writer, msg T) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    data = append(data, '\n') // Framing delimiter
    _, err = w.Write(data)
    return err
}

// ReadMessage reads byte-by-byte until '\n' delimiter and unmarshals into target struct T
func ReadMessage[T any](r io.Reader) (*T, error) {
    var buf []byte
    singleByte := make([]byte, 1)

    for {
        _, err := r.Read(singleByte)
        if err != nil {
            return nil, err
        }
        if singleByte[0] == '\n' {
            break // Complete frame found!
        }
        buf = append(buf, singleByte[0])
    }

    var msg T
    if err := json.Unmarshal(buf, &msg); err != nil {
        return nil, err
    }
    return &msg, nil
}
```

#### Rahul's Client Handshake (`internal/client/handshake.go`):
```go
package client

import (
    "fmt"
    "net"
    "runtime"
    "time"

    "github.com/apoorvslst/go-ngrok/internal/protocol"
)

func PerformClientHandshake(conn net.Conn, token string, subdomain string) (*protocol.AuthResponseMessage, error) {
    // 1. Set 5-second deadline for safety
    conn.SetDeadline(time.Now().Add(5 * time.Second))
    defer conn.SetDeadline(time.Time{}) // Reset on exit

    // 2. Prepare Hello frame
    hello := protocol.HelloMessage{
        Type:             protocol.MsgHello,
        Token:            token,
        DesiredSubdomain: subdomain,
        ClientVersion:    "1.0.0",
        OSPlatform:       runtime.GOOS,
    }

    // 3. Send framed hello message to server
    if err := protocol.WriteMessage(conn, hello); err != nil {
        return nil, fmt.Errorf("failed to send hello: %w", err)
    }

    // 4. Read server's auth response
    resp, err := protocol.ReadMessage[protocol.AuthResponseMessage](conn)
    if err != nil {
        return nil, fmt.Errorf("failed to read auth response: %w", err)
    }

    if !resp.Success {
        return nil, fmt.Errorf("server rejected auth: %s", resp.ErrorMessage)
    }

    return resp, nil
}
```

---

# Day 3: Connection Multiplexing with Yamux

📁 **Client Files:** `internal/client/session.go`, `internal/client/stream_worker.go`  
📁 **Server Files:** `internal/server/listener.go` (`NewYamuxServerSession`)

### 1. What We Built & Why

Here is the central challenge of reverse tunneling:
- Rahul's laptop opened **one single TCP connection** out to our server.
- Later on, **dozens of web visitors** will visit `rahul.tunnel.dev` at the same time.
- The server cannot dial back into Rahul's laptop because his laptop is behind a home Wi-Fi router (NAT/firewall).
- How can the server send 50 separate visitor requests to Rahul's laptop over just **one** established connection?

**The Solution: Multiplexing with HashiCorp Yamux!**
Yamux takes our single physical TCP connection and turns it into a multi-lane highway:
- **On the Server:** The server wraps the TCP connection with `yamux.Server(conn, cfg)`. Whenever a visitor arrives, the server calls `session.OpenStream()` to open a new virtual lane.
- **On Rahul's Client:** Rahul's client wraps its side with `yamux.Client(conn, cfg)`. It enters a loop calling `session.AcceptStream()`. Whenever the server opens a stream, the client wakes up and handles it in a worker goroutine: `go handleIncomingStream(stream)`.
- **Keepalive Heartbeats:** Both sides configure a 15-second keepalive interval. Periodic ping packets travel across the connection so home Wi-Fi routers don't kill the connection due to inactivity.

---

### 2. Flowchart: Day 3 Yamux Stream Flow

```
+-------------------------------------------------------------------------------+
|                            TUNNEL SERVER (:9000)                              |
|  Elevated to Yamux Server Session: yamux.Server(conn, cfg)                    |
|                                                                               |
|  Visitor 1 Request ---> session.OpenStream() [Virtual Stream 1]  \            |
|  Visitor 2 Request ---> session.OpenStream() [Virtual Stream 3]  -- [YAMUX]   |
|  Visitor 3 Request ---> session.OpenStream() [Virtual Stream 5]  /            |
+-------------------------------------------------------------------------------+
                                       ||
                                       || ONE SINGLE PHYSICAL TCP PIPE
                                       || (Automatic 15-second keepalive heartbeats)
                                       ||
+-------------------------------------------------------------------------------+
|                        RAHUL'S CLIENT LAPTOP                                  |
|  Elevated to Yamux Client Session: yamux.Client(conn, cfg)                    |
|                                                                               |
|  session.AcceptStream() Loop:                                                 |
|    - Accepts Stream 1 ---> Spawns go handleIncomingStream(stream1)            |
|    - Accepts Stream 3 ---> Spawns go handleIncomingStream(stream3)            |
|    - Accepts Stream 5 ---> Spawns go handleIncomingStream(stream5)            |
+-------------------------------------------------------------------------------+
```

---

### 3. Technical Words Explained (Day 3)
- **Multiplexing:** The technique of carrying multiple separate data streams over a single physical wire at the same time.
- **Yamux (Yet Another Multiplexer):** A battle-tested Go library by HashiCorp that adds stream multiplexing over raw TCP connections.
- **Virtual Stream (`yamux.Stream`):** A lightweight logical connection running inside a Yamux session. It satisfies Go's `net.Conn` interface (you can `Read`, `Write`, and `Close` it just like a regular network socket).
- **Keepalive Interval:** Periodic small ping packets sent across the wire (every 15s) to tell intermediate firewalls and NAT routers: *"This connection is still active, please keep it open!"*

---

### 4. Code Walkthrough (Day 3)

#### Rahul's Client Yamux Session (`internal/client/session.go`):
```go
package client

import (
    "net"
    "time"

    "github.com/hashicorp/yamux"
)

// NewYamuxClientSession elevates a raw TCP connection into a multiplexed Yamux client session
func NewYamuxClientSession(conn net.Conn) (*yamux.Session, error) {
    cfg := yamux.DefaultConfig()
    cfg.EnableKeepAlive = true
    cfg.KeepAliveInterval = 15 * time.Second      // Heartbeat every 15s
    cfg.ConnectionWriteTimeout = 10 * time.Second

    session, err := yamux.Client(conn, cfg)
    if err != nil {
        return nil, err
    }
    return session, nil
}
```

#### Rahul's Stream Listener Loop (`internal/client/stream_worker.go`):
```go
package client

import (
    "fmt"
    "io"
    "log/slog"

    "github.com/hashicorp/yamux"
)

// ListenForServerStreams accepts virtual streams initiated by the server
func ListenForServerStreams(session *yamux.Session) {
    for {
        // Blocks until server forwards a visitor request via a new virtual stream
        stream, err := session.AcceptStream()
        if err != nil {
            slog.Info("yamux session closed, stopping stream listener")
            return
        }

        slog.Info("accepted new virtual stream from server", "stream_id", stream.StreamID())

        // Handle each stream in an independent parallel goroutine!
        go handleIncomingStream(stream)
    }
}

func handleIncomingStream(stream *yamux.Stream) {
    defer stream.Close()

    // Read payload sent by server over this stream
    buf := make([]byte, 1024)
    n, err := stream.Read(buf)
    if err != nil && err != io.EOF {
        return
    }

    incoming := string(buf[:n])
    slog.Info("received payload over stream", "payload", incoming)

    // Write acknowledgment response back to server
    reply := fmt.Sprintf("ACK: Rahul's laptop processed payload [%s] on stream %d", incoming, stream.StreamID())
    stream.Write([]byte(reply))
}
```

#### Server Yamux Session (`internal/server/listener.go`):
```go
// Elevates raw TCP connection to a Yamux server session
func NewYamuxServerSession(conn net.Conn) (*yamux.Session, error) {
    cfg := yamux.DefaultConfig()
    cfg.EnableKeepAlive = true
    cfg.KeepAliveInterval = 15 * time.Second

    session, err := yamux.Server(conn, cfg)
    if err != nil {
        return nil, err
    }
    return session, nil
}
```

---

# Day 4: Two-Door Server Architecture, Registry & Router

📁 **Server Files:** `internal/server/registry/registry.go`, `internal/server/router/router.go`, `internal/server/httpserver/server.go`, `cmd/server/main.go`  
📁 **Client File:** `cmd/client/main.go`

### 1. What We Built & Why

Now the server needs to handle real web traffic from the internet. To do this cleanly, the server runs **two separate listeners (two doors)**:

1. **Door 1: Port `:9000` (Control Door):**
   - Rahul's laptop connects here via TCP.
   - Handshake authentication happens here.
   - Rahul's persistent Yamux session stays open here.
2. **Door 2: Port `:8080` (Public Front Gate):**
   - Public internet visitors knock here in their web browser (e.g. `http://rahul.tunnel.dev:8080`).

**How does Port `:8080` find Rahul's laptop?**
We built two core components:
1. **The In-Memory Registry (`registry.go`):**
   - A thread-safe phonebook (`map[string]*Tunnel`) protected by a read-write mutex lock (`sync.RWMutex`).
   - When Rahul authenticates on Port `:9000`, the server registers:
     `"rahul"` -> `Rahul's Yamux Session`.
   - When Rahul disconnects, his entry is safely deleted (`defer reg.Delete("rahul")`).
2. **The Virtual Host Router (`router.go`):**
   - When a visitor visits `http://rahul.tunnel.dev:8080`, the browser sends an HTTP header: `Host: rahul.tunnel.dev:8080`.
   - The router function `ExtractSubdomain` strips the port (`:8080`) and the root domain (`.tunnel.dev`), extracting the clean subdomain: `"rahul"`.
   - The public listener looks up `"rahul"` in the Registry phonebook. If found, it routes to Rahul's tunnel! If not found, it returns `404 Not Found`.

---

### 2. Flowchart: Day 4 Two-Door Server Architecture

```
===================================================================================
                                 TUNNEL SERVER
===================================================================================

 [DOOR 1: PORT :9000]                                     [DOOR 2: PORT :8080]
  Control Listener (TCP)                                   Public Listener (HTTP)
           |                                                         |
 (Rahul connects CLI)                                     (Visitor opens website)
           |                                                         |
           v                                                         v
   Authenticates & creates                                 Visitor requests:
   Yamux Session                                           "rahul.tunnel.dev:8080"
           |                                                         |
           v                                                         v
   reg.Register("rahul", session)                          router.ExtractSubdomain(...)
           |                                               Result: "rahul"
           |                                                         |
           +-------------------> [REGISTRY PHONEBOOK] <--------------+
                                  "rahul" -> Yamux Session
                                             |
                                             v
                                  Tunnel Found! 200 OK
```

---

### 3. Technical Words Explained (Day 4)
- **In-Memory Registry:** A fast lookup dictionary stored in server RAM (`map[string]*Tunnel`) linking subdomains to active client connections.
- **`sync.RWMutex` (Read-Write Lock):** A thread-safety lock. Allows 10,000 visitors to search the map at the same exact millisecond (`RLock`), but prevents race condition crashes when adding or removing tunnels (`Lock`).
- **Host Header:** The domain name sent by web browsers in HTTP requests (e.g. `Host: rahul.tunnel.dev:8080`).
- **Virtual Host Routing:** Routing web traffic to different destinations based on the domain name in the `Host` header.
- **Two-Listener Architecture:** Running two separate network listeners on different ports in the same server process (`:9000` for developer clients, `:8080` for internet visitors).

---

### 4. Code Walkthrough (Day 4)

#### Subdomain Router (`internal/server/router/router.go`):
```go
package router

import (
    "fmt"
    "net"
    "strings"
)

// ExtractSubdomain parses the Host header: "rahul.tunnel.dev:8080" -> "rahul"
func ExtractSubdomain(hostHeader string, rootDomain string) (string, error) {
    // 1. Strip port if present: "rahul.tunnel.dev:8080" -> "rahul.tunnel.dev"
    host, _, err := net.SplitHostPort(hostHeader)
    if err != nil {
        host = hostHeader // No port in header
    }

    host = strings.ToLower(host)
    rootDomain = strings.ToLower(rootDomain)

    // 2. Check suffix: must end with ".tunnel.dev"
    suffix := "." + rootDomain
    if !strings.HasSuffix(host, suffix) {
        return "", fmt.Errorf("host '%s' is not a subdomain of '%s'", host, rootDomain)
    }

    // 3. Extract prefix: "rahul"
    subdomain := strings.TrimSuffix(host, suffix)
    if subdomain == "" {
        return "", fmt.Errorf("empty subdomain in host '%s'", hostHeader)
    }

    return subdomain, nil
}
```

#### Thread-Safe Registry (`internal/server/registry/registry.go`):
```go
package registry

import (
    "fmt"
    "sync"
    "time"

    "github.com/hashicorp/yamux"
)

type Tunnel struct {
    Subdomain  string
    Session    *yamux.Session
    ClientAddr string
    CreatedAt  time.Time
}

type Registry struct {
    mu      sync.RWMutex
    tunnels map[string]*Tunnel
}

func NewRegistry() *Registry {
    return &Registry{tunnels: make(map[string]*Tunnel)}
}

// Register adds Rahul's tunnel. Thread-safe exclusive write lock.
func (r *Registry) Register(subdomain string, session *yamux.Session, addr string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if existing, ok := r.tunnels[subdomain]; ok {
        return fmt.Errorf("subdomain '%s' already registered by %s", subdomain, existing.ClientAddr)
    }

    r.tunnels[subdomain] = &Tunnel{
        Subdomain:  subdomain,
        Session:    session,
        ClientAddr: addr,
        CreatedAt:  time.Now(),
    }
    return nil
}

// Get looks up a tunnel. Thread-safe concurrent read lock (10,000 visitors at once).
func (r *Registry) Get(subdomain string) (*Tunnel, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    tunnel, ok := r.tunnels[subdomain]
    return tunnel, ok
}

// Delete removes Rahul's tunnel when he disconnects.
func (r *Registry) Delete(subdomain string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.tunnels, subdomain)
}
```

#### Rahul's Client Pipeline (`cmd/client/main.go`):
```go
package main

import (
    "fmt"
    "log/slog"
    "os"

    "github.com/apoorvslst/go-ngrok/internal/client"
    "github.com/apoorvslst/go-ngrok/internal/config"
)

func main() {
    config.SetupLogger(true)

    token := "dev-secret-token"
    desiredSubdomain := "rahul"

    slog.Info("Starting Client...", "subdomain", desiredSubdomain)

    // Step 1: Connect to Server over TCP (:9000)
    conn, err := client.Connect("localhost:9000")
    if err != nil {
        slog.Error("Connection error:", "err", err)
        os.Exit(1)
    }
    defer conn.Close()

    // Step 2: Perform 5-second Handshake
    authResp, err := client.PerformClientHandshake(conn, token, desiredSubdomain)
    if err != nil {
        slog.Error("Handshake failed:", "err", err)
        os.Exit(1)
    }

    // Step 3: Print Live Banner
    fmt.Printf("TUNNEL LIVE AT: %s\n", authResp.AssignedURL)
    fmt.Println("Press Ctrl+C to quit.")

    // Step 4: Elevate connection to Yamux Multiplexed Client Session
    session, err := client.NewYamuxClientSession(conn)
    if err != nil {
        slog.Error("Failed to create yamux session:", "err", err)
        os.Exit(1)
    }
    defer session.Close()

    // Step 5: Listen for incoming virtual streams from server
    client.ListenForServerStreams(session)
}
```

---

# Summary Comparison Cheat Sheet (Days 1 to 4)

| Day | Feature | What Rahul's Client Does | What the Tunnel Server Does | Key Files |
| :---: | :--- | :--- | :--- | :--- |
| **Day 1** | **Raw TCP Plumbing** | Dials outbound to `server:9000` via `net.Dial`. | Listens on `:9000`, accepts socket, spawns goroutine, handles graceful shutdown. | `connector.go`<br>`listener.go` |
| **Day 2** | **Framing & Handshake** | Sends `HelloMessage` with token and `"rahul"` framed with `\n`. | Reads until `\n` under 5s deadline, verifies token, returns `https://rahul.tunnel.dev`. | `messages.go`<br>`codec.go`<br>`handshake.go` |
| **Day 3** | **Yamux Multiplexing** | Wraps TCP with `yamux.Client`, runs `AcceptStream()` loop. | Wraps TCP with `yamux.Server`, sends 15s keepalives, opens virtual streams for visitors. | `session.go`<br>`stream_worker.go` |
| **Day 4** | **Two-Door Architecture** | Stays connected on `:9000` waiting for streams. | Runs Door 1 (`:9000`) and Door 2 (`:8080`); extracts `"rahul"` from Host header; looks up session in Registry. | `registry.go`<br>`router.go`<br>`httpserver/server.go` |

---
*Created for the Go-Ngrok reverse tunneling team.*

