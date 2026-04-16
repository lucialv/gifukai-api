package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"
)

const maxReportDetailsLen = 500

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
		if len(*body.Details) > maxReportDetailsLen {
			u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("details must be %d characters or less", maxReportDetailsLen))
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
	limit, offset := ParsePagination(r, 50, 200)

	reports, total, err := h.Store.ListReports(status, limit, offset)
	if err != nil {
		return err
	}

	for i := range reports {
		if reports[i].GifURL != nil {
			url := fmt.Sprintf("%s/%s", h.CDNBaseURL, *reports[i].GifURL)
			reports[i].GifURL = &url
		}
	}
	reports = u.OrEmpty(reports)

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"reports": reports,
		"total":   total,
	})
}

func (h *Handler) UpdateReportHandler(w http.ResponseWriter, r *http.Request) error {
	id, ok := ParseIDParam(w, r, "reportId", "report ID")
	if !ok {
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