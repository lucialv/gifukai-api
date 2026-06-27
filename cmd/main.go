package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lucialv/gifukai-api/cmd/api"
	"github.com/lucialv/gifukai-api/pkg/env"
	"github.com/lucialv/gifukai-api/pkg/logging"
	"github.com/lucialv/gifukai-api/pkg/storage"
	"github.com/lucialv/gifukai-api/pkg/store"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

const (
	cacheRefreshInterval = 12 * time.Hour
	statsRefreshInterval = 2 * time.Hour
)

func main() {
	if os.Getenv("ENV") != "production" {
		env.Load()
	}

	addr := env.GetString("ADDR", ":8001")
	if p := os.Getenv("PORT"); p != "" {
		addr = fmt.Sprintf(":%s", p)
	}

	appEnv := env.GetString("ENV", "development")
	logger, logCloser := logging.New(logging.Options{
		Service:   "gifukai-api",
		Env:       appEnv,
		Version:   env.GetString("SERVICE_VERSION", ""),
		Level:     logging.ParseLevel(env.GetString("LOG_LEVEL", "info")),
		Pretty:    appEnv != "production",
		LokiURL:   env.GetString("LOKI_URL", ""),
		LokiUser:  env.GetString("LOKI_USERNAME", ""),
		LokiToken: env.GetString("LOKI_TOKEN", ""),
	})
	slog.SetDefault(logger)
	defer logCloser.Close()

	cfg := api.Config{
		Addr:       addr,
		Env:        appEnv,
		CDNBaseURL: env.GetString("CDN_BASE_URL", "https://cdn.example.com"),
		AdminKey:   env.GetString("ADMIN_API_KEY", ""),
		AdminUser:  env.GetString("ADMIN_USER", ""),
		AdminPass:  env.GetString("ADMIN_PASS", ""),
		LibraryKey: env.GetString("LIBRARY_KEY", ""),
		R2: storage.R2Config{
			AccountID:       env.GetString("R2_ACCOUNT_ID", ""),
			AccessKeyID:     env.GetString("R2_ACCESS_KEY_ID", ""),
			AccessKeySecret: env.GetString("R2_ACCESS_KEY_SECRET", ""),
			BucketName:      env.GetString("R2_BUCKET_NAME", ""),
		},

		GoogleClientID:     env.GetString("GOOGLE_CLIENT_ID", ""),
		GitHubClientID:     env.GetString("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: env.GetString("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURL:  env.GetString("GITHUB_REDIRECT_URL", ""),
		FrontendURL:        env.GetString("FRONTEND_URL", "https://gifukai.com"),
	}

	boot := logger.With(slog.String("component", "startup"))

	sqlStore, err := store.NewGifStore(
		env.GetString("TURSO_DATABASE_URL", ""),
		env.GetString("TURSO_AUTH_TOKEN", ""),
	)
	if err != nil {
		boot.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	boot.Info("connected to Turso database", slog.String("event", "db_connected"))

	gifStore, err := store.NewCachedGifStore(sqlStore)
	if err != nil {
		boot.Error("failed to initialize GIF cache", slog.Any("error", err))
		os.Exit(1)
	}
	gifStore.StartRefreshWorker(cacheRefreshInterval)

	r2Storage, err := storage.NewR2Storage(cfg.R2)
	if err != nil {
		boot.Error("failed to initialize R2 storage", slog.Any("error", err))
		os.Exit(1)
	}
	boot.Info("connected to Cloudflare R2", slog.String("event", "r2_connected"))

	server := api.NewAPIServer(cfg, logger, gifStore, r2Storage)
	server.StartStatsWorker(statsRefreshInterval)
	if err := server.Run(); err != nil {
		boot.Error("server failed", slog.Any("error", err))
		logCloser.Close()
		os.Exit(1)
	}
}
