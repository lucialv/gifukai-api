package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lucialv/anime-api-cdn/cmd/api"
	"github.com/lucialv/anime-api-cdn/pkg/env"
	"github.com/lucialv/anime-api-cdn/pkg/storage"
	"github.com/lucialv/anime-api-cdn/pkg/store"

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
		R2: api.R2Config{
			AccountID:       env.GetString("R2_ACCOUNT_ID", ""),
			AccessKeyID:     env.GetString("R2_ACCESS_KEY_ID", ""),
			AccessKeySecret: env.GetString("R2_ACCESS_KEY_SECRET", ""),
			BucketName:      env.GetString("R2_BUCKET_NAME", ""),
		},
	}

	gifStore, err := store.NewGifStore(
		env.GetString("TURSO_DATABASE_URL", ""),
		env.GetString("TURSO_AUTH_TOKEN", ""),
	)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Connected to Turso database")

	r2Storage, err := storage.NewR2Storage(storage.R2Config{
		AccountID:       cfg.R2.AccountID,
		AccessKeyID:     cfg.R2.AccessKeyID,
		AccessKeySecret: cfg.R2.AccessKeySecret,
		BucketName:      cfg.R2.BucketName,
	})
	if err != nil {
		log.Fatalf("Failed to initialize R2 storage: %v", err)
	}
	log.Println("Connected to Cloudflare R2")

	server := api.NewAPIServer(cfg, gifStore, r2Storage)
	server.StartStatsWorker(2 * time.Hour)
	server.Run()
}
