package db

import (
	"context"
	_ "embed"
	"fmt"
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
		return migrateExisting(ctx, pool)
	}

	log.Println("applying schema.sql")
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	return migrateExisting(ctx, pool)
}

// migrateExisting applies idempotent post-schema migrations so already
// deployed databases pick up new columns without a rebuild-from-zero.
func migrateExisting(ctx context.Context, pool *pgxpool.Pool) error {
	statements := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE`,
	}
	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migration %q: %w", stmt, err)
		}
	}
	return nil
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
