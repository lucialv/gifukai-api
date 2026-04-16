package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/lucialv/gifukai-api/cmd/api/handlers"
	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"
)

type adminGifItem struct {
	ID          int64   `json:"id"`
	Action      string  `json:"action"`
	Pairing     string  `json:"pairing"`
	URL         string  `json:"url"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
	NSFW        bool    `json:"nsfw"`
	Tags        string  `json:"tags,omitempty"`
	AnimeID     *int64  `json:"anime_id,omitempty"`
	AnimeName   *string `json:"anime,omitempty"`
}

func buildAdminGifItems(gifs []store.Gif, cdnBaseURL string) []adminGifItem {
	items := make([]adminGifItem, 0, len(gifs))
	for _, g := range gifs {
		item := adminGifItem{
			ID:          g.ID,
			Action:      g.Action,
			Pairing:     g.Pairing,
			URL:         fmt.Sprintf("%s/%s", cdnBaseURL, g.R2Key),
			Filename:    filepath.Base(g.R2Key),
			ContentType: g.ContentType,
			SizeBytes:   g.SizeBytes,
			NSFW:        g.NSFW,
			AnimeID:     g.AnimeID,
			AnimeName:   g.AnimeName,
		}
		if g.Tags != nil {
			item.Tags = *g.Tags
		}
		items = append(items, item)
	}
	return items
}

const maxUploadSize = 2 << 20 // 2 MB

func (s *APIServer) uploadGifHandler(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
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
	if !handlers.ValidPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s (valid: f, m, ff, mm, fm, mf)", pairing))
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
	if animeIDStr := r.FormValue("anime_id"); animeIDStr != "" {
		if aid, err := strconv.ParseInt(animeIDStr, 10, 64); err == nil {
			gif.AnimeID = &aid
		}
	}

	if err := s.Store.CreateGif(gif); err != nil {
		_ = s.R2Storage.DeleteFile(r2Key)
		return fmt.Errorf("failed to create gif record: %w", err)
	}

	return u.WriteJSON(w, http.StatusCreated, handlers.BuildGifResponse(gif, s.Config.CDNBaseURL))
}

func (s *APIServer) deleteGifHandler(w http.ResponseWriter, r *http.Request) error {
	gif, err := s.requireGif(w, r)
	if gif == nil || err != nil {
		return err
	}

	if err := s.R2Storage.DeleteFile(gif.R2Key); err != nil {
		return fmt.Errorf("failed to delete from R2: %w", err)
	}

	if err := s.Store.DeleteGif(gif.ID); err != nil {
		return fmt.Errorf("failed to delete gif record: %w", err)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *APIServer) listGifsHandler(w http.ResponseWriter, r *http.Request) error {
	action := r.URL.Query().Get("action")
	if action == "" {
		u.WriteError(w, http.StatusBadRequest, "action query param is required")
		return nil
	}

	pairing := r.URL.Query().Get("pairing")
	limit, offset := handlers.ParsePagination(r, 50, 200)

	gifs, err := s.Store.GetGifsByActionAndPairing(action, pairing, limit, offset)
	if err != nil {
		return err
	}

	items := buildAdminGifItems(gifs, s.Config.CDNBaseURL)

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"gifs":  items,
		"count": len(items),
	})
}

func (s *APIServer) listAllGifsHandler(w http.ResponseWriter, r *http.Request) error {
	pairing := r.URL.Query().Get("pairing")
	limit, offset := handlers.ParsePagination(r, 50, 200)

	gifs, err := s.Store.ListAllGifs(pairing, limit, offset)
	if err != nil {
		return err
	}

	total, err := s.Store.CountAllGifs(pairing)
	if err != nil {
		return err
	}

	items := buildAdminGifItems(gifs, s.Config.CDNBaseURL)

	byPairing, err := s.Store.CountGifsByPairing("")
	if err != nil {
		return err
	}
	byPairing = u.OrEmpty(byPairing)

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"gifs":       items,
		"total":      total,
		"by_pairing": byPairing,
	})
}

func (s *APIServer) requireGif(w http.ResponseWriter, r *http.Request) (*store.Gif, error) {
	id, ok := handlers.ParseIDParam(w, r, "gifId", "gif ID")
	if !ok {
		return nil, nil
	}

	gif, err := s.Store.GetGifByID(id)
	if err != nil {
		return nil, err
	}
	if gif == nil {
		u.WriteError(w, http.StatusNotFound, "gif not found")
		return nil, nil
	}
	return gif, nil
}

func (s *APIServer) updateGifTagsHandler(w http.ResponseWriter, r *http.Request) error {
	gif, err := s.requireGif(w, r)
	if gif == nil || err != nil {
		return err
	}

	var body struct {
		Tags string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil
	}

	var tagsPtr *string
	if body.Tags != "" {
		tagsPtr = &body.Tags
	}

	if err := s.Store.UpdateGifTags(gif.ID, tagsPtr); err != nil {
		return fmt.Errorf("failed to update tags: %w", err)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *APIServer) updateGifPairingHandler(w http.ResponseWriter, r *http.Request) error {
	gif, err := s.requireGif(w, r)
	if gif == nil || err != nil {
		return err
	}

	var body struct {
		Pairing string `json:"pairing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil
	}

	if !handlers.ValidPairings[body.Pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s (valid: f, m, ff, mm, fm, mf)", body.Pairing))
		return nil
	}

	if err := s.Store.UpdateGifPairing(gif.ID, body.Pairing); err != nil {
		return fmt.Errorf("failed to update pairing: %w", err)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *APIServer) updateGifAnimeHandler(w http.ResponseWriter, r *http.Request) error {
	gif, err := s.requireGif(w, r)
	if gif == nil || err != nil {
		return err
	}

	var body struct {
		AnimeID *int64 `json:"anime_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil
	}

	if err := s.Store.UpdateGifAnime(gif.ID, body.AnimeID); err != nil {
		return fmt.Errorf("failed to update anime: %w", err)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *APIServer) listAnimesHandler(w http.ResponseWriter, r *http.Request) error {
	animes, err := s.Store.GetAllAnimes()
	if err != nil {
		return err
	}
	animes = u.OrEmpty(animes)
	return u.WriteJSON(w, http.StatusOK, animes)
}

func (s *APIServer) createAnimeHandler(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil
	}
	if body.Name == "" {
		u.WriteError(w, http.StatusBadRequest, "name is required")
		return nil
	}

	anime := &store.Anime{Name: body.Name}
	if err := s.Store.CreateAnime(anime); err != nil {
		return fmt.Errorf("failed to create anime: %w", err)
	}

	return u.WriteJSON(w, http.StatusCreated, anime)
}

func (s *APIServer) updateAnimeHandler(w http.ResponseWriter, r *http.Request) error {
	id, ok := handlers.ParseIDParam(w, r, "animeId", "anime ID")
	if !ok {
		return nil
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil
	}
	if body.Name == "" {
		u.WriteError(w, http.StatusBadRequest, "name is required")
		return nil
	}

	if err := s.Store.UpdateAnime(id, body.Name); err != nil {
		return fmt.Errorf("failed to update anime: %w", err)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *APIServer) deleteAnimeHandler(w http.ResponseWriter, r *http.Request) error {
	id, ok := handlers.ParseIDParam(w, r, "animeId", "anime ID")
	if !ok {
		return nil
	}

	if err := s.Store.DeleteAnime(id); err != nil {
		return fmt.Errorf("failed to delete anime: %w", err)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
