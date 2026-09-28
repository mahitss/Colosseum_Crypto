package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a thread-safe token bucket rate limiter.
type TokenBucket struct {
	mu       sync.Mutex
	capacity int
	tokens   float64
	rate     float64 // tokens per second
	last     time.Time
}

// NewTokenBucket creates a new token bucket with the given capacity and fill rate.
func NewTokenBucket(capacity int, ratePerSecond float64) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		tokens:   float64(capacity),
		rate:     ratePerSecond,
		last:     time.Now(),
	}
}

// Allow attempts to take 1 token from the bucket.
// Returns true if token was available, false otherwise.
func (tb *TokenBucket) Allow(key string) bool {
	return tb.Take(1)
}

// Take attempts to take n tokens from the bucket.
// Returns true if tokens were available, false otherwise.
func (tb *TokenBucket) Take(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.last).Seconds()
	tb.tokens = min(float64(tb.capacity), tb.tokens+elapsed*tb.rate)
	tb.last = now

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}
	return false
}

// RateLimiter manages per-key token buckets.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*TokenBucket
	capacity int
	rate     float64
	cleanup  *time.Ticker
	stopCh   chan struct{}
}

// NewRateLimiter creates a new rate limiter with the given capacity and rate.
func NewRateLimiter(capacity int, ratePerSecond float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*TokenBucket),
		capacity: capacity,
		rate:     ratePerSecond,
		cleanup:  time.NewTicker(5 * time.Minute),
		stopCh:   make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if the key is allowed to make a request.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	bucket, ok := rl.buckets[key]
	if !ok {
		bucket = NewTokenBucket(rl.capacity, rl.rate)
		rl.buckets[key] = bucket
	}
	rl.mu.Unlock()

	return bucket.Take(1)
}

// Stop stops the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
	rl.cleanup.Stop()
}

func (rl *RateLimiter) cleanupLoop() {
	for {
		select {
		case <-rl.cleanup.C:
			rl.mu.Lock()
			now := time.Now()
			for key, bucket := range rl.buckets {
				bucket.mu.Lock()
				if now.Sub(bucket.last) > 10*time.Minute {
					delete(rl.buckets, key)
				}
				bucket.mu.Unlock()
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// SlidingWindowLog implements a sliding window log rate limiter for more precise limiting.
type SlidingWindowLog struct {
	mu      sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewSlidingWindowLog creates a new sliding window log rate limiter.
func NewSlidingWindowLog(limit int, window time.Duration) *SlidingWindowLog {
	return &SlidingWindowLog{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if the key is allowed to make a request.
func (swl *SlidingWindowLog) Allow(key string) bool {
	swl.mu.Lock()
	defer swl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-swl.window)

	requests := swl.requests[key]
	// Remove old requests outside the window
	valid := make([]time.Time, 0, len(requests))
	for _, t := range requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= swl.limit {
		swl.requests[key] = valid
		return false
	}

	valid = append(valid, now)
	swl.requests[key] = valid
	return true
}

// Cleanup removes old entries to prevent memory leaks.
func (swl *SlidingWindowLog) Cleanup() {
	swl.mu.Lock()
	defer swl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-swl.window)
	for key, requests := range swl.requests {
		valid := make([]time.Time, 0, len(requests))
		for _, t := range requests {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(swl.requests, key)
		} else {
			swl.requests[key] = valid
		}
	}
}