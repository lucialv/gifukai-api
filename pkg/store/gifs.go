package store

import (
	"database/sql"
	"time"
)

type Gif struct {
	ID          int64     `json:"id"`
	Action      string    `json:"action"`
	Pairing     string    `json:"pairing"`
	R2Key       string    `json:"r2_key"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Tags        *string   `json:"tags,omitempty"`
	NSFW        bool      `json:"nsfw"`
	AnimeID     *int64    `json:"anime_id,omitempty"`
	AnimeName   *string   `json:"anime_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Anime struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type PairingCount struct {
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

type GifStore interface {
	GetRandomGif(action, pairing string, nsfw *bool) (*Gif, error)
	GetRandomGifAnyPairing(action string, nsfw *bool) (*Gif, error)
	GetGifsByAction(action string, limit, offset int) ([]Gif, error)
	GetGifsByActionAndPairing(action, pairing string, limit, offset int) ([]Gif, error)
	GetAllActions() ([]string, error)
	CreateGif(gif *Gif) error
	DeleteGif(id int64) error
	GetGifByID(id int64) (*Gif, error)
	CountGifs(action, pairing string) (int, error)
	CountGifsByPairing(action string) ([]PairingCount, error)
	RefreshIDPool(action, pairing string) ([]int64, error)
	UpdateGifTags(id int64, tags *string) error
	UpdateGifPairing(id int64, pairing string) error
	UpdateGifAnime(gifID int64, animeID *int64) error
	ListAllGifs(pairing string, limit, offset int) ([]Gif, error)
	CountAllGifs(pairing string) (int, error)
	GetStats() (*Stats, error)
	CreateAnime(anime *Anime) error
	GetAllAnimes() ([]Anime, error)
	UpdateAnime(id int64, name string) error
	DeleteAnime(id int64) error
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

// GetRandomGif selects a random GIF matching action + pairing.
func (s *SQLiteGifStore) GetRandomGif(action, pairing string, nsfw *bool) (*Gif, error) {
	countQuery := `SELECT COUNT(*) FROM gifs WHERE action = ? AND pairing = ?`
	args := []any{action, pairing}

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
		SELECT g.id, g.action, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE g.action = ? AND g.pairing = ?`
	selectArgs := []any{action, pairing}

	if nsfw != nil {
		selectQuery += ` AND g.nsfw = ?`
		selectArgs = append(selectArgs, *nsfw)
	}

	selectQuery += ` LIMIT 1 OFFSET (ABS(RANDOM()) % ?)`
	selectArgs = append(selectArgs, count)

	return s.scanGif(s.db.QueryRow(selectQuery, selectArgs...))
}

// GetRandomGifAnyPairing selects a random GIF matching action (any pairing).
func (s *SQLiteGifStore) GetRandomGifAnyPairing(action string, nsfw *bool) (*Gif, error) {
	countQuery := `SELECT COUNT(*) FROM gifs WHERE action = ?`
	args := []any{action}

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
		SELECT g.id, g.action, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE g.action = ?`
	selectArgs := []any{action}

	if nsfw != nil {
		selectQuery += ` AND g.nsfw = ?`
		selectArgs = append(selectArgs, *nsfw)
	}

	selectQuery += ` LIMIT 1 OFFSET (ABS(RANDOM()) % ?)`
	selectArgs = append(selectArgs, count)

	return s.scanGif(s.db.QueryRow(selectQuery, selectArgs...))
}

// GetGifsByAction returns all GIFs for a given action.
func (s *SQLiteGifStore) GetGifsByAction(action string, limit, offset int) ([]Gif, error) {
	const q = `
		SELECT g.id, g.action, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE g.action = ?
		ORDER BY g.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(q, action, limit, offset)
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
	return gifs, nil
}

// GetAllActions returns a list of distinct actions.
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
	return actions, nil
}

// CreateGif inserts a new GIF record.
func (s *SQLiteGifStore) CreateGif(gif *Gif) error {
	const q = `
		INSERT INTO gifs (action, pairing, r2_key, content_type, size_bytes, tags, nsfw, anime_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	return s.db.QueryRow(
		q,
		gif.Action, gif.Pairing, gif.R2Key, gif.ContentType, gif.SizeBytes,
		gif.Tags, gif.NSFW, gif.AnimeID, gif.CreatedAt,
	).Scan(&gif.ID)
}

// DeleteGif removes a GIF record by ID.
func (s *SQLiteGifStore) DeleteGif(id int64) error {
	const q = `DELETE FROM gifs WHERE id = ?`
	_, err := s.db.Exec(q, id)
	return err
}

// GetGifByID returns a single GIF by ID.
func (s *SQLiteGifStore) GetGifByID(id int64) (*Gif, error) {
	const q = `
		SELECT g.id, g.action, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE g.id = ?
	`
	return s.scanGif(s.db.QueryRow(q, id))
}

// CountGifs returns the number of GIFs matching action + pairing.
func (s *SQLiteGifStore) CountGifs(action, pairing string) (int, error) {
	q := `SELECT COUNT(*) FROM gifs WHERE 1=1`
	var args []any

	if action != "" {
		q += ` AND action = ?`
		args = append(args, action)
	}
	if pairing != "" {
		q += ` AND pairing = ?`
		args = append(args, pairing)
	}

	var count int
	err := s.db.QueryRow(q, args...).Scan(&count)
	return count, err
}

func (s *SQLiteGifStore) CountGifsByPairing(action string) ([]PairingCount, error) {
	q := `SELECT pairing, COUNT(*) FROM gifs`
	var args []any

	if action != "" {
		q += ` WHERE action = ?`
		args = append(args, action)
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
	return counts, nil
}

// RefreshIDPool returns all IDs for a given action+pairing (for in-memory caching).
func (s *SQLiteGifStore) RefreshIDPool(action, pairing string) ([]int64, error) {
	const q = `SELECT id FROM gifs WHERE action = ? AND pairing = ?`
	rows, err := s.db.Query(q, action, pairing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *SQLiteGifStore) scanGif(row *sql.Row) (*Gif, error) {
	g := &Gif{}
	err := row.Scan(
		&g.ID, &g.Action, &g.Pairing, &g.R2Key, &g.ContentType, &g.SizeBytes,
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
		&g.ID, &g.Action, &g.Pairing, &g.R2Key, &g.ContentType, &g.SizeBytes,
		&g.Tags, &g.NSFW, &g.AnimeID, &g.AnimeName, &g.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// GetGifsByActionAndPairing returns GIFs for a given action, optionally filtered by pairing.
func (s *SQLiteGifStore) GetGifsByActionAndPairing(action, pairing string, limit, offset int) ([]Gif, error) {
	q := `
		SELECT g.id, g.action, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
		FROM gifs g LEFT JOIN animes a ON g.anime_id = a.id
		WHERE g.action = ?`
	args := []any{action}

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
	return gifs, nil
}

// UpdateGifTags updates the tags field for a GIF.
func (s *SQLiteGifStore) UpdateGifTags(id int64, tags *string) error {
	const q = `UPDATE gifs SET tags = ? WHERE id = ?`
	_, err := s.db.Exec(q, tags, id)
	return err
}

// UpdateGifPairing updates the pairing field for a GIF.
func (s *SQLiteGifStore) UpdateGifPairing(id int64, pairing string) error {
	const q = `UPDATE gifs SET pairing = ? WHERE id = ?`
	_, err := s.db.Exec(q, pairing, id)
	return err
}

func (s *SQLiteGifStore) ListAllGifs(pairing string, limit, offset int) ([]Gif, error) {
	q := `
		SELECT g.id, g.action, g.pairing, g.r2_key, g.content_type, g.size_bytes, g.tags, g.nsfw, g.anime_id, a.name, g.created_at
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
	rows, err := s.db.Query(`SELECT id, name FROM animes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var animes []Anime
	for rows.Next() {
		var a Anime
		if err := rows.Scan(&a.ID, &a.Name); err != nil {
			return nil, err
		}
		animes = append(animes, a)
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

	return &stats, nil
}