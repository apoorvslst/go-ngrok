package registry

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// Tunnel represents an active reverse tunnel connection from a client.
type Tunnel struct {
	Subdomain  string
	Session    *yamux.Session
	ClientAddr string
	CreatedAt  time.Time
}

// Registry holds all registered tunnels safely across multiple goroutines.
type Registry struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel
}

// NewRegistry initializes and returns a ready-to-use Registry.
func NewRegistry() *Registry {
	return &Registry{
		tunnels: make(map[string]*Tunnel),
	}
}

// Register adds a tunnel to the registry. Returns error if subdomain already taken.
func (r *Registry) Register(subdomain string, session *yamux.Session, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Comma-ok idiom: 'ok' is true if the key exists in the map
	if existing, ok := r.tunnels[subdomain]; ok {
		return fmt.Errorf("subdomain '%s' already registered by client %s", subdomain, existing.ClientAddr)
	}

	r.tunnels[subdomain] = &Tunnel{
		Subdomain:  subdomain,
		Session:    session,
		ClientAddr: addr,
		CreatedAt:  time.Now(),
	}

	slog.Info("tunnel registered", "subdomain", subdomain, "client_addr", addr)
	return nil
}

// Get retrieves a tunnel by subdomain. Thread-safe for concurrent reads.
func (r *Registry) Get(subdomain string) (*Tunnel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tunnel, ok := r.tunnels[subdomain]
	return tunnel, ok
}

// Delete removes a tunnel from the registry (on client disconnect).
func (r *Registry) Delete(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tunnels[subdomain]; ok {
		delete(r.tunnels, subdomain)
		slog.Info("tunnel deregistered", "subdomain", subdomain)
	}
}

// ActiveCount returns how many tunnels are currently connected.
func (r *Registry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tunnels)
}

/*
================================================================================
 REAL-LIFE ANALOGY: The Hotel Receptionist Desk (Registry)
================================================================================

Imagine our server is a modern hotel with a private intercom system:

1. NewRegistry():
   -> "Setting up the front desk register"
   When the hotel opens in the morning, the receptionist buys a fresh, empty
   guest ledger book (`make(map[string]*Tunnel)`). Without this book, trying
   to write down names would make you write on thin air (nil pointer panic!).

2. Register("apoorv", session, "192.168.1.5:54321"):
   -> "Guest check-in"
   You walk in and say: "I want room name 'apoorv' and here is my private intercom
   phone (yamux.Session)".
   - The receptionist grabs the ONLY pen (mu.Lock() - exclusive write).
   - Checks the book: Is someone already in 'apoorv'? If yes, reject!
   - If free, writes down your details and intercom line in the book.
   - Puts down the pen (defer mu.Unlock()).

3. Get("apoorv"):
   -> "Delivery person arrives"
   A delivery driver walks in with a pizza for "apoorv.tunnel.dev".
   - They look up at the electronic display board (mu.RLock() - read only).
   - 100 delivery drivers can look at the board at the EXACT SAME TIME without
     bumping into each other.
   - They find your intercom line (yamux.Session) to buzz your room directly!

4. Delete("apoorv"):
   -> "Guest check-out"
   You pack your bags, close your laptop, and disconnect.
   - The receptionist grabs the pen (mu.Lock() - exclusive write).
   - Crosses your name off the register so room 'apoorv' is free again.

5. ActiveCount():
   -> "Manager morning check"
   The hotel manager asks: "How many rooms are occupied right now?"
   - Receptionist counts the lines on the board (mu.RLock()).
   - Returns how many tunnels are alive.
================================================================================
*/
