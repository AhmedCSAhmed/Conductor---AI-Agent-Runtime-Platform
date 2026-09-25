// Package db owns the connection pool, the schema, and the repository
// functions. Nothing above it should touch SQL directly.
package db

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// pool is the process-wide connection pool.
//
// TODO for myself (3): this package-level pool is the same problem the Python
// version had with its module-level DatabaseSession. Every function below
// reaches for a global, so a test cannot hand in its own transaction and roll
// it back afterwards. Fix: make a `type Store struct { pool *pgxpool.Pool }`
// and hang the repository functions off it, or take a pgx.Tx argument so the
// caller owns the lifecycle. Do this before writing the first test.
var pool *pgxpool.Pool

// Init builds the pool from DATABASE_URL. Call once from main.
// pgxpool does not dial until the first query, so this is cheap and a bad
// address surfaces at first use, not here.
func Init(ctx context.Context) error {
	databaseURL, err := loadEnv()
	if err != nil {
		return err
	}

	p, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	pool = p
	return nil
}

// Pool exposes the pool for callers that need raw access, such as the schema
// bootstrap in initdb.
func Pool() *pgxpool.Pool {
	return pool
}

// Close releases every connection in the pool.
func Close() {
	if pool != nil {
		pool.Close()
	}
}

// TODO for myself (11): config is scattered os.Getenv calls, this one and the
// ones in cmd/worker. Make one internal/config package that loads and
// validates everything once at startup, and have both read from it.
func loadEnv() (string, error) {
	// A missing .env is fine when the variables come from the real
	// environment, so only a malformed file is worth reporting.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		// Deliberately not fatal.
		_ = err
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return "", fmt.Errorf("DATABASE_URL is not set; copy .env.example to .env")
	}

	return normalizeURL(databaseURL), nil
}

// normalizeURL strips the SQLAlchemy driver suffix, so a DATABASE_URL left
// over from the Python version ("postgresql+asyncpg://...") still works with
// pgx. Go has no equivalent of SQLAlchemy's driver dialects.
func normalizeURL(databaseURL string) string {
	for _, suffix := range []string{"+asyncpg", "+psycopg", "+psycopg2"} {
		databaseURL = strings.Replace(databaseURL, suffix, "", 1)
	}
	return databaseURL
}
