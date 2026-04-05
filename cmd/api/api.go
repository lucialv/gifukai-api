package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lucialv/gifukai-api/pkg/env"
	"github.com/lucialv/gifukai-api/pkg/storage"
	"github.com/lucialv/gifukai-api/pkg/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
)

type Config struct {
	Addr       string
	Env        string
	CDNBaseURL string
	AdminKey   string
	AdminUser  string
	AdminPass  string
	R2         storage.R2Config
	// OAuth
	GoogleClientID     string
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string
	FrontendURL        string
}

func ParsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	offset = 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= maxLimit {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	return
}

type APIServer struct {
	Config       Config
	Store        store.GifStore
	R2Storage    *storage.R2Storage
	statsCache   StatsCache
	sessions     sync.Map
	userSessions sync.Map
	oauthStates  sync.Map
}

func NewAPIServer(config Config, gifStore store.GifStore, r2Storage *storage.R2Storage) *APIServer {
	return &APIServer{
		Config:    config,
		Store:     gifStore,
		R2Storage: r2Storage,
	}
}

func (s *APIServer) Run() {
	router := chi.NewRouter()

	router.Use(
		render.SetContentType(render.ContentTypeJSON),
		middleware.Logger,
		middleware.Compress(5),
		middleware.RealIP,
		middleware.Recoverer,
		cors.Handler(cors.Options{
			AllowedOrigins:   strings.Split(env.GetString("CORS_ALLOWED_ORIGIN", "*"), ","),
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
			AllowCredentials: false,
			MaxAge:           3600,
		}),
	)

	router.Use(middleware.Timeout(90 * time.Second))

	router.Mount("/", s.Routes())

	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("  %s %s", method, route)
		return nil
	}
	log.Println("Registered routes:")
	if err := chi.Walk(router, walkFunc); err != nil {
		log.Printf("Chi router walk error: %v", err)
	}

	log.Printf("Running on %s [%s]", s.Config.Addr, s.Config.Env)
	if err := http.ListenAndServe(s.Config.Addr, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
