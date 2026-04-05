package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	u "github.com/lucialv/gifukai-api/pkg/utils"
)

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

func (h *Handler) UpdateProfileHandler(w http.ResponseWriter, r *http.Request) error {
	userID := GetUserID(r)

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid request body")
		return nil
	}

	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		u.WriteError(w, http.StatusBadRequest, "display name is required")
		return nil
	}
	if len(name) > 25 {
		u.WriteError(w, http.StatusBadRequest, "display name must be 25 characters or less")
		return nil
	}

	if err := h.Store.UpdateUserDisplayName(userID, name); err != nil {
		return err
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil {
		return err
	}

	return u.WriteJSON(w, http.StatusOK, user)
}