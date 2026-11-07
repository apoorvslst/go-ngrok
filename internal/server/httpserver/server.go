/*Humare ngrok server ke do alag-alag darwaze (ports) honge:*/

/*Port :9000 (Control Listener): Jo humne Day 1-3 mein banaya. Yahan hamara laptop (client) connect hota hai, handshake karta hai, aur Yamux session chalu rehti hai.
Port :8080 (Public HTTP Listener): Yahan internet se aane wale visitors ya webhooks hit karenge (e.g. curl -H "Host: gaurav.tunnel.dev" http://localhost:8080/).

Visitor aayega toh yeh public server:

r.Host se subdomain nikaalega humare router.ExtractSubdomain se.
reg.Get(subdomain) se check karega ki koi tunnel registered hai ya nahi.
Agar mil gaya toh Day 4 mein bol dega: "Tunnel found! Proxying will be implemented on Day 5." (Day 5 mein hum is request ko yamux session ke through laptop tak pipe karenge).*/


package httpserver

import (
	"log/slog"
	"net/http"
	"github.com/apoorvslst/go-ngrok/internal/server/registry"
	"github.com/apoorvslst/go-ngrok/internal/server/router"
)

// StartPublicListener starts the HTTP server that receives visitor requests.
func StartPublicListener(addr string, reg *registry.Registry, rootDomain string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. Host header se subdomain extract karo
		subdomain, err := router.ExtractSubdomain(r.Host, rootDomain)
		if err != nil {
			slog.Warn("invalid host header", "host", r.Host, "err", err)
			http.Error(w, "Invalid tunnel URL", http.StatusBadRequest)
			return
		}

		// 2. Registry mein lookup karo ki tunnel online hai ya nahi
		tunnel, ok := reg.Get(subdomain)
		if !ok {
			slog.Info("tunnel not found", "subdomain", subdomain)
			http.Error(w, "Tunnel not found or offline", http.StatusNotFound)
			return
		}

		// 3. Match mil gaya! Confirm route ab log me bhar de 
		slog.Info("request routed successfully",
			"subdomain", subdomain,
			"client_addr", tunnel.ClientAddr,
			"method", r.Method,
			"path", r.URL.Path,
		)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Tunnel found! Proxying will be implemented on Day 5.\n"))
	})

	slog.Info("public HTTP listener starting", "addr", addr)
	return http.ListenAndServe(addr, mux)
}