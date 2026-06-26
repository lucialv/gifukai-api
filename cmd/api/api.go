package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lucialv/gifukai-api/cmd/api/middlewares"
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

type APIServer struct {
	Config      Config
	Logger      *slog.Logger
	Auth        *middlewares.Auth
	Store       store.GifStore
	R2Storage   *storage.R2Storage
	statsCache  StatsCache
	oauthStates sync.Map
}

func NewAPIServer(config Config, logger *slog.Logger, gifStore store.GifStore, r2Storage *storage.R2Storage) *APIServer {
	config.CDNBaseURL = strings.TrimRight(config.CDNBaseURL, "/")
	if logger == nil {
		logger = slog.Default()
	}
	return &APIServer{
		Config:    config,
		Logger:    logger,
		Auth:      middlewares.NewAuth(config.AdminKey, config.AdminUser, config.AdminPass),
		Store:     gifStore,
		R2Storage: r2Storage,
	}
}

func (s *APIServer) Run() error {
	router := chi.NewRouter()

	router.Use(
		render.SetContentType(render.ContentTypeJSON),
		middleware.RequestID,
		middleware.RealIP,
		middlewares.AccessLog(s.Logger),
		middlewares.Recover,
		middleware.Compress(5),
		cors.Handler(cors.Options{
			AllowedOrigins:   strings.Split(env.GetString("CORS_ALLOWED_ORIGIN", "*"), ","),
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
			AllowCredentials: false,
			MaxAge:           3600,
		}),
	)

	const requestTimeout = 90 * time.Second
	router.Use(middleware.Timeout(requestTimeout))

	router.Mount("/", s.Routes())

	log := s.Logger.With(slog.String("component", "startup"))
	routeCount := 0
	_ = chi.Walk(router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routeCount++
		return nil
	})

	srv := &http.Server{Addr: s.Config.Addr, Handler: router}

	idleClosed := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Info("shutting down", slog.String("event", "shutdown"))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("graceful shutdown failed", slog.Any("error", err))
		}
		close(idleClosed)
	}()

	log.Info("server started",
		slog.String("event", "server_start"),
		slog.String("addr", s.Config.Addr),
		slog.String("env", s.Config.Env),
		slog.Int("routes", routeCount),
	)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	<-idleClosed
	return nil
}
