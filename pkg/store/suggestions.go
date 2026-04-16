package store

import "database/sql"

func (s *SQLiteGifStore) CreateSuggestion(suggestion *Suggestion) error {
	const q = `
		INSERT INTO suggestions (user_id, file_key, content_type, size_bytes, action, pairing, anime, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at
	`
	return s.db.QueryRow(q,
		suggestion.UserID, suggestion.FileKey, suggestion.ContentType, suggestion.SizeBytes,
		suggestion.Action, suggestion.Pairing, suggestion.Anime, suggestion.Tags,
	).Scan(&suggestion.ID, &suggestion.CreatedAt)
}

func (s *SQLiteGifStore) ListSuggestions(status string, limit, offset int) ([]Suggestion, int, error) {
	where := `WHERE 1=1`
	var args []any
	if status != "" {
		where += ` AND s.status = ?`
		args = append(args, status)
	}

	var total int
	countQ := `SELECT COUNT(*) FROM suggestions s ` + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `
		SELECT s.id, s.user_id, s.file_key, s.content_type, s.size_bytes,
			s.action, s.pairing, s.anime, s.tags, s.status, s.created_at,
			u.display_name
		FROM suggestions s
		LEFT JOIN users u ON s.user_id = u.id
		` + where + ` ORDER BY s.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var suggestions []Suggestion
	for rows.Next() {
		sg, err := s.scanSuggestionRow(rows)
		if err != nil {
			return nil, 0, err
		}
		suggestions = append(suggestions, *sg)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return suggestions, total, nil
}

func (s *SQLiteGifStore) GetSuggestionByID(id int64) (*Suggestion, error) {
	const q = `
		SELECT s.id, s.user_id, s.file_key, s.content_type, s.size_bytes,
			s.action, s.pairing, s.anime, s.tags, s.status, s.created_at,
			u.display_name
		FROM suggestions s
		LEFT JOIN users u ON s.user_id = u.id
		WHERE s.id = ?
	`
	row := s.db.QueryRow(q, id)
	sg := &Suggestion{}
	err := row.Scan(&sg.ID, &sg.UserID, &sg.FileKey, &sg.ContentType, &sg.SizeBytes,
		&sg.Action, &sg.Pairing, &sg.Anime, &sg.Tags, &sg.Status, &sg.CreatedAt,
		&sg.UserName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sg, nil
}

func (s *SQLiteGifStore) UpdateSuggestionStatus(id int64, status string) error {
	const q = `UPDATE suggestions SET status = ? WHERE id = ?`
	_, err := s.db.Exec(q, status, id)
	return err
}

func (s *SQLiteGifStore) UpdateSuggestionApproved(id int64, newFileKey string) error {
	const q = `UPDATE suggestions SET status = 'approved', file_key = ? WHERE id = ?`
	_, err := s.db.Exec(q, newFileKey, id)
	return err
}

func (s *SQLiteGifStore) CountUserSuggestionsToday(userID int64) (int, error) {
	const q = `SELECT COUNT(*) FROM suggestions WHERE user_id = ? AND created_at >= date('now')`
	var count int
	err := s.db.QueryRow(q, userID).Scan(&count)
	return count, err
}

func (s *SQLiteGifStore) ListUserSuggestions(userID int64, limit, offset int) ([]Suggestion, error) {
	const q = `
		SELECT s.id, s.user_id, s.file_key, s.content_type, s.size_bytes,
			s.action, s.pairing, s.anime, s.tags, s.status, s.created_at,
			u.display_name
		FROM suggestions s
		LEFT JOIN users u ON s.user_id = u.id
		WHERE s.user_id = ?
		ORDER BY s.created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var suggestions []Suggestion
	for rows.Next() {
		sg, err := s.scanSuggestionRow(rows)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, *sg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return suggestions, nil
}

func (s *SQLiteGifStore) scanSuggestionRow(rows *sql.Rows) (*Suggestion, error) {
	sg := &Suggestion{}
	err := rows.Scan(&sg.ID, &sg.UserID, &sg.FileKey, &sg.ContentType, &sg.SizeBytes,
		&sg.Action, &sg.Pairing, &sg.Anime, &sg.Tags, &sg.Status, &sg.CreatedAt,
		&sg.UserName)
	if err != nil {
		return nil, err
	}
	return sg, nil
}
