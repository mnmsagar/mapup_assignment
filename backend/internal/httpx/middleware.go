package httpx

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Chain applies middleware so that the first one listed is the outermost.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController (and the WebSocket upgrader) reach the
// underlying writer's Hijacker.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Hijack is required by gorilla/websocket, which type-asserts http.Hijacker directly.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
	}
	r.status = http.StatusSwitchingProtocols
	return h.Hijack()
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration", time.Since(start))
	})
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic", "value", v, "stack", string(debug.Stack()))
				WriteError(w, http.StatusInternalServerError, start, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func CORS(origins []string) func(http.Handler) http.Handler {
	allowAll := slices.Contains(origins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowAll || slices.Contains(origins, origin)) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// APIKey requires X-API-Key on non-GET requests when key is non-empty.
func APIKey(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Header.Get("X-API-Key") != key {
				WriteError(w, http.StatusUnauthorized, time.Now(), "missing or invalid API key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter is a per-client-IP token bucket.
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientLimiter
	rps     rate.Limit
	burst   int
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{clients: map[string]*clientLimiter{}, rps: rate.Limit(rps), burst: burst}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	c, ok := rl.clients[ip]
	if !ok {
		c = &clientLimiter{limiter: rate.NewLimiter(rl.rps, rl.burst)}
		rl.clients[ip] = c
	}
	c.lastSeen = time.Now()
	return c.limiter.Allow()
}

func (rl *RateLimiter) cleanup() {
	for range time.Tick(time.Minute) {
		rl.mu.Lock()
		for ip, c := range rl.clients {
			if time.Since(c.lastSeen) > 3*time.Minute {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "1")
			WriteError(w, http.StatusTooManyRequests, time.Now(), "rate limit exceeded, slow down")
			return
		}
		next(w, r)
	}
}

func clientIP(r *http.Request) string {
	// Behind a platform proxy the left-most X-Forwarded-For entry is the client.
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip, _, _ := strings.Cut(fwd, ",")
		if ip = strings.TrimSpace(ip); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
