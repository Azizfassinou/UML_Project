// seances_admin.go : création de séance (gestionnaire), annulation
// avec notification des adhérents (seq07), validation des présences (coach).
package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/favfit/backend/db"
	"github.com/favfit/backend/middleware"
	"github.com/favfit/backend/models"
	"github.com/favfit/backend/services"
)

type creerSeanceRequest struct {
	Intitule    string `json:"intitule" binding:"required,min=2,max=150"`
	Description string `json:"description"`
	SalleID     int    `json:"salle_id" binding:"required"`
	CoachID     int    `json:"coach_id" binding:"required"`
	DateHeure   string `json:"date_heure" binding:"required"` // RFC3339 : 2026-07-20T18:00:00Z
	Duree       int    `json:"duree" binding:"required,min=15,max=240"`
	CapaciteMax int    `json:"capacite_max" binding:"required,min=1,max=200"`
}

// CreerSeance gère POST /seances (gestionnaire uniquement, RBAC en amont).
func CreerSeance(c *gin.Context) {
	var req creerSeanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "données invalides : " + err.Error()})
		return
	}

	dateHeure, err := time.Parse(time.RFC3339, req.DateHeure)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_heure invalide (format attendu : 2026-07-20T18:00:00Z)"})
		return
	}
	if dateHeure.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "impossible de programmer une séance dans le passé"})
		return
	}

	ctx := c.Request.Context()

	// La salle doit exister et être ouverte.
	var statutSalle string
	err = db.Pool.QueryRow(ctx,
		`SELECT statut FROM salles WHERE id = $1`, req.SalleID,
	).Scan(&statutSalle)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "salle introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.CreerSeance: salle : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if statutSalle != "ouverte" {
		c.JSON(http.StatusConflict, gin.H{"error": "cette salle est fermée"})
		return
	}

	// Le coach doit exister.
	var coachExiste bool
	if err := db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM coachs WHERE id = $1)`, req.CoachID,
	).Scan(&coachExiste); err != nil {
		log.Printf("handlers.CreerSeance: coach : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if !coachExiste {
		c.JSON(http.StatusNotFound, gin.H{"error": "coach introuvable"})
		return
	}

	var seanceID int
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO seances (intitule, description, salle_id, coach_id, date_heure, duree, capacite_max, places_restantes)
		VALUES ($1, NULLIF($2,''), $3, $4, $5, $6, $7, $7)
		RETURNING id`,
		req.Intitule, req.Description, req.SalleID, req.CoachID, dateHeure, req.Duree, req.CapaciteMax,
	).Scan(&seanceID)
	if err != nil {
		log.Printf("handlers.CreerSeance: insert : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "séance créée", "seance_id": seanceID})
}

// AnnulerSeance gère DELETE /seances/:id (diagramme seq07).
// Accessible au gestionnaire (toute séance) et au coach (SES séances).
//
// Règle 4.4 : l'annulation libère toutes les places, restitue les quotas
// des formules limitées, et notifie les adhérents concernés par email
// (extends "Notifier les adhérents" — le loop du seq07).
// Annulation logique : statut 'annulee', la séance reste en base.
func AnnulerSeance(c *gin.Context) {
	seanceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id invalide"})
		return
	}

	ctx := c.Request.Context()
	role := c.GetString(middleware.CtxRole)

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer tx.Rollback(ctx)

	var statut, intitule string
	var coachID int
	err = tx.QueryRow(ctx,
		`SELECT statut, intitule, coach_id FROM seances WHERE id = $1 FOR UPDATE`,
		seanceID,
	).Scan(&statut, &intitule, &coachID)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "séance introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.AnnulerSeance: select : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if statut != "programmee" {
		c.JSON(http.StatusConflict, gin.H{"error": "cette séance n'est pas annulable (statut : " + statut + ")"})
		return
	}

	// Un coach ne peut annuler QUE ses propres séances.
	if role == models.RoleCoach {
		var monCoachID int
		err = tx.QueryRow(ctx,
			`SELECT id FROM coachs WHERE utilisateur_id = $1`,
			c.GetInt(middleware.CtxUserID),
		).Scan(&monCoachID)
		if err != nil || monCoachID != coachID {
			c.JSON(http.StatusForbidden, gin.H{"error": "vous ne pouvez annuler que vos propres séances"})
			return
		}
	}

	// --- Récupérer les adhérents à notifier AVANT d'annuler leurs résas ---
	rows, err := tx.Query(ctx, `
		SELECT u.email
		FROM reservations r
		JOIN adherents a ON a.id = r.adherent_id
		JOIN utilisateurs u ON u.id = a.utilisateur_id
		WHERE r.seance_id = $1 AND r.statut = 'confirmee'`,
		seanceID,
	)
	if err != nil {
		log.Printf("handlers.AnnulerSeance: emails : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	emails := []string{}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err == nil {
			emails = append(emails, e)
		}
	}
	rows.Close()

	// --- Restituer le quota des formules limitées (avant de basculer
	//     les réservations, la condition porte sur 'confirmee') ---
	if _, err := tx.Exec(ctx, `
		UPDATE abonnements ab
		SET seances_restantes = ab.seances_restantes + 1
		FROM reservations r
		WHERE r.seance_id = $1 AND r.statut = 'confirmee'
		  AND r.adherent_id = ab.adherent_id
		  AND ab.statut = 'en cours'
		  AND ab.seances_restantes IS NOT NULL`,
		seanceID,
	); err != nil {
		log.Printf("handlers.AnnulerSeance: quotas : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- Annuler les réservations confirmées ---
	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET statut = 'annulee' WHERE seance_id = $1 AND statut = 'confirmee'`,
		seanceID,
	); err != nil {
		log.Printf("handlers.AnnulerSeance: reservations : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- Annuler la séance et libérer toutes les places ---
	if _, err := tx.Exec(ctx,
		`UPDATE seances SET statut = 'annulee', places_restantes = capacite_max WHERE id = $1`,
		seanceID,
	); err != nil {
		log.Printf("handlers.AnnulerSeance: seance : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.AnnulerSeance: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- Le LOOP du seq07 : notification après commit (l'échec d'un
	//     email ne doit pas annuler l'annulation) ---
	for _, email := range emails {
		_ = services.SendSeanceAnnuleeEmail(email, intitule)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "séance annulée, places libérées",
		"adherents_notifies": len(emails),
	})
}

type presenceRequest struct {
	Present *bool `json:"present" binding:"required"`
}

// ValiderPresence gère PUT /reservations/:id/presence (coach, RBAC en amont).
// Le coach marque un adhérent présent ou absent sur SA séance.
func ValiderPresence(c *gin.Context) {
	reservationID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id invalide"})
		return
	}

	var req presenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "champ present (true/false) requis"})
		return
	}

	ctx := c.Request.Context()

	// La réservation doit être confirmée, et la séance animée par CE coach.
	var statutResa string
	var coachSeance int
	err = db.Pool.QueryRow(ctx, `
		SELECT r.statut, s.coach_id
		FROM reservations r JOIN seances s ON s.id = r.seance_id
		WHERE r.id = $1`, reservationID,
	).Scan(&statutResa, &coachSeance)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "réservation introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.ValiderPresence: select : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	var monCoachID int
	err = db.Pool.QueryRow(ctx,
		`SELECT id FROM coachs WHERE utilisateur_id = $1`,
		c.GetInt(middleware.CtxUserID),
	).Scan(&monCoachID)
	if err != nil || monCoachID != coachSeance {
		c.JSON(http.StatusForbidden, gin.H{"error": "vous ne pouvez valider les présences que sur vos séances"})
		return
	}

	if statutResa != "confirmee" {
		c.JSON(http.StatusConflict, gin.H{"error": "cette réservation n'est pas validable (statut : " + statutResa + ")"})
		return
	}

	nouveau := "absent"
	if *req.Present {
		nouveau = "present"
	}
	if _, err := db.Pool.Exec(ctx,
		`UPDATE reservations SET statut = $1 WHERE id = $2`,
		nouveau, reservationID,
	); err != nil {
		log.Printf("handlers.ValiderPresence: update : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "présence enregistrée", "statut": nouveau})
}