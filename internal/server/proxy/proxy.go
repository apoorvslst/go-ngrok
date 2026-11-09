/*Normal Reverse Proxy (jaise Nginx ya Caddy) kaise kaam karta hai?
Jab Nginx ko request milti hai, toh woh backend tak ek naya TCP connection dial karta hai: Visitor -> Nginx -> net.Dial("tcp", "backend:3000") -> Backend
Visitor -> Nginx -> net.Dial("tcp", "backend:3000") -> Backend

Humare sath problem kya hai?
Humara backend developer ke ghar ke laptop par hai, jo Wi-Fi Router / NAT ke peeche band hai. Server laptop ko net.Dial kar hi nahi sakta!

The Genius Solution:
Go standard library ka httputil.ReverseProxy internally ek http.Transport use karta hai. Aur http.Transport mein ek setting hoti hai: DialContext (jo decide karti hai ki backend se kaise connect hona hai).
Hum is DialContext function ko hijack (override) kar dete hain!
Normal ReverseProxy:
  Transport.DialContext ──► net.Dial("tcp", ...) ──► New TCP Connection
HAMARA ReverseProxy:
  Transport.DialContext ──► tunnel.Session.OpenStream() ──► Yamux Virtual Stream!

  Aur kyunki yamux.Stream Go ke net.Conn interface ko implement karta hai, Go ke ReverseProxy ko pata hi nahi chalta ki yeh koi naya socket nahi balki purane TCP connection ke andar ka ek virtual channel hai!
HTTP headers, request body, query parameters, chunked encoding — sab kuch Go ka built-in httputil.ReverseProxy automatically handle kar leta hai!
  */
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
		subdomain, err := router.ExtractSubdomain(r.Host, rootDomain)  // extracting domain name 
		if err != nil {
			http.Error(w, "Invalid tunnel URL", http.StatusBadRequest)
			return
		}
		//check karo ki kya yeh tunnel online hai
		tunnel, ok := reg.Get(subdomain)
		if !ok {
			http.Error(w, "Tunnel not found or offline", http.StatusNotFound)
			return
		}

		//Check karo ki yamux session abhi bhi zinda hai ya nahi
		if tunnel.Session.IsClosed() {
			reg.Delete(subdomain)
			http.Error(w, "Tunnel client disconnected", http.StatusBadGateway)
			return
		}
		// ReverseProxy configure karo custom DialContext ke saath
		proxy := &httputil.ReverseProxy{
			// Director incoming request ko backend ke format mein set karta hai
			Director: func(req *http.Request) {
				req.URL.Scheme = "http"
				req.URL.Host = r.Host
				req.Host = r.Host // Preserve original Host header
			},
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// IMP PART : Naya TCP dial karne ke bajaye Yamux virtual stream open karo!
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
			// Agar client laptop response na de ya crash ho jaye
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				slog.Error("proxy error", "subdomain", subdomain, "err", err)
				http.Error(w, "Bad Gateway: tunnel client failed to respond", http.StatusBadGateway)
			},
		}
		// Request ko actually forward karo
		proxy.ServeHTTP(w, r)
	})
}