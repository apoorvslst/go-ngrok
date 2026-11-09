package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/apoorvslst/go-ngrok/internal/server/proxy"
	"github.com/apoorvslst/go-ngrok/internal/server/registry"
)

/*
================================================================================
 REAL-WORLD EXAMPLE: The Public Front Gate (Upgraded to Full Proxy)
================================================================================

On Day 4:
- The guard at Port :8080 checked if a client was registered. If yes, it printed:
  "Found it! Proxying will be implemented on Day 5."

On Day 5:
- The guard is replaced by proxy.NewTunnelProxy:
  When a visitor visits "http://apoorv.tunnel.dev:8080/api", the request is
  immediately packed into a Yamux stream and driven straight into the client's laptop!
================================================================================
*/

// StartPublicListener starts the HTTP server that proxies visitor traffic through tunnels.
func StartPublicListener(addr string, reg *registry.Registry, rootDomain string) error {
	// Use the real reverse proxy handler with Yamux DialContext trick!
	handler := proxy.NewTunnelProxy(reg, rootDomain)
	slog.Info("public HTTP listener starting", "addr", addr)
	return http.ListenAndServe(addr, handler)
}
