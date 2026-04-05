package api

import (
	"log"
	"net/http"

	"github.com/lucialv/gifukai-api/cmd/api/handlers"
	u "github.com/lucialv/gifukai-api/pkg/utils"

	"github.com/go-chi/chi/v5"
)

type apiFunc func(http.ResponseWriter, *http.Request) error

func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			log.Printf("Handler error: %v", err)
			u.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
	}
}

func (s *APIServer) Routes() *chi.Mux {
	h := &handlers.Handler{
		Store:      s.Store,
		CDNBaseURL: s.Config.CDNBaseURL,
		R2Storage:  s.R2Storage,
	}

	r := chi.NewRouter()

	r.Get("/healthz", makeHTTPHandleFunc(h.HealthzHandler))
	r.Head("/healthz", makeHTTPHandleFunc(h.HealthzHandler))

	r.Get("/stats", makeHTTPHandleFunc(s.statsHandler))
	r.Get("/actions", makeHTTPHandleFunc(h.ListActionsHandler))

	r.Get("/library", makeHTTPHandleFunc(s.publicListGifsHandler))
	r.Get("/library/animes", makeHTTPHandleFunc(s.publicListAnimesHandler))
	r.Get("/leaderboard", makeHTTPHandleFunc(h.LeaderboardHandler))

	r.Post("/auth/google/onetap", makeHTTPHandleFunc(s.googleOneTapHandler))
	r.Get("/auth/github", makeHTTPHandleFunc(s.githubAuthHandler))
	r.Get("/auth/github/callback", makeHTTPHandleFunc(s.githubCallbackHandler))
	r.Get("/auth/me", makeHTTPHandleFunc(s.authMeHandler))
	r.Post("/auth/logout", makeHTTPHandleFunc(s.authLogoutHandler))

	r.Route("/user", func(r chi.Router) {
		r.Use(s.UserAuthMiddleware)
		r.Patch("/profile", makeHTTPHandleFunc(h.UpdateProfileHandler))
		r.Post("/reports", makeHTTPHandleFunc(h.CreateReportHandler))
		r.Post("/suggestions", makeHTTPHandleFunc(h.CreateSuggestionHandler))
		r.Get("/suggestions", makeHTTPHandleFunc(h.ListUserSuggestionsHandler))
		r.Get("/suggestions/limit", makeHTTPHandleFunc(h.UserSuggestionsLimitHandler))
	})

	r.Get("/{action}", makeHTTPHandleFunc(h.GetRandomGifHandler))

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
		r.Get("/actions/{action}/count", makeHTTPHandleFunc(h.CountGifsHandler))
		r.Route("/animes", func(r chi.Router) {
			r.Get("/", makeHTTPHandleFunc(s.listAnimesHandler))
			r.Post("/", makeHTTPHandleFunc(s.createAnimeHandler))
			r.Put("/{animeId}", makeHTTPHandleFunc(s.updateAnimeHandler))
			r.Delete("/{animeId}", makeHTTPHandleFunc(s.deleteAnimeHandler))
		})
		r.Route("/reports", func(r chi.Router) {
			r.Get("/", makeHTTPHandleFunc(h.ListReportsHandler))
			r.Patch("/{reportId}", makeHTTPHandleFunc(h.UpdateReportHandler))
		})
		r.Route("/suggestions", func(r chi.Router) {
			r.Get("/", makeHTTPHandleFunc(h.ListSuggestionsHandler))
			r.Post("/{suggestionId}/approve", makeHTTPHandleFunc(h.ApproveSuggestionHandler))
			r.Post("/{suggestionId}/reject", makeHTTPHandleFunc(h.RejectSuggestionHandler))
		})
	})

	return r
}
