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
- The guard at Port :8080 checked if Apoorv's van was parked. If yes, he just shouted:
  "Found it! Proxying will be implemented on Day 5."

On Day 5 (NOW):
- The guard is replaced by proxy.NewTunnelProxy:
  When a visitor visits "http://apoorv.tunnel.dev:8080/api", the request is
  immediately packed into a Yamux stream and driven straight into Apoorv's laptop!
================================================================================
*/

// StartPublicListener starts the HTTP server that proxies visitor traffic through tunnels.
func StartPublicListener(addr string, reg *registry.Registry, rootDomain string) error {
	// Hand incoming HTTP traffic directly to our reverse proxy handler
	handler := proxy.NewTunnelProxy(reg, rootDomain)

	slog.Info("public HTTP listener starting", "addr", addr)
	return http.ListenAndServe(addr, handler)
}
