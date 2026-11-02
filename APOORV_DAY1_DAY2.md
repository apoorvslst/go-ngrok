# 🚀 Apoorv's Simple Guide — Day 1 & Day 2 (Server & Protocol)

Hey! This document explains **exactly what you built, why you built it, and how it works in the real world** — in simple words, with zero complex diagrams.

---

## 🗺️ The Big Picture

Imagine your friend **Rahul** made a website on his laptop (`localhost:3000`). Only he can see it.  
He wants to show it to a client in the US.

**Your job (Apoorv)**: You built the **VPS Server** running in the cloud.  
Rahul connects his laptop to your server, and your server gives him a public link (`https://rahul-app.tunnel.dev`) to share with the world!

---

# 📅 DAY 1: The Server Core (Connections & Shutdown)

---

### 1️⃣ Task: The TCP Listener & Accept Loop
**File**: `internal/server/listener.go`

* **What you did:**
  You wrote `StartListener()`. It opens TCP port `:9000` on the server and waits in a loop for incoming connections. Whenever someone connects, it immediately kicks them into a new **goroutine** (`go handleConnection`).

* **Why you did it:**
  If you don't use goroutines, your server is **single-threaded**.  
  That means if Rahul connects, Gaurav CANNOT connect until Rahul disconnects! By adding `go handleConnection(...)`, your server can handle 10,000 Rahuls at the exact same time.

* **Real-World Analogy:**
  > Think of a **Restaurant Receptionist**.  
  > The receptionist doesn't cook your food or serve you. She greets you at the door, assigns a waiter to your table, and immediately turns around to greet the next customer in line.

```text
Rahul connects  ──┐
                  ├──► [Receptionist: listener.Accept()] ──► Assigns Waiter 1 (Goroutine)
Gaurav connects ──┘                                      ──► Assigns Waiter 2 (Goroutine)
```

---

### 2️⃣ Task: Graceful Shutdown (No Broken Downloads)
**Files**: `cmd/server/main.go` & `internal/server/listener.go`

* **What you did:**
  You added `signal.NotifyContext` to catch `Ctrl+C`, and `sync.WaitGroup` (`wg.Add`, `wg.Done`, `wg.Wait`) to track active connections.

* **Why you did it:**
  If you are running a server and press `Ctrl+C` without this:
  The operating system **kills the server instantly**. If Rahul was in the middle of sending a 50MB file, his transfer instantly fails!  
  With graceful shutdown:
  1. The server stops accepting *new* users.
  2. The server *waits* for Rahul to finish his active transfer.
  3. The server shuts down cleanly.

* **Real-World Analogy:**
  > Think of **Closing Time at a Restaurant**.  
  > At 11:00 PM, the manager flips the door sign to "CLOSED" (no new customers can enter), but they **don't kick out** the people currently eating dinner. They wait for them to finish, pay the bill, and then lock up.

```text
Admin presses Ctrl+C
   │
   ▼
1. Flip door sign to "CLOSED" (listener.Close())
   │
   ▼
2. Wait for current diners to finish eating (wg.Wait())
   │
   ▼
3. Everyone leaves -> Turn off lights safely! (Server Exit Code 0)
```

---

# 📅 DAY 2: The Framing Codec & Handshake Engine

---

### 3️⃣ Task: Message Framing (`WriteMessage` & `ReadMessage`)
**File**: `internal/protocol/codec.go`

* **What you did:**
  You built a codec that writes and reads messages using **Newline-Delimited JSON** (`\n` at the end of every message), powered by Go Generics `[T any]`.

* **Why you did it:**
  TCP is a **stream of water (bytes)**, NOT individual packets.  
  If Rahul quickly sends two messages:
  1. `{"type":"HELLO"}`
  2. `{"type":"PING"}`  
  
  The internet can stick them together into one chunk:  
  `{"type":"HELLO"}{"type":"PING"}` $\rightarrow$ **JSON parser crashes!**  
  
  Your code appends a `\n` after every message:  
  `{"type":"HELLO"}\n{"type":"PING"}\n`  
  Now the server reads line-by-line and never gets confused!

* **Real-World Analogy:**
  > Imagine receiving a telegram with NO SPACES OR FULL STOPS:  
  > `IARRIVEDSAFELYCALLME` $\rightarrow$ Hard to read.  
  > With your `\n` delimiter, it's:  
  > `I ARRIVED SAFELY. CALL ME.` $\rightarrow$ Perfect clear sentences.

```text
Raw TCP Pipe:
[ {"type":"HELLO"} ] + [ {"type":"PING"} ]
       │
       ▼
Without Codec: {"type":"HELLO"}{"type":"PING"}         ──► CRASH! (Invalid JSON)
With Codec:    {"type":"HELLO"}\n + {"type":"PING"}\n  ──► SAFE! Reads line by line
```

---

### 4️⃣ Task: The 5-Second Anti-DoS Stopwatch
**File**: `internal/server/handshake.go`

* **What you did:**
  Inside `PerformHandshake`, you added:
  `conn.SetReadDeadline(time.Now().Add(5 * time.Second))`

* **Why you did it:**
  To stop the famous **Slowloris DoS Attack**.  
  A hacker could write a script that opens 5,000 connections to your server and sends **nothing**. If you don't have a timer, your server will wait forever, waste all its RAM, and crash!  
  Your timer says: *"Send your HELLO within 5 seconds, or get kicked out!"*

* **Real-World Analogy:**
  > An **ATM Machine Timeout**.  
  > You insert your ATM card. If you stand there staring at the screen for 30 seconds without typing your PIN, the ATM beeps and cancels the session. It won't let you block the line forever.

---

### 5️⃣ Task: Subdomain & Token Verification
**File**: `internal/server/handshake.go`

* **What you did:**
  You validated that:
  1. The client actually sent a `"HELLO"` message.
  2. The secret token matches `"dev-secret-token"`.
  3. The subdomain is valid (3-30 letters, numbers, hyphens only).
  4. If good $\rightarrow$ send back `AUTH_OK` with URL `https://rahul-app.tunnel.dev`.
  5. If bad $\rightarrow$ send back `AUTH_ERR` with an explanation.

* **Why you did it:**
  1. **Security**: We can't let random strangers use our tunnel without a valid token.
  2. **DNS Safety**: If Rahul asks for `"bad@sub!domain"`, domain routing would break on the internet. Your `isValidSubdomain` ensures it's clean and safe.
  3. **User Experience**: If Rahul gives the wrong password, we don't just cut the wire silently. We send a clean JSON error message so his terminal says: *"Error: Invalid or expired token"*.

* **Real-World Analogy:**
  > Think of **Airport Boarding Gate**:
  > - Check 1: Show your boarding pass (`MsgHello`).
  > - Check 2: Check your ID against the ticket (`Token`).
  > - Check 3: Check your bag weight (`isValidSubdomain`).
  > - Passed? Here is your gate number (`https://rahul-app.tunnel.dev`). Welcome aboard!

---

## 🎬 The Complete 6-Step Story: Putting It All Together

```text
Step 1: Apoorv's Server starts on :9000 and waits.
        (listener.Accept)

Step 2: Rahul runs: $ tunnel --subdomain "rahul-app" --token "secret123"
        (TCP Dial connects to :9000)

Step 3: Apoorv's Server starts a 5-second stopwatch.
        (conn.SetReadDeadline)

Step 4: Rahul's client sends:
        {"type":"HELLO", "token":"secret123", "desired_subdomain":"rahul-app"}\n

Step 5: Apoorv's Server checks:
        - Is it a HELLO?                 -> YES ✅
        - Is token correct?              -> YES ✅
        - Is subdomain "rahul-app" valid?-> YES ✅

Step 6: Apoorv's Server replies:
        {"type":"AUTH_OK", "success":true, "assigned_url":"https://rahul-app.tunnel.dev"}\n

Step 7: The 5-second stopwatch is turned off. 
        Rahul's tunnel is LIVE and ready to stream traffic! 🎉
```

---

## 📋 Quick Summary Table

| What You Wrote | File | Why You Wrote It |
|:---|:---|:---|
| `StartListener` | `listener.go` | So thousands of users can connect simultaneously without blocking. |
| `NotifyContext` + `WaitGroup` | `main.go` & `listener.go` | So `Ctrl+C` waits for active transfers instead of abruptly cutting users off. |
| `WriteMessage` / `ReadMessage` | `codec.go` | Appends `\n` so TCP doesn't merge multiple JSON packets together. |
| `SetReadDeadline(5s)` | `handshake.go` | Kicks out silent hackers so the server doesn't run out of memory. |
| `isValidSubdomain` | `handshake.go` | Ensures users only pick safe website names (3-30 letters/numbers). |
| `PerformHandshake` | `handshake.go` | Checks the password and returns the public `https://...` URL. |
