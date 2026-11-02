# 📅 Day 3 — Multiplexing with Yamux (Odin Project Style)

> **Rule**: Hum code nahi denge. Hum batayenge KYA likhna hai, KIS file mein, KAUNSA function, KAUNSA concept pehle samjho. Tum khud likhoge.
>
> **Apoorv** = 🔵 | **Gaurav** = 🟢 | **Both** = ⚪

---

## 🧠 What We Are Building on Day 3

Abhi tak humare paas ek single TCP connection hai between Client and Server.
Lekin socho: Agar internet se 50 visitors ek hi second mein tumhare tunnel URL (`rahul.tunnel.dev`) pe hit karte hain:
- Kya hum har visitor ke liye ek naya TCP connection banayenge?
- **NO!** Client ek laptop par hai (behind NAT/firewall). Server laptop ko naya connection nahi bhej sakta.
- Aur agar har request ko baari-baari bhejenge (FIFO), toh ek slow request (jaise file upload) baki 49 requests ko block kar degi (**Head-of-Line Blocking**)!

**The Solution: Connection Multiplexing (Yamux).**
Yamux (**Y**et **A**nother **M**Ultiple**X**er) ek standard library hai jo ek single TCP pipe ke andar hazaro independent **virtual streams** chalati hai.
Har stream apna `net.Conn` hoti hai!
```
Client Laptop <================ yamux session ================> Server VPS
(behind NAT)   ─── Stream 1: GET /api/users ──────────────────►
               ◄── Stream 1: 200 OK Response ─────────────────
               ─── Stream 2: POST /upload (Slow 5MB) ────────►
               ─── Stream 3: GET /favicon.ico (Fast) ────────► (Parallel, no blocking!)
```

---

## 🔵 Step 1: Install Yamux & Server Session Wrapper (Apoorv — 30 min)

### Concept samjho pehle (10 min):
> **`yamux.Server` vs `yamux.Client`:**
> Dono sides ek physical `net.Conn` ko wrap karti hain.
> - Server side pe hum `yamux.Server(conn, cfg)` call karte hain.
> - Client side pe hum `yamux.Client(conn, cfg)` call karte hain.
>
> **Keepalive Configuration:**
> Agar developer apna laptop band kar deta hai (lid close), toh OS ko TCP connection gracefully close karne ka mauka nahi milta. Server ko lagta hai laptop abhi bhi connected hai!
> Yamux ke andar built-in **KeepAlive** hota hai. Har 15 second mein background ping bhejta hai. Agar 3 pings fail ho jayein $\rightarrow$ session automatically terminate ho jata hai!

### Terminal mein dependency add karo:
```bash
go get github.com/hashicorp/yamux
```

### File: `internal/server/session.go`
### Package: `package server`

### Function: `NewYamuxServerSession(conn net.Conn) (*yamux.Session, error)`
**Ye function kya karega:**
1. Yamux config banao:
   ```go
   cfg := yamux.DefaultConfig()
   cfg.EnableKeepAlive = true
   cfg.KeepAliveInterval = 15 * time.Second
   cfg.ConnectionWriteTimeout = 10 * time.Second
   ```
2. Call karo: `session, err := yamux.Server(conn, cfg)`
3. Error check karo.
4. Return `session, nil`.

---

## 🟢 Step 2: Client Yamux Session Wrapper (Gaurav — 30 min)

### Concept samjho pehle:
> Client side par physical TCP connection handshake ke turant baad `yamux.Client` mein convert ho jayega.
> Ek baar session ban gaya, client `session.AcceptStream()` loop mein baith jayega, server se aane wali visitor requests ka wait karega.

### File: `internal/client/session.go`
### Package: `package client`

### Function: `NewYamuxClientSession(conn net.Conn) (*yamux.Session, error)`
**Ye function kya karega:**
1. Config banao:
   ```go
   cfg := yamux.DefaultConfig()
   cfg.EnableKeepAlive = true
   cfg.KeepAliveInterval = 15 * time.Second
   ```
2. Call karo: `session, err := yamux.Client(conn, cfg)`
3. Error check karo.
4. Return `session, nil`.

---

## 🟢 Step 3: Client Stream Acceptor Loop (Gaurav — 30 min)

### Concept samjho pehle:
> Ek `yamux.Session` khud Go ka `net.Listener` interface implement karta hai!
> Iska matlab hum `session.Accept()` ya `session.AcceptStream()` ko normal server ki tarah loop mein chala sakte hain!

### File: `internal/client/stream_worker.go`
### Package: `package client`

### Function 1: `ListenForServerStreams(session *yamux.Session)`
**Ye function kya karega:**
1. Ek infinite loop chalao:
   ```go
   for {
       stream, err := session.AcceptStream()
       if err != nil {
           slog.Info("yamux session closed by server", "err", err)
           return
       }
       slog.Info("accepted new incoming virtual stream", "stream_id", stream.StreamID())
       go handleIncomingStream(stream)
   }
   ```

### Function 2: `handleIncomingStream(stream *yamux.Stream)`
*(Abhi ke liye dummy echo ya message return karenge)*:
1. `defer stream.Close()`
2. Stream se message read karo.
3. Stream mein reply likho: `"Hello from client over virtual stream!"`.

---

## 🔵 Step 4: Server Stream Initiator & Test Generator (Apoorv — 30 min)

### Concept samjho pehle:
> Jab server ko visitor ka request milega, server client ke session par **naya stream open karega**:
> `stream, err := session.OpenStream()`
> Ye ek naya virtual channel banata hai. Server is stream mein HTTP request push karega aur response read karega.

### File: `internal/server/test_dispatcher.go`
### Package: `package server`

### Function: `DispatchTestStreams(session *yamux.Session, count int)`
**Ye function kya karega:**
1. Ek `sync.WaitGroup` banao.
2. `count` number of goroutines launch karo (e.g. 10 streams):
   ```go
   for i := 1; i <= count; i++ {
       wg.Add(1)
       go func(id int) {
           defer wg.Done()
           stream, err := session.OpenStream()
           if err != nil {
               slog.Error("failed to open stream", "stream_id", id, "err", err)
               return
           }
           defer stream.Close()

           // Write test payload
           fmt.Fprintf(stream, "Ping on virtual stream %d", id)

           // Read reply from client
           buf := make([]byte, 128)
           n, _ := stream.Read(buf)
           slog.Info("received reply over virtual stream", "stream_id", id, "reply", string(buf[:n]))
       }(i)
   }
   wg.Wait()
   ```

---

## ⚪ Step 5: The Multiplexing Load Test (Both — 30 min)

### Verification Test:
1. Terminal 1 mein server chalu karo.
2. Terminal 2 mein client connect karo.
3. Client handshake pass karega $\rightarrow$ Dono taraf Yamux session activate hoga.
4. Server turant 10 parallel streams launch karega ek hi TCP socket ke upar!

### Expected Console Output:
```text
[Server] Handshake successful! Initialized Yamux session.
[Client] Yamux client session ready. Waiting for virtual streams...
[Server] Opened virtual stream id=1
[Server] Opened virtual stream id=2
...
[Client] Accepted new incoming virtual stream stream_id=1
[Client] Accepted new incoming virtual stream stream_id=2
[Server] Received reply over virtual stream stream_id=1 reply="Hello from client over virtual stream!"
```

### Experiments to Try:
1. **Lid Close Simulation**: Client terminal ko band kiye bina internet disconnect karo. 45 seconds ke baad observe karo — kya server ka keepalive session ko automatically close kar deta hai?
2. **Stream Count**: Count ko 10 se badha kar 100 karo. Dekho kya saare 100 streams ek saath complete hote hain bina packet drop ke!

---

## ✅ Day 3 Checklist
- [ ] `github.com/hashicorp/yamux` installed
- [ ] Server wraps connection in `yamux.Server` with keepalives
- [ ] Client wraps connection in `yamux.Client`
- [ ] Client successfully accepts multiple simultaneous virtual streams
- [ ] Zero connection collisions or head-of-line blocking observed
