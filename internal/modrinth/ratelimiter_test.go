package modrinth

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"
)

type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestRateLimitTransport(t *testing.T) {
	callCount := 0
	mockBase := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			callCount++
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString("OK")),
			}
			
			if callCount == 1 {
				resp.Header.Set("X-Ratelimit-Remaining", "0")
				resp.Header.Set("X-Ratelimit-Reset", "1") // 1 second reset
			}
			return resp, nil
		},
	}

	transport := NewRateLimitTransport(mockBase)
	client := &http.Client{Transport: transport}

	// 1st request, gets remaining=0, reset=1s
	req1, _ := http.NewRequest("GET", "http://test", nil)
	start1 := time.Now()
	_, err := client.Do(req1)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	dur1 := time.Since(start1)
	if dur1 > 500*time.Millisecond {
		t.Errorf("First request should not wait, took %v", dur1)
	}

	// 2nd request, should wait ~1s because remaining=0
	req2, _ := http.NewRequest("GET", "http://test", nil)
	start2 := time.Now()
	_, err = client.Do(req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	dur2 := time.Since(start2)
	
	if dur2 < 900*time.Millisecond {
		t.Errorf("Second request should have waited for rate limit, took %v", dur2)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}
