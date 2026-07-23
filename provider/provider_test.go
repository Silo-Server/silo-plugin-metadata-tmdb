package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Silo-Server/silo-plugin-tmdb/metadata"
)

func TestTMDBLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: "en-US"},
		{input: "en", want: "en-US"},
		{input: "en-US", want: "en-US"},
		{input: "en_us", want: "en-US"},
		{input: "fr", want: "fr"},
		{input: "fr-CA", want: "fr-CA"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := tmdbLanguage(tt.input); got != tt.want {
				t.Fatalf("tmdbLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTMDBLocalizedRequestsUseEnglishUSForEnglish(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		path            string
		expectImageLang bool
		response        map[string]any
		invoke          func(context.Context, *Provider) error
	}{
		{
			name:            "movie metadata",
			path:            "/movie/42",
			expectImageLang: true,
			response: map[string]any{
				"id":                   42,
				"title":                "Movie",
				"overview":             "Overview",
				"original_language":    "ko",
				"genres":               []any{},
				"production_companies": []any{},
				"credits": map[string]any{
					"cast": []any{},
					"crew": []any{},
				},
				"external_ids": map[string]any{},
				"images":       map[string]any{},
				"release_dates": map[string]any{
					"results": []any{},
				},
			},
			invoke: func(ctx context.Context, p *Provider) error {
				_, err := p.GetMetadata(ctx, metadata.MetadataRequest{
					ProviderIDs: map[string]string{"tmdb": "42"},
					ContentType: "movie",
					Language:    "en",
				})
				return err
			},
		},
		{
			name:            "series metadata",
			path:            "/tv/77",
			expectImageLang: true,
			response: map[string]any{
				"id":                77,
				"name":              "Series",
				"overview":          "Overview",
				"original_language": "ko",
				"genres":            []any{},
				"networks":          []any{},
				"seasons":           []any{},
				"credits": map[string]any{
					"cast": []any{},
					"crew": []any{},
				},
				"external_ids": map[string]any{},
				"images":       map[string]any{},
				"content_ratings": map[string]any{
					"results": []any{},
				},
			},
			invoke: func(ctx context.Context, p *Provider) error {
				_, err := p.GetMetadata(ctx, metadata.MetadataRequest{
					ProviderIDs: map[string]string{"tmdb": "77"},
					ContentType: "series",
					Language:    "en",
				})
				return err
			},
		},
		{
			name:            "images",
			path:            "/movie/42",
			expectImageLang: true,
			response: map[string]any{
				"id":     42,
				"images": map[string]any{},
			},
			invoke: func(ctx context.Context, p *Provider) error {
				_, err := p.GetImages(ctx, metadata.ImageRequest{
					ProviderIDs: map[string]string{"tmdb": "42"},
					ContentType: "movie",
					Language:    "en",
				})
				return err
			},
		},
		{
			name:            "person detail",
			path:            "/person/287",
			expectImageLang: false,
			response: map[string]any{
				"id":           287,
				"name":         "Brad Pitt",
				"external_ids": map[string]any{},
			},
			invoke: func(ctx context.Context, p *Provider) error {
				_, err := p.GetPersonDetail(ctx, metadata.PersonDetailRequest{
					ProviderIDs: map[string]string{"tmdb": "287"},
					Language:    "en",
				})
				return err
			},
		},
		{
			name:            "seasons",
			path:            "/tv/77",
			expectImageLang: true,
			response: map[string]any{
				"id":      77,
				"seasons": []any{},
			},
			invoke: func(ctx context.Context, p *Provider) error {
				_, err := p.GetSeasons(ctx, metadata.SeasonsRequest{
					ProviderIDs: map[string]string{"tmdb": "77"},
					ContentType: "series",
					Language:    "en",
				})
				return err
			},
		},
		{
			name:            "episodes",
			path:            "/tv/77/season/2",
			expectImageLang: false,
			response: map[string]any{
				"id":       2,
				"episodes": []any{},
			},
			invoke: func(ctx context.Context, p *Provider) error {
				_, err := p.GetEpisodes(ctx, metadata.EpisodesRequest{
					ProviderIDs:  map[string]string{"tmdb": "77"},
					SeasonNumber: 2,
					Language:     "en",
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				switch r.URL.Path {
				case "/configuration":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"images": map[string]any{
							"secure_base_url": serverURL(t, r) + "/images/",
						},
					})
				case tt.path:
					if got := r.URL.Query().Get("language"); got != "en-US" {
						t.Fatalf("language = %q, want en-US", got)
					}
					if tt.expectImageLang {
						if got := r.URL.Query().Get("include_image_language"); got != "en-US,null" {
							t.Fatalf("include_image_language = %q, want en-US,null", got)
						}
					}
					_ = json.NewEncoder(w).Encode(tt.response)
				default:
					t.Fatalf("unexpected path: %s", r.URL.String())
				}
			}))
			defer server.Close()

			p := newTMDBTestProvider(server.URL)
			if err := tt.invoke(context.Background(), p); err != nil {
				t.Fatalf("invoke error = %v", err)
			}
		})
	}
}

func TestSearchReturnsLocalizedAndOriginalTitles(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{"secure_base_url": serverURL(t, r) + "/images/"},
			})
		case "/search/movie":
			if got := r.URL.Query().Get("query"); got != "Ten Tricks" {
				t.Fatalf("query = %q, want Ten Tricks", got)
			}
			if got := r.URL.Query().Get("language"); got != "en-US" {
				t.Fatalf("language = %q, want en-US", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{
					"id":                42,
					"title":             "10 Tricks",
					"original_title":    "Dieci trucchi",
					"original_language": "it",
					"release_date":      "2022-01-01",
				}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	results, err := newTMDBTestProvider(server.URL).Search(context.Background(), metadata.SearchQuery{
		Title:       "Ten Tricks",
		Year:        2022,
		ContentType: "movie",
		Language:    "en",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	got := results[0]
	if got.Name != "10 Tricks" || got.OriginalTitle != "Dieci trucchi" {
		t.Fatalf("titles = (%q, %q)", got.Name, got.OriginalTitle)
	}
	if got.TitleLanguage != "en" || got.TitleIsFallback || got.OriginalLanguage != "it" {
		t.Fatalf("language metadata = (%q, %v, %q)", got.TitleLanguage, got.TitleIsFallback, got.OriginalLanguage)
	}
	if len(got.TitleAliases) != 1 || got.TitleAliases[0].Title != "Dieci trucchi" || got.TitleAliases[0].Kind != "original" {
		t.Fatalf("aliases = %#v, want original title", got.TitleAliases)
	}
}

func TestGetMetadataReturnsAlternativeTitlesAndMarksNativeFallback(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{"secure_base_url": serverURL(t, r) + "/images/"},
			})
		case "/tv/77":
			if got := r.URL.Query().Get("language"); got != "en-US" {
				t.Fatalf("language = %q, want en-US", got)
			}
			if appended := r.URL.Query().Get("append_to_response"); !strings.Contains(appended, "alternative_titles") {
				t.Fatalf("append_to_response = %q, want alternative_titles", appended)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 77, "name": "倒凶十将伝", "original_name": "倒凶十将伝", "original_language": "ja",
				"first_air_date": "1999-01-01", "genres": []any{}, "networks": []any{}, "seasons": []any{},
				"credits": map[string]any{"cast": []any{}, "crew": []any{}}, "external_ids": map[string]any{},
				"images": map[string]any{}, "content_ratings": map[string]any{"results": []any{}},
				"alternative_titles": map[string]any{"results": []map[string]any{{"title": "10 Tokyo Warriors"}}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	result, err := newTMDBTestProvider(server.URL).GetMetadata(context.Background(), metadata.MetadataRequest{
		ProviderIDs: map[string]string{"tmdb": "77"}, ContentType: "series", Language: "en",
	})
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if result == nil || !result.TitleIsFallback || result.TitleLanguage != "ja" {
		t.Fatalf("fallback title metadata = %#v", result)
	}
	found := false
	for _, alias := range result.TitleAliases {
		found = found || alias.Title == "10 Tokyo Warriors" && alias.Kind == "alternate" && alias.Language == ""
	}
	if !found {
		t.Fatalf("aliases = %#v, want unknown-language alternative title", result.TitleAliases)
	}
}

func TestGetImagesReturnsRawPaths(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{
					"secure_base_url": serverURL(t, r) + "/images/",
				},
			})
		case "/movie/42":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 42,
				"images": map[string]any{
					"posters": []map[string]any{
						{"file_path": "/poster.jpg", "width": 2000, "height": 3000, "vote_average": 8.0},
					},
					"backdrops": []map[string]any{
						{"file_path": "/backdrop.jpg", "width": 3840, "height": 2160, "vote_average": 7.0},
					},
					"logos": []map[string]any{
						{"file_path": "/logo.png", "width": 1200, "height": 600, "vote_average": 6.0},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	p := newTMDBTestProvider(server.URL)

	images, err := p.GetImages(context.Background(), metadata.ImageRequest{
		ProviderIDs: map[string]string{"tmdb": "42"},
		ContentType: "movie",
	})
	if err != nil {
		t.Fatalf("GetImages() error = %v", err)
	}
	if len(images) != 3 {
		t.Fatalf("len(images) = %d, want 3", len(images))
	}

	got := map[metadata.ImageType]string{}
	for _, img := range images {
		got[img.Type] = img.URL
	}

	if got[metadata.ImagePoster] != "/poster.jpg" {
		t.Fatalf("poster URL = %q", got[metadata.ImagePoster])
	}
	if got[metadata.ImageBackdrop] != "/backdrop.jpg" {
		t.Fatalf("backdrop URL = %q", got[metadata.ImageBackdrop])
	}
	if got[metadata.ImageLogo] != "/logo.png" {
		t.Fatalf("logo URL = %q", got[metadata.ImageLogo])
	}
}

func TestGetImagesPrefersTMDBPrimaryPoster(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{
					"secure_base_url": serverURL(t, r) + "/images/",
				},
			})
		case "/movie/42":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          42,
				"poster_path": "/poster-primary.jpg",
				"images": map[string]any{
					"posters": []map[string]any{
						{
							"file_path":    "/poster-primary.jpg",
							"iso_639_1":    "en",
							"width":        2000,
							"height":       3000,
							"vote_average": 5.0,
						},
						{
							"file_path":    "/poster-textless.jpg",
							"iso_639_1":    nil,
							"width":        2000,
							"height":       3000,
							"vote_average": 8.0,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	p := newTMDBTestProvider(server.URL)

	images, err := p.GetImages(context.Background(), metadata.ImageRequest{
		ProviderIDs: map[string]string{"tmdb": "42"},
		ContentType: "movie",
	})
	if err != nil {
		t.Fatalf("GetImages() error = %v", err)
	}

	var primary, textless *metadata.RemoteImage
	for i := range images {
		switch images[i].URL {
		case "/poster-primary.jpg":
			primary = &images[i]
		case "/poster-textless.jpg":
			textless = &images[i]
		}
	}

	if primary == nil {
		t.Fatal("primary poster missing from GetImages() result")
	}
	if textless == nil {
		t.Fatal("textless poster missing from GetImages() result")
	}
	if primary.Rating <= textless.Rating {
		t.Fatalf("primary rating = %v, textless rating = %v; want primary > textless", primary.Rating, textless.Rating)
	}
}

func TestGetImagesAddsPrimaryPosterWhenImagesMissIt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{
					"secure_base_url": serverURL(t, r) + "/images/",
				},
			})
		case "/movie/42":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          42,
				"poster_path": "/poster-primary.jpg",
				"images": map[string]any{
					"posters": []map[string]any{
						{
							"file_path":    "/poster-alt.jpg",
							"width":        2000,
							"height":       3000,
							"vote_average": 8.0,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	p := newTMDBTestProvider(server.URL)

	images, err := p.GetImages(context.Background(), metadata.ImageRequest{
		ProviderIDs: map[string]string{"tmdb": "42"},
		ContentType: "movie",
		Language:    "en",
	})
	if err != nil {
		t.Fatalf("GetImages() error = %v", err)
	}

	var primary, alt *metadata.RemoteImage
	for i := range images {
		switch images[i].URL {
		case "/poster-primary.jpg":
			primary = &images[i]
		case "/poster-alt.jpg":
			alt = &images[i]
		}
	}

	if primary == nil {
		t.Fatal("primary poster was not appended to GetImages() result")
	}
	if alt == nil {
		t.Fatal("alternate poster missing from GetImages() result")
	}
	if primary.Language != "en" {
		t.Fatalf("primary language = %q, want en", primary.Language)
	}
	if primary.Rating <= alt.Rating {
		t.Fatalf("primary rating = %v, alt rating = %v; want primary > alt", primary.Rating, alt.Rating)
	}
}

func TestGetSeasonsReturnsRawPosterPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{
					"secure_base_url": serverURL(t, r) + "/images/",
				},
			})
		case "/tv/77":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 77,
				"seasons": []map[string]any{
					{"season_number": 2, "poster_path": "/season-two.jpg"},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	p := newTMDBTestProvider(server.URL)

	seasons, err := p.GetSeasons(context.Background(), metadata.SeasonsRequest{
		ProviderIDs: map[string]string{"tmdb": "77"},
		ContentType: "series",
	})
	if err != nil {
		t.Fatalf("GetSeasons() error = %v", err)
	}
	if len(seasons) != 1 {
		t.Fatalf("len(seasons) = %d, want 1", len(seasons))
	}
	if seasons[0].PosterPath != "/season-two.jpg" {
		t.Fatalf("season poster = %q", seasons[0].PosterPath)
	}
}

func TestGetEpisodesReturnsRawStillPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{
					"secure_base_url": serverURL(t, r) + "/images/",
				},
			})
		case "/tv/77/season/2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 2,
				"episodes": []map[string]any{
					{
						"id":             9001,
						"season_number":  2,
						"episode_number": 5,
						"name":           "Test Episode",
						"still_path":     "/still.jpg",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	p := newTMDBTestProvider(server.URL)

	episodes, err := p.GetEpisodes(context.Background(), metadata.EpisodesRequest{
		ProviderIDs:  map[string]string{"tmdb": "77"},
		SeasonNumber: 2,
	})
	if err != nil {
		t.Fatalf("GetEpisodes() error = %v", err)
	}
	if len(episodes) != 1 {
		t.Fatalf("len(episodes) = %d, want 1", len(episodes))
	}
	if episodes[0].StillPath != "/still.jpg" {
		t.Fatalf("episode still = %q", episodes[0].StillPath)
	}
}

func TestGetPersonDetail_UsesTMDBID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/person/287":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             287,
				"name":           "Brad Pitt",
				"biography":      "Biography",
				"birthday":       "1963-12-18",
				"place_of_birth": "Shawnee, Oklahoma, USA",
				"homepage":       "https://example.test/brad",
				"profile_path":   "/brad.jpg",
				"external_ids": map[string]any{
					"imdb_id": "nm0000093",
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	p := newTMDBTestProvider(server.URL)

	person, err := p.GetPersonDetail(context.Background(), metadata.PersonDetailRequest{
		ProviderIDs: map[string]string{"tmdb": "287"},
	})
	if err != nil {
		t.Fatalf("GetPersonDetail() error = %v", err)
	}
	if person == nil {
		t.Fatal("GetPersonDetail() returned nil person")
	}
	if person.Name != "Brad Pitt" {
		t.Fatalf("Name = %q, want Brad Pitt", person.Name)
	}
	if person.BirthDate != "1963-12-18" {
		t.Fatalf("BirthDate = %q, want 1963-12-18", person.BirthDate)
	}
	if person.Birthplace != "Shawnee, Oklahoma, USA" {
		t.Fatalf("Birthplace = %q", person.Birthplace)
	}
	if person.Homepage != "https://example.test/brad" {
		t.Fatalf("Homepage = %q", person.Homepage)
	}
	if person.PhotoPath != "/brad.jpg" {
		t.Fatalf("PhotoPath = %q, want /brad.jpg", person.PhotoPath)
	}
	if person.ProviderIDs["tmdb"] != "287" {
		t.Fatalf("ProviderIDs[tmdb] = %q, want 287", person.ProviderIDs["tmdb"])
	}
	if person.ProviderIDs["imdb"] != "nm0000093" {
		t.Fatalf("ProviderIDs[imdb] = %q, want nm0000093", person.ProviderIDs["imdb"])
	}
}

func TestGetPersonDetail_FindsTMDBPersonByIMDbID(t *testing.T) {
	t.Parallel()

	findSource := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/find/nm0000093":
			findSource = r.URL.Query().Get("external_source")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"person_results": []map[string]any{
					{"id": 287},
				},
			})
		case "/person/287":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           287,
				"name":         "Brad Pitt",
				"profile_path": "/brad.jpg",
				"external_ids": map[string]any{
					"imdb_id": "nm0000093",
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	p := newTMDBTestProvider(server.URL)

	person, err := p.GetPersonDetail(context.Background(), metadata.PersonDetailRequest{
		ProviderIDs: map[string]string{"imdb": "nm0000093"},
	})
	if err != nil {
		t.Fatalf("GetPersonDetail() error = %v", err)
	}
	if person == nil {
		t.Fatal("GetPersonDetail() returned nil person")
	}
	if person.ProviderIDs["tmdb"] != "287" {
		t.Fatalf("ProviderIDs[tmdb] = %q, want 287", person.ProviderIDs["tmdb"])
	}
	if findSource != "imdb_id" {
		t.Fatalf("external_source = %q, want imdb_id", findSource)
	}
}

func newTMDBTestProvider(baseURL string) *Provider {
	client := NewClient(1000)
	client.SetBaseURL(baseURL)
	return NewProviderWithClient(client)
}

func serverURL(t *testing.T, r *http.Request) string {
	t.Helper()
	return "http://" + r.Host
}

func TestGetTVMetadataCarriesShowStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{
					"secure_base_url": serverURL(t, r) + "/images/",
				},
			})
		case "/tv/77":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                77,
				"name":              "Series",
				"overview":          "Overview",
				"original_language": "en",
				"status":            "Returning Series",
				"genres":            []any{},
				"networks":          []any{},
				"seasons":           []any{},
				"credits": map[string]any{
					"cast": []any{},
					"crew": []any{},
				},
				"external_ids": map[string]any{},
				"images":       map[string]any{},
				"content_ratings": map[string]any{
					"results": []any{},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	p := newTMDBTestProvider(server.URL)
	result, err := p.GetMetadata(context.Background(), metadata.MetadataRequest{
		ProviderIDs: map[string]string{"tmdb": "77"},
		ContentType: "series",
		Language:    "en",
	})
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if result == nil {
		t.Fatal("GetMetadata() returned nil result")
	}
	if result.ShowStatus != "Returning Series" {
		t.Fatalf("ShowStatus = %q, want %q", result.ShowStatus, "Returning Series")
	}
}
