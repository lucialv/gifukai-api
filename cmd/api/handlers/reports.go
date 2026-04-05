package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"

	"github.com/go-chi/chi/v5"
)

func parsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	offset = 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= maxLimit {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	return
}

var validReasons = map[string]bool{
	"Low quality gif":                  true,
	"Bugged pixels":                    true,
	"Text in the gif that is annoying": true,
	"Other":                            true,
}

func (h *Handler) CreateReportHandler(w http.ResponseWriter, r *http.Request) error {
	userID := GetUserID(r)

	var body struct {
		GifID   int64   `json:"gif_id"`
		Reason  string  `json:"reason"`
		Details *string `json:"details,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil
	}

	if body.GifID == 0 {
		u.WriteError(w, http.StatusBadRequest, "gif_id is required")
		return nil
	}
	if !validReasons[body.Reason] {
		u.WriteError(w, http.StatusBadRequest, "invalid reason")
		return nil
	}
	if body.Reason == "Other" {
		if body.Details == nil || *body.Details == "" {
			u.WriteError(w, http.StatusBadRequest, "details are required when reason is 'Other'")
			return nil
		}
		if len(*body.Details) > 500 {
			u.WriteError(w, http.StatusBadRequest, "details must be 500 characters or less")
			return nil
		}
	}

	gif, err := h.Store.GetGifByID(body.GifID)
	if err != nil {
		return err
	}
	if gif == nil {
		u.WriteError(w, http.StatusNotFound, "gif not found")
		return nil
	}

	already, err := h.Store.HasUserReportedGif(userID, body.GifID)
	if err != nil {
		return err
	}
	if already {
		u.WriteError(w, http.StatusConflict, "you have already reported this gif")
		return nil
	}

	report := &store.Report{
		GifID:   &body.GifID,
		UserID:  userID,
		Reason:  body.Reason,
		Details: body.Details,
		Status:  "pending",
	}
	if err := h.Store.CreateReport(report); err != nil {
		return fmt.Errorf("failed to create report: %w", err)
	}

	return u.WriteJSON(w, http.StatusCreated, report)
}

func (h *Handler) ListReportsHandler(w http.ResponseWriter, r *http.Request) error {
	status := r.URL.Query().Get("status")
	limit, offset := parsePagination(r, 50, 200)

	reports, total, err := h.Store.ListReports(status, limit, offset)
	if err != nil {
		return err
	}

	cdnBase := strings.TrimRight(h.CDNBaseURL, "/")
	for i := range reports {
		if reports[i].GifURL != nil {
			url := fmt.Sprintf("%s/%s", cdnBase, *reports[i].GifURL)
			reports[i].GifURL = &url
		}
	}
	if reports == nil {
		reports = []store.Report{}
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"reports": reports,
		"total":   total,
	})
}

func (h *Handler) UpdateReportHandler(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "reportId")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid report ID")
		return nil
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil
	}

	if body.Status != "resolved" && body.Status != "rejected" {
		u.WriteError(w, http.StatusBadRequest, "status must be 'resolved' or 'rejected'")
		return nil
	}

	if err := h.Store.UpdateReportStatus(id, body.Status); err != nil {
		return fmt.Errorf("failed to update report: %w", err)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}