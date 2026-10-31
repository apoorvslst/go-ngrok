# 📅 Day 1 — Step-by-Step Guide (Odin Project Style)

> **Rule**: Hum code nahi denge. Hum batayenge KYA likhna hai, KIS file mein, KAUNSA function, KAUNSA concept pehle samjho. Tum khud likhoge.
>
> **Apoorv** = 🔵 | **Gaurav** = 🟢 | **Both** = ⚪

---

## ⚪ Step 0: Go Environment Check (Both — 5 min)

**Before anything**, terminal mein ye run karo:

```bash
go version
```

Agar `go1.21` ya upar dikhta hai → ✅ aage badho.
Agar nahi → [Go install karo](https://go.dev/doc/install) pehle.

Also check:
```bash
git --version
```

---

## 🔵 Step 1: Project Initialize (Apoorv — 15 min)

### Concept samjho pehle (5 min):
> **Go Modules** = Go ka dependency management system. Har Go project ka ek `go.mod` file hota hai jo batata hai:
> - Project ka naam (module path)
> - Go version
> - Dependencies
>
> `go mod init` se ye file banti hai. Iske bina Go ko nahi pata ki tumhara code ek "project" hai.

### Kya karna hai:
1. Repo folder mein jao (`go-ngrok/`)
2. Run karo:
   ```bash
   go mod init github.com/yourusername/go-ngrok
   ```
3. Dekho ki `go.mod` file ban gayi. Usse kholo aur padho — 2 lines hongi.

### Verify:
- [ ] `go.mod` file exists
- [ ] Usme module name aur go version dikh raha hai

---

## 🔵 Step 2: Folder Structure (Apoorv — 10 min)

### Concept samjho pehle (5 min):
> **Go project layout convention:**
> - `cmd/` → Har binary ka entry point. `cmd/server/main.go` matlab ek `server` binary banega.
> - `internal/` → Private packages. Sirf is project ke andar se import ho sakte hain, bahar se nahi.
> - Har folder ek "package" hai. Folder ka naam = package ka naam.
>
> 📖 Read: [Project layout README](https://github.com/golang-standards/project-layout) — sirf `cmd/` aur `internal/` sections padho.

### Kya karna hai:
Ye folders banao (files abhi empty rakh sakte ho ya next step mein banao):

```
go-ngrok/
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── client/
│       └── main.go
├── internal/
│   ├── server/
│   ├── client/
│   └── config/
├── go.mod
├── PROBLEM_STATEMENT.md
├── BEFORE_YOU_CODE.md
└── DAY1_TASKS.md
```

### Verify:
- [ ] `cmd/server/` aur `cmd/client/` folders exist
- [ ] `internal/` ke andar `server/`, `client/`, `config/` folders hain

---

## 🔵 Step 3: Server ka `main.go` — Hello World (Apoorv — 10 min)

### Concept samjho pehle:
> Har Go program `package main` se shuru hota hai aur usmein ek `func main()` hota hai. Yahi entry point hai.
>
> `cmd/server/main.go` matlab jab tum `go run ./cmd/server` karoge, ye file chalegi.

### File: `cmd/server/main.go`

### Kya likhna hai:
1. `package main`
2. `func main()` jo print kare: `"GoNgrok Server starting..."`
3. Run karo: `go run ./cmd/server`
4. Output dekho

### Verify:
```bash
go run ./cmd/server
# Output: GoNgrok Server starting...
```
- [ ] Runs without errors
- [ ] Print dikhta hai

---

## 🟢 Step 4: Client ka `main.go` — Hello World (Gaurav — 10 min)

### Same concept as Step 3.

### File: `cmd/client/main.go`

### Kya likhna hai:
1. `package main`
2. `func main()` jo print kare: `"GoNgrok Client starting..."`
3. Run karo: `go run ./cmd/client`

### Verify:
```bash
go run ./cmd/client
# Output: GoNgrok Client starting...
```
- [ ] Runs without errors

---

## 🔵 Step 5: TCP Listener Function (Apoorv — 30 min)

### Concept samjho pehle (10 min):
> **TCP Server in Go — 3 building blocks:**
>
> 1. **`net.Listen("tcp", ":9000")`** → "Main is port pe sun raha hoon." Returns a `net.Listener`.
> 2. **`listener.Accept()`** → "Koi aaya? Ruko, wait karta hoon." Blocks until a client connects. Returns a `net.Conn`.
> 3. **`net.Conn`** → Ek connection. Isme tum `Read()` aur `Write()` kar sakte ho. Jab kaam khatam → `Close()`.
>
> `Accept()` ek loop mein chalta hai — har baar ek naya client aata hai.
>
> 📖 Read: [net.Listen docs](https://pkg.go.dev/net#Listen) — example dekho page pe

### File: `internal/server/listener.go`

### Package: `package server`

### Function 1: `StartListener`

**Signature likhni hai:**
```
func StartListener(addr string) error
```

**Ye function kya karega:**
1. `net.Listen("tcp", addr)` call karo — error handle karo
2. `defer listener.Close()` — function khatam hone pe listener band ho
3. Print karo: `"Listening on <addr>"`
4. Ek **infinite `for` loop** mein:
   - `listener.Accept()` call karo
   - Error aaye toh print karo aur `continue`
   - Connection aaye toh print karo: `"New connection from <remote address>"`
   - Abhi ke liye **turant `conn.Close()`** kar do (baad mein handle karenge)

### `cmd/server/main.go` update karo:
- Import `internal/server` package
- `main()` mein call karo: `server.StartListener(":9000")`

### Verify:
```bash
# Terminal 1:
go run ./cmd/server
# Output: Listening on :9000

# Terminal 2 (test connection):
# Windows PowerShell:
Test-NetConnection -ComputerName localhost -Port 9000
# Ya agar telnet hai:
telnet localhost 9000
```
- [ ] Server start hota hai bina error ke
- [ ] Connection aane pe "New connection from ..." print hota hai
- [ ] Connection turant close ho jaata hai (kyunki abhi handle nahi kar rahe)

---

## 🟢 Step 6: TCP Dial Function (Gaurav — 30 min)

### Concept samjho pehle (10 min):
> **TCP Client in Go:**
>
> **`net.Dial("tcp", "localhost:9000")`** → Server se connect karo. Returns `net.Conn`.
>
> `net.Conn` ek **interface** hai with 3 important methods:
> - `Read(buf []byte) (n int, err error)` — bytes padho
> - `Write(buf []byte) (n int, err error)` — bytes likho
> - `Close() error` — connection band karo
>
> Jab server ya client connection close karta hai, doosri side ko `Read()` se `io.EOF` error milta hai.
>
> 📖 Read: [net.Dial docs](https://pkg.go.dev/net#Dial)

### File: `internal/client/connector.go`

### Package: `package client`

### Function 1: `Connect`

**Signature:**
```
func Connect(serverAddr string) (net.Conn, error)
```

**Ye function kya karega:**
1. `net.Dial("tcp", serverAddr)` call karo
2. Error aaye toh return karo
3. Print karo: `"Connected to <serverAddr>"`
4. `net.Conn` return karo (caller close karega)

### `cmd/client/main.go` update karo:
- Import `internal/client` package
- `main()` mein:
  1. `client.Connect("localhost:9000")` call karo
  2. Error handle karo
  3. `defer conn.Close()`
  4. Print karo: `"Connection established, local addr: <conn.LocalAddr()>"`
  5. Abhi ke liye bas `time.Sleep(2 * time.Second)` karo taaki connection thodi der rahe, phir exit

### Verify:
```bash
# Terminal 1: pehle server chalu karo (Apoorv ka code)
go run ./cmd/server

# Terminal 2: client chalao
go run ./cmd/client
# Output: Connected to localhost:9000
# Output: Connection established, local addr: 127.0.0.1:xxxxx
```
- [ ] Client connect hota hai
- [ ] Server pe "New connection from ..." dikhta hai
- [ ] 2 seconds baad client exit hota hai

---

## 🔵 Step 7: Handle Connection with `io.Copy` — Echo (Apoorv — 30 min)

### Concept samjho pehle (10 min):
> **`io.Copy(dst, src)`** — src se bytes padhta hai, dst mein likhta hai. Jab tak src se `EOF` ya error na aaye, tab tak chalta rehta hai.
>
> **Echo server** = jo bhi client bheje, wahi wapas bhejo.
> Trick: `io.Copy(conn, conn)` — conn se padho, conn mein likho = echo!
>
> **Goroutines**: Agar tum `Accept()` ke baad directly handle karte ho, toh ek time pe sirf ek client handle hoga. `go handleConn(conn)` se har client apna goroutine paata hai — sab parallel chalte hain.
>
> 📖 Read: [io.Copy docs](https://pkg.go.dev/io#Copy) — sirf 1 paragraph hai, dhyan se padho
> 📖 Read: [Go by Example: Goroutines](https://gobyexample.com/goroutines)

### File: `internal/server/listener.go` (same file, add a new function)

### Function 2: `handleConnection`

**Signature:**
```
func handleConnection(conn net.Conn)
```

**Ye function kya karega:**
1. `defer conn.Close()`
2. Print karo: `"Handling connection from <remote addr>"`
3. `io.Copy(conn, conn)` — echo! Jo aaye wahi wapas
4. `io.Copy` ke baad print karo: `"Connection closed from <remote addr>"`

### `StartListener` update karo:
- `conn.Close()` hatao (woh `handleConnection` mein hoga ab)
- `go handleConnection(conn)` karo — **goroutine** mein!

### Verify:
Step 8 ke baad test karenge (Gaurav ka client chahiye)

---

## 🟢 Step 8: Send & Receive Data (Gaurav — 30 min)

### Concept samjho pehle (10 min):
> **Bidirectional communication** ke liye 2 kaam parallel hone chahiye:
> 1. User se input lo → server ko bhejo
> 2. Server se response aaye → screen pe dikhao
>
> Ye dono ek saath hone chahiye. Agar sequentially karoge (pehle bhejo, phir padho) toh block ho jaoge.
>
> Solution: **2 goroutines**.
>
> **`io.Copy(conn, os.Stdin)`** → keyboard se padho, server ko bhejo
> **`io.Copy(os.Stdout, conn)`** → server se padho, screen pe dikhao
>
> 📖 Read: [io.Copy docs](https://pkg.go.dev/io#Copy) — agar pehle padha hai toh phir se padho, is baar sochte hue ki stdin/stdout ke saath kaise use hoga

### File: `internal/client/connector.go` (same file, new function)

### Function 2: `HandleEcho`

**Signature:**
```
func HandleEcho(conn net.Conn)
```

**Ye function kya karega:**
1. Print karo: `"Type something and press Enter (Ctrl+C to quit):"`
2. Ek goroutine launch karo: `go io.Copy(conn, os.Stdin)` — stdin → server
3. Main goroutine mein: `io.Copy(os.Stdout, conn)` — server → stdout
4. Jab `io.Copy` return kare (matlab connection band hua), print karo: `"Disconnected"`

### `cmd/client/main.go` update karo:
- `time.Sleep` hatao
- `client.HandleEcho(conn)` call karo

### Verify — THE BIG TEST:
```bash
# Terminal 1:
go run ./cmd/server
# Output: Listening on :9000

# Terminal 2:
go run ./cmd/client
# Output: Connected to localhost:9000
# Output: Type something and press Enter:

# Ab type karo "hello" aur Enter dabaao
# Output: hello     ← ye server se echo hoke wapas aaya!

# Multiple clients test karo — Terminal 3 mein bhi client chalao
# Dono independently kaam karenge
```

### Experiments karo (IMPORTANT):
Ye experiments karo aur note karo kya hua:

| Experiment | Kya karo | Observe karo |
|---|---|---|
| **E1** | Client chal raha hai, server band karo (Ctrl+C) | Client ko kya hota hai? Kya error aata hai? |
| **E2** | Dono chal rahe hain, client band karo (Ctrl+C) | Server ko kya print hota hai? |
| **E3** | 3 clients ek saath connect karo, sab mein type karo | Kya sab independently kaam karte hain? |
| **E4** | Client mein `io.Copy` ke return value (bytes, error) print karo | Kitne bytes copy hue? Error kya hai? |

- [ ] Echo kaam karta hai (type karo → wahi wapas aaye)
- [ ] Multiple clients parallel kaam karte hain
- [ ] Sab experiments kiye aur samjhe

---

## 🔵 Step 9: Graceful Shutdown (Apoorv — 40 min)

### Concept samjho pehle (15 min):
> **Problem**: Abhi agar tum server pe Ctrl+C karte ho, toh program turant mar jaata hai. Active connections brutally cut ho jaate hain.
>
> **Graceful shutdown** = "Naye connections mat lo, but jo chal rahe hain unhe finish hone do, phir bando."
>
> **3 tools chahiye:**
>
> 1. **`signal.NotifyContext`** — Ctrl+C (SIGINT) aane pe context cancel karo
>    ```
>    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
>    ```
>    Jab Ctrl+C aaye → `ctx.Done()` channel close ho jaata hai → sabko pata chal jaata hai
>
> 2. **`sync.WaitGroup`** — count rakho kitne connections active hain
>    - Connection aaye → `wg.Add(1)`
>    - Connection close ho → `wg.Done()`
>    - Shutdown pe → `wg.Wait()` (sab khatam hone ka intezaar karo)
>
> 3. **Listener close karna** — `listener.Close()` karo toh `Accept()` turant error return karta hai → loop khatam
>
> 📖 Read: [signal.NotifyContext docs](https://pkg.go.dev/os/signal#NotifyContext)
> 📖 Read: [sync.WaitGroup docs](https://pkg.go.dev/sync#WaitGroup)
> 📖 Read: [Go blog: Context](https://go.dev/blog/context) — "WithCancel" aur "Done" sections

### File: `internal/server/listener.go` (modify existing functions)

### Changes:

**`StartListener` ko update karo — signature badlo:**
```
func StartListener(ctx context.Context, addr string) error
```

**Function ke andar:**
1. `net.Listen` se listener banao (same as before)
2. Ek `sync.WaitGroup` banao (local variable)
3. Ek **alag goroutine** launch karo jo:
   - `<-ctx.Done()` pe wait kare (matlab Ctrl+C ka intezaar)
   - Jab context cancel ho → `listener.Close()` kare
   - Print kare: `"Shutdown signal received, stopping listener..."`
4. Accept loop mein:
   - `Accept()` se error aaye aur context cancelled hai → loop se bahar aao (ye normal shutdown hai, error nahi)
   - Connection aaye → `wg.Add(1)` karo
   - Goroutine mein handle karo, aur `handleConnection` ke baad `wg.Done()` karo
5. Loop ke baad:
   - Print karo: `"Waiting for active connections to finish..."`
   - `wg.Wait()`
   - Print karo: `"All connections done. Server stopped."`

**`cmd/server/main.go` update karo:**
1. `signal.NotifyContext` se ctx banao
2. `defer stop()` karo
3. `server.StartListener(ctx, ":9000")` call karo

### Verify:
```bash
# Terminal 1: Server start karo
go run ./cmd/server

# Terminal 2: Client connect karo, type karte raho
go run ./cmd/client

# Terminal 1 mein Ctrl+C dabaao
# Expected:
#   "Shutdown signal received, stopping listener..."
#   "Waiting for active connections to finish..."
#   (client ka connection continue rehta hai jab tak woh type kar raha hai)
#   (jab client band ho → "Connection closed from ...")
#   "All connections done. Server stopped."
```

- [ ] Ctrl+C pe server turant nahi marta
- [ ] Active connections finish hone deta hai
- [ ] Sab connections khatam hone ke baad exit hota hai
- [ ] Naye connections accept nahi hote Ctrl+C ke baad

---

## 🟢 Step 10: Structured Logging with `slog` (Gaurav — 30 min)

### Concept samjho pehle (10 min):
> **Problem with `fmt.Println`:**
> ```
> connected                          ← kab? kis client ne? kaunsa port?
> ```
>
> **`slog` se structured logging:**
> ```
> level=INFO msg="client connected" remote_addr=127.0.0.1:52341 time=2024-01-15T10:30:00Z
> ```
>
> Har log line mein **key-value pairs** hote hain. Searchable, parseable, professional.
>
> **Basic usage:**
> ```go
> slog.Info("something happened", "key1", value1, "key2", value2)
> slog.Error("something failed", "err", err, "key", value)
> ```
>
> 📖 Read: [Go blog: Structured Logging (slog)](https://go.dev/blog/slog) — poora padho, 10 min lagega

### File: `internal/client/connector.go` (modify existing functions)

### Changes:

1. **`Connect` function mein:**
   - `fmt.Println` hatao, `slog.Info` lagao
   - Fields: `"server_addr"`, value
   - Error pe: `slog.Error` with `"err"` field

2. **`HandleEcho` function mein:**
   - Connection start pe: `slog.Info` with `"local_addr"` aur `"remote_addr"`
   - `io.Copy` ke baad: `slog.Info` with `"bytes_copied"` (io.Copy returns byte count)
   - Error pe: `slog.Error` with `"err"` field

### BONUS — Naya file banao: `internal/config/logger.go`

### Package: `package config`

### Function: `SetupLogger`

**Signature:**
```
func SetupLogger(debug bool)
```

**Kya karega:**
1. Agar `debug` true hai → log level `slog.LevelDebug` set karo
2. Agar false → log level `slog.LevelInfo`
3. `slog.SetDefault()` se default logger set karo
4. Text handler use karo (`slog.NewTextHandler`)

**`cmd/client/main.go` mein:**
- `config.SetupLogger(true)` call karo sabse pehle

### Verify:
```bash
go run ./cmd/client
# Output should look like:
# time=2024-01-15T10:30:00.000+05:30 level=INFO msg="connecting to server" server_addr=localhost:9000
# time=2024-01-15T10:30:00.001+05:30 level=INFO msg="connected" local_addr=127.0.0.1:52341 remote_addr=127.0.0.1:9000
```

- [ ] Koi bhi `fmt.Println` nahi hai client code mein — sab `slog`
- [ ] Har log line mein structured fields hain
- [ ] `SetupLogger` function kaam karta hai

---

## 🔵 Step 11: Server Mein Bhi `slog` Lagao (Apoorv — 15 min)

### Kya karna hai:
Gaurav ka `config.SetupLogger` function use karo aur server ke saare `fmt.Println` ko `slog` se replace karo:

1. `cmd/server/main.go` mein `config.SetupLogger(true)` call karo
2. `StartListener` mein:
   - `slog.Info("server starting", "addr", addr)`
   - `slog.Info("new connection", "remote_addr", conn.RemoteAddr())`
   - `slog.Info("shutdown signal received")`
   - `slog.Info("waiting for connections to drain", "active", <count>)`
   - `slog.Info("server stopped")`
3. `handleConnection` mein:
   - `slog.Info("handling connection", "remote_addr", ...)`
   - `slog.Info("connection closed", "remote_addr", ..., "bytes_copied", ...)`

### Verify:
- [ ] Server ke saare logs structured hain
- [ ] `fmt.Println` koi nahi bacha

---

## 🤝 Step 12: PR, Review & Explain (Both — 30 min)

### Gaurav:
```bash
git checkout -b day1/gaurav
git add .
git commit -m "day1: TCP client, HandleEcho with io.Copy, slog logging, SetupLogger"
git push origin day1/gaurav
# GitHub pe PR banao
```

### Apoorv:
```bash
git checkout -b day1/apoorv
git add .
git commit -m "day1: project structure, TCP echo server, graceful shutdown, slog"
git push origin day1/apoorv
# GitHub pe PR banao
```

### Review:
- Apoorv reviews Gaurav's PR → comment karo agar kuch samajh nahi aaya
- Gaurav reviews Apoorv's PR → comment karo agar kuch samajh nahi aaya
- Merge karo (resolve conflicts if any — `main.go` files mein conflict aa sakta hai)

### Explain to each other:
**Apoorv explains to Gaurav:**
1. `StartListener` function line by line
2. Goroutine kaise launch hota hai har connection ke liye
3. Graceful shutdown: context cancel → listener close → wg.Wait → exit

**Gaurav explains to Apoorv:**
1. `Connect` aur `HandleEcho` line by line
2. Two `io.Copy` goroutines kaise kaam karte hain
3. `slog` kaise use kiya, `SetupLogger` kya karta hai

---

## ✅ Day 1 Final Checklist

### Files Created:

| File | Kaun | Contains |
|------|------|----------|
| `go.mod` | Apoorv | Module definition |
| `cmd/server/main.go` | Apoorv | Entry point, context setup, calls `StartListener` |
| `cmd/client/main.go` | Gaurav | Entry point, calls `Connect` + `HandleEcho` |
| `internal/server/listener.go` | Apoorv | `StartListener(ctx, addr)` + `handleConnection(conn)` |
| `internal/client/connector.go` | Gaurav | `Connect(addr)` + `HandleEcho(conn)` |
| `internal/config/logger.go` | Gaurav | `SetupLogger(debug)` |

### Functions Written:

| Function | File | Kya karta hai |
|----------|------|---------------|
| `server.StartListener(ctx, addr)` | listener.go | TCP listen, accept loop, graceful shutdown |
| `server.handleConnection(conn)` | listener.go | Echo with `io.Copy`, defer close |
| `client.Connect(addr)` | connector.go | `net.Dial`, return conn |
| `client.HandleEcho(conn)` | connector.go | 2x `io.Copy` (stdin↔conn), bidirectional |
| `config.SetupLogger(debug)` | logger.go | Configure `slog` default logger |

### Concepts Understood:

- [ ] `net.Listen` + `Accept` loop
- [ ] `net.Dial` — outbound connection
- [ ] `net.Conn` interface — Read, Write, Close
- [ ] `io.Copy` — blocks until EOF or error
- [ ] Goroutine per connection — `go handleConn(conn)`
- [ ] `context.Context` — cancellation signal
- [ ] `signal.NotifyContext` — Ctrl+C handling
- [ ] `sync.WaitGroup` — wait for goroutines
- [ ] `slog` — structured logging
- [ ] `defer` — cleanup on function exit

---

## 📚 Sources — In Order of Reading

### Before Starting (15 min total):
1. [Go project layout](https://github.com/golang-standards/project-layout) — cmd/ aur internal/ samjho (5 min)
2. [Go modules tutorial](https://go.dev/doc/tutorial/create-module) — go mod init samjho (5 min)
3. [io.Copy docs](https://pkg.go.dev/io#Copy) — 1 paragraph, dhyan se padho (2 min)

### During Steps 5-8 (TCP):
4. [net.Listen docs + example](https://pkg.go.dev/net#Listen) — TCP server pattern (5 min)
5. [net.Dial docs](https://pkg.go.dev/net#Dial) — TCP client (3 min)
6. [Go by Example: Goroutines](https://gobyexample.com/goroutines) — `go` keyword (5 min)

### During Step 9 (Shutdown):
7. [Go blog: Context](https://go.dev/blog/context) — context pattern (10 min)
8. [signal.NotifyContext docs](https://pkg.go.dev/os/signal#NotifyContext) (3 min)
9. [sync.WaitGroup docs](https://pkg.go.dev/sync#WaitGroup) (3 min)

### During Step 10 (Logging):
10. [Go blog: Structured Logging (slog)](https://go.dev/blog/slog) (10 min)

---

*Day 1 khatam hone pe mujhe batao. Day 2 mein yamux introduce karenge — ek connection ke andar multiple streams!*
