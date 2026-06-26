package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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
	Variant     *string `json:"type,omitempty"`
	Pairing     string  `json:"pairing"`
	AnimeName   *string `json:"anime,omitempty"`
	URL         string  `json:"url"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
}

var PairingOrder = []string{"f", "m", "ff", "mm", "fm", "mf"}

func IsDirectionalPairing(pairing string) bool {
	return pairing == "mf" || pairing == "fm"
}

var typePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

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
		Variant:     g.Variant,
		Pairing:     g.Pairing,
		AnimeName:   g.AnimeName,
		URL:         fmt.Sprintf("%s/%s", cdnBase, g.R2Key),
		Filename:    filepath.Base(g.R2Key),
		ContentType: g.ContentType,
		SizeBytes:   g.SizeBytes,
	}
}

func NormalizeGifType(raw string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(raw))
	if t == "" {
		return "", nil
	}
	if !typePattern.MatchString(t) {
		return "", fmt.Errorf("invalid type: %s (use 1-32 chars: a-z, 0-9, _, -)", raw)
	}
	return t, nil
}

func NormalizeAction(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func NormalizePairing(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

type ActionTypePolicy struct {
	Action   string
	Type     string
	HasTypes bool
}

func ResolveActionTypePolicy(gifStore store.GifStore, rawAction, rawType string, requireActionForType bool) (ActionTypePolicy, string, error) {
	action := NormalizeAction(rawAction)

	// resolve aliases (e.g. peck -> kiss, locked to cheek) :3
	alias, err := gifStore.ResolveAlias(action)
	if err != nil {
		return ActionTypePolicy{Action: action}, "", err
	}
	if alias != nil {
		action = alias.Action
		if alias.Variant != nil {
			rawType = *alias.Variant
		}
	}

	policy := ActionTypePolicy{Action: action}

	gifType, err := NormalizeGifType(rawType)
	if err != nil {
		return policy, err.Error(), nil
	}
	policy.Type = gifType

	if policy.Action != "" {
		hasTypes, err := gifStore.ActionHasTypes(policy.Action)
		if err != nil {
			return policy, "", err
		}
		policy.HasTypes = hasTypes
	}

	if policy.Type == "" {
		return policy, "", nil
	}

	if policy.Action == "" {
		if requireActionForType {
			return policy, "action is required when using type filter", nil
		}
		return policy, "", nil
	}

	if !policy.HasTypes {
		return policy, fmt.Sprintf("action %s does not support type filtering", policy.Action), nil
	}

	return policy, "", nil
}

func RequireTypeForTypedAction(policy ActionTypePolicy) string {
	if policy.Action == "" || policy.Type != "" {
		return ""
	}
	return RequireVariantForTypedAction(policy.Action, policy.HasTypes, nil)
}

// write-path twin for handlers that already hold a normalized *string variant :3
func RequireVariantForTypedAction(action string, hasTypes bool, variant *string) string {
	if hasTypes && variant == nil {
		return fmt.Sprintf("type is required for action: %s", action)
	}
	return ""
}

func HideVariantIfUntyped(resp *GifResponse, hasTypes bool) {
	if !hasTypes {
		resp.Variant = nil
	}
}

func HideVariantsForUntypedActions(gifStore store.GifStore, gifs []store.Gif) error {
	actionHasTypes := make(map[string]bool)
	for i := range gifs {
		action := NormalizeAction(gifs[i].Action)
		hasTypes, ok := actionHasTypes[action]
		if !ok {
			var err error
			hasTypes, err = gifStore.ActionHasTypes(action)
			if err != nil {
				return err
			}
			actionHasTypes[action] = hasTypes
		}
		if !hasTypes {
			gifs[i].Variant = nil
		}
	}
	return nil
}

func (h *Handler) buildGifResponse(g *store.Gif) GifResponse {
	return BuildGifResponse(g, h.CDNBaseURL)
}
