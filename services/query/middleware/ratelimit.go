package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TierLimits defines per-tier rate limits (requests per hour).
type TierLimits struct {
	Free       int
	Pro        int
	Enterprise int // 0 = unlimited
}

// DefaultTierLimits returns the default rate limits per tier.
// Free: 20/hr, Pro: 200/hr, Enterprise: 2000/hr, Admin: 0 (unlimited).
// TODO: roadmap specifies 15/month for free — align before Phase 2 billing.
func DefaultTierLimits() TierLimits {
	return TierLimits{
		Free:       20,
		Pro:        200,
		Enterprise: 2000,
	}
}

// RateLimiter is an in-memory sliding window rate limiter.
// Phase 1: In-memory. Phase 2: Migrate to Redis sliding window.
type RateLimiter struct {
	limits  TierLimits
	mu      sync.Mutex
	windows map[string][]time.Time
	stop    chan struct{}
}

// NewRateLimiter creates a rate limiter with the given tier limits.
// Starts a background goroutine to evict stale entries every 10 minutes.
// Call Close() on shutdown to stop the background goroutine.
func NewRateLimiter(limits TierLimits) *RateLimiter {
	rl := &RateLimiter{
		limits:  limits,
		windows: make(map[string][]time.Time),
		stop:    make(chan struct{}),
	}
	go rl.evictLoop()
	return rl
}

// Close stops the background eviction goroutine.
func (rl *RateLimiter) Close() {
	close(rl.stop)
}

// evictLoop periodically removes entries with no recent requests.
func (rl *RateLimiter) evictLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-1 * time.Hour)
			for key, entries := range rl.windows {
				valid := entries[:0]
				for _, t := range entries {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.windows, key)
				} else {
					rl.windows[key] = valid
				}
			}
			rl.mu.Unlock()
		case <-rl.stop:
			return
		}
	}
}

// Middleware returns a Gin middleware that enforces per-role rate limits.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Auth middleware sets "role"; fall back to "free" if absent.
		roleVal, _ := c.Get("role")
		tier, _ := roleVal.(string)
		if tier == "" {
			tier = "free"
		}

		userVal, _ := c.Get("user_id")
		userID, _ := userVal.(string)
		if userID == "" {
			userID = c.ClientIP()
		}

		limit := rl.limitForTier(tier)
		if limit == 0 {
			// Unlimited (enterprise)
			c.Next()
			return
		}

		key := tier + ":" + userID
		now := time.Now()
		windowStart := now.Add(-1 * time.Hour)

		rl.mu.Lock()

		// Clean expired entries
		entries := rl.windows[key]
		valid := make([]time.Time, 0, len(entries))
		for _, t := range entries {
			if t.After(windowStart) {
				valid = append(valid, t)
			}
		}

		if len(valid) >= limit {
			rl.mu.Unlock()

			retryAfter := valid[0].Add(time.Hour).Sub(now)
			c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"tier":        tier,
				"limit":       limit,
				"retry_after": int(retryAfter.Seconds()),
			})
			return
		}

		valid = append(valid, now)
		rl.windows[key] = valid
		rl.mu.Unlock()

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(limit-len(valid)))

		c.Next()
	}
}

func (rl *RateLimiter) limitForTier(tier string) int {
	switch tier {
	case "pro":
		return rl.limits.Pro
	case "enterprise":
		return rl.limits.Enterprise
	case "admin":
		return 0 // unlimited
	default:
		return rl.limits.Free
	}
}
