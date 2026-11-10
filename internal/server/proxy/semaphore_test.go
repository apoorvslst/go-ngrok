package proxy

import (
	"sync"
	"testing"
)

func TestStreamSemaphore_AcquireAndRelease(t *testing.T) {
	sem := NewStreamSemaphore(3)

	if sem.Available() != 3 || sem.InUse() != 0 {
		t.Fatalf("expected available=3, in_use=0, got available=%d, in_use=%d", sem.Available(), sem.InUse())
	}

	// Claim 1 slot
	if !sem.TryAcquire() {
		t.Fatalf("expected to acquire slot 1")
	}
	if sem.InUse() != 1 || sem.Available() != 2 {
		t.Errorf("expected in_use=1, available=2, got in_use=%d, available=%d", sem.InUse(), sem.Available())
	}

	// Release slot
	sem.Release()
	if sem.InUse() != 0 || sem.Available() != 3 {
		t.Errorf("expected in_use=0, available=3 after release, got in_use=%d, available=%d", sem.InUse(), sem.Available())
	}
}

func TestStreamSemaphore_ExceedCapacity(t *testing.T) {
	sem := NewStreamSemaphore(2)

	// Fill both slots
	if !sem.TryAcquire() {
		t.Fatalf("failed to acquire slot 1")
	}
	if !sem.TryAcquire() {
		t.Fatalf("failed to acquire slot 2")
	}

	// 3rd attempt must FAIL (non-blocking)
	if sem.TryAcquire() {
		t.Fatalf("expected 3rd acquire to fail when full, but it succeeded")
	}

	// Release 1 slot
	sem.Release()

	// Now 1 acquire should succeed
	if !sem.TryAcquire() {
		t.Fatalf("expected acquire to succeed after 1 slot was freed")
	}
}

func TestStreamSemaphore_DoubleReleaseGuard(t *testing.T) {
	sem := NewStreamSemaphore(2)

	// Releasing on empty semaphore should not panic or block
	sem.Release()
	if sem.InUse() != 0 {
		t.Errorf("expected in_use=0, got %d", sem.InUse())
	}
}

func TestStreamSemaphore_ConcurrentSafety(t *testing.T) {
	const capacity = 10
	const workers = 100
	sem := NewStreamSemaphore(capacity)

	var wg sync.WaitGroup
	var successfulAcquires int64
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sem.TryAcquire() {
				mu.Lock()
				successfulAcquires++
				mu.Unlock()

				// Simulate short work then release
				sem.Release()
			}
		}()
	}

	wg.Wait()

	// All acquired slots must have been released back
	if sem.InUse() != 0 {
		t.Errorf("expected all slots to be freed, got in_use=%d", sem.InUse())
	}
}

