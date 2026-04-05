package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) HealthzHandler(w http.ResponseWriter, r *http.Request) error {
	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ListActionsHandler(w http.ResponseWriter, r *http.Request) error {
	actions, err := h.Store.GetAllActions()
	if err != nil {
		return err
	}
	if actions == nil {
		actions = []string{}
	}
	return u.WriteJSON(w, http.StatusOK, map[string]any{"actions": actions})
}

func (h *Handler) CountGifsHandler(w http.ResponseWriter, r *http.Request) error {
	action := chi.URLParam(r, "action")

	count, err := h.Store.CountGifs(action, "")
	if err != nil {
		return err
	}

	byPairing, err := h.Store.CountGifsByPairing(action)
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

func (h *Handler) GetRandomGifHandler(w http.ResponseWriter, r *http.Request) error {
	action := strings.ToLower(chi.URLParam(r, "action"))
	if action == "" {
		u.WriteError(w, http.StatusBadRequest, "action is required")
		return nil
	}

	pairing := strings.ToLower(r.URL.Query().Get("pairing"))
	if pairing != "" && !ValidPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s (valid: f, m, ff, mm, fm, mf)", pairing))
		return nil
	}

	var nsfwFilter *bool
	if nsfwParam := r.URL.Query().Get("nsfw"); nsfwParam != "" {
		val := nsfwParam == "true" || nsfwParam == "1"
		nsfwFilter = &val
	} else {
		val := false
		nsfwFilter = &val
	}

	var (
		result *store.Gif
		err    error
	)
	if pairing == "" {
		result, err = h.Store.GetRandomGifAnyPairing(action, nsfwFilter)
	} else {
		result, err = h.Store.GetRandomGif(action, pairing, nsfwFilter)
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

	return u.WriteJSON(w, http.StatusOK, h.buildGifResponse(result))
}