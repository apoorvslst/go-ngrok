package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	// 1. Register a client with a token
	err := reg.Register("apoorv", nil, "127.0.0.1:5000", "token123")
	if err != nil {
		t.Fatalf("expected registration to succeed, got: %v", err)
	}

	// 2. Lookup existing tunnel
	tunnel, ok := reg.Get("apoorv")
	if !ok {
		t.Fatalf("expected tunnel 'apoorv' to be found")
	}
	if tunnel.ClientAddr != "127.0.0.1:5000" {
		t.Errorf("expected client addr '127.0.0.1:5000', got '%s'", tunnel.ClientAddr)
	}
	if tunnel.State != StateActive {
		t.Errorf("expected state ACTIVE, got '%s'", tunnel.State)
	}

	// 3. Lookup nonexistent tunnel
	_, ok = reg.Get("ghost")
	if ok {
		t.Errorf("expected 'ghost' to not be found")
	}
}

func TestRegistry_DuplicateSubdomain(t *testing.T) {
	reg := NewRegistry()

	// First registration should succeed
	if err := reg.Register("apoorv", nil, "addr1", "token123"); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration with the same subdomain must fail!
	err := reg.Register("apoorv", nil, "addr2", "other-token")
	if err == nil {
		t.Fatal("expected duplicate registration to return an error, got nil")
	}
}

func TestRegistry_GracePeriodAndReattach(t *testing.T) {
	reg := NewRegistry()

	// 1. Register owner
	_ = reg.Register("apoorv", nil, "addr1", "my-secret-token")

	// 2. Client drops -> enter Grace Period
	reg.MarkGracePeriod("apoorv", 2*time.Second)

	tunnel, ok := reg.Get("apoorv")
	if !ok || tunnel.State != StateGracePeriod {
		t.Fatalf("expected tunnel to be in GRACE_PERIOD state")
	}

	// 3. Hacker tries to register "apoorv" during grace period -> MUST FAIL!
	err := reg.Register("apoorv", nil, "hacker-addr", "hacker-token")
	if err == nil {
		t.Fatal("expected hacker registration during grace period to be rejected, but it succeeded")
	}

	// 4. True owner returns with original token -> MUST SUCCEED (Reclaim)!
	err = reg.Register("apoorv", nil, "addr2", "my-secret-token")
	if err != nil {
		t.Fatalf("expected original owner to reclaim tunnel, got: %v", err)
	}

	tunnel, _ = reg.Get("apoorv")
	if tunnel.State != StateActive {
		t.Errorf("expected tunnel state to be ACTIVE after reclaim, got %s", tunnel.State)
	}
}

func TestRegistry_CleanerExpiry(t *testing.T) {
	reg := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start cleaner with fast 10ms interval
	go reg.StartCleaner(ctx, 10*time.Millisecond)

	// Register and drop with 30ms grace period
	_ = reg.Register("temp", nil, "addr", "token")
	reg.MarkGracePeriod("temp", 30*time.Millisecond)

	// Immediately, it should still be in registry
	_, ok := reg.Get("temp")
	if !ok {
		t.Fatal("expected 'temp' to still be in registry immediately after disconnect")
	}

	// Wait 80ms for cleaner to expire and sweep it
	time.Sleep(80 * time.Millisecond)

	_, ok = reg.Get("temp")
	if ok {
		t.Fatal("expected expired grace period tunnel to be cleaned up by background cleaner")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRegistry()
	var wg sync.WaitGroup

	numGoroutines := 100

	// Launch concurrent readers, writers, and grace-period transitions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			subdomain := fmt.Sprintf("client-%d", id)
			token := fmt.Sprintf("token-%d", id)

			// Concurrent Write (Register)
			_ = reg.Register(subdomain, nil, fmt.Sprintf("10.0.0.%d:8080", id), token)

			// Concurrent Read (Get)
			_, _ = reg.Get(subdomain)

			// Concurrent Read (ActiveCount)
			_ = reg.ActiveCount()

			// Concurrent Grace Period transition
			if id%2 == 0 {
				reg.MarkGracePeriod(subdomain, 50*time.Millisecond)
			} else {
				reg.Delete(subdomain)
			}
		}(i)
	}

	// Wait for all 100 goroutines to finish
	wg.Wait()
}
