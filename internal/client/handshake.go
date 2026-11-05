package client
// this file doesnt do anything except doing mic testing 1 2 3 4 for client to check server
import (
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/protocol"
)

// PerformClientHandshake initiates authentication with the tunnel server and retrieves the public URL.
func PerformClientHandshake(conn net.Conn, token string, subdomain string) (*protocol.AuthResponseMessage, error) {
	// 1. Strict 5-second deadline lagao (Read aur Write dono ke liye)
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetDeadline(time.Time{}) // Function exit hone pe reset to normal

	// 2. HelloMessage payload ready karo
	hello := protocol.HelloMessage{
		Type:             protocol.MsgHello,
		Token:            token,
		DesiredSubdomain: subdomain,
		ClientVersion:    "1.0.0",
		OSPlatform:       runtime.GOOS, // Auto detects "windows", "linux", "darwin"
	}

	// 3. Hello frame server ko send karo
	if err := protocol.WriteMessage(conn, hello); err != nil {
		return nil, fmt.Errorf("failed to send hello frame: %w", err)
	}

	// 4. Server se response wait & decode karo
	resp, err := protocol.ReadMessage[protocol.AuthResponseMessage](conn)
	if err != nil {
		return nil, fmt.Errorf("failed to receive handshake response: %w", err)
	}

	// 5. Check karo server ne allow kiya ya reject
	if !resp.Success {
		return nil, fmt.Errorf("server rejected authentication: %s", resp.ErrorMessage)
	}

	return resp, nil
}