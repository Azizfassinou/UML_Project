// login.go : connexion JWT (seq02) et rafraîchissement de token.
package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/favfit/backend/db"
	"github.com/favfit/backend/models"
	"github.com/favfit/backend/services"
)

type loginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	MotDePasse string `json:"mot_de_passe" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// dummyHash est un hash bcrypt valide d'une chaîne aléatoire.
// Utilisé quand l'email n'existe pas : on fait quand même une comparaison
// bcrypt pour que la durée de réponse soit la même que pour un email
// existant (protection contre l'énumération d'emails par timing attack).
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Login gère POST /auth/login (diagramme seq02).
//
// Déroulé :
//  1. Validation du corps JSON
//  2. Recherche de l'utilisateur par email
//  3. Vérification bcrypt du mot de passe
//     -> bloc alt seq02 : identifiants incorrects = 401 (message volontairement
//     identique que l'email existe ou non, pour ne rien révéler)
//  4. Contrôle du statut du compte (en_attente / suspendu / resilie)
//  5. Génération access token (1h) + refresh token (7j)
func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "données invalides : " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	var (
		userID int
		hash   string
		statut string
		role   string
		prenom string
		nom    string
	)
	err := db.Pool.QueryRow(ctx, `
		SELECT id, mot_de_passe, statut, role, prenom, nom
		FROM utilisateurs
		WHERE email = $1`,
		req.Email,
	).Scan(&userID, &hash, &statut, &role, &prenom, &nom)

	if err == pgx.ErrNoRows {
		// Email inconnu : comparaison factice pour un temps de réponse
		// constant, puis MÊME message que pour un mauvais mot de passe.
		services.CheckPassword(dummyHash, req.MotDePasse)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identifiants incorrects"})
		return
	}
	if err != nil {
		log.Printf("handlers.Login: select : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- Bloc alt seq02 : mot de passe incorrect ---
	if !services.CheckPassword(hash, req.MotDePasse) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identifiants incorrects"})
		return
	}

	// --- Contrôle du statut du compte ---
	switch statut {
	case models.StatutActif:
		// OK, on continue.
	case models.StatutEnAttente:
		c.JSON(http.StatusForbidden, gin.H{
			"error": "compte non validé : cliquez sur le lien reçu par email",
		})
		return
	default: // suspendu, resilie
		c.JSON(http.StatusForbidden, gin.H{
			"error": "compte " + statut + " : contactez votre salle",
		})
		return
	}

	// --- Génération des tokens ---
	access, err1 := services.GenerateAccessToken(userID, role)
	refresh, err2 := services.GenerateRefreshToken(userID, role)
	if err1 != nil || err2 != nil {
		log.Printf("handlers.Login: génération tokens : %v / %v", err1, err2)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur interne"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"expires_in":    3600, // secondes (aligné sur JWT_TTL_MINUTES=60)
		"user": gin.H{
			"id":     userID,
			"prenom": prenom,
			"nom":    nom,
			"role":   role,
		},
	})
}

// Refresh gère POST /auth/refresh.
//
// Reçoit un refresh token valide et renvoie une NOUVELLE paire
// access + refresh (rotation : l'ancien refresh ne devrait plus être
// utilisé côté client, on limite ainsi la fenêtre d'exploitation
// d'un token volé).
//
// On revérifie le statut en base : un compte suspendu entre-temps
// ne doit pas pouvoir régénérer de tokens, même avec un refresh valide.
func Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token manquant"})
		return
	}

	claims, err := services.ValidateToken(req.RefreshToken, "refresh")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token invalide ou expiré"})
		return
	}

	ctx := c.Request.Context()

	var statut, role string
	err = db.Pool.QueryRow(ctx,
		`SELECT statut, role FROM utilisateurs WHERE id = $1`, claims.UserID,
	).Scan(&statut, &role)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "compte introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.Refresh: select : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if statut != models.StatutActif {
		c.JSON(http.StatusForbidden, gin.H{"error": "compte " + statut})
		return
	}

	access, err1 := services.GenerateAccessToken(claims.UserID, role)
	refresh, err2 := services.GenerateRefreshToken(claims.UserID, role)
	if err1 != nil || err2 != nil {
		log.Printf("handlers.Refresh: génération tokens : %v / %v", err1, err2)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur interne"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"expires_in":    3600,
	})
}
