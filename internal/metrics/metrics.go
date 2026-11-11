package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ActiveTunnels tracks how many client tunnels are currently registered.
// Type: Gauge — it goes UP when a tunnel connects, DOWN when it disconnects.
var ActiveTunnels = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "tunnel_active_tunnels",
	Help: "Number of currently active tunnel connections.",
})

// ActiveStreams tracks active HTTP streams per subdomain.
// Type: GaugeVec — same as Gauge but split by label (subdomain name).
var ActiveStreams = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "tunnel_active_streams",
	Help: "Number of currently active HTTP streams, labelled by subdomain.",
}, []string{"subdomain"})

// RequestsTotal counts every HTTP request ever served.
// Type: CounterVec — only goes UP, split by subdomain + method + status.
var RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "tunnel_requests_total",
	Help: "Total number of HTTP requests proxied through tunnels.",
}, []string{"subdomain", "method", "status"})

// RequestDuration measures how long each request took (in seconds).
// Type: HistogramVec — buckets let Grafana answer "95% of requests < X ms".
var RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "tunnel_request_duration_seconds",
	Help:    "Duration of HTTP requests proxied through tunnels.",
	Buckets: []float64{0.005, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
}, []string{"subdomain", "status"})

// BytesTransferred counts total bytes in and out per subdomain.
// Type: CounterVec — labels: subdomain + direction ("in" or "out").
var BytesTransferred = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "tunnel_bytes_transferred_total",
	Help: "Total bytes transferred through tunnels, labelled by direction.",
}, []string{"subdomain", "direction"})

// ResponseWriterWrapper wraps http.ResponseWriter to intercept (capture) the
// status code and bytes written — because the standard ResponseWriter forgets
// them the moment they are sent.
type ResponseWriterWrapper struct {
	http.ResponseWriter       // embeds the real writer — all original methods still work
	StatusCode          int   // the HTTP status code our handler sent (200, 404, 503…)
	BytesWritten        int64 // total bytes written to the response body
}

// NewResponseWriterWrapper creates a wrapper around the original ResponseWriter.
// StatusCode defaults to 200 (Go's http package behaviour: if you never call
// WriteHeader, the response is implicitly 200 OK).
func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{
		ResponseWriter: w,
		StatusCode:     http.StatusOK, // default: 200
	}
}

// WriteHeader intercepts the status code, saves it, then forwards to the real writer.
// "Intercept → record → pass through" — the core trick.
func (rw *ResponseWriterWrapper) WriteHeader(code int) {
	rw.StatusCode = code                // 1. save it ourselves
	rw.ResponseWriter.WriteHeader(code) // 2. still send it to the client
}

// Write intercepts the response body bytes, counts them, then forwards to the real writer.
func (rw *ResponseWriterWrapper) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b) // 1. send to client first
	rw.BytesWritten += int64(n)          // 2. count how many bytes actually sent
	return n, err
}

// InstrumentHandler records metrics for a completed HTTP request.
// Call this AFTER the request finishes — pass everything you captured.
//
//	subdomain  — which tunnel served this request (e.g. "rahul")
//	method     — HTTP verb (e.g. "GET", "POST")
//	duration   — how long the full request took (from start to finish)
//	status     — HTTP status code sent to the visitor (e.g. 200, 503)
//	bytesIn    — bytes received from the visitor (r.ContentLength)
//	bytesOut   — bytes sent back to the visitor (from ResponseWriterWrapper)
func InstrumentHandler(subdomain, method string, duration time.Duration, status int, bytesIn, bytesOut int64) {
	// Convert status int (200) → string ("200") for use as a Prometheus label.
	// Labels must be strings — Prometheus doesn't accept int labels.
	statusStr := strconv.Itoa(status)

	// 1. Count this request (+1 to the total counter for this subdomain/method/status)
	RequestsTotal.WithLabelValues(subdomain, method, statusStr).Inc()

	// 2. Record how long it took (in seconds — Histogram expects seconds as float64)
	RequestDuration.WithLabelValues(subdomain, statusStr).Observe(duration.Seconds())

	// 3. Count bytes received FROM the visitor ("in" direction)
	if bytesIn > 0 {
		BytesTransferred.WithLabelValues(subdomain, "in").Add(float64(bytesIn))
	}

	// 4. Count bytes sent TO the visitor ("out" direction)
	if bytesOut > 0 {
		BytesTransferred.WithLabelValues(subdomain, "out").Add(float64(bytesOut))
	}

	// 5. Log it so we can see it in the terminal too (not just Grafana)
	fmt.Printf("[metrics] %s %s %s %d %.2fms in=%dB out=%dB\n",
		subdomain, method, statusStr, status, duration.Seconds()*1000, bytesIn, bytesOut,
	)
}

/*
================================================================================
 REAL-WORLD EXAMPLE: Rahul's Tunnel & The Carbon-Copy Metric Clerk
================================================================================

1. THE SCENARIO:
   Rahul runs an API locally on `localhost:3000` via tunnel subdomain "rahul".
   A visitor hits `http://rahul.tunnel.domain:8080/api/users` with a GET request.

2. THE PROBLEM WITHOUT WRAPPER:
   Standard Go `http.ResponseWriter` streams bytes and headers directly onto the
   network wire. Once written, the server cannot inspect what status code (e.g., 200 vs 500)
   or body size was sent out.

3. THE CARBON-COPY FLOW:
   [Visitor Request: GET /api/users (in: 312 bytes)]
              │
              ▼
   [Start Stopwatch: t0 = time.Now()]
   [ActiveStreams.WithLabelValues("rahul").Inc()]  <-- In-flight request stream: +1
              │
              ▼
   [NewResponseWriterWrapper(w)]                   <-- Wraps real writer like carbon copy
              │
              ▼
   [Reverse Proxy dials Rahul's local yamux stream]
              │
              ▼
   [Rahul's local app sends HTTP 200 OK + JSON body (out: 8420 bytes)]
              │
              ├── rw.WriteHeader(200) captures rw.StatusCode = 200
              └── rw.Write(jsonBytes) increments rw.BytesWritten = 8420
              │
              ▼
   [Request Finished! defer ActiveStreams.WithLabelValues("rahul").Dec()] <-- Stream: -1
              │
              ▼
   [InstrumentHandler("rahul", "GET", duration, 200, 312, 8420)]
              ├── RequestsTotal["rahul","GET","200"]        += 1
              ├── RequestDuration["rahul","200"]            observe(45ms)
              ├── BytesTransferred["rahul","in"]            += 312 B
              └── BytesTransferred["rahul","out"]           += 8420 B

4. THE RESULT:
   Prometheus scrapes port :9090 and Grafana renders real-time traffic graphs,
   error rates, p95 latency, and bandwidth utilization per developer tunnel!
================================================================================
*/
