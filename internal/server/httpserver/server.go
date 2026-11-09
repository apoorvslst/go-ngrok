package httpserver

import (
	"log/slog"
	"net/http"
		"github.com/apoorvslst/go-ngrok/internal/server/proxy"
	"github.com/apoorvslst/go-ngrok/internal/server/registry"
)

/*

 REAL-WORLD EXAMPLE: The Public Front Gate (Public HTTP Listener)

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
*/

// StartPublicListener starts the HTTP server that receives visitor requests.
func StartPublicListener(addr string, reg *registry.Registry, rootDomain string) error {
	// Use the real reverse proxy handler with Yamux DialContext trick!
	handler := proxy.NewTunnelProxy(reg, rootDomain)
	slog.Info("public HTTP listener starting", "addr", addr)
	//Ab server par port :8080 pe jo bhi visitor request aayegi, woh seedha proxy.NewTunnelProxy ke paas jayegi, jo request ko Yamux stream ke zariye client tak pipe kar dega!
	return http.ListenAndServe(addr, handler)
}