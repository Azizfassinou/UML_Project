// adherents.go : profil adhérent, changement de salle (seq06),
// historiques de réservations et de paiements.
package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/favfit/backend/db"
	"github.com/favfit/backend/middleware"
	"github.com/favfit/backend/models"
)

// Règle 4.3 : sans multi-salles, un changement max par période de 30 jours.
const PeriodeChangementSalle = 30 * 24 * time.Hour

// controleAcces vérifie que l'appelant a le droit d'accéder à l'adhérent :id.
// Autorisé si c'est SON propre profil, ou si l'appelant est gestionnaire.
// Retourne l'id demandé et true si OK (sinon la réponse HTTP est déjà émise).
// 404 plutôt que 403 pour ne pas révéler l'existence d'autres comptes.
func controleAcces(c *gin.Context) (int, bool) {
	cibleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id invalide"})
		return 0, false
	}

	if c.GetString(middleware.CtxRole) == models.RoleGestionnaire {
		return cibleID, true
	}

	var monID int
	err = db.Pool.QueryRow(c.Request.Context(),
		`SELECT id FROM adherents WHERE utilisateur_id = $1`,
		c.GetInt(middleware.CtxUserID),
	).Scan(&monID)
	if err != nil || monID != cibleID {
		c.JSON(http.StatusNotFound, gin.H{"error": "adhérent introuvable"})
		return 0, false
	}
	return cibleID, true
}

// GetProfil gère GET /adherents/:id.
// Renvoie le profil complet + l'abonnement actif éventuel :
// tout ce qu'il faut pour le dashboard d'Aziz en UN seul appel.
func GetProfil(c *gin.Context) {
	adherentID, ok := controleAcces(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	var p struct {
		AdherentID    int        `json:"adherent_id"`
		Nom           string     `json:"nom"`
		Prenom        string     `json:"prenom"`
		Email         string     `json:"email"`
		Telephone     *string    `json:"telephone"`
		DateNaissance *time.Time `json:"date_naissance"`
		Adresse       *string    `json:"adresse"`
		SalleID       *int       `json:"salle_id"`
		SalleNom      *string    `json:"salle_nom"`
		Statut        string     `json:"statut"`
	}
	err := db.Pool.QueryRow(ctx, `
		SELECT a.id, u.nom, u.prenom, u.email, u.telephone,
		       a.date_naissance, a.adresse, a.salle_id, s.nom, u.statut
		FROM adherents a
		JOIN utilisateurs u ON u.id = a.utilisateur_id
		LEFT JOIN salles s ON s.id = a.salle_id
		WHERE a.id = $1`,
		adherentID,
	).Scan(&p.AdherentID, &p.Nom, &p.Prenom, &p.Email, &p.Telephone,
		&p.DateNaissance, &p.Adresse, &p.SalleID, &p.SalleNom, &p.Statut)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "adhérent introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.GetProfil: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// Abonnement actif éventuel (LEFT JOIN logique : peut ne pas exister).
	var abo *gin.H
	var (
		aboID     int
		formule   string
		dateDebut time.Time
		dateFin   *time.Time
		restantes *int
		multi     bool
	)
	err = db.Pool.QueryRow(ctx, `
		SELECT a.id, f.nom, a.date_debut, a.date_fin, a.seances_restantes, f.acces_multi_salles
		FROM abonnements a
		JOIN formules f ON f.id = a.formule_id
		WHERE a.adherent_id = $1 AND a.statut = 'en cours'`,
		adherentID,
	).Scan(&aboID, &formule, &dateDebut, &dateFin, &restantes, &multi)

	switch {
	case err == nil:
		abo = &gin.H{
			"id": aboID, "formule": formule,
			"date_debut": dateDebut, "date_fin": dateFin,
			"seances_restantes":  restantes, // null = illimité
			"acces_multi_salles": multi,
		}
	case err == pgx.ErrNoRows:
		// pas d'abonnement actif : abo reste nil -> "abonnement": null
	default:
		log.Printf("handlers.GetProfil: abonnement : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"profil": p, "abonnement": abo})
}

type updateProfilRequest struct {
	Nom       string `json:"nom" binding:"omitempty,min=2,max=100"`
	Prenom    string `json:"prenom" binding:"omitempty,min=2,max=100"`
	Telephone string `json:"telephone" binding:"omitempty,max=20"`
	Adresse   string `json:"adresse" binding:"omitempty,max=255"`
}

// UpdateProfil gère PUT /adherents/:id.
// Champs modifiables : nom, prénom, téléphone, adresse.
// L'email et le mot de passe ont volontairement leur propre parcours
// (validation email / sécurité), et la salle passe par PUT :id/salle.
func UpdateProfil(c *gin.Context) {
	adherentID, ok := controleAcces(c)
	if !ok {
		return
	}

	var req updateProfilRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "données invalides : " + err.Error()})
		return
	}
	if req.Nom == "" && req.Prenom == "" && req.Telephone == "" && req.Adresse == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "aucun champ à modifier"})
		return
	}

	ctx := c.Request.Context()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer tx.Rollback(ctx)

	// COALESCE(NULLIF($n,''), colonne) : ne met à jour que les champs fournis.
	if _, err := tx.Exec(ctx, `
		UPDATE utilisateurs u
		SET nom       = COALESCE(NULLIF($1,''), u.nom),
		    prenom    = COALESCE(NULLIF($2,''), u.prenom),
		    telephone = COALESCE(NULLIF($3,''), u.telephone)
		FROM adherents a
		WHERE a.utilisateur_id = u.id AND a.id = $4`,
		req.Nom, req.Prenom, req.Telephone, adherentID,
	); err != nil {
		log.Printf("handlers.UpdateProfil: utilisateurs : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if req.Adresse != "" {
		if _, err := tx.Exec(ctx,
			`UPDATE adherents SET adresse = $1 WHERE id = $2`,
			req.Adresse, adherentID,
		); err != nil {
			log.Printf("handlers.UpdateProfil: adherents : %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.UpdateProfil: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "profil mis à jour"})
}

type changerSalleRequest struct {
	SalleID int `json:"salle_id" binding:"required"`
}

// ChangerSalle gère PUT /adherents/:id/salle (diagramme seq06).
//
// Règles 4.3 :
//   - formule multi-salles -> changement immédiat, illimité
//   - sinon -> une fois par période de 30 jours
//   - chaque changement est tracé dans historique_salles
func ChangerSalle(c *gin.Context) {
	adherentID, ok := controleAcces(c)
	if !ok {
		return
	}

	var req changerSalleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "salle_id requis"})
		return
	}

	ctx := c.Request.Context()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer tx.Rollback(ctx)

	// Salle actuelle, verrouillée (empêche deux changements simultanés
	// de contourner la règle des 30 jours).
	var salleActuelle *int
	err = tx.QueryRow(ctx,
		`SELECT salle_id FROM adherents WHERE id = $1 FOR UPDATE`, adherentID,
	).Scan(&salleActuelle)
	if err != nil {
		log.Printf("handlers.ChangerSalle: adherent : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if salleActuelle != nil && *salleActuelle == req.SalleID {
		c.JSON(http.StatusConflict, gin.H{"error": "c'est déjà votre salle principale"})
		return
	}

	// La salle cible doit exister et être ouverte.
	var statutSalle string
	err = tx.QueryRow(ctx,
		`SELECT statut FROM salles WHERE id = $1`, req.SalleID,
	).Scan(&statutSalle)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "salle introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.ChangerSalle: salle : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if statutSalle != "ouverte" {
		c.JSON(http.StatusConflict, gin.H{"error": "cette salle est temporairement fermée"})
		return
	}

	// Multi-salles ? (pas d'abonnement actif = traité comme sans multi-salles)
	var multi bool
	err = tx.QueryRow(ctx, `
		SELECT f.acces_multi_salles
		FROM abonnements a JOIN formules f ON f.id = a.formule_id
		WHERE a.adherent_id = $1 AND a.statut = 'en cours'`,
		adherentID,
	).Scan(&multi)
	if err != nil && err != pgx.ErrNoRows {
		log.Printf("handlers.ChangerSalle: abonnement : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- Bloc alt seq06 : règle des 30 jours (formules sans multi-salles) ---
	if !multi {
		var dernier time.Time
		err = tx.QueryRow(ctx, `
			SELECT date_changement FROM historique_salles
			WHERE adherent_id = $1
			ORDER BY date_changement DESC LIMIT 1`,
			adherentID,
		).Scan(&dernier)

		if err == nil && time.Since(dernier) < PeriodeChangementSalle {
			prochain := dernier.Add(PeriodeChangementSalle)
			c.JSON(http.StatusConflict, gin.H{
				"error":               "votre formule ne permet qu'un changement de salle tous les 30 jours",
				"prochain_changement": prochain.Format("2006-01-02"),
			})
			return
		}
		if err != nil && err != pgx.ErrNoRows {
			log.Printf("handlers.ChangerSalle: historique : %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
			return
		}
	}

	// --- Écriture atomique : changement + trace historique (règle 4.3) ---
	if _, err := tx.Exec(ctx,
		`UPDATE adherents SET salle_id = $1 WHERE id = $2`,
		req.SalleID, adherentID,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgFKViolation {
			c.JSON(http.StatusNotFound, gin.H{"error": "salle introuvable"})
			return
		}
		log.Printf("handlers.ChangerSalle: update : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO historique_salles (adherent_id, ancienne_salle_id, nouvelle_salle_id)
		VALUES ($1, $2, $3)`,
		adherentID, salleActuelle, req.SalleID,
	); err != nil {
		log.Printf("handlers.ChangerSalle: historique insert : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.ChangerSalle: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "salle principale mise à jour", "salle_id": req.SalleID})
}

// HistoriqueReservations gère GET /adherents/:id/reservations.
// Toutes les réservations (passées, à venir, annulées) avec les infos séance.
func HistoriqueReservations(c *gin.Context) {
	adherentID, ok := controleAcces(c)
	if !ok {
		return
	}

	rows, err := db.Pool.Query(c.Request.Context(), `
		SELECT r.id, r.date_reservation, r.statut,
		       s.id, s.intitule, s.date_heure, sa.nom
		FROM reservations r
		JOIN seances s ON s.id = r.seance_id
		JOIN salles sa ON sa.id = s.salle_id
		WHERE r.adherent_id = $1
		ORDER BY s.date_heure DESC`,
		adherentID,
	)
	if err != nil {
		log.Printf("handlers.HistoriqueReservations: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer rows.Close()

	type item struct {
		ID              int       `json:"id"`
		DateReservation time.Time `json:"date_reservation"`
		Statut          string    `json:"statut"`
		SeanceID        int       `json:"seance_id"`
		Intitule        string    `json:"intitule"`
		DateHeure       time.Time `json:"date_heure"`
		Salle           string    `json:"salle"`
	}
	liste := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.DateReservation, &it.Statut,
			&it.SeanceID, &it.Intitule, &it.DateHeure, &it.Salle); err != nil {
			log.Printf("handlers.HistoriqueReservations: scan : %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
			return
		}
		liste = append(liste, it)
	}
	c.JSON(http.StatusOK, gin.H{"reservations": liste})
}

// HistoriquePaiements gère GET /adherents/:id/paiements.
func HistoriquePaiements(c *gin.Context) {
	adherentID, ok := controleAcces(c)
	if !ok {
		return
	}

	rows, err := db.Pool.Query(c.Request.Context(), `
		SELECT p.id, p.date_paiement, p.montant_ttc, p.mode_paiement,
		       p.statut, p.reference_transaction, f.nom
		FROM paiements p
		JOIN abonnements a ON a.id = p.abonnement_id
		JOIN formules f ON f.id = a.formule_id
		WHERE a.adherent_id = $1
		ORDER BY p.date_paiement DESC`,
		adherentID,
	)
	if err != nil {
		log.Printf("handlers.HistoriquePaiements: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer rows.Close()

	type item struct {
		ID         int       `json:"id"`
		Date       time.Time `json:"date_paiement"`
		MontantTTC float64   `json:"montant_ttc"`
		Mode       *string   `json:"mode_paiement"`
		Statut     string    `json:"statut"`
		Reference  *string   `json:"reference_transaction"`
		Formule    string    `json:"formule"`
	}
	liste := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.Date, &it.MontantTTC, &it.Mode,
			&it.Statut, &it.Reference, &it.Formule); err != nil {
			log.Printf("handlers.HistoriquePaiements: scan : %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
			return
		}
		liste = append(liste, it)
	}
	c.JSON(http.StatusOK, gin.H{"paiements": liste})
}
