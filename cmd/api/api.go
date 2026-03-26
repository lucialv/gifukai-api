package api

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/lucialv/anime-api-cdn/pkg/env"
	"github.com/lucialv/anime-api-cdn/pkg/storage"
	"github.com/lucialv/anime-api-cdn/pkg/store"

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
	R2         R2Config
}

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	AccessKeySecret string
	BucketName      string
}

type APIServer struct {
	Config     Config
	Store      store.GifStore
	R2Storage  *storage.R2Storage
	statsCache StatsCache
	sessions   sync.Map
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
			AllowedOrigins:   []string{env.GetString("CORS_ALLOWED_ORIGIN", "*")},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
			AllowCredentials: false,
			MaxAge:           3600,
		}),
	)

	router.Use(middleware.Timeout(10 * time.Second))

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
