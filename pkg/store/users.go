package store

import (
	"database/sql"
	"fmt"
)

type ErrProviderMismatch struct {
	ExistingProvider string
}

func (e *ErrProviderMismatch) Error() string {
	return fmt.Sprintf("email already registered with %s", e.ExistingProvider)
}

func (s *SQLiteGifStore) CreateOrGetUser(user *User) (*User, error) {
	// First check by provider + provider_id (exact match)
	existing, err := s.GetUserByProvider(user.Provider, user.ProviderID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// If the email is already taken by a different provider, block and tell the user.
	if user.Email != nil && *user.Email != "" {
		existingByEmail, err := s.GetUserByEmail(*user.Email)
		if err != nil {
			return nil, err
		}
		if existingByEmail != nil {
			if existingByEmail.Provider != user.Provider {
				return nil, &ErrProviderMismatch{ExistingProvider: existingByEmail.Provider}
			}
			return existingByEmail, nil
		}
	}

	const q = `
		INSERT INTO users (provider, provider_id, email, display_name, avatar_url)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, provider, provider_id, email, display_name, avatar_url, created_at
	`
	row := s.db.QueryRow(q, user.Provider, user.ProviderID, user.Email, user.DisplayName, user.AvatarURL)
	return s.scanUser(row)
}

func (s *SQLiteGifStore) GetUserByProvider(provider, providerID string) (*User, error) {
	const q = `SELECT id, provider, provider_id, email, display_name, avatar_url, created_at FROM users WHERE provider = ? AND provider_id = ?`
	return s.scanUser(s.db.QueryRow(q, provider, providerID))
}

func (s *SQLiteGifStore) GetUserByEmail(email string) (*User, error) {
	const q = `SELECT id, provider, provider_id, email, display_name, avatar_url, created_at FROM users WHERE email = ?`
	return s.scanUser(s.db.QueryRow(q, email))
}

func (s *SQLiteGifStore) GetUserByID(id int64) (*User, error) {
	const q = `SELECT id, provider, provider_id, email, display_name, avatar_url, created_at FROM users WHERE id = ?`
	return s.scanUser(s.db.QueryRow(q, id))
}

func (s *SQLiteGifStore) UpdateUserDisplayName(id int64, name string) error {
	const q = `UPDATE users SET display_name = ? WHERE id = ?`
	_, err := s.db.Exec(q, name, id)
	return err
}

func (s *SQLiteGifStore) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	const q = `
		SELECT u.id, u.display_name, u.avatar_url, COUNT(s.id) as count
		FROM users u
		INNER JOIN suggestions s ON s.user_id = u.id AND s.status = 'approved'
		GROUP BY u.id
		HAVING count > 0
		ORDER BY count DESC
		LIMIT ?
	`
	rows, err := s.db.Query(q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.UserID, &e.DisplayName, &e.AvatarURL, &e.Count); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *SQLiteGifStore) scanUser(row *sql.Row) (*User, error) {
	u := &User{}
	err := row.Scan(&u.ID, &u.Provider, &u.ProviderID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}
