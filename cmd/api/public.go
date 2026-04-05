package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/lucialv/gifukai-api/cmd/api/handlers"
	u "github.com/lucialv/gifukai-api/pkg/utils"
)

type PublicGifItem struct {
	ID          int64   `json:"id"`
	Action      string  `json:"action"`
	Pairing     string  `json:"pairing"`
	URL         string  `json:"url"`
	AnimeName   *string `json:"anime_name,omitempty"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
	Filename    string  `json:"filename"`
}

func (s *APIServer) publicListGifsHandler(w http.ResponseWriter, r *http.Request) error {
	action := strings.ToLower(r.URL.Query().Get("action"))
	pairing := strings.ToLower(r.URL.Query().Get("pairing"))
	anime := r.URL.Query().Get("anime")

	if pairing != "" && !handlers.ValidPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s", pairing))
		return nil
	}

	limit, offset := ParsePagination(r, 60, 200)

	gifs, total, err := s.Store.ListPublicGifs(action, pairing, anime, limit, offset)
	if err != nil {
		return err
	}

	cdnBase := strings.TrimRight(s.Config.CDNBaseURL, "/")

	var items []PublicGifItem
	for _, g := range gifs {
		items = append(items, PublicGifItem{
			ID:          g.ID,
			Action:      g.Action,
			Pairing:     g.Pairing,
			URL:         fmt.Sprintf("%s/%s", cdnBase, g.R2Key),
			AnimeName:   g.AnimeName,
			ContentType: g.ContentType,
			SizeBytes:   g.SizeBytes,
			Filename:    filepath.Base(g.R2Key),
		})
	}

	if items == nil {
		items = []PublicGifItem{}
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"gifs":  items,
		"total": total,
	})
}

func (s *APIServer) publicListAnimesHandler(w http.ResponseWriter, r *http.Request) error {
	animes, err := s.Store.ListPublicAnimes()
	if err != nil {
		return err
	}
	if animes == nil {
		animes = []string{}
	}
	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"animes": animes,
	})
}
