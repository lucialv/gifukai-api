package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/lucialv/anime-api-cdn/pkg/store"
	u "github.com/lucialv/anime-api-cdn/pkg/utils"

	"github.com/go-chi/chi/v5"
)

type GifResponse struct {
	Action      string  `json:"action"`
	Pairing     string  `json:"pairing"`
	URL         string  `json:"url"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
	AnimeID     *int64  `json:"anime_id,omitempty"`
	AnimeName   *string `json:"anime_name,omitempty"`
}

var validPairings = map[string]bool{
	"f":  true,
	"m":  true,
	"ff": true,
	"mm": true,
	"fm": true,
	"mf": true,
}

// healthzHandler returns 200 OK for health checks.
func (s *APIServer) healthzHandler(w http.ResponseWriter, r *http.Request) error {
	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// listActionsHandler returns all available actions.
func (s *APIServer) listActionsHandler(w http.ResponseWriter, r *http.Request) error {
	actions, err := s.Store.GetAllActions()
	if err != nil {
		return err
	}
	if actions == nil {
		actions = []string{}
	}
	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"actions": actions,
	})
}

// countGifsHandler returns how many GIFs exist for a given action, with breakdown by pairing.
func (s *APIServer) countGifsHandler(w http.ResponseWriter, r *http.Request) error {
	action := chi.URLParam(r, "action")

	count, err := s.Store.CountGifs(action, "")
	if err != nil {
		return err
	}

	byPairing, err := s.Store.CountGifsByPairing(action)
	if err != nil {
		return err
	}
	if byPairing == nil {
		byPairing = []store.PairingCount{}
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"action":     action,
		"count":      count,
		"by_pairing": byPairing,
	})
}

// getRandomGifHandler returns a random GIF for the given action.
//	GET /{action}?pairing=ff&nsfw=false
func (s *APIServer) getRandomGifHandler(w http.ResponseWriter, r *http.Request) error {
	action := strings.ToLower(chi.URLParam(r, "action"))
	if action == "" {
		u.WriteError(w, http.StatusBadRequest, "action is required")
		return nil
	}

	pairing := strings.ToLower(r.URL.Query().Get("pairing"))
	if pairing != "" && !validPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s (valid: f, m, ff, mm, fm, mf)", pairing))
		return nil
	}

	var nsfwFilter *bool
	nsfwParam := r.URL.Query().Get("nsfw")
	if nsfwParam != "" {
		val := nsfwParam == "true" || nsfwParam == "1"
		nsfwFilter = &val
	} else {
		val := false
		nsfwFilter = &val
	}

	var result *store.Gif
	var err error

	if pairing == "" {
		result, err = s.Store.GetRandomGifAnyPairing(action, nsfwFilter)
	} else {
		result, err = s.Store.GetRandomGif(action, pairing, nsfwFilter)
	}
	if err != nil {
		return err
	}
	if result == nil {
		msg := fmt.Sprintf("no GIFs found for action: %s", action)
		if pairing != "" {
			msg = fmt.Sprintf("no GIFs found for action: %s, pairing: %s", action, pairing)
		}
		u.WriteError(w, http.StatusNotFound, msg)
		return nil
	}

	resp := s.buildGifResponse(result)
	return u.WriteJSON(w, http.StatusOK, resp)
}

// buildGifResponse constructs the JSON response object.
func (s *APIServer) buildGifResponse(g *store.Gif) GifResponse {
	cdnBase := strings.TrimRight(s.Config.CDNBaseURL, "/")
	filename := filepath.Base(g.R2Key)

	return GifResponse{
		Action:      g.Action,
		Pairing:     g.Pairing,
		URL:         fmt.Sprintf("%s/%s", cdnBase, g.R2Key),
		Filename:    filename,
		ContentType: g.ContentType,
		SizeBytes:   g.SizeBytes,
		AnimeID:     g.AnimeID,
		AnimeName:   g.AnimeName,
	}
}
