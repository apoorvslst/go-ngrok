package proxy

import (
	"errors"
)

var ErrTunnelOverloaded = errors.New("tunnel concurrent stream limit reached")

// StreamSemaphore bounds the number of concurrent in-flight streams for a tunnel.
// It uses a buffered channel of zero-byte structs (chan struct{}) to guarantee
// non-blocking, zero-allocation concurrency limiting.
type StreamSemaphore struct {
	tokens chan struct{}
	max    int
}

// NewStreamSemaphore creates a semaphore with the specified maximum capacity.
func NewStreamSemaphore(maxConcurrent int) *StreamSemaphore {
	if maxConcurrent <= 0 {
		maxConcurrent = 100 // Safe production default
	}
	return &StreamSemaphore{
		tokens: make(chan struct{}, maxConcurrent),
		max:    maxConcurrent,
	}
}

// TryAcquire attempts to claim a stream slot without blocking.
// Returns true if acquired, or false immediately if the tunnel is at maximum capacity.
func (s *StreamSemaphore) TryAcquire() bool {
	select {
	case s.tokens <- struct{}{}:
		return true // Token claimed! Slot allocated.
	default:
		return false // Channel full! Tunnel overloaded, do NOT wait.
	}
}

// Release returns the claimed slot back to the semaphore pool.
func (s *StreamSemaphore) Release() {
	select {
	case <-s.tokens:
		// Successfully released token
	default:
		// Guard against accidental double release
	}
}

// Available returns how many stream slots are currently free.
func (s *StreamSemaphore) Available() int {
	return s.max - len(s.tokens)
}

// InUse returns how many stream slots are currently occupied.
func (s *StreamSemaphore) InUse() int {
	return len(s.tokens)
}

/*
================================================================================
 REAL-WORLD EXAMPLE: The 100-Seat Restaurant Bouncer (Stream Semaphore)
================================================================================

1. THE CONCURRENCY TRAP:
   Rahul is running his Node.js/Go app on localhost:3000.
   A heavy API endpoint (e.g. POST /generate-invoice) takes 5 seconds to run.
   If 200 visitors hit that endpoint at the same time:
   Without a semaphore, the server opens 200 simultaneous Yamux streams.
   Rahul's laptop tries to process 200 heavy database queries at once!
   CPU hits 100%, RAM runs out, file descriptors exhaust, and the laptop crashes.

2. THE SEMAPHORE FIX:
   Think of a restaurant with exactly 100 seats:
   - Visitors 1 to 100 enter, take a seat (tokens <- struct{}{}).
   - Visitor 101 arrives: All 100 seats are occupied!
   - TryAcquire() instantly hits 'default' and returns false.
   - The server immediately responds with:
     "HTTP 503 Service Unavailable: Tunnel is at max capacity, try again soon!"
   - When Visitor 4 finishes their meal and leaves:
     Release() takes the token back (<-tokens).
     Now 1 seat is free for the next visitor!

3. DIAGRAM:
   Incoming Visitor Request
             │
             ▼
   [ StreamSemaphore.TryAcquire() ]
             │
       Is InUse < Max (100)?
       ├── YES ──► Claim Token (0 bytes RAM) ──► Forward over Yamux to Laptop
       │                                              │
       │                                (Request finishes: Release Token)
       │
       └── NO (At 100) ──► Instantly reject with HTTP 503 Service Unavailable!
                           (Rahul's laptop is completely shielded from overload)

4. WHY chan struct{}?
   In Go, struct{} takes exactly 0 bytes of memory:
   sizeof(struct{}) == 0.
   A channel holding 1,000 tokens uses practically zero RAM!
================================================================================
*/

