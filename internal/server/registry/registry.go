/*Tumhare server ke port :8080 pe internet se visitor request aati hai: Host: gaurav.tunnel.dev
Server ko kaise pata chalega ki:
"gaurav" subdomain kis laptop ka hai?
Iska active yamux.Session kaunsa hai?
Aur agar ek second mein 1,000 visitors aa jayein, toh kya map read karte waqt race condition ya crash nahi hoga?

Day 4 me Humne banaya Thread-Safe In-Memory Registry (sync.RWMutex ke sath) jo subdomain ko active yamux session se link karegi!*/

package registry  // iss package ka naam yaad rakhna

/*Concept of sync.RWMutex
Normal sync.Mutex mein agar 1,000 readers aate hain, toh sabko line mein khada rehna padta hai (slow).
sync.RWMutex (Reader-Writer Lock):
RLock() (Read Lock): 10,000 visitors ek saath map ko padh sakte hain, koi kisi ka wait nahi karega!
Lock() (Write Lock): Sirf tab lagega jab koi naya client connect (Register) ya disconnect (Delete) ho.*/

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// Tunnel represents a single registered client tunnel.
type Tunnel struct {
	Subdomain  string
	Session    *yamux.Session
	ClientAddr string
	CreatedAt  time.Time
}

// Registry holds the in-memory mapping of subdomain -> active tunnel.
type Registry struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel
}

// NewRegistry creates an empty, initialized tunnel registry.
func NewRegistry() *Registry {
	return &Registry{
		tunnels: make(map[string]*Tunnel),
	}
}

// Register adds a tunnel to the registry. Returns error if subdomain already taken.
func (r *Registry) Register(subdomain string, session *yamux.Session, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Subdomain collision check
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

	tunnel, ok := r.tunnels[subdomain]  // tunnnels me se appropriate subdomain khoj lo 
	return tunnel, ok
}

// Delete removes a tunnel from the registry on client disconnect.
func (r *Registry) Delete(subdomain string) {
	r.mu.Lock()  // ate hi lock 
	defer r.mu.Unlock()   // delete hone ke baad unlock

	if _, ok := r.tunnels[subdomain]; ok {
		delete(r.tunnels, subdomain)
		slog.Info("tunnel deregistered", "subdomain", subdomain)
	}
}

// ActiveCount returns the total number of currently connected tunnels.
func (r *Registry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()  // readlock = Rlock

	return len(r.tunnels)
}