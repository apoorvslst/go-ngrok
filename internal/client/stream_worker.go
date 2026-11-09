package client

import (
	"log/slog"
	"github.com/apoorvslst/go-ngrok/internal/client/forwarder"
	"github.com/hashicorp/yamux"
)
/*Jab tak server koi visitor request nahi bhejta, client chupchap efficient sleep
mode mein baitha rehta hai (zero CPU waste).
Jaise hi koi visitor hit karta hai, server ek virtual stream open karta hai.
Client ka AcceptStream() turant jaag jata hai aur use ek naya stream mil jata hai!
Client turant go handleIncomingStream(stream) goroutine mein daal deta hai, taaki ek saath 1000 streams bhi 
aayein toh parallel handle ho sakein!*/

/*Jaise hi Server bolta hai: session.OpenStream()
Wire pe Yamux ka ek chhota sa packet travel karta hai,
Client ka AcceptStream() turant jaag jata hai aur uske haath mein ek naya stream aa jata hai!
Aur Client use turant go handleIncomingStream(stream) bolkar ek worker goroutine ko de deta hai
aur khud wapas agle stream ke intezaar mein so jata hai!*/
// ListenForServerStreams accepts virtual streams initiated by the server over the Yamux session.
func ListenForServerStreams(session *yamux.Session,localTargetAddr string) {
	for {
		// 1. Blocks until server forwards a visitor request via a new virtual stream
		stream, err := session.AcceptStream()
		if err != nil {
			slog.Info("yamux session closed by server, stopping stream listener", "err", err)
			return
		}

		slog.Info("accepted new incoming virtual stream from server",
			"stream_id", stream.StreamID(),
		)

		// 2. Har stream ko parallel goroutine mein handle karo
		go forwarder.ForwardToLocal(stream, localTargetAddr)
	}
}

// Purana dummy handleIncomingStream function delete ho gaya, aur uski jagah go forwarder.ForwardToLocal(stream, localTargetAddr) lag gaya.
//Ab jab bhi koi visitor hit karega, client turant ek goroutine launch karke request tumhare local app ko forward karega!