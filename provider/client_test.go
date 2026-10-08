package provider

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/Silo-Server/silo-plugin-tmdb/metadata"
)

const testAPIKey = "secret-test-key"

func newKeyedTestClient(baseURL string) *Client {
	client := NewClient(1000)
	client.apiKey = testAPIKey
	client.SetBaseURL(baseURL)
	return client
}

func assertNoAPIKey(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	message := err.Error()
	for _, secret := range []string{testAPIKey, defaultAPIKey} {
		if strings.Contains(message, secret) {
			t.Fatalf("error carries %q: %v", secret, err)
		}
	}
}

// refusingTransport refuses every request to refusedHost as a dial would, and
// sends any other request through http.DefaultTransport. An empty refusedHost
// refuses everything. It keeps transport-error tests independent of whether a
// closed loopback port has been handed out again.
type refusingTransport struct {
	refusedHost string
}

func (t refusingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.refusedHost == "" || req.URL.Host == t.refusedHost {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	}
	return http.DefaultTransport.RoundTrip(req)
}

func TestTransportErrorsLeaveOutTheAPIKey(t *testing.T) {
	t.Parallel()

	const baseURL = "http://tmdb.test"
	client := newKeyedTestClient(baseURL)
	client.httpClient.Transport = refusingTransport{}
	p := NewProviderWithClient(client)

	_, err := p.Search(context.Background(), metadata.SearchQuery{Title: "Movie", ContentType: "movie"})
	assertNoAPIKey(t, err)
	if !strings.Contains(err.Error(), "tmdb: load config: tmdb: request failed:") {
		t.Fatalf("error lost its context: %v", err)
	}

	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("error %v does not wrap a *url.Error", err)
	}
	if want := baseURL + "/configuration?api_key=REDACTED"; urlErr.URL != want {
		t.Fatalf("error URL = %q, want %q", urlErr.URL, want)
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("error %v no longer wraps the transport *net.OpError", err)
	}
}

func TestTransportErrorKeepsTheRequestQuery(t *testing.T) {
	t.Parallel()

	const baseURL = "http://tmdb.test"
	client := newKeyedTestClient(baseURL)
	client.httpClient.Transport = refusingTransport{}

	var dest map[string]any
	err := client.doGet(context.Background(), "/search/movie?query=Alien&year=1979&page=2", &dest)
	assertNoAPIKey(t, err)
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("error %v does not wrap a *url.Error", err)
	}
	if want := baseURL + "/search/movie?query=Alien&year=1979&page=2&api_key=REDACTED"; urlErr.URL != want {
		t.Fatalf("error URL = %q, want %q", urlErr.URL, want)
	}
}

func TestRedactURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in, want string
	}{
		{"no query", "https://api.themoviedb.org/3/configuration", "https://api.themoviedb.org/3/configuration"},
		{"key only", "https://api.themoviedb.org/3/configuration?api_key=" + testAPIKey, "https://api.themoviedb.org/3/configuration?api_key=REDACTED"},
		{"key last", "/3/find/tt0078748?external_source=imdb_id&api_key=" + testAPIKey, "/3/find/tt0078748?external_source=imdb_id&api_key=REDACTED"},
		{"key first", "/3/search/tv?api_key=" + testAPIKey + "&query=Andor", "/3/search/tv?api_key=REDACTED&query=Andor"},
		{"fragment dropped", "/3/movie/1?api_key=" + testAPIKey + "#" + testAPIKey, "/3/movie/1?api_key=REDACTED"},
		{"key elsewhere", "/3/" + testAPIKey + "/x?query=a", "/3/REDACTED/x?query=a"},
		{"unparseable", "/3/movie/%zz?api_key=" + testAPIKey, "/3/movie/%zz?api_key=REDACTED"},
		{"escaped name and value", "/3/movie/1?%61pi_key=%73ecret-test-key&page=2", "/3/movie/1?api_key=REDACTED&page=2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := redactURL(tt.in, testAPIKey); got != tt.want {
				t.Fatalf("redactURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanceledRequestErrorKeepsItsCause(t *testing.T) {
	t.Parallel()

	arrived := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		<-r.Context().Done()
	}))
	defer server.Close()

	client := newKeyedTestClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-arrived
		cancel()
	}()

	err := client.loadConfiguration(ctx)
	assertNoAPIKey(t, err)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRedirectTransportErrorLeavesOutTheAPIKey(t *testing.T) {
	t.Parallel()

	const refusedHost = "refused.test"
	target := "http://" + refusedHost + "/configuration?api_key=" + testAPIKey
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}))
	defer server.Close()

	client := newKeyedTestClient(server.URL)
	client.httpClient.Transport = refusingTransport{refusedHost: refusedHost}
	err := client.loadConfiguration(context.Background())
	assertNoAPIKey(t, err)
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("error %v no longer wraps the transport *net.OpError", err)
	}
}

func TestMalformedRedirectLocationLeavesOutTheAPIKey(t *testing.T) {
	t.Parallel()

	// net/http cannot parse this Location and quotes it in the error text.
	location := "/3/configuration/%zz?api_key=" + testAPIKey
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	err := newKeyedTestClient(server.URL).loadConfiguration(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error carries the API key: %v", err)
	}
	if !strings.Contains(err.Error(), "api_key=REDACTED") {
		t.Fatalf("error lost the masked Location: %v", err)
	}
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("error %v does not wrap a *url.Error", err)
	}
	if want := server.URL + "/configuration?api_key=REDACTED"; urlErr.URL != want {
		t.Fatalf("error URL = %q, want %q", urlErr.URL, want)
	}
}

func TestCreateRequestErrorLeavesOutTheAPIKey(t *testing.T) {
	t.Parallel()

	// An unclosed IPv6 literal makes url.Parse fail inside
	// http.NewRequestWithContext, which echoes the URL it rejected.
	err := newKeyedTestClient("http://[::1").loadConfiguration(context.Background())
	assertNoAPIKey(t, err)
	if !strings.Contains(err.Error(), "tmdb: create request:") {
		t.Fatalf("error lost its context: %v", err)
	}
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("error %v does not wrap a *url.Error", err)
	}
}

func TestStatusErrorLeavesOutTheAPIKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status_code":7,"status_message":"Invalid API key: You must be granted a valid key."}`))
	}))
	defer server.Close()

	err := newKeyedTestClient(server.URL).loadConfiguration(context.Background())
	assertNoAPIKey(t, err)
	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("error lost its status: %v", err)
	}
}

func TestLoadConfigurationConcurrentFirstCalls(t *testing.T) {
	t.Parallel()

	var fetches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":{"secure_base_url":"https://image.tmdb.org/t/p/"}}`))
	}))
	defer server.Close()

	const want = "https://image.tmdb.org/t/p/w500/poster.jpg"
	client := newKeyedTestClient(server.URL)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// Half the callers load first, as a metadata request does; the
			// rest only build image URLs, which can overlap the first load.
			if i%2 == 0 {
				if err := client.loadConfiguration(context.Background()); err != nil {
					t.Errorf("loadConfiguration: %v", err)
				} else if got := client.ImageURL("/poster.jpg", "w500"); got != want {
					t.Errorf("ImageURL after loadConfiguration = %q, want %q", got, want)
				}
				return
			}
			_ = client.ImageURL("/poster.jpg", "w500")
		}()
	}
	close(start)
	wg.Wait()

	if got := fetches.Load(); got != 1 {
		t.Fatalf("/configuration fetched %d times, want 1", got)
	}
	if got := client.ImageURL("/poster.jpg", "w500"); got != want {
		t.Fatalf("ImageURL = %q, want %q", got, want)
	}
}

func TestLoadConfigurationRetriesAfterAFailure(t *testing.T) {
	t.Parallel()

	var fetches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if fetches.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status_code":7,"status_message":"Invalid API key"}`))
			return
		}
		_, _ = w.Write([]byte(`{"images":{"secure_base_url":"https://image.tmdb.org/t/p/"}}`))
	}))
	defer server.Close()

	client := newKeyedTestClient(server.URL)
	if err := client.loadConfiguration(context.Background()); err == nil {
		t.Fatal("first loadConfiguration succeeded, want the HTTP 401 error")
	}
	for range 2 {
		if err := client.loadConfiguration(context.Background()); err != nil {
			t.Fatalf("loadConfiguration after a failure: %v", err)
		}
	}
	if got := fetches.Load(); got != 2 {
		t.Fatalf("/configuration fetched %d times, want 2", got)
	}
	if got, want := client.ImageURL("/poster.jpg", "w500"), "https://image.tmdb.org/t/p/w500/poster.jpg"; got != want {
		t.Fatalf("ImageURL = %q, want %q", got, want)
	}
}

func TestRedactURLErrorRedactsNestedURLErrors(t *testing.T) {
	t.Parallel()

	// net/http does not nest a *url.Error; this covers the defensive branch.
	cause := errors.New("connection refused")
	err := redactURLError(&url.Error{
		Op:  "Get",
		URL: "https://api.themoviedb.org/3/configuration?api_key=" + testAPIKey,
		Err: &url.Error{
			Op:  "Get",
			URL: "https://redirect.example/3/configuration?api_key=" + testAPIKey + "#fragment",
			Err: cause,
		},
	}, testAPIKey)

	assertNoAPIKey(t, err)
	want := `Get "https://api.themoviedb.org/3/configuration?api_key=REDACTED": Get "https://redirect.example/3/configuration?api_key=REDACTED": connection refused`
	if err.Error() != want {
		t.Fatalf("redacted error = %q, want %q", err.Error(), want)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("redacted error %v lost its cause", err)
	}
}

func TestRedactURLErrorPassesOtherErrorsThrough(t *testing.T) {
	t.Parallel()

	if err := redactURLError(nil, testAPIKey); err != nil {
		t.Fatalf("redactURLError(nil) = %v, want nil", err)
	}
	if err := redactURLError(context.DeadlineExceeded, testAPIKey); err != context.DeadlineExceeded { //nolint:errorlint // identity is the point
		t.Fatalf("redactURLError changed a non-URL error: %v", err)
	}
}
