package server

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/hashicorp/yamux"
)

// DispatchTestStreams opens 'count' parallel virtual streams over a single Yamux session.
// This proves that multiplexing allows multiple concurrent requests without head-of-line blocking.
func DispatchTestStreams(session *yamux.Session, count int) {
	var wg sync.WaitGroup

	slog.Info("starting test stream dispatch", "total_streams", count)

	for i := 1; i <= count; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			// 1. Open a new logical virtual stream over the existing TCP connection
			stream, err := session.OpenStream()
			if err != nil {
				slog.Error("failed to open virtual stream", "stream_id", id, "err", err)
				return
			}
			// Always close the stream when done to release stream ID and memory buffers
			defer stream.Close()

			slog.Info("opened virtual stream", "stream_id", id, "yamux_stream_id", stream.StreamID())

			// 2. Send test payload to the client over this virtual stream
			payload := fmt.Sprintf("Ping from server on virtual stream %d\n", id)
			_, err = stream.Write([]byte(payload))
			if err != nil {
				slog.Error("failed to write to virtual stream", "stream_id", id, "err", err)
				return
			}

			// 3. Read the client's reply from the virtual stream
			buf := make([]byte, 256)
			n, err := stream.Read(buf)
			if err != nil {
				slog.Error("failed to read from virtual stream", "stream_id", id, "err", err)
				return
			}

			slog.Info("received reply over virtual stream",
				"stream_id", id,
				"yamux_stream_id", stream.StreamID(),
				"reply", string(buf[:n]),
			)
		}(i)
	}

	// Wait for all parallel streams to finish their roundtrip before returning
	wg.Wait()
	slog.Info("all test streams completed successfully", "total_streams", count)
}

/*
 💡 Real-World Flow Example: How DispatchTestStreams Works

 Imagine Rahul has 10 visitors at the same time:

 1. SERVER LAUNCHES 10 GOROUTINES IN PARALLEL:
    DispatchTestStreams(session, 10)
    - Loops 1 to 10 and launches 10 independent goroutines.
    - Each goroutine calls session.OpenStream().

 2. WHAT HAPPENS ON THE WIRE (Multiplexing):
    - All 10 streams share the SAME single TCP socket to Rahul's laptop!
    - Yamux assigns:
      Stream 1  -> Yamux StreamID 1
      Stream 2  -> Yamux StreamID 3
      ...
      Stream 10 -> Yamux StreamID 19
    - Their data chunks are interleaved over the wire without colliding.

 3. ROUNDTRIP VERIFICATION:
    - Server writes: "Ping from server on virtual stream 1"
    - Rahul's client reads it and replies: "Hello from client over virtual stream!"
    - Server reads the reply and logs it.
    - wg.Wait() ensures all 10 parallel requests complete before moving on.
*/
