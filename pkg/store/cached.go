package store

import (
	"log"
	"math/rand/v2"
	"sort"
	"sync/atomic"
	"time"
)

type cachedData struct {
	gifs     []Gif
	byID     map[int64]*Gif
	byAction map[string][]*Gif
	actions  []string
	animes   []Anime
	stats    Stats
}

type CachedGifStore struct {
	inner GifStore
	data  atomic.Pointer[cachedData]
}

func NewCachedGifStore(inner GifStore) (*CachedGifStore, error) {
	c := &CachedGifStore{inner: inner}
	if err := c.Reload(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *CachedGifStore) StartRefreshWorker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := c.Reload(); err != nil {
				log.Printf("Failed to refresh GIF cache: %v", err)
			}
		}
	}()
}

func (c *CachedGifStore) Reload() error {
	gifs, err := c.inner.ListAllGifs("", 1_000_000, 0)
	if err != nil {
		return err
	}

	animes, err := c.inner.GetAllAnimes()
	if err != nil {
		return err
	}

	d := &cachedData{
		gifs:     gifs,
		byID:     make(map[int64]*Gif, len(gifs)),
		byAction: make(map[string][]*Gif),
		animes:   animes,
	}

	actionSet := make(map[string]struct{})
	var totalBytes int64
	pairingCounts := make(map[string]int)

	for i := range d.gifs {
		g := &d.gifs[i]
		d.byID[g.ID] = g
		d.byAction[g.Action] = append(d.byAction[g.Action], g)
		actionSet[g.Action] = struct{}{}
		totalBytes += g.SizeBytes
		pairingCounts[g.Pairing]++
	}

	d.actions = make([]string, 0, len(actionSet))
	for a := range actionSet {
		d.actions = append(d.actions, a)
	}
	sort.Strings(d.actions)

	var gifsByPairing []PairingCount
	for p, cnt := range pairingCounts {
		gifsByPairing = append(gifsByPairing, PairingCount{Pairing: p, Count: cnt})
	}
	sort.Slice(gifsByPairing, func(i, j int) bool {
		return gifsByPairing[i].Pairing < gifsByPairing[j].Pairing
	})

	d.stats = Stats{
		TotalGifs:     len(gifs),
		TotalActions:  len(d.actions),
		TotalAnimes:   len(animes),
		TotalBytes:    totalBytes,
		GifsByPairing: gifsByPairing,
	}

	c.data.Store(d)
	log.Printf("GIF cache loaded: %d gifs, %d actions, %d animes", len(gifs), len(d.actions), len(animes))
	return nil
}

func (c *CachedGifStore) reloadQuiet() {
	if err := c.Reload(); err != nil {
		log.Printf("Failed to reload GIF cache: %v", err)
	}
}

// ~~ GIFs ~~

func (c *CachedGifStore) GetRandomGif(action, pairing string, nsfw *bool) (*Gif, error) {
	d := c.data.Load()
	candidates := d.byAction[action]
	if len(candidates) == 0 {
		return nil, nil
	}

	var filtered []*Gif
	for _, g := range candidates {
		if pairing != "" && g.Pairing != pairing {
			continue
		}
		if nsfw != nil && g.NSFW != *nsfw {
			continue
		}
		filtered = append(filtered, g)
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	picked := *filtered[rand.IntN(len(filtered))]
	return &picked, nil
}

func (c *CachedGifStore) GetRandomGifAnyPairing(action string, nsfw *bool) (*Gif, error) {
	return c.GetRandomGif(action, "", nsfw)
}

func (c *CachedGifStore) GetAllActions() ([]string, error) {
	d := c.data.Load()
	out := make([]string, len(d.actions))
	copy(out, d.actions)
	return out, nil
}

func (c *CachedGifStore) GetGifByID(id int64) (*Gif, error) {
	d := c.data.Load()
	if g, ok := d.byID[id]; ok {
		out := *g
		return &out, nil
	}
	return nil, nil
}

func (c *CachedGifStore) GetGifsByActionAndPairing(action, pairing string, limit, offset int) ([]Gif, error) {
	d := c.data.Load()
	candidates := d.byAction[action]

	var filtered []*Gif
	for _, g := range candidates {
		if pairing != "" && g.Pairing != pairing {
			continue
		}
		filtered = append(filtered, g)
	}

	if offset >= len(filtered) {
		return []Gif{}, nil
	}
	end := min(offset+limit, len(filtered))

	result := make([]Gif, 0, end-offset)
	for _, g := range filtered[offset:end] {
		result = append(result, *g)
	}
	return result, nil
}

func (c *CachedGifStore) CountGifs(action, pairing string) (int, error) {
	d := c.data.Load()
	count := 0
	for i := range d.gifs {
		if action != "" && d.gifs[i].Action != action {
			continue
		}
		if pairing != "" && d.gifs[i].Pairing != pairing {
			continue
		}
		count++
	}
	return count, nil
}

func (c *CachedGifStore) CountGifsByPairing(action string) ([]PairingCount, error) {
	d := c.data.Load()
	counts := make(map[string]int)
	for i := range d.gifs {
		if action != "" && d.gifs[i].Action != action {
			continue
		}
		counts[d.gifs[i].Pairing]++
	}

	result := make([]PairingCount, 0, len(counts))
	for p, cnt := range counts {
		result = append(result, PairingCount{Pairing: p, Count: cnt})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Pairing < result[j].Pairing
	})
	return result, nil
}

func (c *CachedGifStore) RefreshIDPool(action, pairing string) ([]int64, error) {
	d := c.data.Load()
	var ids []int64
	for i := range d.gifs {
		if d.gifs[i].Action == action && d.gifs[i].Pairing == pairing {
			ids = append(ids, d.gifs[i].ID)
		}
	}
	return ids, nil
}

func (c *CachedGifStore) ListAllGifs(pairing string, limit, offset int) ([]Gif, error) {
	d := c.data.Load()

	var src []Gif
	if pairing == "" {
		src = d.gifs
	} else {
		for i := range d.gifs {
			if d.gifs[i].Pairing == pairing {
				src = append(src, d.gifs[i])
			}
		}
	}

	if offset >= len(src) {
		return []Gif{}, nil
	}
	end := min(offset+limit, len(src))

	result := make([]Gif, end-offset)
	copy(result, src[offset:end])
	return result, nil
}

func (c *CachedGifStore) CountAllGifs(pairing string) (int, error) {
	d := c.data.Load()
	if pairing == "" {
		return len(d.gifs), nil
	}
	count := 0
	for i := range d.gifs {
		if d.gifs[i].Pairing == pairing {
			count++
		}
	}
	return count, nil
}

func (c *CachedGifStore) GetStats() (*Stats, error) {
	d := c.data.Load()
	out := d.stats
	return &out, nil
}

// ~~ Animes ~~

func (c *CachedGifStore) GetAllAnimes() ([]Anime, error) {
	d := c.data.Load()
	out := make([]Anime, len(d.animes))
	copy(out, d.animes)
	return out, nil
}

// ~~ Public library ~~

func (c *CachedGifStore) ListPublicGifs(action, pairing, anime string, limit, offset int) ([]Gif, int, error) {
	d := c.data.Load()

	var filtered []Gif
	for i := range d.gifs {
		g := &d.gifs[i]
		if g.NSFW {
			continue
		}
		if action != "" && g.Action != action {
			continue
		}
		if pairing != "" && g.Pairing != pairing {
			continue
		}
		if anime != "" && (g.AnimeName == nil || *g.AnimeName != anime) {
			continue
		}
		filtered = append(filtered, *g)
	}

	total := len(filtered)
	if offset >= total {
		return []Gif{}, total, nil
	}
	end := min(offset+limit, total)
	return filtered[offset:end], total, nil
}

func (c *CachedGifStore) ListPublicAnimes() ([]string, error) {
	d := c.data.Load()

	nameSet := make(map[string]struct{})
	for i := range d.gifs {
		if !d.gifs[i].NSFW && d.gifs[i].AnimeName != nil {
			nameSet[*d.gifs[i].AnimeName] = struct{}{}
		}
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// ~~ Write methods ~~ 
// These all pass through to the inner store, then trigger a cache reload on success ^^

func (c *CachedGifStore) CreateGif(gif *Gif) error {
	if err := c.inner.CreateGif(gif); err != nil {
		return err
	}
	c.reloadQuiet()
	return nil
}

func (c *CachedGifStore) DeleteGif(id int64) error {
	if err := c.inner.DeleteGif(id); err != nil {
		return err
	}
	c.reloadQuiet()
	return nil
}

func (c *CachedGifStore) UpdateGifTags(id int64, tags *string) error {
	if err := c.inner.UpdateGifTags(id, tags); err != nil {
		return err
	}
	c.reloadQuiet()
	return nil
}

func (c *CachedGifStore) UpdateGifPairing(id int64, pairing string) error {
	if err := c.inner.UpdateGifPairing(id, pairing); err != nil {
		return err
	}
	c.reloadQuiet()
	return nil
}

func (c *CachedGifStore) UpdateGifAnime(gifID int64, animeID *int64) error {
	if err := c.inner.UpdateGifAnime(gifID, animeID); err != nil {
		return err
	}
	c.reloadQuiet()
	return nil
}

func (c *CachedGifStore) CreateAnime(anime *Anime) error {
	if err := c.inner.CreateAnime(anime); err != nil {
		return err
	}
	c.reloadQuiet()
	return nil
}

func (c *CachedGifStore) UpdateAnime(id int64, name string) error {
	if err := c.inner.UpdateAnime(id, name); err != nil {
		return err
	}
	c.reloadQuiet()
	return nil
}

func (c *CachedGifStore) DeleteAnime(id int64) error {
	if err := c.inner.DeleteAnime(id); err != nil {
		return err
	}
	c.reloadQuiet()
	return nil
}

// Pass-through — users, reports, suggestions (not cached)

func (c *CachedGifStore) CreateOrGetUser(user *User) (*User, error) {
	return c.inner.CreateOrGetUser(user)
}

func (c *CachedGifStore) GetUserByID(id int64) (*User, error) {
	return c.inner.GetUserByID(id)
}

func (c *CachedGifStore) UpdateUserDisplayName(id int64, name string) error {
	return c.inner.UpdateUserDisplayName(id, name)
}

func (c *CachedGifStore) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	return c.inner.GetLeaderboard(limit)
}

func (c *CachedGifStore) CreateReport(report *Report) error {
	return c.inner.CreateReport(report)
}

func (c *CachedGifStore) ListReports(status string, limit, offset int) ([]Report, int, error) {
	return c.inner.ListReports(status, limit, offset)
}

func (c *CachedGifStore) UpdateReportStatus(id int64, status string) error {
	return c.inner.UpdateReportStatus(id, status)
}

func (c *CachedGifStore) HasUserReportedGif(userID, gifID int64) (bool, error) {
	return c.inner.HasUserReportedGif(userID, gifID)
}

func (c *CachedGifStore) CreateSuggestion(suggestion *Suggestion) error {
	return c.inner.CreateSuggestion(suggestion)
}

func (c *CachedGifStore) ListSuggestions(status string, limit, offset int) ([]Suggestion, int, error) {
	return c.inner.ListSuggestions(status, limit, offset)
}

func (c *CachedGifStore) GetSuggestionByID(id int64) (*Suggestion, error) {
	return c.inner.GetSuggestionByID(id)
}

func (c *CachedGifStore) UpdateSuggestionStatus(id int64, status string) error {
	return c.inner.UpdateSuggestionStatus(id, status)
}

func (c *CachedGifStore) UpdateSuggestionApproved(id int64, newFileKey string) error {
	return c.inner.UpdateSuggestionApproved(id, newFileKey)
}

func (c *CachedGifStore) CountUserSuggestionsToday(userID int64) (int, error) {
	return c.inner.CountUserSuggestionsToday(userID)
}

func (c *CachedGifStore) ListUserSuggestions(userID int64, limit, offset int) ([]Suggestion, error) {
	return c.inner.ListUserSuggestions(userID, limit, offset)
}
