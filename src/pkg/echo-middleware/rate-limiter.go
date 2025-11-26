package echomw

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// basic rate limiter for requests only
// for website-review use additional custom rate limiter (allow 3 domains per ip address per day)
var (
	clients   = make(map[string]*rate.Limiter)
	mu        sync.Mutex
	rateLimit int // Number of requests per second
	burst     int // Burst size (how many requests are allowed instantly)
)

func UptdateRateLimits(rateLimitInput, burstInput int) {
	mu.Lock()
	defer mu.Unlock()
	rateLimit = rateLimitInput
	burst = burstInput
}

// getLimiter returns the rate limiter for the given IP address.
func getLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := clients[ip]
	if !exists {
		// Create a new rate limiter for the client
		limiter = rate.NewLimiter(rate.Limit(rateLimit), burst)
		clients[ip] = limiter

		// Clean up old limiters every minute
		go func() {
			time.Sleep(time.Minute)
			mu.Lock()
			delete(clients, ip)
			mu.Unlock()
		}()
	}
	return limiter
}

// Custom rate limiting middleware based on client IP address
func RateLimiterMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := c.RealIP() // Get the client's IP address
		limiter := getLimiter(ip)

		// Check if the request is allowed by the rate limiter
		if !limiter.Allow() {
			return c.String(http.StatusTooManyRequests, "Too many requests")
		}
		return next(c)
	}
}
