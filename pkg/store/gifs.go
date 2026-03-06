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
	CreatedAt   time.Time `json:"created_at"`
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
	RefreshIDPool(action, pairing string) ([]int64, error)
	UpdateGifTags(id int64, tags *string) error
	UpdateGifPairing(id int64, pairing string) error
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
		SELECT id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, created_at
		FROM gifs
		WHERE action = ? AND pairing = ?`
	selectArgs := []any{action, pairing}

	if nsfw != nil {
		selectQuery += ` AND nsfw = ?`
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
		SELECT id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, created_at
		FROM gifs
		WHERE action = ?`
	selectArgs := []any{action}

	if nsfw != nil {
		selectQuery += ` AND nsfw = ?`
		selectArgs = append(selectArgs, *nsfw)
	}

	selectQuery += ` LIMIT 1 OFFSET (ABS(RANDOM()) % ?)`
	selectArgs = append(selectArgs, count)

	return s.scanGif(s.db.QueryRow(selectQuery, selectArgs...))
}

// GetGifsByAction returns all GIFs for a given action.
func (s *SQLiteGifStore) GetGifsByAction(action string, limit, offset int) ([]Gif, error) {
	const q = `
		SELECT id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, created_at
		FROM gifs
		WHERE action = ?
		ORDER BY created_at DESC
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
		INSERT INTO gifs (action, pairing, r2_key, content_type, size_bytes, tags, nsfw, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	return s.db.QueryRow(
		q,
		gif.Action, gif.Pairing, gif.R2Key, gif.ContentType, gif.SizeBytes,
		gif.Tags, gif.NSFW, gif.CreatedAt,
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
		SELECT id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, created_at
		FROM gifs WHERE id = ?
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
		&g.Tags, &g.NSFW, &g.CreatedAt,
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
		&g.Tags, &g.NSFW, &g.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// GetGifsByActionAndPairing returns GIFs for a given action, optionally filtered by pairing.
func (s *SQLiteGifStore) GetGifsByActionAndPairing(action, pairing string, limit, offset int) ([]Gif, error) {
	q := `
		SELECT id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, created_at
		FROM gifs
		WHERE action = ?`
	args := []any{action}

	if pairing != "" {
		q += ` AND pairing = ?`
		args = append(args, pairing)
	}

	q += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
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