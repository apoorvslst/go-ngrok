package server

import (
	"errors" // For returning standard Go error values
	"fmt"    // For formatting strings (fmt.Sprintf) and error wrapping (%w)
	"net"    // For network connection types like net.Conn
	"time"   // For setting network read timeouts and unix timestamps

	"github.com/apoorvslst/go-ngrok/internal/protocol" // Our custom framing codec and message structs
)

// isValidSubdomain enforces alphanumeric characters and hyphens between 3 and 30 chars.
// This prevents illegal DNS names, path injection, or shell exploit strings.
func isValidSubdomain(sub string) bool {
	// Guard Check 1: Length must be between 3 and 30 characters
	if len(sub) < 3 || len(sub) > 30 {
		return false
	}

	// Guard Check 2: Loop through each character to ensure it is DNS-safe
	for _, ch := range sub {
		isLower := ch >= 'a' && ch <= 'z' // Allowed lowercase letter
		isDigit := ch >= '0' && ch <= '9' // Allowed digit
		isHyphen := ch == '-'             // Allowed hyphen separator

		// If the character is NOT a lowercase letter, NOT a digit, and NOT a hyphen -> reject
		if !(isLower || isDigit || isHyphen) {
			return false
		}
	}

	// All characters are valid and safe
	return true
}

// PerformHandshake validates the incoming client hello frame under a strict 5-second deadline.
func PerformHandshake(conn net.Conn, validToken string, domainRoot string) (*protocol.HelloMessage, error) {
	// Step 1: Set a strict 5-second deadline on the socket for reading the handshake.
	// This protects the server from Slowloris attacks (clients connecting and staying silent forever).
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Step 2: Ensure the deadline is reset (cleared) when this function exits.
	// Passing a zero time (time.Time{}) disables the timeout so regular tunnel data streaming isn't cut off.
	defer conn.SetReadDeadline(time.Time{})

	// Step 3: Read and decode the client's HelloMessage using our generic streaming codec.
	hello, err := protocol.ReadMessage[protocol.HelloMessage](conn)
	if err != nil {
		// If read fails (e.g. client timed out after 5s or sent invalid JSON), return the error.
		return nil, fmt.Errorf("handshake read failed: %w", err)
	}

	// Step 4: Verify Message Type. The very first frame MUST be a "HELLO" message.
	if hello.Type != protocol.MsgHello {
		errMsg := "invalid handshake: expected HELLO message"
		// Politely send a structured error message frame to the client before dropping the socket
		_ = protocol.WriteMessage(conn, protocol.AuthResponseMessage{
			Type:         protocol.MsgAuthErr,
			Success:      false,
			ErrorMessage: errMsg,
			ServerTime:   time.Now().Unix(),
		})
		return nil, errors.New(errMsg)
	}

	// Step 5: Verify Authentication Token against the server's expected secret token.
	if hello.Token != validToken {
		errMsg := "authentication failed: invalid or expired token"
		// Send structured rejection frame so the client CLI can display a user-friendly error
		_ = protocol.WriteMessage(conn, protocol.AuthResponseMessage{
			Type:         protocol.MsgAuthErr,
			Success:      false,
			ErrorMessage: errMsg,
			ServerTime:   time.Now().Unix(),
		})
		return nil, errors.New(errMsg)
	}

	// Step 6: Verify Subdomain Format using our helper function.
	if !isValidSubdomain(hello.DesiredSubdomain) {
		errMsg := fmt.Sprintf("invalid subdomain '%s': must be 3-30 lowercase alphanumeric characters", hello.DesiredSubdomain)
		// Send structured validation error to client
		_ = protocol.WriteMessage(conn, protocol.AuthResponseMessage{
			Type:         protocol.MsgAuthErr,
			Success:      false,
			ErrorMessage: errMsg,
			ServerTime:   time.Now().Unix(),
		})
		return nil, errors.New(errMsg)
	}

	// Step 7: Build the assigned public tunnel URL for this client.
	// Example: "rahul-app" + "tunnel.dev" -> "https://rahul-app.tunnel.dev"
	assignedURL := fmt.Sprintf("https://%s.%s", hello.DesiredSubdomain, domainRoot)

	// Step 8: Build the success response payload.
	// - Type: MsgAuthOK tells client "Authentication and reservation succeeded!"
	// - Success: true tells client CLI to proceed with tunneling
	// - AssignedURL: tells client what public URL was assigned
	// - ServerTime: current Unix timestamp for clock synchronization & latency telemetry
	resp := protocol.AuthResponseMessage{
		Type:        protocol.MsgAuthOK,
		Success:     true,
		AssignedURL: assignedURL,
		ServerTime:  time.Now().Unix(),
	}

	// Step 9: Send the success response JSON frame over the TCP connection to the client.
	if err := protocol.WriteMessage(conn, resp); err != nil {
		// If writing fails (network connection dropped suddenly), return the error.
		return nil, fmt.Errorf("failed to send handshake response: %w", err)
	}

	// Handshake complete! Return the validated client information and nil error.
	return hello, nil
}

/*
 Real-World Flow Example:
Imagine a user named Rahul wants to expose his local Node.js server (localhost:3000) to the internet.

1. Rahul runs in his terminal:
   $ tunnel --subdomain "rahul-app" --token "secret123"

2. Rahul's client opens a TCP socket to our server (:9000).
   The server starts a 5-second stopwatch (deadline). If Rahul's client stays silent for 5s, the server kicks him out.

3. Rahul's client immediately sends a HELLO frame:
   {"type":"HELLO", "token":"secret123", "desired_subdomain":"rahul-app", "client_version":"v1.0"}

4. The server inspects the frame:
   - Check A: Is it a HELLO message? Yes!
   - Check B: Is "secret123" the correct auth token? Yes!
   - Check C: Is "rahul-app" a valid subdomain (3-30 chars, no illegal characters)? Yes!

5. The server creates the public URL "https://rahul-app.tunnel.dev" and replies with AUTH_OK:
   {"type":"AUTH_OK", "success":true, "assigned_url":"https://rahul-app.tunnel.dev"}

6. Rahul's terminal displays:
   "Tunnel Active! Forwarding https://rahul-app.tunnel.dev -> localhost:3000"
   The handshake is complete, the 5-second timer is turned off, and the tunnel is ready to stream traffic!

-----------------------------------------------------------------------------
 HOW THE URL IS FORMED & HOW TRAFFIC REACHES RAHUL'S LAPTOP:

1. HOW THE STRING IS CONSTRUCTED:
   assignedURL := fmt.Sprintf("https://%s.%s", hello.DesiredSubdomain, domainRoot)
   - hello.DesiredSubdomain = "rahul-app" (chosen by the user)
   - domainRoot             = "tunnel.dev" (our server's root domain)
   - Result                 = "https://rahul-app.tunnel.dev"

2. HOW DOES ANYONE ON THE INTERNET CONNECT TO OUR SERVER USING THIS URL?
   Step A: Wildcard DNS Record (*.tunnel.dev)
   - In Cloudflare or GoDaddy, we set a Wildcard DNS A-Record:
     Host: *.tunnel.dev  ->  IP: [Our Server's Public IP, e.g. 159.89.123.45]
   - This guarantees that whether someone opens "rahul-app.tunnel.dev", "apoorv.tunnel.dev",
     or anything.tunnel.dev, the DNS system sends their browser directly to our server's IP!

3. HOW DOES OUR SERVER KNOW WHICH LAPTOP TO FORWARD THE REQUEST TO?
   Step B: HTTP "Host" Header
   - When a browser makes an HTTP request to "https://rahul-app.tunnel.dev/api/users",
     the browser sends an HTTP header:
     Host: rahul-app.tunnel.dev

   Step C: The In-Memory Routing Table (Tunnel Registry)
   - When PerformHandshake succeeds, our server saves a mapping in memory:
     registry["rahul-app"] = Rahul's active TCP connection
   - When our server's public HTTP proxy receives an incoming request for "rahul-app",
     it looks up registry["rahul-app"] and forwards the HTTP bytes down Rahul's open
     TCP socket straight to his laptop running localhost:3000!
*/
