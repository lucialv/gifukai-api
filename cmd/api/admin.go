package api

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/lucialv/anime-api-cdn/pkg/store"
	u "github.com/lucialv/anime-api-cdn/pkg/utils"

	"github.com/go-chi/chi/v5"
)

// uploadGifHandler handles POST /admin/gifs
// Accepts multipart form with: file, action, pairing, tags (optional), nsfw (optional)
func (s *APIServer) uploadGifHandler(w http.ResponseWriter, r *http.Request) error {
	// Max 2MB
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return nil
	}

	action := strings.ToLower(r.FormValue("action"))
	pairing := strings.ToLower(r.FormValue("pairing"))
	tags := r.FormValue("tags")
	nsfwStr := r.FormValue("nsfw")

	if action == "" {
		u.WriteError(w, http.StatusBadRequest, "action is required")
		return nil
	}
	if !validPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s (valid: ff, mm, fm)", pairing))
		return nil
	}

	nsfw := nsfwStr == "true" || nsfwStr == "1"

	file, header, err := r.FormFile("file")
	if err != nil {
		u.WriteError(w, http.StatusBadRequest, "file is required: "+err.Error())
		return nil
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read uploaded file: %w", err)
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	id, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("failed to generate UUID: %w", err)
	}
	ext := strings.ToLower(path.Ext(header.Filename))
	if ext == "" {
		ext = ".gif"
	}
	r2Key := fmt.Sprintf("%s/%s%s", action, id.String(), ext)

	if err := s.R2Storage.UploadFile(r2Key, data, contentType); err != nil {
		return fmt.Errorf("failed to upload to R2: %w", err)
	}

	gif := &store.Gif{
		Action:      action,
		Pairing:     pairing,
		R2Key:       r2Key,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		NSFW:        nsfw,
		CreatedAt:   time.Now().UTC(),
	}
	if tags != "" {
		gif.Tags = &tags
	}

	if err := s.Store.CreateGif(gif); err != nil {
		_ = s.R2Storage.DeleteFile(r2Key)
		return fmt.Errorf("failed to create gif record: %w", err)
	}

	resp := s.buildGifResponse(gif.Action, gif.Pairing, gif.R2Key, gif.ContentType, gif.SizeBytes)
	return u.WriteJSON(w, http.StatusCreated, resp)
}

// deleteGifHandler handles DELETE /admin/gifs/{gifId}
func (s *APIServer) deleteGifHandler(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "gifId")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid gif ID")
		return nil
	}

	gif, err := s.Store.GetGifByID(id)
	if err != nil {
		return err
	}
	if gif == nil {
		u.WriteError(w, http.StatusNotFound, "gif not found")
		return nil
	}

	if err := s.R2Storage.DeleteFile(gif.R2Key); err != nil {
		return fmt.Errorf("failed to delete from R2: %w", err)
	}

	if err := s.Store.DeleteGif(id); err != nil {
		return fmt.Errorf("failed to delete gif record: %w", err)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// listGifsHandler handles GET /admin/gifs?action=hug&limit=50&offset=0
func (s *APIServer) listGifsHandler(w http.ResponseWriter, r *http.Request) error {
	action := r.URL.Query().Get("action")
	if action == "" {
		u.WriteError(w, http.StatusBadRequest, "action query param is required")
		return nil
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	gifs, err := s.Store.GetGifsByAction(action, limit, offset)
	if err != nil {
		return err
	}

	cdnBase := strings.TrimRight(s.Config.CDNBaseURL, "/")

	type gifItem struct {
		ID          int64  `json:"id"`
		Action      string `json:"action"`
		Pairing     string `json:"pairing"`
		URL         string `json:"url"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
		NSFW        bool   `json:"nsfw"`
		Tags        string `json:"tags,omitempty"`
	}

	var items []gifItem
	for _, g := range gifs {
		item := gifItem{
			ID:          g.ID,
			Action:      g.Action,
			Pairing:     g.Pairing,
			URL:         fmt.Sprintf("%s/%s", cdnBase, g.R2Key),
			Filename:    filepath.Base(g.R2Key),
			ContentType: g.ContentType,
			SizeBytes:   g.SizeBytes,
			NSFW:        g.NSFW,
		}
		if g.Tags != nil {
			item.Tags = *g.Tags
		}
		items = append(items, item)
	}

	if items == nil {
		items = []gifItem{}
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"gifs":  items,
		"count": len(items),
	})
}
