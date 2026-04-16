package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"
)

const maxSuggestionsPerDay = 15
const maxSuggestionSize = 10 << 20 // 10 MB

func (h *Handler) CreateSuggestionHandler(w http.ResponseWriter, r *http.Request) error {
	userID := GetUserID(r)

	todayCount, err := h.Store.CountUserSuggestionsToday(userID)
	if err != nil {
		return err
	}
	if todayCount >= maxSuggestionsPerDay {
		u.WriteError(w, http.StatusTooManyRequests, fmt.Sprintf("daily limit reached (%d/%d)", todayCount, maxSuggestionsPerDay))
		return nil
	}

	if err := r.ParseMultipartForm(maxSuggestionSize); err != nil {
		u.WriteError(w, http.StatusBadRequest, "file too large or invalid form (max 10MB)")
		return nil
	}

	action := strings.ToLower(r.FormValue("action"))
	pairing := strings.ToLower(r.FormValue("pairing"))

	if action == "" {
		u.WriteError(w, http.StatusBadRequest, "action is required")
		return nil
	}
	if !ValidPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s", pairing))
		return nil
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		u.WriteError(w, http.StatusBadRequest, "file is required")
		return nil
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	if int64(len(data)) > maxSuggestionSize {
		u.WriteError(w, http.StatusBadRequest, "file exceeds 10MB limit")
		return nil
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if contentType != "image/gif" {
		u.WriteError(w, http.StatusBadRequest, "only GIF files are accepted")
		return nil
	}

	id, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("failed to generate UUID: %w", err)
	}
	ext := strings.ToLower(path.Ext(header.Filename))
	if ext == "" {
		ext = ".gif"
	}
	fileKey := fmt.Sprintf("suggestions/%s%s", id.String(), ext)

	if err := h.R2Storage.UploadFile(fileKey, data, contentType); err != nil {
		return fmt.Errorf("failed to upload to R2: %w", err)
	}

	anime := strings.TrimSpace(r.FormValue("anime"))
	if anime == "" {
		_ = h.R2Storage.DeleteFile(fileKey)
		u.WriteError(w, http.StatusBadRequest, "anime name is required")
		return nil
	}

	suggestion := &store.Suggestion{
		UserID:      userID,
		FileKey:     fileKey,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		Action:      action,
		Pairing:     pairing,
		Status:      "pending",
		Anime:       &anime,
	}
	if tags := r.FormValue("tags"); tags != "" {
		suggestion.Tags = &tags
	}

	if err := h.Store.CreateSuggestion(suggestion); err != nil {
		_ = h.R2Storage.DeleteFile(fileKey)
		return fmt.Errorf("failed to create suggestion: %w", err)
	}

	return u.WriteJSON(w, http.StatusCreated, suggestion)
}

type suggestionItem struct {
	store.Suggestion
	URL string `json:"url"`
}

func (h *Handler) buildSuggestionItems(suggestions []store.Suggestion) []suggestionItem {
	items := make([]suggestionItem, 0, len(suggestions))
	for _, sg := range suggestions {
		items = append(items, suggestionItem{
			Suggestion: sg,
			URL:        fmt.Sprintf("%s/%s", h.CDNBaseURL, sg.FileKey),
		})
	}
	return items
}

func (h *Handler) ListUserSuggestionsHandler(w http.ResponseWriter, r *http.Request) error {
	userID := GetUserID(r)
	limit, offset := ParsePagination(r, 50, 200)

	suggestions, err := h.Store.ListUserSuggestions(userID, limit, offset)
	if err != nil {
		return err
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{"suggestions": h.buildSuggestionItems(suggestions)})
}

func (h *Handler) ListSuggestionsHandler(w http.ResponseWriter, r *http.Request) error {
	status := r.URL.Query().Get("status")
	limit, offset := ParsePagination(r, 50, 200)

	suggestions, total, err := h.Store.ListSuggestions(status, limit, offset)
	if err != nil {
		return err
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"suggestions": h.buildSuggestionItems(suggestions),
		"total":       total,
	})
}

func (h *Handler) ApproveSuggestionHandler(w http.ResponseWriter, r *http.Request) error {
	id, ok := ParseIDParam(w, r, "suggestionId", "suggestion ID")
	if !ok {
		return nil
	}

	suggestion, err := h.Store.GetSuggestionByID(id)
	if err != nil {
		return err
	}
	if suggestion == nil {
		u.WriteError(w, http.StatusNotFound, "suggestion not found")
		return nil
	}
	if suggestion.Status != "pending" {
		u.WriteError(w, http.StatusBadRequest, "suggestion is not pending")
		return nil
	}

	// Optional body: admin can override the anime_id
	var body struct {
		AnimeID *int64 `json:"anime_id"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

	data, err := h.R2Storage.DownloadFile(suggestion.FileKey)
	if err != nil {
		return fmt.Errorf("failed to download suggestion file: %w", err)
	}

	newID, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("failed to generate UUID: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(suggestion.FileKey))
	if ext == "" {
		ext = ".gif"
	}
	r2Key := fmt.Sprintf("%s/%s%s", suggestion.Action, newID.String(), ext)

	if err := h.R2Storage.UploadFile(r2Key, data, suggestion.ContentType); err != nil {
		return fmt.Errorf("failed to upload to production: %w", err)
	}

	gif := &store.Gif{
		Action:      suggestion.Action,
		Pairing:     suggestion.Pairing,
		R2Key:       r2Key,
		ContentType: suggestion.ContentType,
		SizeBytes:   suggestion.SizeBytes,
		CreatedAt:   time.Now().UTC(),
	}

	if body.AnimeID != nil {
		gif.AnimeID = body.AnimeID
	} else if suggestion.Anime != nil && *suggestion.Anime != "" {
		animes, _ := h.Store.GetAllAnimes()
		var animeID *int64
		for _, a := range animes {
			if strings.EqualFold(a.Name, *suggestion.Anime) {
				animeID = &a.ID
				break
			}
		}
		if animeID == nil {
			anime := &store.Anime{Name: *suggestion.Anime}
			if err := h.Store.CreateAnime(anime); err == nil {
				animeID = &anime.ID
			}
		}
		gif.AnimeID = animeID
	}
	if suggestion.Tags != nil {
		gif.Tags = suggestion.Tags
	}

	if err := h.Store.CreateGif(gif); err != nil {
		_ = h.R2Storage.DeleteFile(r2Key)
		return fmt.Errorf("failed to create gif record: %w", err)
	}

	if err := h.Store.UpdateSuggestionApproved(id, r2Key); err != nil {
		return fmt.Errorf("failed to update suggestion status: %w", err)
	}
	_ = h.R2Storage.DeleteFile(suggestion.FileKey)

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"status": "approved",
		"gif":    h.buildGifResponse(gif),
	})
}

func (h *Handler) RejectSuggestionHandler(w http.ResponseWriter, r *http.Request) error {
	id, ok := ParseIDParam(w, r, "suggestionId", "suggestion ID")
	if !ok {
		return nil
	}

	suggestion, err := h.Store.GetSuggestionByID(id)
	if err != nil {
		return err
	}
	if suggestion == nil {
		u.WriteError(w, http.StatusNotFound, "suggestion not found")
		return nil
	}
	if suggestion.Status != "pending" {
		u.WriteError(w, http.StatusBadRequest, "suggestion is not pending")
		return nil
	}

	if err := h.Store.UpdateSuggestionStatus(id, "rejected"); err != nil {
		return fmt.Errorf("failed to update suggestion status: %w", err)
	}
	_ = h.R2Storage.DeleteFile(suggestion.FileKey)

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *Handler) UserSuggestionsLimitHandler(w http.ResponseWriter, r *http.Request) error {
	userID := GetUserID(r)

	count, err := h.Store.CountUserSuggestionsToday(userID)
	if err != nil {
		return err
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"used":      count,
		"remaining": maxSuggestionsPerDay - count,
		"limit":     maxSuggestionsPerDay,
	})
}
