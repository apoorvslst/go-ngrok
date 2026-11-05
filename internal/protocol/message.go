/*The Problem with TCP: TCP ek Byte-Stream Protocol hai, Message Protocol nahi. OS kernel ke liye TCP connection ek paani ke pipe jaisa hai jisme continuous bytes beh rahe hain. TCP ko nahi pata ki ek message kahan khatam hua aur agla kahan shuru hua.

Agar Client do messages bhejta hai: {"type":"HELLO"} aur {"type":"PING"}

Toh network layer dono ko aapas mein chipka sakti hai: {"type":"HELLO"}{"type":"PING"} 
→
→ JSON parser crash ho jayega!

The Solution: Application Layer Framing. Hum Newline-Delimited JSON use karenge: Har message ke aage \n lagega: {"type":"HELLO"}\n {"type":"PING"}\n

Receiver stream se tab tak padhega jab tak \n nahi milta. Ek \n mila = Ek complete message!*/

package protocol

// MessageType represents the type of control message being sent over the wire.
type MessageType string

// ye sab types Msg wale -- sab protocol package ka hisaa hai 
const (
	//  sent by the client immediately after connecting.
	MsgHello MessageType = "HELLO"
	//  by the server when authentication and subdomain reservation succeed.
	MsgAuthOK MessageType = "AUTH_OK"
	// MsgAuthErr is returned by the server if token or subdomain validation fails.
	MsgAuthErr MessageType = "AUTH_ERR"
	// MsgPing is sent periodically to keep the control connection alive.
	MsgPing MessageType = "PING"
	// MsgPong is the response to a MsgPing frame.
	MsgPong MessageType = "PONG"
)

// HelloMessage is the initial payload sent by the client to identify itself.
type HelloMessage struct {
	Type             MessageType `json:"type"`
	Token            string      `json:"token"`
	DesiredSubdomain string      `json:"desired_subdomain"`
	ClientVersion    string      `json:"client_version"`
	OSPlatform       string      `json:"os_platform"`
}

// AuthResponseMessage is the server's handshake decision sent back to the client.
type AuthResponseMessage struct {
	Type         MessageType `json:"type"`
	Success      bool        `json:"success"`
	AssignedURL  string      `json:"assigned_url,omitempty"`
	//Agar auth fail ho gayi aur URL empty hai, toh omitempty ki wajah se JSON payload mein yeh field gayab ho jayegi.
	ErrorMessage string      `json:"error_message,omitempty"`
	ServerTime   int64       `json:"server_time"`
}

// PingPongMessage represents heartbeat telemetry between client and server.
type PingPongMessage struct {  // ping karne ke kaam ayega ki connection h kya bhai 
	// pong matlab ha bhai sab theek h samne wala bolta h BC
	Type      MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
}