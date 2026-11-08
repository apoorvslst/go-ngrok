// bytes.buffer ki concept
/*Network code ko test karne ke liye log aksar galti karte hain 
— woh pehle server start karte hain, port bind karte hain, fir client
 connect karte hain. Yeh bohot slow aur fragile hota hai.
The Pro Pattern: Go standard library ka bytes.Buffer dono 
interfaces implement karta hai:
io.Writer (hum WriteMessage se isme likh sakte hain)
io.Reader (hum ReadMessage se isse wapas padh sakte hain)
Isse hum bina koi socket khole, pure in-memory RAM buffer mein apne protocol 
ka roundtrip test 1 millisecond mein run kar sakte hain!*/


package protocol_test

import (
	"bytes"
	"testing"
	"github.com/apoorvslst/go-ngrok/internal/protocol"
	"time"
)

func TestCodec_HelloMessage_Roundtrip(t *testing.T) {
	//In-memory buffer banao jo Reader aur Writer dono ka kaam karega
	var buf bytes.Buffer  // bytes.buffer is type here 

	// 2. Dummy HelloMessage create karo for testing
	original := protocol.HelloMessage{
		Type:             protocol.MsgHello,
		Token:            "secret_token_123",
		DesiredSubdomain: "gaurav-dev",
		ClientVersion:    "1.0.0",
		OSPlatform:       "windows",
	}

	// 3. Buffer mein encode karke write karo
	err := protocol.WriteMessage(&buf, original)
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}
	/*t.Fatalf vs t.Errorf:
t.Fatalf: Agar encoding/decoding fail ho gayi toh test wahin turant stop 
ho jata hai (kyunki aage check karne ka koi fayda nahi)
t.Errorf: Agar koi specific field match nahi hui toh error log karta hai 
lekin test continue rehne deta hai taaki baki errors bhi dikhein.
*/

	// 4. Same buffer se wapas decode karke read karo
	decoded, err := protocol.ReadMessage[protocol.HelloMessage](&buf)
	/*protocol.ReadMessage[protocol.HelloMessage](&buf):
Notice Generics in action! Humne [protocol.HelloMessage] pass kiya, aur decoded 
ka type automatically *protocol.HelloMessage ban gaya.
  */
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// 5. Fields verify karo (original == decoded)
	if decoded.Type != original.Type {
		t.Errorf("expected type %s, got %s", original.Type, decoded.Type)
	}
	if decoded.Token != original.Token {
		t.Errorf("expected token %s, got %s", original.Token, decoded.Token)
	}
	if decoded.DesiredSubdomain != original.DesiredSubdomain {
		t.Errorf("expected subdomain %s, got %s", original.DesiredSubdomain, decoded.DesiredSubdomain)
	}
}
/*TCP Stream Framing Problem: Agar do messages back-to-back ek hi stream mein bhej diye jayein,
 toh kya hamara codec bina corrupt hue dono ko alag-alag parse kar pata hai?*/
func TestCodec_MultipleFrames(t *testing.T) {
	var buf bytes.Buffer

	now := time.Now().Unix()
	msg1 := protocol.PingPongMessage{Type: protocol.MsgPing, Timestamp: now}
	msg2 := protocol.PingPongMessage{Type: protocol.MsgPong, Timestamp: now + 1}

	// 1. Dono messages back-to-back ek hi buffer mein write karo
	if err := protocol.WriteMessage(&buf, msg1); err != nil {
		t.Fatalf("failed to write msg1: %v", err)
	}
	if err := protocol.WriteMessage(&buf, msg2); err != nil {
		t.Fatalf("failed to write msg2: %v", err)
	}

	// 2. Pehla frame read karo (PING aana chahiye)
	r1, err := protocol.ReadMessage[protocol.PingPongMessage](&buf)
	if err != nil || r1.Type != protocol.MsgPing {
		t.Fatalf("failed to read msg1 properly: %v", err)
	}

	// 3. Doosra frame read karo (PONG aana chahiye)
	r2, err := protocol.ReadMessage[protocol.PingPongMessage](&buf)
	if err != nil || r2.Type != protocol.MsgPong {
		t.Fatalf("failed to read msg2 properly: %v", err)
	}

	// 4. Timestamps verify karo
	if r1.Timestamp != now || r2.Timestamp != now+1 {
		t.Errorf("timestamps mismatch: r1=%d, r2=%d", r1.Timestamp, r2.Timestamp)
	}
}