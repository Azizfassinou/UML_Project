// Package routes déclare toutes les routes de l'API et leurs protections.
// Les handlers seront implémentés au fur et à mesure dans handlers/.
package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/favfit/backend/handlers"
	"github.com/favfit/backend/middleware"
)

// Setup enregistre toutes les routes sur le routeur Gin.
func Setup(r *gin.Engine) {

	// Healthcheck : utile pour docker-compose (healthcheck) et Railway/Render.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// ------------------------------------------------------------------
	// Routes PUBLIQUES (pas de JWT) : auth uniquement.
	// ------------------------------------------------------------------
	auth := r.Group("/auth")
	{
		auth.POST("/register", handlers.Register)        // inscription adhérent
		auth.POST("/login", handlers.Login)              // connexion -> access + refresh token
		auth.GET("/verify/:token", handlers.VerifyEmail) // validation email
		auth.POST("/refresh", handlers.Refresh)          // refresh token -> nouvelle paire
	}

	// Consultation des formules : publique (un visiteur non connecté
	// doit pouvoir voir les tarifs avant de s'inscrire).
	r.GET("/formules", handlers.ListeFormules)

	// ------------------------------------------------------------------
	// Routes PROTÉGÉES : JWT obligatoire.
	// ------------------------------------------------------------------
	api := r.Group("/")
	api.Use(middleware.AuthRequired())
	{
		// --- Adhérents ---
		adherents := api.Group("/adherents")
		{
			adherents.GET("/:id", handlers.GetProfil)                           // profil + abonnement actif
			adherents.PUT("/:id", handlers.UpdateProfil)                        // modifier profil
			adherents.PUT("/:id/salle", handlers.ChangerSalle)                  // changer de salle (seq06)
			adherents.GET("/:id/reservations", handlers.HistoriqueReservations) // historique réservations
			adherents.GET("/:id/paiements", handlers.HistoriquePaiements)       // historique paiements
		}

		// --- Abonnements ---
		abonnements := api.Group("/abonnements")
		{
			abonnements.POST("", todo)                    // souscrire (paiement Stripe)
			abonnements.PUT("/:id/resilier", todo)        // résilier
			abonnements.PUT("/:id/changer-formule", todo) // changer de formule
		}

		// --- Séances ---
		seances := api.Group("/seances")
		{
			seances.GET("", handlers.ListeSeances) // liste avec filtres ?salle=&date=&coach=
			seances.POST("", middleware.RequireRole("gestionnaire"), todo)
			seances.DELETE("/:id", middleware.RequireRole("gestionnaire", "coach"), todo)
		}

		// --- Réservations ---
		reservations := api.Group("/reservations")
		{
			reservations.POST("", handlers.Reserver)      // réserver (seq04)
			reservations.DELETE("/:id", handlers.Annuler) // annuler (seq05, règle des 2h)
			reservations.PUT("/:id/presence", middleware.RequireRole("coach"), todo)
		}
	}
}

// todo est un handler temporaire : il permet de démarrer le serveur
// et de tester le middleware JWT avant d'avoir écrit les vrais handlers.
func todo(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "endpoint pas encore implémenté",
		"route": c.FullPath(),
	})
}
