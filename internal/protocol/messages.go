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

/*
 💡 Real-World Flow Example: Protocol Messages Lifecycle & Wire Format

 Imagine Rahul runs:
 $ tunnel --subdomain "rahul-app" --token "secret123"

 Here is the exact lifecycle of these message structs across the wire:

 1. STEP 1 — CLIENT SENDS HelloMessage:
    Rahul's client constructs and sends:
    msg := protocol.HelloMessage{
        Type:             protocol.MsgHello,
        Token:            "secret123",
        DesiredSubdomain: "rahul-app",
        ClientVersion:    "1.0.0",
        OSPlatform:       "windows",
    }
    -> Wire representation (JSON + '\n'):
    {"type":"HELLO","token":"secret123","desired_subdomain":"rahul-app","client_version":"1.0.0","os_platform":"windows"}\n

 2. STEP 2 — SERVER RESPONDS WITH AuthResponseMessage:
    Scenario A (SUCCESS - Valid token & subdomain):
    resp := protocol.AuthResponseMessage{
        Type:        protocol.MsgAuthOK,
        Success:     true,
        AssignedURL: "https://rahul-app.tunnel.dev",
        ServerTime:  1700000000,
    }
    -> Wire representation:
    {"type":"AUTH_OK","success":true,"assigned_url":"https://rahul-app.tunnel.dev","server_time":1700000000}\n
    (Notice: "error_message" is omitted due to `json:",omitempty"`)

    Scenario B (FAILURE - Wrong token or invalid subdomain):
    resp := protocol.AuthResponseMessage{
        Type:         protocol.MsgAuthErr,
        Success:      false,
        ErrorMessage: "authentication failed: invalid or expired token",
        ServerTime:   1700000000,
    }
    -> Wire representation:
    {"type":"AUTH_ERR","success":false,"error_message":"authentication failed...","server_time":1700000000}\n
    (Notice: "assigned_url" is omitted due to `json:",omitempty"`)

 3. STEP 3 — HEARTBEAT / KEEP-ALIVE (PingPongMessage):
    Every 30 seconds, Rahul's client sends:
    {"type":"PING","timestamp":1700000030}\n

    The server immediately echoes back:
    {"type":"PONG","timestamp":1700000030}\n
    This keeps stateful NAT firewalls and routers from closing idle TCP sockets!
*/

