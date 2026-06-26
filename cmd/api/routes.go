package api

import (
	"log/slog"
	"net/http"

	"github.com/lucialv/gifukai-api/cmd/api/handlers"
	"github.com/lucialv/gifukai-api/pkg/logging"
	u "github.com/lucialv/gifukai-api/pkg/utils"

	"github.com/go-chi/chi/v5"
)

type apiFunc func(http.ResponseWriter, *http.Request) error

func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			logging.FromContext(r.Context()).Error("handler error",
				slog.String("event", "handler_error"),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Any("error", err),
			)
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
	r.Get("/actions/{action}/types", makeHTTPHandleFunc(h.ActionTypesHandler))

	r.Get("/library", makeHTTPHandleFunc(s.publicListGifsHandler))
	r.Get("/library/animes", makeHTTPHandleFunc(s.publicListAnimesHandler))
	r.Get("/leaderboard", makeHTTPHandleFunc(h.LeaderboardHandler))

	r.Post("/auth/google/onetap", makeHTTPHandleFunc(s.googleOneTapHandler))
	r.Get("/auth/github", makeHTTPHandleFunc(s.githubAuthHandler))
	r.Get("/auth/github/callback", makeHTTPHandleFunc(s.githubCallbackHandler))
	r.Get("/auth/me", makeHTTPHandleFunc(s.authMeHandler))
	r.Post("/auth/logout", makeHTTPHandleFunc(s.authLogoutHandler))

	r.Route("/user", func(r chi.Router) {
		r.Use(s.Auth.UserAuth)
		r.Patch("/profile", makeHTTPHandleFunc(h.UpdateProfileHandler))
		r.Post("/reports", makeHTTPHandleFunc(h.CreateReportHandler))
		r.Post("/suggestions", makeHTTPHandleFunc(h.CreateSuggestionHandler))
		r.Get("/suggestions", makeHTTPHandleFunc(h.ListUserSuggestionsHandler))
		r.Get("/suggestions/limit", makeHTTPHandleFunc(h.UserSuggestionsLimitHandler))
	})

	r.Get("/{action}", makeHTTPHandleFunc(h.GetRandomGifHandler))

	r.Post("/admin/login", makeHTTPHandleFunc(s.Auth.Login))

	r.Route("/admin", func(r chi.Router) {
		r.Use(s.Auth.AdminKey)
		r.Route("/gifs", func(r chi.Router) {
			r.Get("/", makeHTTPHandleFunc(s.listGifsHandler))
			r.Post("/", makeHTTPHandleFunc(s.uploadGifHandler))
			r.Get("/all", makeHTTPHandleFunc(s.listAllGifsHandler))
			r.Delete("/{gifId}", makeHTTPHandleFunc(s.deleteGifHandler))
			r.Patch("/{gifId}/tags", makeHTTPHandleFunc(s.updateGifTagsHandler))
			r.Patch("/{gifId}/pairing", makeHTTPHandleFunc(s.updateGifPairingHandler))
			r.Patch("/{gifId}/type", makeHTTPHandleFunc(s.updateGifTypeHandler))
			r.Patch("/{gifId}/actions", makeHTTPHandleFunc(s.updateGifActionsHandler))
			r.Patch("/{gifId}/bidirectional", makeHTTPHandleFunc(s.updateGifBidirectionalHandler))
			r.Patch("/{gifId}/anime", makeHTTPHandleFunc(s.updateGifAnimeHandler))
		})
		r.Get("/actions/coverage", makeHTTPHandleFunc(h.ActionCoverageHandler))
		r.Get("/actions/{action}/variants", makeHTTPHandleFunc(h.ActionVariantsAdminHandler))
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
