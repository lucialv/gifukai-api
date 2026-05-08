package store

import (
	"database/sql"
	"strings"
	"time"
)

type Gif struct {
	ID          int64     `json:"id"`
	Action      string    `json:"action"`
	Variant     *string   `json:"type,omitempty"`
	Pairing     string    `json:"pairing"`
	R2Key       string    `json:"r2_key"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Tags        *string   `json:"tags,omitempty"`
	NSFW        bool      `json:"nsfw"`
	AnimeID     *int64    `json:"anime_id,omitempty"`
	AnimeName   *string   `json:"anime,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Anime struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	GifCount int    `json:"gif_count"`
}

type PairingCount struct {
	Pairing string `json:"pairing"`
	Count   int    `json:"count"`
}

type ActionPairingCount struct {
	Action  string `json:"action"`
	Pairing string `json:"pairing"`
	Count   int    `json:"count"`
}

type Stats struct {
	TotalGifs     int            `json:"total_gifs"`
	TotalActions  int            `json:"total_actions"`
	TotalAnimes   int            `json:"total_animes"`
	TotalBytes    int64          `json:"total_bytes"`
	GifsByPairing []PairingCount `json:"gifs_by_pairing"`
}

type User struct {
	ID          int64     `json:"id"`
	Provider    string    `json:"provider"`
	ProviderID  string    `json:"provider_id"`
	Email       *string   `json:"email,omitempty"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type LeaderboardEntry struct {
	UserID      int64   `json:"user_id"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	Count       int     `json:"count"`
}

type Report struct {
	ID        int64     `json:"id"`
	GifID     *int64    `json:"gif_id,omitempty"`
	UserID    int64     `json:"user_id"`
	Reason    string    `json:"reason"`
	Details   *string   `json:"details,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UserName  *string   `json:"user_name,omitempty"`  // joined
	GifURL    *string   `json:"gif_url,omitempty"`    // joined
	GifAction *string   `json:"gif_action,omitempty"` // joined
}

type Suggestion struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	FileKey     string    `json:"file_key"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Action      string    `json:"action"`
	Variant     *string   `json:"type,omitempty"`
	Pairing     string    `json:"pairing"`
	Anime       *string   `json:"anime,omitempty"`
	Tags        *string   `json:"tags,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UserName    *string   `json:"user_name,omitempty"` // joined
}

type GifStore interface {
	// ~~ gifs ~~
	GetRandomGif(action, pairing, variant string, nsfw *bool) (*Gif, error)
	GetRandomGifAnyPairing(action, variant string, nsfw *bool) (*Gif, error)
	GetGifsByActionAndPairing(action, pairing, variant string, limit, offset int) ([]Gif, error)
	GetAllActions() ([]string, error)
	ActionHasTypes(action string) (bool, error)
	GetActionTypes(action string) ([]string, bool, error)
	ListActionVariants(action string) ([]string, error)
	CreateGif(gif *Gif) error
	DeleteGif(id int64) error
	GetGifByID(id int64) (*Gif, error)
	CountGifs(action, pairing, variant string) (int, error)
	CountGifsByPairing(action, variant string) ([]PairingCount, error)
	GetActionPairingCounts() ([]ActionPairingCount, error)
	UpdateGifTags(id int64, tags *string) error
	UpdateGifPairing(id int64, pairing string) error
	UpdateGifVariant(id int64, variant *string) error
	UpdateGifAnime(gifID int64, animeID *int64) error
	ListAllGifs(pairing string, limit, offset int) ([]Gif, error)
	CountAllGifs(pairing string) (int, error)
	GetStats() (*Stats, error)

	// ~~ animes ~~
	CreateAnime(anime *Anime) error
	GetAllAnimes() ([]Anime, error)
	UpdateAnime(id int64, name string) error
	DeleteAnime(id int64) error

	// ~~ public library ~~
	ListPublicGifs(action, pairing, anime, variant string, limit, offset int) ([]Gif, int, error)
	ListPublicAnimes() ([]string, error)

	// ~~ users ~~
	CreateOrGetUser(user *User) (*User, error)
	GetUserByID(id int64) (*User, error)
	UpdateUserDisplayName(id int64, name string) error
	GetLeaderboard(limit int) ([]LeaderboardEntry, error)

	// ~~ reports ~~
	CreateReport(report *Report) error
	ListReports(status string, limit, offset int) ([]Report, int, error)
	UpdateReportStatus(id int64, status string) error
	HasUserReportedGif(userID, gifID int64) (bool, error)

	// ~~ suggestions ~~
	CreateSuggestion(suggestion *Suggestion) error
	ListSuggestions(status string, limit, offset int) ([]Suggestion, int, error)
	GetSuggestionByID(id int64) (*Suggestion, error)
	UpdateSuggestionStatus(id int64, status string) error
	UpdateSuggestionApproved(id int64, newFileKey string) error
	CountUserSuggestionsToday(userID int64) (int, error)
	ListUserSuggestions(userID int64, limit, offset int) ([]Suggestion, error)
}

type SQLiteGifStore struct {
	db *sql.DB
}

func NewGifStore(dbURL, authToken string) (*SQLiteGifStore, error) {
	url := dbURL
	if authToken != "" {
		url = dbURL + "?authToken=" + authToken
	}

	db, err := sql.Open("libsql", url)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &SQLiteGifStore{db: db}, nil
}

func (s *SQLiteGifStore) GetRandomGif(action, pairing, variant string, nsfw *bool) (*Gif, error) {
	countQuery := `SELECT COUNT(*) FROM gifs WHERE action = ?`
	args := []any{action}

	if variant != "" {
		countQuery += ` AND variant = ?`
		args = append(args, variant)
	}
	if pairing != "" {
		countQuery += ` AND pairing = ?`
		args = append(args, pairing)
	}
	if nsfw != nil {
		countQuery += ` AND nsfw = ?`
		args = append(args, *nsfw)
	}

	var count int
	if err := s.db.QueryRow(countQuery, args...).Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	selectQuery := `
		SELECT g.id, g.action, g.variant, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE g.action = ?`
	selectArgs := []any{action}

	if variant != "" {
		selectQuery += ` AND g.variant = ?`
		selectArgs = append(selectArgs, variant)
	}
	if pairing != "" {
		selectQuery += ` AND g.pairing = ?`
		selectArgs = append(selectArgs, pairing)
	}
	if nsfw != nil {
		selectQuery += ` AND g.nsfw = ?`
		selectArgs = append(selectArgs, *nsfw)
	}

	selectQuery += ` LIMIT 1 OFFSET (ABS(RANDOM()) % ?)`
	selectArgs = append(selectArgs, count)

	return s.scanGif(s.db.QueryRow(selectQuery, selectArgs...))
}

func (s *SQLiteGifStore) GetRandomGifAnyPairing(action, variant string, nsfw *bool) (*Gif, error) {
	return s.GetRandomGif(action, "", variant, nsfw)
}

func (s *SQLiteGifStore) GetAllActions() ([]string, error) {
	const q = `SELECT DISTINCT action FROM gifs ORDER BY action`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return actions, nil
}

func (s *SQLiteGifStore) ActionHasTypes(action string) (bool, error) {
	const q = `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN variant IS NOT NULL AND variant <> '' THEN 1 ELSE 0 END), 0)
		FROM gifs
		WHERE action = ?
	`
	var total, typed int
	if err := s.db.QueryRow(q, action).Scan(&total, &typed); err != nil {
		return false, err
	}
	if total == 0 {
		return false, nil
	}
	return total == typed, nil
}

func (s *SQLiteGifStore) GetActionTypes(action string) ([]string, bool, error) {
	const q = `
		SELECT COALESCE(variant, '')
		FROM gifs
		WHERE action = ?
		GROUP BY COALESCE(variant, '')
		ORDER BY COALESCE(variant, '')
	`
	rows, err := s.db.Query(q, action)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	types := make([]string, 0)
	hasUntyped := false
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, false, err
		}
		if v == "" {
			hasUntyped = true
			continue
		}
		types = append(types, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if hasUntyped || len(types) == 0 {
		return []string{}, false, nil
	}
	return types, true, nil
}

func (s *SQLiteGifStore) ListActionVariants(action string) ([]string, error) {
	const q = `
		SELECT DISTINCT variant
		FROM gifs
		WHERE action = ? AND variant IS NOT NULL AND variant <> ''
		ORDER BY variant
	`
	rows, err := s.db.Query(q, action)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]string, 0)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return variants, nil
}

func (s *SQLiteGifStore) CreateGif(gif *Gif) error {
	const q = `
		INSERT INTO gifs (action, variant, pairing, r2_key, content_type, size_bytes, tags, nsfw, anime_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	return s.db.QueryRow(
		q,
		gif.Action, gif.Variant, gif.Pairing, gif.R2Key, gif.ContentType, gif.SizeBytes,
		gif.Tags, gif.NSFW, gif.AnimeID, gif.CreatedAt,
	).Scan(&gif.ID)
}

func (s *SQLiteGifStore) DeleteGif(id int64) error {
	const q = `DELETE FROM gifs WHERE id = ?`
	_, err := s.db.Exec(q, id)
	return err
}

func (s *SQLiteGifStore) GetGifByID(id int64) (*Gif, error) {
	const q = `
		SELECT g.id, g.action, g.variant, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE g.id = ?
	`
	return s.scanGif(s.db.QueryRow(q, id))
}

func (s *SQLiteGifStore) CountGifs(action, pairing, variant string) (int, error) {
	q := `SELECT COUNT(*) FROM gifs WHERE 1=1`
	var args []any

	if action != "" {
		q += ` AND action = ?`
		args = append(args, action)
	}
	if variant != "" {
		q += ` AND variant = ?`
		args = append(args, variant)
	}
	if pairing != "" {
		q += ` AND pairing = ?`
		args = append(args, pairing)
	}

	var count int
	err := s.db.QueryRow(q, args...).Scan(&count)
	return count, err
}

func (s *SQLiteGifStore) CountGifsByPairing(action, variant string) ([]PairingCount, error) {
	q := `SELECT pairing, COUNT(*) FROM gifs`
	var args []any
	var where []string

	if action != "" {
		where = append(where, `action = ?`)
		args = append(args, action)
	}
	if variant != "" {
		where = append(where, `variant = ?`)
		args = append(args, variant)
	}
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, ` AND `)
	}

	q += ` GROUP BY pairing ORDER BY pairing`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []PairingCount
	for rows.Next() {
		var pc PairingCount
		if err := rows.Scan(&pc.Pairing, &pc.Count); err != nil {
			return nil, err
		}
		counts = append(counts, pc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

func (s *SQLiteGifStore) GetActionPairingCounts() ([]ActionPairingCount, error) {
	const q = `
		SELECT action, pairing, COUNT(*)
		FROM gifs
		GROUP BY action, pairing
		ORDER BY action, pairing
	`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]ActionPairingCount, 0)
	for rows.Next() {
		var row ActionPairingCount
		if err := rows.Scan(&row.Action, &row.Pairing, &row.Count); err != nil {
			return nil, err
		}
		counts = append(counts, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return counts, nil
}

func (s *SQLiteGifStore) scanGif(row *sql.Row) (*Gif, error) {
	g := &Gif{}
	err := row.Scan(
		&g.ID, &g.Action, &g.Variant, &g.Pairing, &g.R2Key, &g.ContentType, &g.SizeBytes,
		&g.Tags, &g.NSFW, &g.AnimeID, &g.AnimeName, &g.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (s *SQLiteGifStore) scanGifRow(rows *sql.Rows) (*Gif, error) {
	g := &Gif{}
	err := rows.Scan(
		&g.ID, &g.Action, &g.Variant, &g.Pairing, &g.R2Key, &g.ContentType, &g.SizeBytes,
		&g.Tags, &g.NSFW, &g.AnimeID, &g.AnimeName, &g.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (s *SQLiteGifStore) GetGifsByActionAndPairing(action, pairing, variant string, limit, offset int) ([]Gif, error) {
	q := `
		SELECT g.id, g.action, g.variant, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE g.action = ?`
	args := []any{action}

	if variant != "" {
		q += ` AND g.variant = ?`
		args = append(args, variant)
	}
	if pairing != "" {
		q += ` AND g.pairing = ?`
		args = append(args, pairing)
	}

	q += ` ORDER BY g.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gifs []Gif
	for rows.Next() {
		g, err := s.scanGifRow(rows)
		if err != nil {
			return nil, err
		}
		gifs = append(gifs, *g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return gifs, nil
}

func (s *SQLiteGifStore) UpdateGifTags(id int64, tags *string) error {
	const q = `UPDATE gifs SET tags = ? WHERE id = ?`
	_, err := s.db.Exec(q, tags, id)
	return err
}

func (s *SQLiteGifStore) UpdateGifPairing(id int64, pairing string) error {
	const q = `UPDATE gifs SET pairing = ? WHERE id = ?`
	_, err := s.db.Exec(q, pairing, id)
	return err
}

func (s *SQLiteGifStore) UpdateGifVariant(id int64, variant *string) error {
	const q = `UPDATE gifs SET variant = ? WHERE id = ?`
	_, err := s.db.Exec(q, variant, id)
	return err
}

func (s *SQLiteGifStore) ListAllGifs(pairing string, limit, offset int) ([]Gif, error) {
	q := `
		SELECT g.id, g.action, g.variant, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE 1=1`
	var args []any

	if pairing != "" {
		q += ` AND g.pairing = ?`
		args = append(args, pairing)
	}

	q += ` ORDER BY g.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gifs []Gif
	for rows.Next() {
		g, err := s.scanGifRow(rows)
		if err != nil {
			return nil, err
		}
		gifs = append(gifs, *g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return gifs, nil
}

func (s *SQLiteGifStore) CountAllGifs(pairing string) (int, error) {
	q := `SELECT COUNT(*) FROM gifs WHERE 1=1`
	var args []any

	if pairing != "" {
		q += ` AND pairing = ?`
		args = append(args, pairing)
	}

	var count int
	err := s.db.QueryRow(q, args...).Scan(&count)
	return count, err
}

func (s *SQLiteGifStore) UpdateGifAnime(gifID int64, animeID *int64) error {
	const q = `UPDATE gifs SET anime_id = ? WHERE id = ?`
	_, err := s.db.Exec(q, animeID, gifID)
	return err
}

func (s *SQLiteGifStore) CreateAnime(anime *Anime) error {
	const q = `INSERT INTO animes (name) VALUES (?) RETURNING id`
	return s.db.QueryRow(q, anime.Name).Scan(&anime.ID)
}

func (s *SQLiteGifStore) GetAllAnimes() ([]Anime, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.name, COUNT(g.id) AS gif_count
		FROM animes a
		LEFT JOIN gifs g ON g.anime_id = a.id
		GROUP BY a.id, a.name
		ORDER BY a.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var animes []Anime
	for rows.Next() {
		var a Anime
		if err := rows.Scan(&a.ID, &a.Name, &a.GifCount); err != nil {
			return nil, err
		}
		animes = append(animes, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return animes, nil
}

func (s *SQLiteGifStore) UpdateAnime(id int64, name string) error {
	const q = `UPDATE animes SET name = ? WHERE id = ?`
	_, err := s.db.Exec(q, name, id)
	return err
}

func (s *SQLiteGifStore) DeleteAnime(id int64) error {
	const q = `DELETE FROM animes WHERE id = ?`
	_, err := s.db.Exec(q, id)
	return err
}

func (s *SQLiteGifStore) ListPublicGifs(action, pairing, anime, variant string, limit, offset int) ([]Gif, int, error) {
	where := `WHERE g.nsfw = 0`
	var args []any

	if action != "" {
		where += ` AND g.action = ?`
		args = append(args, action)
	}
	if variant != "" {
		where += ` AND g.variant = ?`
		args = append(args, variant)
	}
	if pairing != "" {
		where += ` AND g.pairing = ?`
		args = append(args, pairing)
	}
	if anime != "" {
		where += ` AND a.name = ?`
		args = append(args, anime)
	}

	var total int
	countQ := `SELECT COUNT(*) FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id ` + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `SELECT g.id, g.action, g.variant, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id ` + where + ` ORDER BY g.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var gifs []Gif
	for rows.Next() {
		g, err := s.scanGifRow(rows)
		if err != nil {
			return nil, 0, err
		}
		gifs = append(gifs, *g)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return gifs, total, nil
}

func (s *SQLiteGifStore) ListPublicAnimes() ([]string, error) {
	const q = `SELECT DISTINCT a.name FROM animes a INNER JOIN gifs g ON g.anime_id = a.id WHERE g.nsfw = 0 ORDER BY a.name`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func (s *SQLiteGifStore) GetStats() (*Stats, error) {
	var stats Stats

	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size_bytes), 0) FROM gifs`).
		Scan(&stats.TotalGifs, &stats.TotalBytes)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRow(`SELECT COUNT(DISTINCT action) FROM gifs`).Scan(&stats.TotalActions)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM animes`).Scan(&stats.TotalAnimes)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`SELECT pairing, COUNT(*) FROM gifs GROUP BY pairing ORDER BY pairing`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var pc PairingCount
		if err := rows.Scan(&pc.Pairing, &pc.Count); err != nil {
			return nil, err
		}
		stats.GifsByPairing = append(stats.GifsByPairing, pc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &stats, nil
}
