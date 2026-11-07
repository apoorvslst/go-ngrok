//Humein Server side pe ek function banana hai jo yeh test karega ki: Kya Server ek single TCP connection ke andar ek hi second
//  mein 10 parallel streams open karke unpe data bhej aur receive kar sakta hai?
package server

import (
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/hashicorp/yamux"
)

// DispatchTestStreams opens 'count - kuchi bhi x number' concurrent virtual streams over the session to verify multiplexing.
func DispatchTestStreams(session *yamux.Session, count int) {
	var wg sync.WaitGroup  // wg is waitgroup ye counter hota hai 
	// agar wg.add(1) likha so har process ke baad wg me ek jud jayega

	slog.Info("dispatching concurrent test streams over single TCP pipe", "stream_count", count)

	for i := 1; i <= count; i++ {
		wg.Add(1)
        // loop me goroutine bhaisahab SUPER DUPER 
		// Har stream ko ek alag parallel goroutine mein chalate hain
		go func(streamIndex int) {
			defer wg.Done()  // isse counter (reduce hota h -1 se)

			// 1. Naya virtual channel open karo (Server -> Client)
			stream, err := session.OpenStream()
			if err != nil {
				slog.Error("failed to open virtual stream", "index", streamIndex, "err", err)
				return
			}
			defer stream.Close()

			slog.Info("opened virtual stream",
				"stream_index", streamIndex,
				"stream_id", stream.StreamID(),
			)

			// 2. Stream ke andar test payload likho
			payload := fmt.Sprintf("Ping from visitor request #%d", streamIndex)
			_, err = stream.Write([]byte(payload))
			if err != nil {
				slog.Error("failed to write to stream", "stream_id", stream.StreamID(), "err", err)
				return
			}

			// 3. Client se wapas aane wala response read karo
			buf := make([]byte, 1024)
			n, err := stream.Read(buf)
			if err != nil && err != io.EOF {
				slog.Error("failed to read reply from stream", "stream_id", stream.StreamID(), "err", err)
				return
			}

			slog.Info("received client reply over virtual stream",
				"stream_id", stream.StreamID(),
				"reply", string(buf[:n]),
			)
		}(i)
	}

	// Wait karo jab tak saari 10 streams apna kaam poora na kar lein
	wg.Wait()  // tb tk aage nahi badhega hb tk counter 0 na ho kyunki goroutine/process me +1 and -1 hota hai
	slog.Info("all concurrent test streams completed successfully!")
}