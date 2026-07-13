// Package db gère la connexion à PostgreSQL via un pool pgx.
package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool est le pool de connexions partagé par toute l'application.
// Il est initialisé une seule fois au démarrage via Connect().
var Pool *pgxpool.Pool

// Connect initialise le pool de connexions PostgreSQL à partir des
// variables d'environnement (DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME).
// L'application s'arrête si la base est injoignable : pas de base = pas d'API.
func Connect() {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("db: DSN invalide : %v", err)
	}

	// Réglages raisonnables pour un projet de cette taille.
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	Pool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("db: création du pool impossible : %v", err)
	}

	// Vérifie immédiatement que la base répond (fail fast au démarrage,
	// utile avec docker-compose quand le conteneur db n'est pas encore prêt).
	if err := Pool.Ping(ctx); err != nil {
		log.Fatalf("db: ping PostgreSQL échoué : %v", err)
	}

	log.Printf("db: connecté à PostgreSQL (%s:%s/%s)",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))
}

// Close ferme proprement le pool (appelé en fin de main).
func Close() {
	if Pool != nil {
		Pool.Close()
	}
}
