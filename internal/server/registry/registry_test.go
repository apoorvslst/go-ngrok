package registry

import (
	"fmt"
	"sync"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	// 1. Register a client (we can pass nil for yamux.Session in unit tests)
	err := reg.Register("apoorv", nil, "127.0.0.1:5000")
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

	// 3. Lookup nonexistent tunnel
	_, ok = reg.Get("ghost")
	if ok {
		t.Errorf("expected 'ghost' to not be found")
	}
}

func TestRegistry_DuplicateSubdomain(t *testing.T) {
	reg := NewRegistry()

	// First registration should succeed
	if err := reg.Register("apoorv", nil, "addr1"); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration with the same subdomain must fail!
	err := reg.Register("apoorv", nil, "addr2")
	if err == nil {
		t.Fatal("expected duplicate registration to return an error, got nil")
	}
}

func TestRegistry_DeleteAndReuse(t *testing.T) {
	reg := NewRegistry()

	_ = reg.Register("apoorv", nil, "addr1")

	// Delete it
	reg.Delete("apoorv")

	// Should not be found
	if _, ok := reg.Get("apoorv"); ok {
		t.Fatal("expected tunnel to be deleted")
	}

	// Re-registering with same subdomain should now succeed
	if err := reg.Register("apoorv", nil, "addr2"); err != nil {
		t.Fatalf("expected re-registration to succeed after deletion: %v", err)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRegistry()
	var wg sync.WaitGroup

	numGoroutines := 100

	// Launch concurrent readers and writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			subdomain := fmt.Sprintf("client-%d", id)

			// Concurrent Write (Register)
			_ = reg.Register(subdomain, nil, fmt.Sprintf("10.0.0.%d:8080", id))

			// Concurrent Read (Get)
			_, _ = reg.Get(subdomain)

			// Concurrent Read (ActiveCount)
			_ = reg.ActiveCount()

			// Concurrent Write (Delete)
			if id%2 == 0 {
				reg.Delete(subdomain)
			}
		}(i)
	}

	// Wait for all 100 goroutines to finish
	wg.Wait()
}

