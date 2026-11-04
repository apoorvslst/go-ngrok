package proxy

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/apoorvslst/go-ngrok/internal/server/registry"
	"github.com/apoorvslst/go-ngrok/internal/server/router"
)

// NewTunnelProxy creates an HTTP handler that proxies visitor requests through yamux streams.
func NewTunnelProxy(reg *registry.Registry, rootDomain string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract subdomain from Host header (e.g. "apoorv.tunnel.dev:8080" -> "apoorv")
		subdomain, err := router.ExtractSubdomain(r.Host, rootDomain)
		if err != nil {
			slog.Warn("invalid host header", "host", r.Host, "err", err)
			http.Error(w, "Invalid tunnel URL: "+err.Error(), http.StatusBadRequest)
			return
		}

		// 2. Look up the client's tunnel in the registry
		tunnel, ok := reg.Get(subdomain)
		if !ok {
			slog.Info("tunnel not found in registry", "subdomain", subdomain)
			http.Error(w, "Tunnel not found or offline", http.StatusNotFound)
			return
		}

		// 3. Check if the yamux session is still alive
		if tunnel.Session.IsClosed() {
			reg.Delete(subdomain)
			slog.Warn("client session is closed, removing from registry", "subdomain", subdomain)
			http.Error(w, "Tunnel client disconnected", http.StatusBadGateway)
			return
		}

		// 4. Build ReverseProxy with custom DialContext (THE YAMUX TRICK)
		proxy := &httputil.ReverseProxy{
			// Director prepares the request before sending it to the client
			Director: func(req *http.Request) {
				req.URL.Scheme = "http"
				req.URL.Host = r.Host
				req.Host = r.Host // Preserve original Host header for local app
			},

			// Transport: Override DialContext to open a Yamux stream instead of a TCP dial
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// Open a new virtual stream inside the existing persistent connection
					stream, err := tunnel.Session.OpenStream()
					if err != nil {
						slog.Error("failed to open yamux stream", "subdomain", subdomain, "err", err)
						return nil, err
					}

					slog.Info("yamux stream opened for visitor request",
						"subdomain", subdomain,
						"stream_id", stream.StreamID(),
						"method", r.Method,
						"path", r.URL.Path,
					)
					return stream, nil
				},
			},

			// ErrorHandler handles cases where the client suddenly fails to respond
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				slog.Error("reverse proxy error", "subdomain", subdomain, "err", err)
				http.Error(w, "Bad Gateway: tunnel client failed to respond", http.StatusBadGateway)
			},
		}

		// Execute reverse proxy
		proxy.ServeHTTP(w, r)
	})
}

/*
================================================================================
 REAL-WORLD EXAMPLE: The VIP Courier Service (The Yamux ReverseProxy Trick)
================================================================================

SCENARIO:
A customer (Visitor in London) wants an item from Apoorv's private workshop (Laptop at Home).

Normally:
- A postal worker (Nginx / standard ReverseProxy) would drive directly to Apoorv's house
  using net.Dial("tcp", "house_address").
- BUT Apoorv's house has a private gated community that blocks all outside visitors!
  The postal worker gets stuck outside (Connection Refused / NAT block).

HOW THIS FILE SOLVES IT (The DialContext Trick):
1. Apoorv previously hired a dedicated private courier van (the Yamux TCP connection)
   that drives to the Central Post Office (our Cloud Server) and parks at the dock all day.
2. When the London customer's order arrives at the Central Post Office (Port :8080):
   - The receptionist (router.ExtractSubdomain) reads the label: "Order for Apoorv".
   - Checks the ledger (reg.Get("apoorv")): "Yes, Apoorv's van is parked at Bay 4!".
3. THE MAGIC (Transport.DialContext):
   - Instead of trying to send a new delivery van across the city, the Post Office
     simply opens a box (tunnel.Session.OpenStream()) and places the order into
     Apoorv's already-parked courier van!
   - Go's ReverseProxy doesn't care whether it's a real road or a private van —
     as long as it implements net.Conn, the package is delivered!
4. Apoorv's van instantly receives the box, walks inside the workshop (localhost:3000),
   gets the product, and hands it back up the same route to the London customer.
================================================================================
*/

