package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyModeSendsNoCredentialAndUsesProxyPrefix(t *testing.T) {
	var gotPath, gotQuery, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAuth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":550,"title":"Fight Club"}`))
	}))
	defer server.Close()

	c := NewClient(40)
	if err := c.SetProxyURL(server.URL + "/"); err != nil {
		t.Fatalf("SetProxyURL: %v", err)
	}
	if !c.ProxyMode() {
		t.Fatal("expected proxy mode")
	}
	var out MovieDetail
	if err := c.doGet(context.Background(), "/movie/550?language=en-US", &out); err != nil {
		t.Fatalf("doGet: %v", err)
	}
	if gotPath != "/v1/tmdb/3/movie/550" {
		t.Fatalf("path = %q, want proxy-prefixed path", gotPath)
	}
	if strings.Contains(gotQuery, "api_key") || gotAuth != "" {
		t.Fatalf("credential leaked to proxy: query=%q auth=%q", gotQuery, gotAuth)
	}
	if gotQuery != "language=en-US" {
		t.Fatalf("query = %q, want original query preserved", gotQuery)
	}
	if out.ID != 550 {
		t.Fatalf("decoded id = %d", out.ID)
	}
}

func TestProxyModeCanBeDisabledAgain(t *testing.T) {
	c := NewClient(40)
	if err := c.SetProxyURL("https://metadata.example"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetProxyURL(""); err != nil {
		t.Fatal(err)
	}
	if c.ProxyMode() || c.baseURL != defaultBaseURL {
		t.Fatalf("direct mode not restored: proxy=%v base=%q", c.ProxyMode(), c.baseURL)
	}
	if !strings.Contains(c.requestURL("/configuration"), "api_key=") {
		t.Fatal("direct mode must send api_key")
	}
}

func TestSetProxyURLRejectsGarbage(t *testing.T) {
	c := NewClient(40)
	for _, bad := range []string{"metadata.siloserver.org", "ftp://x", "https://", "https://h/?x=1", "https://h/#f"} {
		if err := c.SetProxyURL(bad); err == nil {
			t.Errorf("SetProxyURL(%q) accepted", bad)
		}
	}
	if c.ProxyMode() {
		t.Fatal("rejected URL must not enable proxy mode")
	}
}
