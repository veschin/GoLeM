// Package proxy implements a rate-limiting reverse proxy for the Z.AI API.
// All Claude CLI instances route API requests through this local proxy, which
// serializes requests using a semaphore so only N (default 1) concurrent
// requests hit Z.AI at a time, preventing 429 rate-limit errors.
package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds configuration for the proxy server.
type Config struct {
	TargetURL   string        // e.g. "https://api.z.ai/api/anthropic"
	Concurrency int           // max concurrent requests (default 1)
	IdleTimeout time.Duration // auto-shutdown after inactivity (0 = never)
	Port        int           // 0 = OS assigns a free port
	LogFile     string        // path for log output (empty = stderr)
}

// Proxy is a rate-limiting reverse proxy server.
type Proxy struct {
	cfg           Config
	sem           chan struct{}
	listener      net.Listener
	idle          *time.Timer
	mu            sync.Mutex
	active        int64     // atomic active request count
	totalRequests int64     // atomic counter for total requests served
	startTime     time.Time // when proxy started
	logger        *log.Logger
}

// New creates a new Proxy from cfg. Config defaults are applied here:
// Concurrency defaults to 1 if <= 0.
func New(cfg Config) *Proxy {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}

	var logOut io.Writer = os.Stderr
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			logOut = f
		}
		// On error fall through to stderr silently — the proxy still works.
	}

	return &Proxy{
		cfg:       cfg,
		sem:       make(chan struct{}, cfg.Concurrency),
		startTime: time.Now(),
		logger:    log.New(logOut, "[proxy] ", log.LstdFlags),
	}
}

// Start binds a TCP listener, registers routes, and begins serving requests.
// It blocks until the listener is closed (via Stop or idle timeout expiry).
// Returns the net.Addr the server is listening on, and any startup error.
func (p *Proxy) Start() (net.Addr, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", p.cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("proxy: listen: %w", err)
	}

	p.mu.Lock()
	p.listener = ln
	p.mu.Unlock()

	// Build reverse proxy once — reuse for every request.
	rp, err := p.buildReverseProxy()
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("proxy: build reverse proxy: %w", err)
	}

	// Start idle timer if configured.
	if p.cfg.IdleTimeout > 0 {
		p.mu.Lock()
		p.idle = time.AfterFunc(p.cfg.IdleTimeout, func() {
			p.logger.Printf("idle timeout reached, shutting down")
			p.Stop()
		})
		p.mu.Unlock()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", p.healthHandler)
	mux.HandleFunc("/", p.proxyHandler(rp))

	p.logger.Printf("listening on %s (concurrency=%d target=%s)",
		ln.Addr(), p.cfg.Concurrency, p.cfg.TargetURL)

	// http.Serve returns when ln is closed.
	if err := http.Serve(ln, mux); err != nil {
		// net.ErrClosed is the normal "listener closed" error — not a real error.
		if isClosedErr(err) {
			return ln.Addr(), nil
		}
		return ln.Addr(), fmt.Errorf("proxy: serve: %w", err)
	}
	return ln.Addr(), nil
}

// Port returns the actual TCP port the proxy is listening on.
// Returns 0 if Start has not been called yet.
func (p *Proxy) Port() int {
	p.mu.Lock()
	ln := p.listener
	p.mu.Unlock()
	if ln == nil {
		return 0
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0
	}
	return addr.Port
}

// Stop closes the listener, causing Start to return.
func (p *Proxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.idle != nil {
		p.idle.Stop()
	}
	if p.listener != nil {
		p.listener.Close()
	}
}

// resetIdle resets the idle timer on every proxied request.
func (p *Proxy) resetIdle() {
	if p.cfg.IdleTimeout <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.idle != nil {
		p.idle.Reset(p.cfg.IdleTimeout)
	}
}

// healthHandler returns a JSON health document.
func (p *Proxy) healthHandler(w http.ResponseWriter, r *http.Request) {
	active := atomic.LoadInt64(&p.active)
	total := atomic.LoadInt64(&p.totalRequests)
	// queued = slots taken - active (waiting in the semaphore channel).
	queued := int64(len(p.sem)) - active
	if queued < 0 {
		queued = 0
	}
	uptime := int64(time.Since(p.startTime).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "ok",
		"active":         active,
		"queued":         queued,
		"port":           p.Port(),
		"total_requests": total,
		"uptime_sec":     uptime,
	})
}

// proxyHandler wraps a reverse proxy inside a semaphore gate.
func (p *Proxy) proxyHandler(rp *httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p.resetIdle()

		queued := time.Now()

		// Acquire a concurrency slot — blocks if all slots are taken.
		p.sem <- struct{}{}
		waitDur := time.Since(queued)
		atomic.AddInt64(&p.active, 1)
		atomic.AddInt64(&p.totalRequests, 1)

		start := time.Now()

		// Wrap response writer to capture status code for logging.
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		defer func() {
			atomic.AddInt64(&p.active, -1)
			<-p.sem // release slot
			p.logger.Printf("%s %s wait=%s duration=%s status=%d",
				r.Method, r.URL.Path, waitDur.Round(time.Millisecond), time.Since(start).Round(time.Millisecond), lrw.statusCode)
		}()

		rp.ServeHTTP(lrw, r)
	}
}

// buildReverseProxy constructs and configures an httputil.ReverseProxy
// targeting cfg.TargetURL, with path joining and header pass-through.
func (p *Proxy) buildReverseProxy() (*httputil.ReverseProxy, error) {
	target, err := url.Parse(p.cfg.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("parse target URL: %w", err)
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	// Flush immediately — required for SSE / streaming responses.
	rp.FlushInterval = -1

	// Transport with generous timeouts suitable for long-running API calls.
	rp.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 5 * time.Minute,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}

	// Custom director: join the target base path with the request path,
	// rewrite Host header, and preserve auth headers as-is.
	targetPath := target.Path // e.g. "/api/anthropic"
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host // rewrite Host header to target

		// Join base path + request path, avoiding double slashes.
		req.URL.Path = joinPaths(targetPath, req.URL.Path)

		// Preserve the original raw query.
		// (req.URL.RawQuery is already set by the incoming request.)

		// Remove the X-Forwarded-For header that NewSingleHostReverseProxy
		// would otherwise set — we want the upstream to see our IP.
		req.Header.Del("X-Forwarded-For")
	}

	return rp, nil
}

// joinPaths concatenates base and suffix, ensuring exactly one slash between them.
func joinPaths(base, suffix string) string {
	if base == "" {
		return suffix
	}
	if suffix == "" {
		return base
	}
	// Trim trailing slash from base, leading slash from suffix, then join.
	for len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	if len(suffix) == 0 || suffix[0] != '/' {
		return base + "/" + suffix
	}
	return base + suffix
}

// loggingResponseWriter captures the HTTP status code written by the handler.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.statusCode = code
	l.ResponseWriter.WriteHeader(code)
}

// isClosedErr reports whether err is the expected "use of closed network connection" error.
func isClosedErr(err error) bool {
	if err == nil {
		return false
	}
	// The standard library does not export this sentinel; string matching is the
	// canonical approach used throughout the stdlib itself.
	return err.Error() == "http: Server closed" ||
		isNetClosedErr(err)
}

func isNetClosedErr(err error) bool {
	if err == nil {
		return false
	}
	// net package wraps the closed error; check the error string.
	const closed = "use of closed network connection"
	type unwrapper interface{ Unwrap() error }
	for e := err; e != nil; {
		if e.Error() == closed {
			return true
		}
		u, ok := e.(unwrapper)
		if !ok {
			break
		}
		e = u.Unwrap()
	}
	return false
}
