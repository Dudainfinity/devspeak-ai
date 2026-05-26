package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	// padrão amigável pro docker-compose local
	return "postgres://devspeak:devspeak123@postgres:5432/devspeak_db?sslmode=disable"
}

// initDB abre a conexão e roda a migration. Faz retry simples
// pra cobrir o tempo entre o container do postgres ficar healthy
// e aceitar conexões TCP.
func initDB() error {
	dsn := databaseURL()
	var d *sql.DB
	var err error

	for i := 0; i < 10; i++ {
		d, err = sql.Open("postgres", dsn)
		if err == nil {
			if pingErr := d.Ping(); pingErr == nil {
				break
			} else {
				err = pingErr
				_ = d.Close()
				d = nil
			}
		}
		log.Printf("db: not ready (%v), retrying in 2s...", err)
		time.Sleep(2 * time.Second)
	}
	if d == nil {
		return fmt.Errorf("db: could not connect after retries: %w", err)
	}
	d.SetMaxOpenConns(15)
	d.SetMaxIdleConns(5)
	d.SetConnMaxLifetime(30 * time.Minute)
	db = d
	if err := migrate(); err != nil {
		return fmt.Errorf("db: migration failed: %w", err)
	}
	log.Println("db: connected and migrated")
	return nil
}

func migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			stack TEXT NOT NULL DEFAULT '',
			level TEXT NOT NULL DEFAULT '',
			primary_language TEXT NOT NULL DEFAULT '',
			target_role TEXT NOT NULL DEFAULT '',
			years_experience INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
		`CREATE TABLE IF NOT EXISTS evaluations (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			question TEXT NOT NULL,
			answer TEXT NOT NULL,
			score TEXT NOT NULL,
			score_class TEXT NOT NULL,
			technical TEXT NOT NULL,
			english JSONB NOT NULL DEFAULT '[]'::jsonb,
			vocab   JSONB NOT NULL DEFAULT '[]'::jsonb,
			level TEXT NOT NULL DEFAULT '',
			stack TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
		`CREATE INDEX IF NOT EXISTS evaluations_user_idx ON evaluations(user_id, created_at DESC);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migration stmt failed: %w; sql=%s", err, s)
		}
	}
	return nil
}
