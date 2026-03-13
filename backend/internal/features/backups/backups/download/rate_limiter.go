package backups_download

import (
	"io"
	"sync"
	"time"
)

type RateLimiter struct {
	mu              sync.Mutex
	bytesPerSecond  int64
	bucketSize      int64
	availableTokens float64
	lastRefill      time.Time
}

func NewRateLimiter(bytesPerSecond int64) *RateLimiter {
	if bytesPerSecond <= 0 {
		bytesPerSecond = 1024 * 1024 * 100
	}

	return &RateLimiter{
		bytesPerSecond:  bytesPerSecond,
		bucketSize:      bytesPerSecond * 2,
		availableTokens: float64(bytesPerSecond * 2),
		lastRefill:      time.Now().UTC(),
	}
}

func (rl *RateLimiter) UpdateRate(bytesPerSecond int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if bytesPerSecond <= 0 {
		bytesPerSecond = 1024 * 1024 * 100
	}

	rl.bytesPerSecond = bytesPerSecond
	rl.bucketSize = bytesPerSecond * 2

	if rl.availableTokens > float64(rl.bucketSize) {
		rl.availableTokens = float64(rl.bucketSize)
	}
}

func (rl *RateLimiter) Wait(bytes int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for {
		now := time.Now().UTC()
		elapsed := now.Sub(rl.lastRefill).Seconds()

		tokensToAdd := elapsed * float64(rl.bytesPerSecond)
		rl.availableTokens += tokensToAdd
		if rl.availableTokens > float64(rl.bucketSize) {
			rl.availableTokens = float64(rl.bucketSize)
		}
		rl.lastRefill = now

		if rl.availableTokens >= float64(bytes) {
			rl.availableTokens -= float64(bytes)
			return
		}

		tokensNeeded := float64(bytes) - rl.availableTokens
		waitTime := time.Duration(tokensNeeded/float64(rl.bytesPerSecond)*1000) * time.Millisecond

		waitTime = max(waitTime, time.Millisecond)

		rl.mu.Unlock()
		time.Sleep(waitTime)
		rl.mu.Lock()
	}
}

type RateLimitedReader struct {
	reader      io.ReadCloser
	rateLimiter *RateLimiter
}

func NewRateLimitedReader(reader io.ReadCloser, limiter *RateLimiter) *RateLimitedReader {
	return &RateLimitedReader{
		reader:      reader,
		rateLimiter: limiter,
	}
}

func (r *RateLimitedReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if n > 0 {
		r.rateLimiter.Wait(int64(n))
	}
	return n, err
}

func (r *RateLimitedReader) Close() error {
	return r.reader.Close()
}
