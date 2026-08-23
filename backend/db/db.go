package db

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gofiber/storage/postgres/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect() *pgxpool.Pool {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatalf("parse DATABASE_URL: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}

	return pool
}

func NewLimiterStore(pool *pgxpool.Pool) *postgres.Storage {
	return postgres.New(postgres.Config{
		DB:         pool,
		Table:      "fiber_storage",
		GCInterval: 10 * time.Second,
	})
}
