package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Silo-Server/silo-plugin-tmdb/metadata"
)

// ErrUnknownOrdering reports an ordering that is neither the default order nor
// an episode group TMDB knows. The plugin answers it with gRPC NOT_FOUND so the
// host can fall back to the default order.
var ErrUnknownOrdering = errors.New("tmdb: unknown episode order")

// DefaultEpisodeOrderID is the EpisodeOrder id of TMDB's own season layout,
// which GetSeasons and GetEpisodes serve when no ordering is requested.
const DefaultEpisodeOrderID = "default"

// episodeGroupKinds maps TMDB episode group types to the SDK's episode order
// kinds. TMDB documents 1 Original air date, 2 Absolute, 3 DVD, 4 Digital,
// 5 Story arc, 6 Production and 7 TV.
var episodeGroupKinds = map[int]string{
	1: "aired",
	2: "absolute",
	3: "dvd",
	4: "digital",
	5: "story_arc",
	6: "production",
	7: "tv",
}

// episodeGroupKind returns the order kind for a TMDB group type. A type TMDB
// adds later is reported as "alternate", which hosts already understand.
func episodeGroupKind(groupType int) string {
	if kind, ok := episodeGroupKinds[groupType]; ok {
		return kind
	}
	return "alternate"
}

// ListEpisodeOrders returns TMDB's default season layout followed by every
// episode group of the series. The default order's counts cover every season
// GetSeasons serves, Specials included, the way TMDB's group_count counts a
// group's Specials group.
func (p *Provider) ListEpisodeOrders(ctx context.Context, req metadata.EpisodeOrdersRequest) ([]metadata.EpisodeOrder, error) {
	tmdbID := req.ProviderIDs["tmdb"]
	if tmdbID == "" {
		return nil, nil
	}
	id, err := strconv.Atoi(tmdbID)
	if err != nil {
		return nil, fmt.Errorf("tmdb: invalid TMDB ID: %w", err)
	}

	tv, err := p.client.GetTV(ctx, id, tmdbLanguage(req.Language))
	if err != nil {
		return nil, err
	}
	groups, err := p.client.GetEpisodeGroups(ctx, id)
	if err != nil {
		return nil, err
	}

	defaultOrder := metadata.EpisodeOrder{
		ID:        DefaultEpisodeOrderID,
		Name:      "Aired order",
		Kind:      "aired",
		IsDefault: true,
	}
	// Regular seasons only: Specials (season 0) are not part of the order.
	for _, season := range tv.Seasons {
		if season.SeasonNumber > 0 {
			defaultOrder.SeasonCount++
			defaultOrder.EpisodeCount += season.EpisodeCount
		}
	}

	orders := make([]metadata.EpisodeOrder, 0, len(groups.Results)+1)
	orders = append(orders, defaultOrder)
	for _, group := range groups.Results {
		if group.ID == "" {
			continue
		}
		orders = append(orders, metadata.EpisodeOrder{
			ID:           group.ID,
			Name:         group.Name,
			Kind:         episodeGroupKind(group.Type),
			Description:  group.Description,
			SeasonCount:  group.GroupCount,
			EpisodeCount: group.EpisodeCount,
		})
	}
	return orders, nil
}

// isDefaultOrdering reports whether ordering asks for TMDB's own seasons: no
// ordering, or the id ListEpisodeOrders gives the default order.
func isDefaultOrdering(ordering string) bool {
	ordering = strings.TrimSpace(ordering)
	return ordering == "" || ordering == DefaultEpisodeOrderID
}

// orderedSeason is one group of an episode group, numbered as a season.
type orderedSeason struct {
	Number int
	Name   string
	// Episodes are sorted by their order within the group.
	Episodes []EpisodeGroupEpisode
	// FirstAbsolute is the absolute number of the first episode. It is 0 for
	// the Specials season, whose episodes have no absolute number.
	FirstAbsolute int
}

// layoutEpisodeGroup numbers the groups of an episode group as seasons. The
// same rules serve GetSeasons and GetEpisodes, so both agree on the numbers:
//
//   - Groups are taken in TMDB's group order (the "order" field), ties kept
//     in response order.
//   - The first group that is named "Specials" (any case), or whose episodes
//     are all in TMDB's season 0, becomes season 0. Later groups that look the
//     same are numbered like any other group, so no two seasons share a number.
//   - Every other group is numbered 1..N in group order, empty groups included,
//     which matches TMDB's group_count.
//   - Episodes in a group are sorted by their "order" field, ties kept in
//     response order, and numbered 1..N.
//   - Absolute numbers count episodes through the numbered seasons in that
//     order, starting at 1 and skipping season 0.
func layoutEpisodeGroup(detail *EpisodeGroupDetail) []orderedSeason {
	groups := append([]EpisodeGroup(nil), detail.Groups...)
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Order < groups[j].Order })

	seasons := make([]orderedSeason, 0, len(groups))
	specialsTaken := false
	nextSeason := 1
	absolute := 0
	for _, group := range groups {
		episodes := append([]EpisodeGroupEpisode(nil), group.Episodes...)
		sort.SliceStable(episodes, func(i, j int) bool { return episodes[i].Order < episodes[j].Order })
		season := orderedSeason{Name: group.Name, Episodes: episodes}
		if !specialsTaken && isSpecialsGroup(group.Name, episodes) {
			specialsTaken = true
		} else {
			season.Number = nextSeason
			nextSeason++
			season.FirstAbsolute = absolute + 1
			absolute += len(episodes)
		}
		seasons = append(seasons, season)
	}
	return seasons
}

func isSpecialsGroup(name string, episodes []EpisodeGroupEpisode) bool {
	if strings.EqualFold(strings.TrimSpace(name), "Specials") {
		return true
	}
	if len(episodes) == 0 {
		return false
	}
	for _, episode := range episodes {
		if episode.SeasonNumber != 0 {
			return false
		}
	}
	return true
}

// episodeGroup returns the episode group behind ordering, from the cache when
// it was fetched recently. GetSeasons and then GetEpisodes for each season all
// read the same group, so the cache saves one TMDB request per season. A
// group of another show (an order saved before the series was rematched) is
// not an ordering of the series showID.
func (p *Provider) episodeGroup(ctx context.Context, showID int, ordering, lang string) (*EpisodeGroupDetail, error) {
	key := ordering + "|" + lang
	detail := p.episodeGroups.get(key)
	if detail == nil {
		fetched, err := p.client.GetEpisodeGroup(ctx, ordering, lang)
		if err != nil {
			if isNotFound(err) {
				return nil, fmt.Errorf("%w %q", ErrUnknownOrdering, ordering)
			}
			return nil, err
		}
		p.episodeGroups.put(key, fetched)
		detail = fetched
	}
	owned := false
	for _, group := range detail.Groups {
		for _, episode := range group.Episodes {
			if episode.ShowID != 0 && episode.ShowID != showID {
				return nil, fmt.Errorf("%w %q: it belongs to another show", ErrUnknownOrdering, ordering)
			}
			owned = owned || episode.ShowID == showID
		}
	}
	if !owned {
		// No episode names its show: the show's own list of groups decides.
		listed, err := p.showListsEpisodeGroup(ctx, showID, ordering)
		if err != nil {
			return nil, err
		}
		if !listed {
			return nil, fmt.Errorf("%w %q: not an episode group of this show", ErrUnknownOrdering, ordering)
		}
	}
	return detail, nil
}

// showListsEpisodeGroup reports whether showID's episode groups include
// ordering. A confirmation is cached with the group, so the seasons of one
// order ask TMDB once.
func (p *Provider) showListsEpisodeGroup(ctx context.Context, showID int, ordering string) (bool, error) {
	key := fmt.Sprintf("owner|%s|%d", ordering, showID)
	if p.episodeGroups.get(key) != nil {
		return true, nil
	}
	groups, err := p.client.GetEpisodeGroups(ctx, showID)
	if err != nil {
		return false, err
	}
	for _, group := range groups.Results {
		if group.ID == ordering {
			p.episodeGroups.put(key, &EpisodeGroupDetail{ID: ordering})
			return true, nil
		}
	}
	return false, nil
}

func (p *Provider) getOrderedSeasons(ctx context.Context, showID int, req metadata.SeasonsRequest) ([]metadata.SeasonResult, error) {
	detail, err := p.episodeGroup(ctx, showID, strings.TrimSpace(req.Ordering), tmdbLanguage(req.Language))
	if err != nil {
		return nil, err
	}
	layout := layoutEpisodeGroup(detail)
	seasons := make([]metadata.SeasonResult, 0, len(layout))
	for _, season := range layout {
		// Groups carry no artwork or overview; the host fills those in.
		seasons = append(seasons, metadata.SeasonResult{
			SeasonNumber: season.Number,
			Title:        season.Name,
			AirDate:      earliestAirDate(season.Episodes),
		})
	}
	return seasons, nil
}

func (p *Provider) getOrderedEpisodes(ctx context.Context, showID int, req metadata.EpisodesRequest) ([]metadata.EpisodeResult, error) {
	detail, err := p.episodeGroup(ctx, showID, strings.TrimSpace(req.Ordering), tmdbLanguage(req.Language))
	if err != nil {
		return nil, err
	}
	absoluteOrder := episodeGroupKind(detail.Type) == "absolute"
	for _, season := range layoutEpisodeGroup(detail) {
		if season.Number != req.SeasonNumber {
			continue
		}
		episodes := make([]metadata.EpisodeResult, 0, len(season.Episodes))
		for i, ep := range season.Episodes {
			result := metadata.EpisodeResult{
				// The TMDB episode id is the same in every order.
				ProviderIDs:          map[string]string{"tmdb": strconv.Itoa(ep.ID)},
				SeasonNumber:         season.Number,
				EpisodeNumber:        i + 1,
				Title:                ep.Name,
				Overview:             ep.Overview,
				Runtime:              ep.Runtime,
				AirDate:              ep.AirDate,
				StillPath:            ep.StillPath,
				DefaultSeasonNumber:  intPtr(ep.SeasonNumber),
				DefaultEpisodeNumber: intPtr(ep.EpisodeNumber),
			}
			if absoluteOrder && season.FirstAbsolute > 0 {
				result.AbsoluteNumber = intPtr(season.FirstAbsolute + i)
			}
			if ep.VoteAverage > 0 {
				result.Ratings.TMDB = ep.VoteAverage
			}
			episodes = append(episodes, result)
		}
		return episodes, nil
	}
	return nil, nil
}

// earliestAirDate is the earliest valid YYYY-MM-DD air date of episodes;
// malformed dates are skipped.
func earliestAirDate(episodes []EpisodeGroupEpisode) string {
	var earliest time.Time
	for _, episode := range episodes {
		date, err := time.Parse(time.DateOnly, episode.AirDate)
		if err == nil && (earliest.IsZero() || date.Before(earliest)) {
			earliest = date
		}
	}
	if earliest.IsZero() {
		return ""
	}
	return earliest.Format(time.DateOnly)
}

func intPtr(value int) *int { return &value }

const (
	episodeGroupCacheTTL     = 10 * time.Minute
	episodeGroupCacheEntries = 64
)

// episodeGroupCache keeps recently fetched episode groups for a few minutes.
// It is bounded, and a stale entry is refetched, so edits on TMDB show up
// after the TTL.
type episodeGroupCache struct {
	mu      sync.Mutex
	entries map[string]episodeGroupCacheEntry
	now     func() time.Time
}

type episodeGroupCacheEntry struct {
	detail  *EpisodeGroupDetail
	fetched time.Time
}

func newEpisodeGroupCache() *episodeGroupCache {
	return &episodeGroupCache{entries: make(map[string]episodeGroupCacheEntry), now: time.Now}
}

func (c *episodeGroupCache) get(key string) *EpisodeGroupDetail {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	if c.now().Sub(entry.fetched) >= episodeGroupCacheTTL {
		delete(c.entries, key)
		return nil
	}
	return entry.detail
}

func (c *episodeGroupCache) put(key string, detail *EpisodeGroupDetail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.entries) >= episodeGroupCacheEntries {
		oldestKey := ""
		var oldest time.Time
		for k, entry := range c.entries {
			if now.Sub(entry.fetched) >= episodeGroupCacheTTL {
				delete(c.entries, k)
				continue
			}
			if oldestKey == "" || entry.fetched.Before(oldest) {
				oldestKey, oldest = k, entry.fetched
			}
		}
		if len(c.entries) >= episodeGroupCacheEntries {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[key] = episodeGroupCacheEntry{detail: detail, fetched: now}
}
