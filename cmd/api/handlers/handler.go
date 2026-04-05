package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/lucialv/gifukai-api/pkg/storage"
	"github.com/lucialv/gifukai-api/pkg/store"
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
	AnimeName   *string `json:"anime_name,omitempty"`
}

func GetUserID(r *http.Request) int64 {
	return r.Context().Value(userIDKey).(int64)
}

func SetUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// BuildGifResponse is exported so admin.go can use it too :3
func BuildGifResponse(g *store.Gif, cdnBase string) GifResponse {
	base := strings.TrimRight(cdnBase, "/")
	return GifResponse{
		Action:      g.Action,
		Pairing:     g.Pairing,
		URL:         fmt.Sprintf("%s/%s", base, g.R2Key),
		Filename:    filepath.Base(g.R2Key),
		ContentType: g.ContentType,
		SizeBytes:   g.SizeBytes,
		AnimeName:   g.AnimeName,
	}
}

func (h *Handler) buildGifResponse(g *store.Gif) GifResponse {
	return BuildGifResponse(g, h.CDNBaseURL)
}