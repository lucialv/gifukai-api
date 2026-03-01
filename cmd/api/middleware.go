package api

import (
	"net/http"
	"strings"

	u "github.com/lucialv/anime-api-cdn/pkg/utils"
)

// AdminKeyMiddleware checks for a valid admin API key in the Authorization header.
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

		key := strings.TrimPrefix(auth, "Bearer ")
		if key != s.Config.AdminKey {
			u.WriteError(w, http.StatusUnauthorized, "invalid admin API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}
