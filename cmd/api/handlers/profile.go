package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	u "github.com/lucialv/gifukai-api/pkg/utils"
)

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

const maxDisplayNameLen = 25

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
	if len(name) > maxDisplayNameLen {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("display name must be %d characters or less", maxDisplayNameLen))
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