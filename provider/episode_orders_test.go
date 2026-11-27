package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/Silo-Server/silo-plugin-tmdb/metadata"
)

// Hand-written fixtures in the shape TMDB documents for
// /tv/{id}/episode_groups and /tv/episode_group/{id}.
const (
	episodeOrdersTVFixture = `{
		"id": 1399,
		"name": "Example Show",
		"seasons": [
			{"season_number": 0, "name": "Specials", "episode_count": 2},
			{"season_number": 1, "name": "Season 1", "episode_count": 10},
			{"season_number": 2, "name": "Season 2", "episode_count": 8}
		]
	}`

	episodeGroupsListFixture = `{
		"id": 1399,
		"results": [
			{
				"id": "5b11ba820e0a265847002c6e",
				"name": "DVD Order",
				"type": 3,
				"group_count": 3,
				"episode_count": 5,
				"description": "Episodes as released on DVD.",
				"network": null
			},
			{
				"id": "5e0b6c0a9f0e19001352a9c4",
				"name": "Absolute Order",
				"type": 2,
				"group_count": 3,
				"episode_count": 6,
				"description": ""
			},
			{
				"id": "5f0000000000000000000001",
				"name": "Story Arcs",
				"type": 5,
				"group_count": 4,
				"episode_count": 18,
				"description": "Grouped by story arc."
			}
		]
	}`

	// The groups arrive out of order, and the first group by order holds only
	// season 0 episodes, so it becomes season 0 although it is not named
	// "Specials".
	dvdEpisodeGroupFixture = `{
		"id": "5b11ba820e0a265847002c6e",
		"name": "DVD Order",
		"type": 3,
		"groups": [
			{
				"id": "g-vol2",
				"name": "Volume 2",
				"order": 2,
				"locked": true,
				"episodes": [
					{"show_id": 1399, "id": 1003, "name": "The Third", "overview": "Third aired.", "air_date": "2020-01-15", "still_path": "/s1e3.jpg", "runtime": 50, "season_number": 1, "episode_number": 3, "order": 1, "vote_average": 7.5},
					{"show_id": 1399, "id": 2001, "name": "Second Season Opener", "overview": "", "air_date": "2021-01-01", "still_path": "/s2e1.jpg", "runtime": 55, "season_number": 2, "episode_number": 1, "order": 0}
				]
			},
			{
				"id": "g-extras",
				"name": "Bonus Disc",
				"order": 0,
				"episodes": [
					{"show_id": 1399, "id": 9001, "name": "Behind the Scenes", "air_date": "2019-12-01", "season_number": 0, "episode_number": 1, "order": 0}
				]
			},
			{
				"id": "g-vol1",
				"name": "Volume 1",
				"order": 1,
				"episodes": [
					{"show_id": 1399, "id": 1002, "name": "The Second", "air_date": "2020-01-08", "season_number": 1, "episode_number": 2, "order": 1},
					{"show_id": 1399, "id": 1001, "name": "Pilot", "air_date": "2020-01-01", "season_number": 1, "episode_number": 1, "order": 0}
				]
			}
		]
	}`

	// An absolute order whose "Specials" group comes last and holds a regular
	// season episode, so only its name makes it season 0.
	absoluteEpisodeGroupFixture = `{
		"id": "5e0b6c0a9f0e19001352a9c4",
		"name": "Absolute Order",
		"type": 2,
		"groups": [
			{
				"id": "a-1",
				"name": "Part 1",
				"order": 0,
				"episodes": [
					{"id": 1001, "name": "Pilot", "season_number": 1, "episode_number": 1, "order": 0},
					{"id": 1002, "name": "The Second", "season_number": 1, "episode_number": 2, "order": 1}
				]
			},
			{
				"id": "a-sp",
				"name": "Specials",
				"order": 2,
				"episodes": [
					{"id": 1010, "name": "Recap", "season_number": 1, "episode_number": 10, "order": 0},
					{"id": 9001, "name": "Behind the Scenes", "season_number": 0, "episode_number": 1, "order": 1}
				]
			},
			{
				"id": "a-2",
				"name": "Part 2",
				"order": 1,
				"episodes": [
					{"id": 1003, "name": "The Third", "season_number": 1, "episode_number": 3, "order": 0},
					{"id": 2001, "name": "Second Season Opener", "season_number": 2, "episode_number": 1, "order": 1}
				]
			}
		]
	}`

	defaultSeasonFixture = `{
		"id": 501,
		"season_number": 1,
		"episodes": [
			{"id": 1001, "name": "Pilot", "season_number": 1, "episode_number": 1},
			{"id": 1002, "name": "The Second", "season_number": 1, "episode_number": 2}
		]
	}`
)

type episodeOrdersFixture struct {
	server       *httptest.Server
	groupFetches atomic.Int32
}

func newEpisodeOrdersFixture(t *testing.T) (*Provider, *episodeOrdersFixture) {
	t.Helper()
	fixture := &episodeOrdersFixture{}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := ""
		switch r.URL.Path {
		case "/configuration":
			body = `{"images": {"secure_base_url": "https://image.example/"}}`
		case "/tv/1399":
			body = episodeOrdersTVFixture
		case "/tv/1399/episode_groups":
			body = episodeGroupsListFixture
		case "/tv/1399/season/1":
			body = defaultSeasonFixture
		case "/tv/episode_group/5b11ba820e0a265847002c6e":
			fixture.groupFetches.Add(1)
			body = dvdEpisodeGroupFixture
		case "/tv/episode_group/5e0b6c0a9f0e19001352a9c4":
			fixture.groupFetches.Add(1)
			body = absoluteEpisodeGroupFixture
		case "/tv/episode_group/missing":
			w.WriteHeader(http.StatusNotFound)
			body = `{"success": false, "status_code": 34, "status_message": "The resource you requested could not be found."}`
		default:
			t.Errorf("unexpected TMDB request: %s", r.URL.String())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(fixture.server.Close)
	return newTMDBTestProvider(fixture.server.URL), fixture
}

func TestListEpisodeOrdersReturnsDefaultAndEpisodeGroups(t *testing.T) {
	t.Parallel()
	p, _ := newEpisodeOrdersFixture(t)

	orders, err := p.ListEpisodeOrders(context.Background(), metadata.EpisodeOrdersRequest{
		ProviderIDs: map[string]string{"tmdb": "1399"},
	})
	if err != nil {
		t.Fatalf("ListEpisodeOrders() error = %v", err)
	}
	want := []metadata.EpisodeOrder{
		// Regular seasons only, as the host shows them: Specials do not count.
		{ID: "default", Name: "Aired order", Kind: "aired", SeasonCount: 2, EpisodeCount: 18, IsDefault: true},
		{ID: "5b11ba820e0a265847002c6e", Name: "DVD Order", Kind: "dvd", Description: "Episodes as released on DVD.", SeasonCount: 3, EpisodeCount: 5},
		{ID: "5e0b6c0a9f0e19001352a9c4", Name: "Absolute Order", Kind: "absolute", SeasonCount: 3, EpisodeCount: 6},
		{ID: "5f0000000000000000000001", Name: "Story Arcs", Kind: "story_arc", Description: "Grouped by story arc.", SeasonCount: 4, EpisodeCount: 18},
	}
	if !reflect.DeepEqual(orders, want) {
		t.Fatalf("ListEpisodeOrders() =\n%+v\nwant\n%+v", orders, want)
	}
}

func TestGetSeasonsServesEpisodeGroupAsSeasons(t *testing.T) {
	t.Parallel()
	p, _ := newEpisodeOrdersFixture(t)

	seasons, err := p.GetSeasons(context.Background(), metadata.SeasonsRequest{
		ProviderIDs: map[string]string{"tmdb": "1399"},
		ContentType: "series",
		Ordering:    "5b11ba820e0a265847002c6e",
	})
	if err != nil {
		t.Fatalf("GetSeasons() error = %v", err)
	}
	want := []metadata.SeasonResult{
		{SeasonNumber: 0, Title: "Bonus Disc", AirDate: "2019-12-01"},
		{SeasonNumber: 1, Title: "Volume 1", AirDate: "2020-01-01"},
		{SeasonNumber: 2, Title: "Volume 2", AirDate: "2020-01-15"},
	}
	if !reflect.DeepEqual(seasons, want) {
		t.Fatalf("GetSeasons() =\n%+v\nwant\n%+v", seasons, want)
	}
}

func TestGetEpisodesNumbersEpisodeGroupAndKeepsDefaultCoordinates(t *testing.T) {
	t.Parallel()
	p, fixture := newEpisodeOrdersFixture(t)
	ctx := context.Background()
	ids := map[string]string{"tmdb": "1399"}
	const dvd = "5b11ba820e0a265847002c6e"

	if _, err := p.GetSeasons(ctx, metadata.SeasonsRequest{ProviderIDs: ids, Ordering: dvd}); err != nil {
		t.Fatalf("GetSeasons() error = %v", err)
	}
	episodes, err := p.GetEpisodes(ctx, metadata.EpisodesRequest{ProviderIDs: ids, SeasonNumber: 2, Ordering: dvd})
	if err != nil {
		t.Fatalf("GetEpisodes() error = %v", err)
	}
	want := []metadata.EpisodeResult{
		{
			ProviderIDs: map[string]string{"tmdb": "2001"}, SeasonNumber: 2, EpisodeNumber: 1,
			Title: "Second Season Opener", AirDate: "2021-01-01", Runtime: 55, StillPath: "/s2e1.jpg",
			DefaultSeasonNumber: intPtr(2), DefaultEpisodeNumber: intPtr(1),
		},
		{
			ProviderIDs: map[string]string{"tmdb": "1003"}, SeasonNumber: 2, EpisodeNumber: 2,
			Title: "The Third", Overview: "Third aired.", AirDate: "2020-01-15", Runtime: 50, StillPath: "/s1e3.jpg",
			Ratings:             metadata.Ratings{TMDB: 7.5},
			DefaultSeasonNumber: intPtr(1), DefaultEpisodeNumber: intPtr(3),
		},
	}
	if !reflect.DeepEqual(episodes, want) {
		t.Fatalf("GetEpisodes() =\n%+v\nwant\n%+v", episodes, want)
	}

	specials, err := p.GetEpisodes(ctx, metadata.EpisodesRequest{ProviderIDs: ids, SeasonNumber: 0, Ordering: dvd})
	if err != nil {
		t.Fatalf("GetEpisodes(season 0) error = %v", err)
	}
	if len(specials) != 1 || specials[0].ProviderIDs["tmdb"] != "9001" || specials[0].EpisodeNumber != 1 {
		t.Fatalf("GetEpisodes(season 0) = %+v, want the Bonus Disc episode 9001 as E1", specials)
	}
	if got := fixture.groupFetches.Load(); got != 1 {
		t.Fatalf("episode group fetched %d times, want 1 (cached across GetSeasons and GetEpisodes)", got)
	}
}

func TestGetEpisodesAbsoluteGroupSetsAbsoluteNumbersAndNamedSpecials(t *testing.T) {
	t.Parallel()
	p, _ := newEpisodeOrdersFixture(t)
	ctx := context.Background()
	ids := map[string]string{"tmdb": "1399"}
	const absolute = "5e0b6c0a9f0e19001352a9c4"

	seasons, err := p.GetSeasons(ctx, metadata.SeasonsRequest{ProviderIDs: ids, Ordering: absolute})
	if err != nil {
		t.Fatalf("GetSeasons() error = %v", err)
	}
	var numbered []string
	for _, season := range seasons {
		numbered = append(numbered, strconv.Itoa(season.SeasonNumber)+":"+season.Title)
	}
	if want := []string{"1:Part 1", "2:Part 2", "0:Specials"}; !reflect.DeepEqual(numbered, want) {
		t.Fatalf("seasons = %v, want %v", numbered, want)
	}

	type coords struct{ id, absolute, defaultSeason, defaultEpisode int }
	collect := func(season int) []coords {
		t.Helper()
		episodes, err := p.GetEpisodes(ctx, metadata.EpisodesRequest{ProviderIDs: ids, SeasonNumber: season, Ordering: absolute})
		if err != nil {
			t.Fatalf("GetEpisodes(%d) error = %v", season, err)
		}
		out := make([]coords, 0, len(episodes))
		for _, ep := range episodes {
			id, _ := strconv.Atoi(ep.ProviderIDs["tmdb"])
			c := coords{id: id, defaultSeason: *ep.DefaultSeasonNumber, defaultEpisode: *ep.DefaultEpisodeNumber}
			if ep.AbsoluteNumber != nil {
				c.absolute = *ep.AbsoluteNumber
			}
			out = append(out, c)
		}
		return out
	}
	if got, want := collect(2), []coords{{1003, 3, 1, 3}, {2001, 4, 2, 1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("season 2 = %v, want %v", got, want)
	}
	// Season 0 episodes have no absolute number, even the regular-season recap.
	if got, want := collect(0), []coords{{1010, 0, 1, 10}, {9001, 0, 0, 1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("season 0 = %v, want %v", got, want)
	}
}

// An episode group of another show (an order saved before the series was
// rematched) is not an ordering of this series.
func TestEpisodeOrderingGroupOfAnotherShowIsErrUnknownOrdering(t *testing.T) {
	t.Parallel()
	p, _ := newEpisodeOrdersFixture(t)
	ids := map[string]string{"tmdb": "4242"}
	const dvd = "5b11ba820e0a265847002c6e"

	if _, err := p.GetSeasons(context.Background(), metadata.SeasonsRequest{ProviderIDs: ids, Ordering: dvd}); !errors.Is(err, ErrUnknownOrdering) {
		t.Fatalf("GetSeasons() error = %v, want ErrUnknownOrdering", err)
	}
	if _, err := p.GetEpisodes(context.Background(), metadata.EpisodesRequest{ProviderIDs: ids, SeasonNumber: 1, Ordering: dvd}); !errors.Is(err, ErrUnknownOrdering) {
		t.Fatalf("GetEpisodes() error = %v, want ErrUnknownOrdering", err)
	}
}

func TestEpisodeOrderingUnknownGroupIsErrUnknownOrdering(t *testing.T) {
	t.Parallel()
	p, _ := newEpisodeOrdersFixture(t)
	ids := map[string]string{"tmdb": "1399"}

	_, err := p.GetSeasons(context.Background(), metadata.SeasonsRequest{ProviderIDs: ids, Ordering: "missing"})
	if !errors.Is(err, ErrUnknownOrdering) {
		t.Fatalf("GetSeasons() error = %v, want ErrUnknownOrdering", err)
	}
	_, err = p.GetEpisodes(context.Background(), metadata.EpisodesRequest{ProviderIDs: ids, SeasonNumber: 1, Ordering: "missing"})
	if !errors.Is(err, ErrUnknownOrdering) {
		t.Fatalf("GetEpisodes() error = %v, want ErrUnknownOrdering", err)
	}
}

func TestEpisodeOrderingEmptyOrDefaultServesTMDBSeasons(t *testing.T) {
	t.Parallel()
	p, fixture := newEpisodeOrdersFixture(t)
	ids := map[string]string{"tmdb": "1399"}

	for _, ordering := range []string{"", DefaultEpisodeOrderID} {
		seasons, err := p.GetSeasons(context.Background(), metadata.SeasonsRequest{ProviderIDs: ids, Ordering: ordering})
		if err != nil {
			t.Fatalf("GetSeasons(%q) error = %v", ordering, err)
		}
		if len(seasons) != 3 || seasons[1].SeasonNumber != 1 || seasons[1].Title != "Season 1" {
			t.Fatalf("GetSeasons(%q) = %+v, want TMDB's seasons 0-2", ordering, seasons)
		}
		episodes, err := p.GetEpisodes(context.Background(), metadata.EpisodesRequest{ProviderIDs: ids, SeasonNumber: 1, Ordering: ordering})
		if err != nil {
			t.Fatalf("GetEpisodes(%q) error = %v", ordering, err)
		}
		if len(episodes) != 2 || episodes[1].EpisodeNumber != 2 || episodes[1].DefaultSeasonNumber != nil || episodes[1].AbsoluteNumber != nil {
			t.Fatalf("GetEpisodes(%q) = %+v, want TMDB's season 1 without order coordinates", ordering, episodes)
		}
	}
	if got := fixture.groupFetches.Load(); got != 0 {
		t.Fatalf("episode group fetched %d times, want 0", got)
	}
}
