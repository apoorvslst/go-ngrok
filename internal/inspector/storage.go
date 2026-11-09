package inspector

import (
	"sync"
	"time"
)

// RequestRecord stores the full telemetry of an inspected HTTP exchange.
type RequestRecord struct {
	ID          string            `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers"`
	RequestBody string            `json:"request_body"`
	BodyLength  int64             `json:"body_length"`

	// -------------------------------------------------------------------------
	// ⚠️ TRAP 1: Memory Exhaustion (OOM Crash)
	// If a visitor uploads a 200MB video and we load it into memory, RAM runs out
	// and the OS kills the process.
	// FIX: Cap capture at 1MB, set IsTruncated = true, and let remaining 199MB
	// pass directly to localhost:3000 without holding it in inspector RAM!
	// -------------------------------------------------------------------------
	IsTruncated bool              `json:"is_truncated"`

	// -------------------------------------------------------------------------
	// ⚠️ TRAP 2: Streams Can Only Be Read ONCE!
	// HTTP request body is a one-way pipe (io.ReadCloser). Reading it drains it,
	// leaving 0 bytes (EOF) for localhost:3000!
	// FIX: Use io.TeeReader to clone bytes into RequestBody while localhost:3000
	// reads the stream, ensuring both get the data in one single pass.
	// -------------------------------------------------------------------------

	// Response fields
	StatusCode   int    `json:"status_code"`
	DurationMs   int64  `json:"duration_ms"`
	ResponseBody string `json:"response_body"`
}

// -----------------------------------------------------------------------------
// ⚠️ TRAP 3: Unbounded Slices & Memory Leaks
// If the tunnel runs for 3 days and handles 100,000 requests, storing them in
// a standard slice (append) causes RAM to grow infinitely until the laptop crashes.
// FIX: RingBuffer is a fixed-capacity circular queue (e.g. 100 items).
// When request #101 arrives, it overwrites slot 0 (modulo % operator), keeping
// memory usage strictly capped at ~2MB forever!
// -----------------------------------------------------------------------------
type RingBuffer struct {
	mu       sync.RWMutex
	capacity int
	items    []*RequestRecord
	cursor   int
	isFull   bool
}

// NewRingBuffer initializes a fixed-capacity ring buffer.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 100
	}
	return &RingBuffer{
		capacity: capacity,
		items:    make([]*RequestRecord, capacity),
	}
}

// Add appends a new record, overwriting the oldest entry if capacity is reached.
func (rb *RingBuffer) Add(record *RequestRecord) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.items[rb.cursor] = record
	rb.cursor = (rb.cursor + 1) % rb.capacity
	if rb.cursor == 0 {
		rb.isFull = true
	}
}

// GetAll returns all stored records in reverse chronological order (newest first).
func (rb *RingBuffer) GetAll() []*RequestRecord {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var result []*RequestRecord
	count := rb.capacity
	if !rb.isFull {
		count = rb.cursor
	}

	// Loop backwards from newest to oldest
	idx := rb.cursor - 1
	for i := 0; i < count; i++ {
		if idx < 0 {
			idx = rb.capacity - 1
		}
		if rb.items[idx] != nil {
			result = append(result, rb.items[idx])
		}
		idx--
	}

	return result
}

// GetByID finds a specific request by ID for inspection or replay.
func (rb *RingBuffer) GetByID(id string) (*RequestRecord, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	for _, item := range rb.items {
		if item != nil && item.ID == id {
			return item, true
		}
	}
	return nil, false
}

/*
================================================================================
 REAL-WORLD EXAMPLE: The 100-Page Circular Notebook (Ring Buffer)
================================================================================

1. THE PROBLEM:
   Rahul is running his online store on localhost:3000. He leaves his reverse
   tunnel running for 3 days to test checkout traffic.
   - Day 1: 10,000 visitors arrive.
   - Day 2: 30,000 visitors arrive.
   - Day 3: 50,000 visitors arrive.
   If we stored all requests in an unbounded slice ([]*RequestRecord):
   Memory would store 90,000 requests, consuming 500MB+ RAM. Rahul's laptop
   runs out of memory (OOM) and crashes!

2. THE SOLUTION (Ring Buffer):
   Think of a 100-page spiral notebook with an eraser:
   - Slots 0 to 99 are filled sequentially by Request #1 to #100.
   - When Request #101 arrives, rb.cursor wraps back to Slot 0 using the modulo
     operator (100 % 100 = 0).
   - It erases Request #1 and overwrites it with Request #101!
   - Result: Memory is permanently capped at ~2MB, while Rahul always has the
     latest 100 requests on his dashboard!

3. DIAGRAM:
                     [ Slot 0 ] <--- Request #101 OVERWRITES Request #1!
                    /          \
        [ Slot 99 ]             [ Slot 1 ] <--- Next to be overwritten
            |                       |
        [ Slot 98 ]             [ Slot 2 ]
            \                      /
             [====== Slots ======]

4. WHY sync.RWMutex?
   - Multiple browser tabs can view the dashboard (GetAll) concurrently (mu.RLock).
   - When a new webhook arrives (Add), it locks for 1 microsecond (mu.Lock)
     to safely insert without race condition crashes.
================================================================================
*/


