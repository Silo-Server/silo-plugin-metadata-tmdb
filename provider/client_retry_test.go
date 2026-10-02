package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The proxy answers overload with 503 + Retry-After; in proxy mode the client
// must keep waiting for as long as the caller's deadline allows.
func TestProxyModeHonoursRetryAfterBeyondFixedRetries(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 5 {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"UPSTREAM_DEFERRED"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":550}`))
	}))
	defer server.Close()
	c := NewClient(100)
	if err := c.SetProxyURL(server.URL); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var out MovieDetail
	if err := c.doGet(ctx, "/movie/550", &out); err != nil {
		t.Fatalf("expected success after six attempts, got %v", err)
	}
	if got := calls.Load(); got != 6 {
		t.Fatalf("calls = %d, want 6", got)
	}
}

func TestRetryStopsWhenBackoffWouldExceedDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	c := NewClient(100)
	if err := c.SetProxyURL(server.URL); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	var out MovieDetail
	err := c.doGet(ctx, "/movie/550", &out)
	if err == nil {
		t.Fatal("expected an error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("client slept instead of failing fast: %v", time.Since(start))
	}
}

func TestDirectModeStillCapsRetries(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	c := NewClient(100)
	c.SetBaseURL(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var out MovieDetail
	if err := c.doGet(ctx, "/movie/550", &out); err == nil {
		t.Fatal("expected failure")
	}
	if got := calls.Load(); got != maxRetries+1 {
		t.Fatalf("calls = %d, want %d", got, maxRetries+1)
	}
}
