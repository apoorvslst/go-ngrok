package proxy

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/server/registry"
	"github.com/apoorvslst/go-ngrok/internal/server/router"
)

/*
Normal Reverse Proxy (jaise Nginx ya Caddy) kaise kaam karta hai?
Jab Nginx ko request milti hai, toh woh backend tak ek naya TCP connection dial karta hai:
Visitor -> Nginx -> net.Dial("tcp", "backend:3000") -> Backend

Humare sath problem kya hai?
Humara backend developer ke ghar ke laptop par hai, jo Wi-Fi Router / NAT ke peeche band hai.
Server laptop ko net.Dial kar hi nahi sakta!

The Genius Solution:
Go standard library ka httputil.ReverseProxy internally ek http.Transport use karta hai.
Aur http.Transport mein ek setting hoti hai: DialContext (jo decide karti hai ki backend se kaise connect hona hai).
Hum is DialContext function ko hijack (override) kar dete hain!

Normal ReverseProxy:
  Transport.DialContext ──► net.Dial("tcp", ...) ──► New TCP Connection
HAMARA ReverseProxy:
  Transport.DialContext ──► tunnel.Session.OpenStream() ──► Yamux Virtual Stream!

Aur kyunki yamux.Stream Go ke net.Conn interface ko implement karta hai, Go ke ReverseProxy ko
pata hi nahi chalta ki yeh koi naya socket nahi balki purane TCP connection ke andar ka ek virtual channel hai!
HTTP headers, request body, query parameters, chunked encoding — sab kuch Go ka built-in
httputil.ReverseProxy automatically handle kar leta hai!
*/

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

		// 3. Check if tunnel is in Grace Period (temporarily disconnected) or session is nil
		if tunnel.State == registry.StateGracePeriod || tunnel.Session == nil {
			slog.Info("tunnel is in grace period, client reconnecting", "subdomain", subdomain)
			http.Error(w, "503 Service Unavailable: Tunnel is reconnecting, please retry in a moment", http.StatusServiceUnavailable)
			return
		}

		// 4. Check if the yamux session is closed
		if tunnel.Session.IsClosed() {
			reg.MarkGracePeriod(subdomain, 45*time.Second)
			slog.Warn("client session is closed, entered grace period", "subdomain", subdomain)
			http.Error(w, "503 Service Unavailable: Tunnel client disconnected", http.StatusServiceUnavailable)
			return
		}

		// 5. Build ReverseProxy with custom DialContext (THE YAMUX TRICK)
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
