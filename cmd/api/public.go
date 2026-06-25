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
	ID          int64    `json:"id"`
	Action      string   `json:"action"`
	Actions     []string `json:"actions"`
	Variant     *string  `json:"type,omitempty"`
	Pairing     string   `json:"pairing"`
	URL         string   `json:"url"`
	AnimeName   *string  `json:"anime,omitempty"`
	ContentType string   `json:"content_type"`
	SizeBytes   int64    `json:"size_bytes"`
	Filename    string   `json:"filename"`
}

func (s *APIServer) publicListGifsHandler(w http.ResponseWriter, r *http.Request) error {
	policy, badRequest, err := handlers.ResolveActionTypePolicy(s.Store, r.URL.Query().Get("action"), r.URL.Query().Get("type"), true)
	if err != nil {
		return err
	}
	if badRequest != "" {
		u.WriteError(w, http.StatusBadRequest, badRequest)
		return nil
	}

	pairing := handlers.NormalizePairing(r.URL.Query().Get("pairing"))
	anime := strings.TrimSpace(r.URL.Query().Get("anime"))

	if pairing != "" && !handlers.ValidPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s", pairing))
		return nil
	}

	limit, offset := handlers.ParsePagination(r, 60, 200)

	gifs, total, err := s.Store.ListPublicGifs(policy.Action, pairing, anime, policy.Type, limit, offset)
	if err != nil {
		return err
	}
	if err := handlers.HideVariantsForUntypedActions(s.Store, gifs); err != nil {
		return err
	}

	items := make([]PublicGifItem, 0, len(gifs))
	for _, g := range gifs {
		items = append(items, PublicGifItem{
			ID:          g.ID,
			Action:      g.Action,
			Variant:     g.Variant,
			Pairing:     g.Pairing,
			AnimeName:   g.AnimeName,
			URL:         fmt.Sprintf("%s/%s", s.Config.CDNBaseURL, g.R2Key),
			ContentType: g.ContentType,
			SizeBytes:   g.SizeBytes,
			Filename:    filepath.Base(g.R2Key),
		})
	}

	response := map[string]any{
		"gifs":  items,
		"total": total,
	}
	if policy.Action != "" {
		response["action"] = policy.Action
		response["has_types"] = policy.HasTypes
	}
	if policy.Type != "" {
		response["type"] = policy.Type
	}

	return u.WriteJSON(w, http.StatusOK, response)
}

func (s *APIServer) publicListAnimesHandler(w http.ResponseWriter, r *http.Request) error {
	animes, err := s.Store.ListPublicAnimes()
	if err != nil {
		return err
	}
	animes = u.OrEmpty(animes)
	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"animes": animes,
	})
}
