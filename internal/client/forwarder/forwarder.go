/*Server ne Yamux stream open karke visitor ka HTTP request wire pe fek diya. Ab client ke laptop par kya hona chahiye?
Jaise hi server se naya stream aata hai, Client laptop par chal rahe local app (localhost:3000) se connect karta hai (net.Dial("tcp", localAddr)).
Agar local app band hai? Client yamux stream ke andar seedha ek raw HTTP 502 Bad Gateway response likh deta hai! ReverseProxy us response ko visitor ke browser tak bhej deti hai.
Agar local app chal raha hai? Hum 2 concurrent goroutines launch karte hain:
Goroutine 1: Stream se bytes padho -> local app ko bhejo (Request).
Goroutine 2: Local app se bytes padho -> stream mein bhejo (Response).
Jaise hi koi ek side close hoti hai (request/response poora hua), <-errChan unblock hota hai aur dono connections safely close ho jate hain.
*/

package forwarder

import (
	"io"
	"log/slog"
	"net"
)

// ForwardToLocal connects an incoming yamux stream to the local application.
func ForwardToLocal(stream net.Conn, localAddr string) {
	defer stream.Close()

	//Local application se connect karo (e.g. localhost:3000)
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		slog.Error("local app unreachable — is it running?",
			"local_addr", localAddr,
			"err", err,
		)
		// agar error h toh,Visitor ko clean HTTP 502 Bad Gateway error return karo
		raw502 := "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\n\r\nLocal application at " + localAddr + " is not reachable. Is it running?\n"
		_, _ = stream.Write([]byte(raw502))
		return
	}
	defer localConn.Close()

	slog.Info("forwarding stream to local app",
		"local_addr", localAddr,
		"stream_remote", stream.RemoteAddr(),
	)

	// Bidirectional copy: stream <-> localConn
	errChan := make(chan error, 2) 

	// Goroutine 1: Stream -> Local App (Visitor request aata hai)
	go func() {
		_, copyErr := io.Copy(localConn, stream)
		errChan <- copyErr
	}()

	// Goroutine 2: Local App -> Stream (App ka response wapas jaata hai)
	go func() {
		_, copyErr := io.Copy(stream, localConn)
		errChan <- copyErr
	}()

	// Dono mein se kisi ek ke complete hone ka wait karo
	_ = <-errChan
	/*Jaise hi koi ek direction poori hoti hai (e.g. Local app ne response bhej diya aur connection close kiya):
Goroutine errChan <- copyErr bhejti hai.
Channel mein data aate hi yeh line jaag jati hai (Unblock)!
Function aage badhta hai, exit hota hai, aur defer dono connections ko cleanly band kar deta hai.*/
}