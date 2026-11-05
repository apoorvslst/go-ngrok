/*Socho ek hacker tumhare server ke port :9000 pe connect karta hai, lekin ek bhi byte nahi bhejta (chup baith jata hai).

Agar tumhara server seedha protocol.ReadMessage(conn) call karega, toh woh hamesha ke liye block (freeze) ho jayega! 
Agar 10,000 bots ne sirf connect karke chhod diya, toh server ki saari memory aur goroutines exhaust ho jayengi (Slowloris DoS Attack).
Humara Defense:
go
conn.SetReadDeadline(time.Now().Add(5 * time.Second))

Hum OS kernel ko bolte hain: "Agar 5 second ke andar client ne complete HELLO frame nahi bheja, 
toh turant i/o timeout error fenko aur connection kaat do!"*/

package server
import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/protocol"
)
// isValidSubdomain enforces alphanumeric characters and hyphens between 3 and 30 chars.
func isValidSubdomain(sub string) bool {
	// 1. Length check 
	if len(sub) < 3 || len(sub) > 30 {
		return false
	}

	// 2. Sirf lowercase letters, numbers, ya '-' allow karo
	for _, ch := range sub {
		isLower := ch >= 'a' && ch <= 'z'
		isDigit := ch >= '0' && ch <= '9'
		isHyphen := ch == '-'

		if !isLower && !isDigit && !isHyphen {
			return false
		}
	}

	// 3. Hyphen se shuru ya khatam nahi hona chahiye
	if sub[0] == '-' || sub[len(sub)-1] == '-' {
		return false
	}

	return true
}

// PerformHandshake validates the incoming client hello frame under a strict 5-second deadline.
func PerformHandshake(conn net.Conn, validToken string, domainRoot string) (*protocol.HelloMessage, error) {
	//Strict 5-second read deadline lagao (Slowloris dos attack protection)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{}) // Function exit hote hi deadline reset to normal (infinite)
	// 2. Client se HelloMessage decode karo
	hello, err := protocol.ReadMessage[protocol.HelloMessage](conn)
	if err != nil {
		return nil, fmt.Errorf("handshake read failed: %w", err)
	}


	// 3. Message Type verify karo
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

	// 4. Auth Token verify karo
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

	// 5. Subdomain format verify karo
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
	// 6. Subdomain aur Token valid hai -> Assigned Public URL banao
	assignedURL := fmt.Sprintf("https://%s.%s", hello.DesiredSubdomain, domainRoot)
	resp := protocol.AuthResponseMessage{
		Type:        protocol.MsgAuthOK,
		Success:     true,
		AssignedURL: assignedURL,//eg:- https://gfg34.mydomain.com isko apan auth resonse bhej rhe 
		ServerTime:  time.Now().Unix(),
	}
	// 7. Success frame client ko bhej do
	if err := protocol.WriteMessage(conn, resp); err != nil {
		return nil, fmt.Errorf("failed to send handshake response: %w", err)
	}
	return hello, nil
}

