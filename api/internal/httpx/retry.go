// Package httpx provides an http.RoundTripper that retries transient
// failures of idempotent requests with exponential backoff.
package httpx

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// RetryTransport retries idempotent requests (GET, HEAD, OPTIONS, PUT,
// DELETE) on network errors and on 429/502/503/504. Non-idempotent requests
// (POST, PATCH) are sent once, since a retry could repeat a side effect.
type RetryTransport struct {
	Base        http.RoundTripper
	MaxAttempts int           // total attempts including the first; default 3
	BaseDelay   time.Duration // first backoff; doubles each retry; default 200ms
	MaxDelay    time.Duration // cap per wait; default 5s
}

// NewRetryTransport wraps base (http.DefaultTransport when nil) with defaults.
func NewRetryTransport(base http.RoundTripper) *RetryTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RetryTransport{Base: base, MaxAttempts: 3, BaseDelay: 200 * time.Millisecond, MaxDelay: 5 * time.Second}
}

// Client returns an http.Client using a retrying transport.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: NewRetryTransport(nil)}
}

func idempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

func retryable(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempts := t.MaxAttempts
	if attempts < 1 || !idempotent(req.Method) || (req.Body != nil && req.GetBody == nil) {
		attempts = 1
	}
	var resp *http.Response
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			if req.GetBody != nil {
				body, berr := req.GetBody()
				if berr != nil {
					return nil, berr
				}
				req.Body = body
			}
			wait := t.backoff(i, resp)
			if resp != nil {
				resp.Body.Close()
			}
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(wait):
			}
		}
		resp, err = t.Base.RoundTrip(req)
		if !retryable(resp, err) || req.Context().Err() != nil {
			return resp, err
		}
	}
	return resp, err
}

// backoff returns the wait before attempt i (1-based retries), honoring a
// Retry-After header in seconds when the server sent one.
func (t *RetryTransport) backoff(i int, resp *http.Response) time.Duration {
	if resp != nil {
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs >= 0 {
			if d := time.Duration(secs) * time.Second; d <= t.MaxDelay {
				return d
			}
			return t.MaxDelay
		}
	}
	d := t.BaseDelay << (i - 1)
	if d > t.MaxDelay || d <= 0 {
		d = t.MaxDelay
	}
	// Full jitter avoids synchronized retries across API replicas.
	return time.Duration(rand.Int64N(int64(d)) + 1)
}
