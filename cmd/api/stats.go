package api

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/lucialv/anime-api-cdn/pkg/store"
	u "github.com/lucialv/anime-api-cdn/pkg/utils"
)

type StatsCache struct {
	mu    sync.RWMutex
	stats *store.Stats
}

func (sc *StatsCache) Get() *store.Stats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.stats
}

func (sc *StatsCache) Set(stats *store.Stats) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.stats = stats
}

func (s *APIServer) StartStatsWorker(interval time.Duration) {
	s.refreshStats()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.refreshStats()
		}
	}()
}

func (s *APIServer) refreshStats() {
	stats, err := s.Store.GetStats()
	if err != nil {
		log.Printf("Failed to refresh stats: %v", err)
		return
	}
	s.statsCache.Set(stats)
	log.Println("Stats cache refreshed")
}

func formatBytes(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func (s *APIServer) statsHandler(w http.ResponseWriter, r *http.Request) error {
	stats := s.statsCache.Get()
	if stats == nil {
		u.WriteError(w, http.StatusServiceUnavailable, "stats not available yet")
		return nil
	}

	pairings := stats.GifsByPairing
	if pairings == nil {
		pairings = []store.PairingCount{}
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"total_gifs":       stats.TotalGifs,
		"total_actions":    stats.TotalActions,
		"total_animes":     stats.TotalAnimes,
		"total_size":       formatBytes(stats.TotalBytes),
		"total_size_bytes": stats.TotalBytes,
		"gifs_by_pairing":  pairings,
	})
}
