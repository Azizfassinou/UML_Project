// FAV FIT — API REST backend (Go + Gin + PostgreSQL + JWT)
// Point d'entrée : charge la config, connecte la base, monte les routes.
package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/favfit/backend/db"
	"github.com/favfit/backend/middleware"
	"github.com/favfit/backend/routes"
)

func main() {
	// 1. Charger le .env (silencieux s'il n'existe pas : en prod / Docker,
	//    les variables sont injectées directement dans l'environnement).
	if err := godotenv.Load(); err != nil {
		log.Println("main: pas de fichier .env, utilisation des variables d'environnement")
	}

	// Garde-fou : refuser de démarrer sans secret JWT.
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("main: JWT_SECRET manquant dans l'environnement")
	}

	// 2. Connexion PostgreSQL (fatal si la base ne répond pas).
	db.Connect()
	defer db.Close()

	// 3. Mode Gin (debug en dev, release en prod via GIN_MODE=release).
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	}

	// 4. Routeur + middlewares globaux.
	r := gin.New()
	r.Use(gin.Logger())   // log de chaque requête
	r.Use(gin.Recovery()) // un panic dans un handler ne tue pas le serveur
	r.Use(middleware.CORS())

	// 5. Routes.
	routes.Setup(r)

	// 6. Démarrage.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("main: API FAV FIT démarrée sur :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("main: serveur arrêté : %v", err)
	}
}
