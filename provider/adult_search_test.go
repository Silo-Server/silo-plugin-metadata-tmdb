package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchIncludesExplicitAdultFlag(t *testing.T) {
	t.Parallel()

	for _, includeAdult := range []bool{false, true} {
		includeAdult := includeAdult
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[includeAdult], func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				want := map[bool]string{false: "false", true: "true"}[includeAdult]
				if got := r.URL.Query().Get("include_adult"); got != want {
					t.Errorf("include_adult = %q, want %q", got, want)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
			}))
			defer server.Close()

			client := NewClient(40)
			client.SetBaseURL(server.URL)
			client.SetIncludeAdult(includeAdult)
			if _, err := client.SearchMovie(context.Background(), "Koihime", 2000, "en-US"); err != nil {
				t.Fatalf("SearchMovie() error = %v", err)
			}
			if _, err := client.SearchTV(context.Background(), "Koihime", 2000, "en-US"); err != nil {
				t.Fatalf("SearchTV() error = %v", err)
			}
		})
	}
}
