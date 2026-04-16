package store

func (s *SQLiteGifStore) CreateReport(report *Report) error {
	const q = `
		INSERT INTO reports (gif_id, user_id, reason, details)
		VALUES (?, ?, ?, ?)
		RETURNING id, created_at
	`
	return s.db.QueryRow(q, report.GifID, report.UserID, report.Reason, report.Details).
		Scan(&report.ID, &report.CreatedAt)
}

func (s *SQLiteGifStore) ListReports(status string, limit, offset int) ([]Report, int, error) {
	where := `WHERE 1=1`
	var args []any
	if status != "" {
		where += ` AND r.status = ?`
		args = append(args, status)
	}

	var total int
	countQ := `SELECT COUNT(*) FROM reports r ` + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `
		SELECT r.id, r.gif_id, r.user_id, r.reason, r.details, r.status, r.created_at,
			u.display_name, g.r2_key, g.action
		FROM reports r
		LEFT JOIN users u ON r.user_id = u.id
		LEFT JOIN gifs g ON r.gif_id = g.id
		` + where + ` ORDER BY r.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []Report
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.GifID, &r.UserID, &r.Reason, &r.Details, &r.Status, &r.CreatedAt,
			&r.UserName, &r.GifURL, &r.GifAction); err != nil {
			return nil, 0, err
		}
		reports = append(reports, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return reports, total, nil
}

func (s *SQLiteGifStore) UpdateReportStatus(id int64, status string) error {
	const q = `UPDATE reports SET status = ? WHERE id = ?`
	_, err := s.db.Exec(q, status, id)
	return err
}

func (s *SQLiteGifStore) HasUserReportedGif(userID, gifID int64) (bool, error) {
	const q = `SELECT COUNT(*) FROM reports WHERE user_id = ? AND gif_id = ?`
	var count int
	err := s.db.QueryRow(q, userID, gifID).Scan(&count)
	return count > 0, err
}
