package middleware

import (
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS autorise le frontend Vue.js (FRONTEND_URL) à appeler l'API.
// En dev : http://localhost:3000. En prod : l'URL Railway/Render du front.
func CORS() gin.HandlerFunc {
	frontend := os.Getenv("FRONTEND_URL")
	if frontend == "" {
		frontend = "http://localhost:3000"
	}

	return cors.New(cors.Config{
		AllowOrigins:     []string{frontend},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
