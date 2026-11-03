package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/apoorvslst/go-ngrok/internal/server/registry"
	"github.com/apoorvslst/go-ngrok/internal/server/router"
)

/*
================================================================================
 REAL-WORLD EXAMPLE: The Public Front Gate (Public HTTP Listener)
================================================================================

WHO IS WHO?
1. CLIENT:
   - Your physical laptop sitting at home or in an office.
   - Running your web app locally (e.g. Next.js on localhost:3000).
   - Running ./cmd/client which connected to Server's Port :9000 and stays connected.

2. SERVER (This Program):
   - The public cloud machine (like AWS EC2) with a real public IP address.
   - Has TWO doors:
       Door 1 (Port :9000): Private control line where laptops (clients) connect.
       Door 2 (Port :8080): Public front gate where anyone on the internet arrives.

3. VISITOR:
   - A friend, a customer, or a Stripe payment webhook sitting anywhere on earth.
   - Opens their browser and types: "http://apoorv.tunnel.dev:8080/pay"
   - They CANNOT see your laptop directly; they only talk to the Server on Port :8080.

WHAT HAPPENS AT THIS PUBLIC LISTENER?
1. Visitor knocks on Port :8080 asking for "apoorv.tunnel.dev:8080/pay".
2. This HTTP handler reads the Host header: "apoorv.tunnel.dev:8080".
3. ExtractSubdomain extracts the name: "apoorv".
4. Looks up "apoorv" in the Registry (Phonebook).
   - If Apoorv's laptop is NOT connected -> Responds with 404 "Tunnel not found".
   - If Apoorv's laptop IS connected -> Confirms the tunnel is alive!
     (On Day 5, it will pipe the HTTP request directly into Apoorv's laptop).
================================================================================
*/

// StartPublicListener starts the HTTP server that receives visitor requests.
func StartPublicListener(addr string, reg *registry.Registry, rootDomain string) error {
	mux := http.NewServeMux()

	// Catch-all handler: catches every visitor request (any URL path)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract subdomain from Host header (e.g. "apoorv.tunnel.dev" -> "apoorv")
		subdomain, err := router.ExtractSubdomain(r.Host, rootDomain)
		if err != nil {
			slog.Warn("invalid host header received", "host", r.Host, "err", err)
			http.Error(w, "Invalid tunnel URL: "+err.Error(), http.StatusBadRequest)
			return
		}

		// 2. Look up the client tunnel in our Registry
		tunnel, ok := reg.Get(subdomain)
		if !ok {
			slog.Info("tunnel not found in registry", "subdomain", subdomain)
			http.Error(w, "Tunnel not found or offline", http.StatusNotFound)
			return
		}

		// 3. Client found! Confirm the routing works
		slog.Info("visitor request routed to active tunnel",
			"subdomain", subdomain,
			"client_addr", tunnel.ClientAddr,
			"method", r.Method,
			"path", r.URL.Path,
		)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Tunnel found! Proxying will be implemented on Day 5.\n"))
	})

	slog.Info("public HTTP listener starting", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

