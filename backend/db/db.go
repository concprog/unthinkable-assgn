package db

import (
	"context"
	_ "embed"
	"log"
	"os"
	"time"

	"github.com/gofiber/storage/postgres/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	// to_regclass returns NULL when the table doesn't exist yet,
	// so scan into *string, not string.
	var existing *string
	err := pool.QueryRow(ctx, `SELECT to_regclass('public.users')::text`).Scan(&existing)
	if err != nil {
		return err
	}
	if existing != nil && *existing != "" {
		return nil
	}

	log.Println("applying schema.sql")
	_, err = pool.Exec(ctx, schemaSQL)
	return err
}

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
