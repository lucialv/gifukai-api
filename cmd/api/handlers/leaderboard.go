package handlers

import (
	"net/http"

	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"
)

func (h *Handler) LeaderboardHandler(w http.ResponseWriter, r *http.Request) error {
	entries, err := h.Store.GetLeaderboard(50)
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []store.LeaderboardEntry{}
	}
	return u.WriteJSON(w, http.StatusOK, map[string]any{"leaderboard": entries})
}