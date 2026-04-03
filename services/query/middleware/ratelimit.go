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
func DefaultTierLimits() TierLimits {
	return TierLimits{
		Free:       20,
		Pro:        500,
		Enterprise: 0,
	}
}

// RateLimiter is an in-memory sliding window rate limiter.
// Phase 1: In-memory. Phase 2: Migrate to Redis sliding window.
type RateLimiter struct {
	limits  TierLimits
	mu      sync.Mutex
	windows map[string][]time.Time
}

// NewRateLimiter creates a rate limiter with the given tier limits.
// Starts a background goroutine to evict stale entries every 10 minutes.
func NewRateLimiter(limits TierLimits) *RateLimiter {
	rl := &RateLimiter{
		limits:  limits,
		windows: make(map[string][]time.Time),
	}
	go rl.evictLoop()
	return rl
}

// evictLoop periodically removes entries with no recent requests.
func (rl *RateLimiter) evictLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
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
	}
}

// Middleware returns a Gin middleware that enforces per-tier rate limits.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tierVal, _ := c.Get("tier")
		tier, _ := tierVal.(string)
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
	default:
		return rl.limits.Free
	}
}
