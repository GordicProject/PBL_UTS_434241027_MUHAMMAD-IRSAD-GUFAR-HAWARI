package httpapi

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

const maxLoginAttempts = 5

// loginRateLimiter tracks failed login attempts per client IP within
// a sliding 1-minute window.
type loginRateLimiter struct {
	mu      sync.Mutex
	records map[string][]time.Time
}

// package-level instance so state persists across handler calls
var loginLimiter = newLoginRateLimiter()

func newLoginRateLimiter() *loginRateLimiter {
	lim := &loginRateLimiter{
		records: make(map[string][]time.Time),
	}
	// Periodically prune entries older than the window to keep the map bounded.
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			lim.prune()
		}
	}()
	return lim
}

// prune removes IPs whose records are all outside the 1-minute window.
func (lim *loginRateLimiter) prune() {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	for ip, recs := range lim.records {
		kept := recs[:0]
		for _, t := range recs {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(lim.records, ip)
		} else {
			lim.records[ip] = kept
		}
	}
}

// RecordFailedLogin records a failed attempt for the given IP.
func (lim *loginRateLimiter) RecordFailedLogin(ip string) {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	lim.records[ip] = append(lim.records[ip], time.Now())
}

// IsLimited returns true if the IP has exceeded maxLoginAttempts failures
// within the last minute.
func (lim *loginRateLimiter) IsLimited(ip string) bool {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	count := 0
	for _, t := range lim.records[ip] {
		if t.After(cutoff) {
			count++
		}
	}
	return count > maxLoginAttempts
}

// ClearAttempts removes all recorded failures for the IP (called on success).
func (lim *loginRateLimiter) ClearAttempts(ip string) {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	delete(lim.records, ip)
}

// RateLimitLogin returns a fiber middleware that checks per-IP rate limiting
// before the login handler runs. If the IP has exceeded maxLoginAttempts
// failures in the last minute, the request is rejected with 429.
func RateLimitLogin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		if loginLimiter.IsLimited(ip) {
			return Error(c, fiber.StatusTooManyRequests,
				"Terlalu banyak percobaan login. Coba lagi dalam 1 menit.")
		}
		return c.Next()
	}
}

// RecordLoginFailure logs a failed login attempt for the given IP.
func RecordLoginFailure(ip string) {
	loginLimiter.RecordFailedLogin(ip)
}

// ClearLoginAttempts removes recorded failures for the given IP
// (called after a successful login).
func ClearLoginAttempts(ip string) {
	loginLimiter.ClearAttempts(ip)
}
