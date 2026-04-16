package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/lucialv/gifukai-api/pkg/storage"
	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"

	"github.com/go-chi/chi/v5"
)

type contextKey string

const userIDKey contextKey = "userID"

type Handler struct {
	Store      store.GifStore
	CDNBaseURL string
	R2Storage  *storage.R2Storage
}

var ValidPairings = map[string]bool{
	"f":  true,
	"m":  true,
	"ff": true,
	"mm": true,
	"fm": true,
	"mf": true,
}

type GifResponse struct {
	Action      string  `json:"action"`
	Pairing     string  `json:"pairing"`
	URL         string  `json:"url"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
	AnimeName   *string `json:"anime,omitempty"`
}

func GetUserID(r *http.Request) int64 {
	return r.Context().Value(userIDKey).(int64)
}

func SetUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func ParsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
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

func ParseIDParam(w http.ResponseWriter, r *http.Request, param, label string) (int64, bool) {
	idStr := chi.URLParam(r, param)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid "+label)
		return 0, false
	}
	return id, true
}

// BuildGifResponse is exported so admin.go can use it too :3
func BuildGifResponse(g *store.Gif, cdnBase string) GifResponse {
	return GifResponse{
		Action:      g.Action,
		Pairing:     g.Pairing,
		URL:         fmt.Sprintf("%s/%s", cdnBase, g.R2Key),
		Filename:    filepath.Base(g.R2Key),
		ContentType: g.ContentType,
		SizeBytes:   g.SizeBytes,
		AnimeName:   g.AnimeName,
	}
}

func (h *Handler) buildGifResponse(g *store.Gif) GifResponse {
	return BuildGifResponse(g, h.CDNBaseURL)
}