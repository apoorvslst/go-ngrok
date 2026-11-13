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

	// 2. Fetch the client
	tunnel, ok := reg.Get("apoorv")
	if !ok {
		t.Fatalf("expected to find subdomain 'apoorv', but got false")
	}

	if tunnel.Subdomain != "apoorv" {
		t.Errorf("expected subdomain 'apoorv', got '%s'", tunnel.Subdomain)
	}

	if tunnel.ClientAddr != "127.0.0.1:5000" {
		t.Errorf("expected client addr '127.0.0.1:5000', got '%s'", tunnel.ClientAddr)
	}

	if tunnel.State != StateActive {
		t.Errorf("expected state ACTIVE, got '%s'", tunnel.State)
	}

	// 3. Fetch a non-existent client
	_, ok = reg.Get("unknown")
	if ok {
		t.Fatalf("expected 'unknown' subdomain to be not found, but it was found")
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

	// 1. Client connects
	_ = reg.Register("gaurav", nil, "addr1", "my-secret-token")

	// 2. WiFi drops -> Mark Grace Period for 1 second
	reg.MarkGracePeriod("gaurav", 1*time.Second)

	// 3. Grace period ke dauran visitor lookup fail hona chahiye (tunnel inactive hai)
	if _, ok := reg.Get("gaurav"); ok {
		t.Fatalf("tunnel in grace period should not be returned by Get()")
	}

	// 4. Doosra hacker wrong token se reattach karne ki koshish kare -> Must Fail!
	err := reg.Reattach("gaurav", "hacker-token", nil, "hacker-addr")
	if err == nil {
		t.Fatalf("expected reattach with wrong token to fail")
	}

	// 5. Asli client matching token se wapas aaye -> Must Succeed!
	err = reg.Reattach("gaurav", "my-secret-token", nil, "new-addr")
	if err != nil {
		t.Fatalf("expected reattach with correct token to succeed, got: %v", err)
	}

	// 6. Subdomain wapas Active ho gaya!
	tunnel, ok := reg.Get("gaurav")
	if !ok || tunnel.State != StateActive {
		t.Fatalf("expected tunnel to be active again after reattach")
	}
}

func TestRegistry_DeleteAndReuse(t *testing.T) {
	reg := NewRegistry()

	_ = reg.Register("apoorv", nil, "addr1", "token1")

	// Delete it
	reg.Delete("apoorv")

	// Should not be found
	if _, ok := reg.Get("apoorv"); ok {
		t.Fatal("expected tunnel to be deleted")
	}

	// Re-registering with same subdomain should now succeed
	if err := reg.Register("apoorv", nil, "addr2", "token2"); err != nil {
		t.Fatalf("expected re-registration to succeed after deletion: %v", err)
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

	// While in grace period, Get() should return false (inactive)
	_, ok := reg.Get("temp")
	if ok {
		t.Fatal("expected 'temp' to be inactive in Get() during grace period")
	}

	// Wait 80ms for cleaner to expire and sweep it
	time.Sleep(80 * time.Millisecond)

	// Verify that now registering fresh with a new token works because it was cleaned up
	err := reg.Register("temp", nil, "new-addr", "different-token")
	if err != nil {
		t.Fatalf("expected expired tunnel to be cleaned up and re-registerable: %v", err)
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
			addr := fmt.Sprintf("10.0.0.%d:8080", id)
			token := fmt.Sprintf("token-%d", id)

			_ = reg.Register(subdomain, nil, addr, token)
			_, _ = reg.Get(subdomain)

			if id%2 == 0 {
				reg.MarkGracePeriod(subdomain, 50*time.Millisecond)
				_ = reg.Reattach(subdomain, token, nil, addr)
			} else {
				reg.Delete(subdomain)
			}
		}(i)
	}

	wg.Wait()
}
