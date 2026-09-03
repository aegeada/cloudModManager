package modrinth

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

type RateLimitTransport struct {
	Transport http.RoundTripper

	mu        sync.Mutex
	remaining int
	resetTime time.Time
	known     bool
}

func NewRateLimitTransport(base http.RoundTripper) *RateLimitTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RateLimitTransport{
		Transport: base,
		remaining: 300,
	}
}

func (t *RateLimitTransport) acquire() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for {
		now := time.Now()
		if t.known && now.After(t.resetTime) {
			t.known = false
			t.remaining = 300
		}

		if !t.known || t.remaining > 0 {
			break
		}

		sleepDur := t.resetTime.Sub(now)
		if sleepDur <= 0 {
			t.known = false
			t.remaining = 300
			break
		}

		t.mu.Unlock()
		time.Sleep(sleepDur)
		t.mu.Lock()
	}

	t.remaining--
}

func (t *RateLimitTransport) updateLimits(header http.Header) {
	remStr := header.Get("X-Ratelimit-Remaining")
	resStr := header.Get("X-Ratelimit-Reset")

	if remStr != "" && resStr != "" {
		if rem, err := strconv.Atoi(remStr); err == nil {
			if resSec, err := strconv.Atoi(resStr); err == nil {
				t.mu.Lock()
				t.known = true
				t.remaining = rem
				t.resetTime = time.Now().Add(time.Duration(resSec) * time.Second)
				t.mu.Unlock()
			}
		}
	} else if retryAfter := header.Get("Retry-After"); retryAfter != "" {
		if sec, err := strconv.Atoi(retryAfter); err == nil {
			t.mu.Lock()
			t.known = true
			t.remaining = 0
			t.resetTime = time.Now().Add(time.Duration(sec) * time.Second)
			t.mu.Unlock()
		}
	}
}

func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.acquire()

	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	t.updateLimits(resp.Header)

	// Auto-retry once if we hit 429 Too Many Requests
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		t.acquire()
		resp, err = t.Transport.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		t.updateLimits(resp.Header)
	}

	return resp, nil
}
