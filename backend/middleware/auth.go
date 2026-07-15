// Package middleware regroupe les middlewares Gin (auth JWT, RBAC, CORS).
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/favfit/backend/services"
)

// Clés utilisées pour stocker les infos utilisateur dans le contexte Gin.
// Les handlers les récupèrent avec c.GetInt(middleware.CtxUserID) etc.
const (
	CtxUserID = "user_id"
	CtxRole   = "user_role"
)

// AuthRequired protège une route : exige un header
// "Authorization: Bearer <access_token>" valide.
// En cas de succès, injecte user_id et user_role dans le contexte Gin.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			abortUnauthorized(c, "header Authorization manquant")
			return
		}

		// Format attendu : "Bearer eyJhbGciOi..."
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abortUnauthorized(c, "format attendu : Bearer <token>")
			return
		}

		claims, err := services.ValidateToken(parts[1], "access")
		if err != nil {
			abortUnauthorized(c, "token invalide ou expiré")
			return
		}

		// Injection dans le contexte : les handlers savent QUI appelle.
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)

		c.Next()
	}
}

// RequireRole restreint une route à certains rôles (RBAC).
// À chaîner APRÈS AuthRequired().
// Exemple : seances.POST("", middleware.RequireRole("gestionnaire"), h.CreerSeance)
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(CtxRole)
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "accès refusé : rôle insuffisant",
		})
	}
}

func abortUnauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": msg})
}
