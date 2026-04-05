package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lucialv/gifukai-api/cmd/api"
	"github.com/lucialv/gifukai-api/pkg/env"
	"github.com/lucialv/gifukai-api/pkg/storage"
	"github.com/lucialv/gifukai-api/pkg/store"


	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func main() {
	if os.Getenv("ENV") != "production" {
		env.Load()
	}

	addr := env.GetString("ADDR", ":8001")
	if p := os.Getenv("PORT"); p != "" {
		addr = fmt.Sprintf(":%s", p)
	}

	cfg := api.Config{
		Addr:       addr,
		Env:        env.GetString("ENV", "development"),
		CDNBaseURL: env.GetString("CDN_BASE_URL", "https://cdn.example.com"),
		AdminKey:   env.GetString("ADMIN_API_KEY", ""),
		AdminUser:  env.GetString("ADMIN_USER", ""),
		AdminPass:  env.GetString("ADMIN_PASS", ""),
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

	gifStore, err := store.NewGifStore(
		env.GetString("TURSO_DATABASE_URL", ""),
		env.GetString("TURSO_AUTH_TOKEN", ""),
	)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Connected to Turso database")

	r2Storage, err := storage.NewR2Storage(cfg.R2)
	if err != nil {
		log.Fatalf("Failed to initialize R2 storage: %v", err)
	}
	log.Println("Connected to Cloudflare R2")

	server := api.NewAPIServer(cfg, gifStore, r2Storage)
	server.StartStatsWorker(2 * time.Hour)
	server.Run()
}
