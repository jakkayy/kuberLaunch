package httpx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func server(t *testing.T, failures int32, status int) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		if n <= failures {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func fastClient() *http.Client {
	tr := NewRetryTransport(nil)
	tr.BaseDelay = time.Millisecond
	return &http.Client{Transport: tr}
}

func TestRetriesIdempotentRequests(t *testing.T) {
	srv, calls := server(t, 2, http.StatusServiceUnavailable)
	req, _ := http.NewRequest(http.MethodPut, srv.URL, strings.NewReader("payload"))
	resp, err := fastClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "payload" || *calls != 3 {
		t.Fatalf("status %d body %q calls %d", resp.StatusCode, body, *calls)
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	srv, calls := server(t, 10, http.StatusBadGateway)
	resp, err := fastClient().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadGateway || *calls != 3 {
		t.Fatalf("status %d calls %d", resp.StatusCode, *calls)
	}
}

func TestDoesNotRetryPostOrClientErrors(t *testing.T) {
	srv, calls := server(t, 1, http.StatusServiceUnavailable)
	if _, err := fastClient().Post(srv.URL, "text/plain", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if *calls != 1 {
		t.Fatalf("POST was retried: calls %d", *calls)
	}

	srv2, calls2 := server(t, 1, http.StatusNotFound)
	if _, err := fastClient().Get(srv2.URL); err != nil {
		t.Fatal(err)
	}
	if *calls2 != 1 {
		t.Fatalf("404 was retried: calls %d", *calls2)
	}
}
