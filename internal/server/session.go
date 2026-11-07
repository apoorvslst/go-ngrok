package server

import (
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/yamux"
)

// NewYamuxServerSession wraps an authenticated TCP connection into a Yamux multiplexer session.
// This allows multiple virtual HTTP/TCP streams to run simultaneously through one physical connection.
func NewYamuxServerSession(conn net.Conn) (*yamux.Session, error) {
	// 1. Load sensible defaults from Yamux
	cfg := yamux.DefaultConfig()

	// 2. Enable KeepAlive heartbeats to detect disconnected or sleeping clients
	cfg.EnableKeepAlive = true
	cfg.KeepAliveInterval = 15 * time.Second

	// 3. Set a timeout for writing frames to the network pipe
	cfg.ConnectionWriteTimeout = 10 * time.Second

	// 4. Wrap the raw TCP socket into a Yamux server multiplexer
	session, err := yamux.Server(conn, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create yamux server session: %w", err)
	}

	return session, nil
}

/*
 💡 Real-World Flow Example: Why Yamux & How KeepAlive Works

 1. WHAT YAMUX DOES:
    Imagine Rahul has a single physical highway lane (the TCP socket).
    Normally, if a huge truck (a 50MB video) is in the lane, sports cars (fast API calls)
    are stuck behind it (Head-of-Line Blocking).
    Yamux paints multiple virtual lanes onto that same road so the video and API calls
    travel side-by-side without blocking!

 2. WHAT HAPPENS IF SOMEONE TAKES >45 SECONDS?
    Scenario A (A slow operation, e.g. 60-second AI generation / heavy SQL query):
    - Rahul's website is waiting 60s for a response.
    - DOES YAMUX KILL IT? NO!
    - Yamux's KeepAlive runs in the background at the transport layer. While the HTTP
      request is waiting, Yamux automatically replies "PONG" to the server's "PING".
      The session stays healthy and alive forever.

    Scenario B (Rahul closes his laptop lid or Wi-Fi router gets unplugged):
    - Rahul's laptop shuts down instantly without sending a TCP close (FIN) packet.
    - Second 15: Server sends PING 1 -> No answer.
    - Second 30: Server sends PING 2 -> No answer.
    - Second 45: Server sends PING 3 -> No answer.
    - Result: Yamux marks the session DEAD and closes the socket, preventing zombie
      connections from eating up your VPS memory!
*/
