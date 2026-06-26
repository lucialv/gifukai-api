package middlewares

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lucialv/gifukai-api/cmd/api/handlers"
	"github.com/lucialv/gifukai-api/pkg/logging"
	u "github.com/lucialv/gifukai-api/pkg/utils"
)

const (
	adminSessionDuration = 24 * time.Hour
	userSessionDuration  = 30 * 24 * time.Hour
)

type adminSession struct{ ExpiresAt time.Time }

type userSession struct {
	UserID    int64
	ExpiresAt time.Time
}

type Auth struct {
	adminKey  string
	adminUser string
	adminPass string

	sessions     sync.Map // admin token -> adminSession
	userSessions sync.Map // user token  -> userSession
}

func NewAuth(adminKey, adminUser, adminPass string) *Auth {
	return &Auth{adminKey: adminKey, adminUser: adminUser, adminPass: adminPass}
}

func (a *Auth) AdminKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.adminKey == "" {
			u.WriteError(w, http.StatusForbidden, "admin endpoints disabled")
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			u.WriteError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token == a.adminKey {
			next.ServeHTTP(w, r)
			return
		}

		if val, ok := a.sessions.Load(token); ok {
			sess := val.(adminSession)
			if time.Now().Before(sess.ExpiresAt) {
				next.ServeHTTP(w, r)
				return
			}
			a.sessions.Delete(token)
		}

		logging.FromContext(r.Context()).Warn("admin auth failed",
			slog.String("component", "auth"),
			slog.String("event", "admin_key_failed"),
		)
		u.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
	})
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) error {
	if a.adminUser == "" || a.adminPass == "" {
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

	if body.User != a.adminUser || body.Pass != a.adminPass {
		logging.FromContext(r.Context()).Warn("admin login failed",
			slog.String("component", "auth"),
			slog.String("event", "admin_login_failed"),
			slog.String("username", body.User),
		)
		u.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return nil
	}

	token, err := randomToken()
	if err != nil {
		return err
	}
	a.sessions.Store(token, adminSession{ExpiresAt: time.Now().Add(adminSessionDuration)})

	return u.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (a *Auth) UserAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			u.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		val, ok := a.userSessions.Load(token)
		if !ok {
			logging.FromContext(r.Context()).Warn("user auth failed",
				slog.String("component", "auth"),
				slog.String("event", "user_auth_failed"),
				slog.String("reason", "invalid_token"),
			)
			u.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		sess := val.(userSession)
		if time.Now().After(sess.ExpiresAt) {
			a.userSessions.Delete(token)
			logging.FromContext(r.Context()).Warn("user auth failed",
				slog.String("component", "auth"),
				slog.String("event", "user_auth_failed"),
				slog.String("reason", "expired"),
				slog.Int64("user_id", sess.UserID),
			)
			u.WriteError(w, http.StatusUnauthorized, "token expired")
			return
		}

		ctx := handlers.SetUserID(r.Context(), sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) GenerateUserToken(userID int64) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	a.userSessions.Store(token, userSession{UserID: userID, ExpiresAt: time.Now().Add(userSessionDuration)})
	return token, nil
}

func (a *Auth) ResolveUser(token string) (int64, bool) {
	val, ok := a.userSessions.Load(token)
	if !ok {
		return 0, false
	}
	sess := val.(userSession)
	if time.Now().After(sess.ExpiresAt) {
		a.userSessions.Delete(token)
		return 0, false
	}
	return sess.UserID, true
}

func (a *Auth) DeleteUserSession(token string) {
	a.userSessions.Delete(token)
}

func randomToken() (string, error) {
	return u.RandomHex(32)
}
