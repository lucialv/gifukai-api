package handlers

import (
	"net/http"

	u "github.com/lucialv/gifukai-api/pkg/utils"
)

func (h *Handler) HealthzHandler(w http.ResponseWriter, r *http.Request) error {
	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}