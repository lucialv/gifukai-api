package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/lucialv/gifukai-api/cmd/api/handlers"
	u "github.com/lucialv/gifukai-api/pkg/utils"
)

// ~~ admin auth ~~

type session struct {
	ExpiresAt time.Time
}

func (s *APIServer) AdminKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Config.AdminKey == "" {
			u.WriteError(w, http.StatusForbidden, "admin endpoints disabled")
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			u.WriteError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")

		if token == s.Config.AdminKey {
			next.ServeHTTP(w, r)
			return
		}

		if val, ok := s.sessions.Load(token); ok {
			sess := val.(session)
			if time.Now().Before(sess.ExpiresAt) {
				next.ServeHTTP(w, r)
				return
			}
			s.sessions.Delete(token)
		}

		u.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
	})
}

func (s *APIServer) loginHandler(w http.ResponseWriter, r *http.Request) error {
	if s.Config.AdminUser == "" || s.Config.AdminPass == "" {
		u.WriteError(w, http.StatusForbidden, "admin login disabled")
		return nil
	}

	var body struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		u.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return nil
	}

	if body.User != s.Config.AdminUser || body.Pass != s.Config.AdminPass {
		u.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return nil
	}

	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	token := hex.EncodeToString(bytes)

	s.sessions.Store(token, session{
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})

	return u.WriteJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

// ~~ user auth ~~

type userSession struct {
	UserID    int64
	ExpiresAt time.Time
}

func (s *APIServer) UserAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			u.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")

		val, ok := s.userSessions.Load(token)
		if !ok {
			u.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		sess := val.(userSession)
		if time.Now().After(sess.ExpiresAt) {
			s.userSessions.Delete(token)
			u.WriteError(w, http.StatusUnauthorized, "token expired")
			return
		}

		ctx := handlers.SetUserID(r.Context(), sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
