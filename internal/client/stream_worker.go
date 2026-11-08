package client

import (
	"fmt"
	"io"
	"log/slog"

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
func ListenForServerStreams(session *yamux.Session) {
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
		go handleIncomingStream(stream)
	}
}

// handleIncomingStream processes an individual virtual stream.
func handleIncomingStream(stream *yamux.Stream) {
	defer stream.Close()

	// Day 3 ke test ke liye: Hum stream se padhenge aur ek response return karenge
	// (Day 5 mein yahan localhost:3000 ka forwarder aayega!)
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		slog.Error("failed to read from virtual stream", "stream_id", stream.StreamID(), "err", err)
		return
	}

	incomingMsg := string(buf[:n])
	slog.Info("received payload on virtual stream",
		"stream_id", stream.StreamID(),
		"payload", incomingMsg,
	)

	// Stream pe reply likho
	reply := fmt.Sprintf("ACK: Server sent [%s] over virtual stream %d", incomingMsg, stream.StreamID())
	stream.Write([]byte(reply))
}