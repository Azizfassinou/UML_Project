// Package handlers contient la logique de chaque endpoint.
// auth.go : inscription (seq01) et validation email.
package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/favfit/backend/db"
	"github.com/favfit/backend/models"
	"github.com/favfit/backend/services"
)

// Codes d'erreur PostgreSQL utiles.
const (
	pgUniqueViolation = "23505" // contrainte UNIQUE (email déjà utilisé)
	pgFKViolation     = "23503" // clé étrangère (salle inexistante)
)

// registerRequest = corps attendu sur POST /auth/register.
// Les tags binding déclenchent une 400 automatique si invalide.
type registerRequest struct {
	Nom             string `json:"nom" binding:"required,min=2,max=100"`
	Prenom          string `json:"prenom" binding:"required,min=2,max=100"`
	Email           string `json:"email" binding:"required,email"`
	MotDePasse      string `json:"mot_de_passe" binding:"required,min=8"`
	Telephone       string `json:"telephone" binding:"omitempty,max=20"`
	DateNaissance   string `json:"date_naissance" binding:"required"` // format AAAA-MM-JJ
	Adresse         string `json:"adresse" binding:"required"`
	SalleID         int    `json:"salle_id" binding:"required"`
	AccordParental  bool   `json:"accord_parental"` // requis si 16-17 ans
}

// Register gère POST /auth/register (diagramme seq01).
//
// Déroulé :
//  1. Validation du corps JSON (binding Gin)
//  2. Règle métier 4.1 : âge >= 16 ans, accord parental si < 18
//  3. Hachage bcrypt du mot de passe
//  4. Transaction : INSERT utilisateurs (statut 'en_attente') + INSERT adherents
//     -> bloc alt "email déjà utilisé" géré par la contrainte UNIQUE (23505)
//  5. Génération du token de validation + envoi de l'email
func Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "données invalides : " + err.Error()})
		return
	}

	// --- Règle métier : âge minimum 16 ans, accord parental entre 16 et 18 ---
	naissance, err := time.Parse("2006-01-02", req.DateNaissance)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_naissance invalide (format attendu : AAAA-MM-JJ)"})
		return
	}
	age := ageEnAnnees(naissance, time.Now())
	switch {
	case age < 16:
		c.JSON(http.StatusForbidden, gin.H{"error": "l'inscription est réservée aux personnes de 16 ans et plus"})
		return
	case age < 18 && !req.AccordParental:
		c.JSON(http.StatusForbidden, gin.H{"error": "un accord parental est requis pour les mineurs de 16 à 17 ans"})
		return
	}

	// --- Hachage du mot de passe (jamais stocké en clair) ---
	hash, err := services.HashPassword(req.MotDePasse)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur interne"})
		return
	}

	ctx := c.Request.Context()

	// --- Transaction : les deux INSERT réussissent ou aucun ---
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	// Rollback est un no-op si le Commit a réussi : filet de sécurité.
	defer tx.Rollback(ctx)

	var userID int
	err = tx.QueryRow(ctx, `
		INSERT INTO utilisateurs (nom, prenom, email, mot_de_passe, telephone, statut, role)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		req.Nom, req.Prenom, req.Email, hash, req.Telephone,
		models.StatutEnAttente, models.RoleAdherent,
	).Scan(&userID)

	if err != nil {
		// Bloc ALT du diagramme seq01 : email déjà utilisé -> 409 Conflict
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			c.JSON(http.StatusConflict, gin.H{"error": "cette adresse email est déjà utilisée"})
			return
		}
		log.Printf("handlers.Register: insert utilisateurs : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	var adherentID int
	err = tx.QueryRow(ctx, `
		INSERT INTO adherents (utilisateur_id, date_naissance, adresse, salle_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		userID, naissance, req.Adresse, req.SalleID,
	).Scan(&adherentID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgFKViolation {
			c.JSON(http.StatusBadRequest, gin.H{"error": "salle_id inconnu : cette salle n'existe pas"})
			return
		}
		log.Printf("handlers.Register: insert adherents : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.Register: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- Extension seq01 : envoi de l'email de validation ---
	// L'échec de l'envoi ne fait pas échouer l'inscription : le compte
	// existe, l'utilisateur pourra redemander un lien plus tard.
	token, err := services.GenerateEmailVerifyToken(userID)
	if err == nil {
		_ = services.SendVerificationEmail(req.Email, token)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "compte créé, un email de validation a été envoyé",
		"user_id":     userID,
		"adherent_id": adherentID,
		"statut":      models.StatutEnAttente,
	})
}

// VerifyEmail gère GET /auth/verify/:token.
// Passe le compte de 'en_attente' à 'actif'. Idempotent : re-cliquer
// sur le lien après validation renvoie un message OK, pas une erreur.
func VerifyEmail(c *gin.Context) {
	claims, err := services.ValidateToken(c.Param("token"), "email_verify")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "lien de validation invalide ou expiré"})
		return
	}

	ctx := c.Request.Context()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE utilisateurs SET statut = $1
		WHERE id = $2 AND statut = $3`,
		models.StatutActif, claims.UserID, models.StatutEnAttente,
	)
	if err != nil {
		log.Printf("handlers.VerifyEmail: update : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if tag.RowsAffected() == 0 {
		// Deux cas possibles : compte déjà validé (re-clic sur le lien),
		// ou compte suspendu/supprimé entre-temps. On vérifie lequel.
		var statut string
		err := db.Pool.QueryRow(ctx,
			`SELECT statut FROM utilisateurs WHERE id = $1`, claims.UserID,
		).Scan(&statut)
		if err == nil && statut == models.StatutActif {
			c.JSON(http.StatusOK, gin.H{"message": "compte déjà validé, vous pouvez vous connecter"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "compte introuvable ou non validable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email validé, votre compte est actif"})
}

// ageEnAnnees calcule l'âge exact en années révolues à la date donnée.
// (time.Since / 365 est faux à cause des années bissextiles.)
func ageEnAnnees(naissance, now time.Time) int {
	age := now.Year() - naissance.Year()
	// Anniversaire pas encore passé cette année -> -1
	if now.Month() < naissance.Month() ||
		(now.Month() == naissance.Month() && now.Day() < naissance.Day()) {
		age--
	}
	return age
}
