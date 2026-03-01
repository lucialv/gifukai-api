package api

import (
	"log"
	"net/http"

	u "github.com/lucialv/anime-api-cdn/pkg/utils"

	"github.com/go-chi/chi/v5"
)

// apiFunc is a handler that returns an error.
type apiFunc func(http.ResponseWriter, *http.Request) error

// makeHTTPHandleFunc wraps an apiFunc into an http.HandlerFunc.
func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			log.Printf("Handler error: %v", err)
			u.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
	}
}

// Routes returns the chi router with all API routes.
func (s *APIServer) Routes() *chi.Mux {
	r := chi.NewRouter()

	r.Get("/healthz", makeHTTPHandleFunc(s.healthzHandler))

	r.Get("/actions", makeHTTPHandleFunc(s.listActionsHandler))
	r.Get("/actions/{action}/count", makeHTTPHandleFunc(s.countGifsHandler))
	r.Get("/{action}", makeHTTPHandleFunc(s.getRandomGifHandler))

	r.Route("/admin", func(r chi.Router) {
		r.Use(s.AdminKeyMiddleware)
		r.Post("/gifs", makeHTTPHandleFunc(s.uploadGifHandler))
		r.Delete("/gifs/{gifId}", makeHTTPHandleFunc(s.deleteGifHandler))
		r.Get("/gifs", makeHTTPHandleFunc(s.listGifsHandler))
	})

	return r
}
