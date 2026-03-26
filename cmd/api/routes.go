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
	r.Head("/healthz", makeHTTPHandleFunc(s.healthzHandler))

	r.Get("/stats", makeHTTPHandleFunc(s.statsHandler))
	r.Get("/actions", makeHTTPHandleFunc(s.listActionsHandler))
	r.Get("/{action}", makeHTTPHandleFunc(s.getRandomGifHandler))

	r.Post("/admin/login", makeHTTPHandleFunc(s.loginHandler))

	r.Route("/admin", func(r chi.Router) {
		r.Use(s.AdminKeyMiddleware)
		r.Route("/gifs", func(r chi.Router) {
			r.Get("/", makeHTTPHandleFunc(s.listGifsHandler))
			r.Post("/", makeHTTPHandleFunc(s.uploadGifHandler))
			r.Get("/all", makeHTTPHandleFunc(s.listAllGifsHandler))
			r.Delete("/{gifId}", makeHTTPHandleFunc(s.deleteGifHandler))
			r.Patch("/{gifId}/tags", makeHTTPHandleFunc(s.updateGifTagsHandler))
			r.Patch("/{gifId}/pairing", makeHTTPHandleFunc(s.updateGifPairingHandler))
			r.Patch("/{gifId}/anime", makeHTTPHandleFunc(s.updateGifAnimeHandler))
		})
		r.Get("/actions/{action}/count", makeHTTPHandleFunc(s.countGifsHandler))
		r.Route("/animes", func(r chi.Router) {
			r.Get("/", makeHTTPHandleFunc(s.listAnimesHandler))
			r.Post("/", makeHTTPHandleFunc(s.createAnimeHandler))
			r.Put("/{animeId}", makeHTTPHandleFunc(s.updateAnimeHandler))
			r.Delete("/{animeId}", makeHTTPHandleFunc(s.deleteAnimeHandler))
		})
	})

	return r
}
