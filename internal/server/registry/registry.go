package registry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// TunnelState represents the lifecycle status of a registered tunnel.
type TunnelState string

const (
	StateActive      TunnelState = "ACTIVE"
	StateGracePeriod TunnelState = "GRACE_PERIOD"
)

// Tunnel represents an active or held reverse tunnel connection.
type Tunnel struct {
	Subdomain   string
	Session     *yamux.Session
	ClientAddr  string
	Token       string      // Secret auth token to verify ownership on reconnect
	State       TunnelState // ACTIVE or GRACE_PERIOD
	CreatedAt   time.Time
	GraceExpiry time.Time   // The timestamp when the reservation expires
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

// Register adds a tunnel to the registry, or reclaims it if in grace period with matching token.
func (r *Registry) Register(subdomain string, session *yamux.Session, addr string, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if subdomain is already taken
	if existing, ok := r.tunnels[subdomain]; ok {
		// If it is in grace period and the token matches the original owner -> Reclaim!
		if existing.State == StateGracePeriod && existing.Token == token {
			existing.State = StateActive
			existing.Session = session
			existing.ClientAddr = addr
			existing.GraceExpiry = time.Time{}
			slog.Info("tunnel successfully reclaimed from grace period!",
				"subdomain", subdomain,
				"client_addr", addr,
			)
			return nil
		}

		// If occupied by someone else (active or in grace period for another token)
		if existing.State == StateGracePeriod {
			return fmt.Errorf("subdomain '%s' is currently reserved in grace period", subdomain)
		}
		return fmt.Errorf("subdomain '%s' already registered by client %s", subdomain, existing.ClientAddr)
	}

	r.tunnels[subdomain] = &Tunnel{
		Subdomain:  subdomain,
		Session:    session,
		ClientAddr: addr,
		Token:      token,
		State:      StateActive,
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

// MarkGracePeriod marks a tunnel as disconnected but holds the subdomain reservation.
func (r *Registry) MarkGracePeriod(subdomain string, graceDuration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tunnel, ok := r.tunnels[subdomain]
	if !ok {
		return
	}

	tunnel.State = StateGracePeriod
	tunnel.GraceExpiry = time.Now().Add(graceDuration)
	tunnel.Session = nil // Clear the dead TCP/Yamux session

	slog.Info("tunnel entered grace period",
		"subdomain", subdomain,
		"hold_duration", graceDuration.String(),
	)
}

// Reattach reclaims a tunnel in grace period if the token matches.
func (r *Registry) Reattach(subdomain string, token string, newSession *yamux.Session, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tunnel, ok := r.tunnels[subdomain]
	if !ok {
		return errors.New("tunnel not found in registry")
	}

	if tunnel.Token != token {
		return errors.New("unauthorized: token does not match original tunnel owner")
	}

	tunnel.State = StateActive
	tunnel.Session = newSession
	tunnel.ClientAddr = addr
	tunnel.GraceExpiry = time.Time{} // Clear expiry timer

	slog.Info("tunnel successfully reattached from grace period!",
		"subdomain", subdomain,
		"client_addr", addr,
	)
	return nil
}

// Delete removes a tunnel from the registry permanently.
func (r *Registry) Delete(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tunnels[subdomain]; ok {
		delete(r.tunnels, subdomain)
		slog.Info("tunnel deregistered permanently", "subdomain", subdomain)
	}
}

// ActiveCount returns how many tunnels are currently active (not in grace period).
func (r *Registry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, t := range r.tunnels {
		if t.State == StateActive {
			count++
		}
	}
	return count
}

// StartCleaner periodically removes expired tunnels from grace period.
func (r *Registry) StartCleaner(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for sub, tunnel := range r.tunnels {
				if tunnel.State == StateGracePeriod && now.After(tunnel.GraceExpiry) {
					delete(r.tunnels, sub)
					slog.Info("grace period expired, subdomain permanently released", "subdomain", sub)
				}
			}
			r.mu.Unlock()
		}
	}
}

/*
================================================================================
 REAL-WORLD EXAMPLE: The Café Table Reservation (Grace Period)
================================================================================

1. Register("apoorv", session, addr, "secret123"):
   - You sit down at table "apoorv", show your ID ("secret123").
   - State becomes ACTIVE.

2. MarkGracePeriod("apoorv", 45*time.Second):
   - You step away to wash your hands (Wi-Fi drop).
   - The waiter puts a "RESERVED - 45s" sign on your table.
   - Session is cleared (nil), but the table is locked for you!
   - If a stranger comes and asks for table "apoorv", the waiter says:
     "Sorry, reserved in grace period!"

3. Reattach("apoorv", "secret123", newSession, addr):
   - You return, show your ID ("secret123").
   - Waiter removes the sign, re-attaches you, State is ACTIVE again!

4. StartCleaner(ctx, 5*time.Second):
   - Every 5 seconds, the janitor checks all tables.
   - If a table has been empty longer than 45 seconds, the janitor clears it
     so someone else can use it.
================================================================================
*/
