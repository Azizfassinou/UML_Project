// reservations.go : réservation (seq04) et annulation (seq05) de séances.
//
// C'est le handler le plus riche en règles métier du projet
// (cahier des charges 4.4 et 4.5). Les deux opérations sont
// transactionnelles avec verrou pessimiste sur la séance
// (SELECT ... FOR UPDATE) pour empêcher deux adhérents de prendre
// la dernière place en même temps.
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
)

// Délais métier (cahier des charges 4.4 / 4.5).
const (
	DelaiMinReservation = 1 * time.Hour // réservation jusqu'à 1h avant
	DelaiMinAnnulation  = 2 * time.Hour // annulation jusqu'à 2h avant
)

type reserverRequest struct {
	SeanceID int `json:"seance_id" binding:"required"`
}

// adherentDuToken retrouve l'adhérent correspondant à l'utilisateur du JWT.
// Retourne (adherentID, salleID, ok). Répond une erreur HTTP si absent.
func adherentDuToken(c *gin.Context) (int, *int, bool) {
	userID := c.GetInt(middleware.CtxUserID)

	var adherentID int
	var salleID *int
	err := db.Pool.QueryRow(c.Request.Context(),
		`SELECT id, salle_id FROM adherents WHERE utilisateur_id = $1`, userID,
	).Scan(&adherentID, &salleID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusForbidden, gin.H{"error": "réservé aux adhérents"})
		return 0, nil, false
	}
	if err != nil {
		log.Printf("handlers.adherentDuToken: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return 0, nil, false
	}
	return adherentID, salleID, true
}

// Reserver gère POST /reservations (diagramme seq04).
//
// Vérifications, dans l'ordre :
//  1. L'appelant est un adhérent (via JWT)
//  2. Abonnement actif ('en cours')                      <- include seq04
//  3. Séance existante, programmée, verrouillée FOR UPDATE
//  4. Règle 1h : réservation bloquée à moins d'1h du début
//  5. Places restantes > 0                               <- include seq04
//  6. Accès salle : si formule sans multi-salles, la séance doit être
//     dans la salle principale de l'adhérent
//  7. Quota mensuel : si formule limitée, seances_restantes > 0
//  8. Pas de double réservation sur la même séance
//
// Puis, atomiquement : INSERT reservation + décrément places
// (+ décrément quota si formule limitée).
func Reserver(c *gin.Context) {
	var req reserverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "seance_id requis"})
		return
	}

	adherentID, salleAdherent, ok := adherentDuToken(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer tx.Rollback(ctx)

	// --- 2. Abonnement actif + caractéristiques de la formule ---
	var (
		abonnementID     int
		seancesRestantes *int // NULL si formule illimitée
		multiSalles      bool
		quotaLimite      bool // la formule a-t-elle un nombre de séances ?
	)
	err = tx.QueryRow(ctx, `
		SELECT a.id, a.seances_restantes, f.acces_multi_salles,
		       f.nombre_seances IS NOT NULL
		FROM abonnements a
		JOIN formules f ON f.id = a.formule_id
		WHERE a.adherent_id = $1 AND a.statut = 'en cours'`,
		adherentID,
	).Scan(&abonnementID, &seancesRestantes, &multiSalles, &quotaLimite)

	if err == pgx.ErrNoRows {
		// Bloc alt seq04 : pas d'abonnement actif.
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun abonnement actif : souscrivez une formule pour réserver"})
		return
	}
	if err != nil {
		log.Printf("handlers.Reserver: abonnement : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- 3. Séance verrouillée : personne d'autre ne peut la modifier
	//        tant que cette transaction n'est pas terminée ---
	var (
		salleSeance     int
		dateHeure       time.Time
		placesRestantes int
		statutSeance    string
	)
	err = tx.QueryRow(ctx, `
		SELECT salle_id, date_heure, places_restantes, statut
		FROM seances
		WHERE id = $1
		FOR UPDATE`,
		req.SeanceID,
	).Scan(&salleSeance, &dateHeure, &placesRestantes, &statutSeance)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "séance introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.Reserver: seance : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if statutSeance != "programmee" {
		c.JSON(http.StatusConflict, gin.H{"error": "cette séance n'est plus ouverte à la réservation (statut : " + statutSeance + ")"})
		return
	}

	// --- 4. Règle métier : jusqu'à 1h avant le début ---
	if time.Until(dateHeure) < DelaiMinReservation {
		c.JSON(http.StatusConflict, gin.H{"error": "réservation impossible à moins d'1 heure du début de la séance"})
		return
	}

	// --- 5. Bloc alt seq04 : plus de places ---
	if placesRestantes <= 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "séance complète"})
		return
	}

	// --- 6. Restriction salle principale (formules sans multi-salles) ---
	if !multiSalles {
		if salleAdherent == nil || *salleAdherent != salleSeance {
			c.JSON(http.StatusForbidden, gin.H{"error": "votre formule ne permet de réserver que dans votre salle principale"})
			return
		}
	}

	// --- 7. Quota mensuel (formules limitées) ---
	if quotaLimite {
		if seancesRestantes == nil || *seancesRestantes <= 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "quota de séances mensuel épuisé"})
			return
		}
	}

	// --- 8. Une seule place par adhérent et par séance ---
	var deja bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM reservations
			WHERE adherent_id = $1 AND seance_id = $2 AND statut = 'confirmee'
		)`,
		adherentID, req.SeanceID,
	).Scan(&deja)
	if err != nil {
		log.Printf("handlers.Reserver: doublon : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if deja {
		c.JSON(http.StatusConflict, gin.H{"error": "vous avez déjà réservé cette séance"})
		return
	}

	// --- Écriture atomique ---
	var reservationID int
	err = tx.QueryRow(ctx, `
		INSERT INTO reservations (adherent_id, seance_id, statut)
		VALUES ($1, $2, 'confirmee')
		RETURNING id`,
		adherentID, req.SeanceID,
	).Scan(&reservationID)
	if err != nil {
		log.Printf("handlers.Reserver: insert : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if _, err := tx.Exec(ctx,
		`UPDATE seances SET places_restantes = places_restantes - 1 WHERE id = $1`,
		req.SeanceID,
	); err != nil {
		log.Printf("handlers.Reserver: update places : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if quotaLimite {
		if _, err := tx.Exec(ctx,
			`UPDATE abonnements SET seances_restantes = seances_restantes - 1 WHERE id = $1`,
			abonnementID,
		); err != nil {
			log.Printf("handlers.Reserver: update quota : %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.Reserver: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":          "réservation confirmée",
		"reservation_id":   reservationID,
		"seance_id":        req.SeanceID,
		"places_restantes": placesRestantes - 1,
	})
}

// Annuler gère DELETE /reservations/:id (diagramme seq05).
//
// Règles (cahier des charges 4.5) :
//   - seul le propriétaire de la réservation peut l'annuler (anti-IDOR)
//   - possible jusqu'à 2h avant le début de la séance
//   - la place est immédiatement libérée
//   - si formule à quota, la séance est restituée au quota
//
// Annulation logique (statut 'annulee'), pas de DELETE SQL :
// l'historique des réservations doit rester consultable.
func Annuler(c *gin.Context) {
	reservationID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de réservation invalide"})
		return
	}

	adherentID, _, ok := adherentDuToken(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer tx.Rollback(ctx)

	// Réservation + séance associée, verrouillées ensemble.
	var (
		proprietaireID int
		seanceID       int
		statutResa     string
		dateHeure      time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT r.adherent_id, r.seance_id, r.statut, s.date_heure
		FROM reservations r
		JOIN seances s ON s.id = r.seance_id
		WHERE r.id = $1
		FOR UPDATE`,
		reservationID,
	).Scan(&proprietaireID, &seanceID, &statutResa, &dateHeure)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "réservation introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.Annuler: select : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- Anti-IDOR : on ne peut annuler QUE ses propres réservations.
	// 404 (et pas 403) pour ne pas révéler l'existence de la réservation.
	if proprietaireID != adherentID {
		c.JSON(http.StatusNotFound, gin.H{"error": "réservation introuvable"})
		return
	}

	if statutResa != "confirmee" {
		c.JSON(http.StatusConflict, gin.H{"error": "cette réservation n'est pas annulable (statut : " + statutResa + ")"})
		return
	}

	// --- Bloc alt seq05 : règle des 2 heures ---
	if time.Until(dateHeure) < DelaiMinAnnulation {
		c.JSON(http.StatusConflict, gin.H{"error": "annulation impossible à moins de 2 heures du début de la séance"})
		return
	}

	// --- Écriture atomique : annulation logique + libération de la place ---
	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET statut = 'annulee' WHERE id = $1`,
		reservationID,
	); err != nil {
		log.Printf("handlers.Annuler: update resa : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if _, err := tx.Exec(ctx,
		`UPDATE seances SET places_restantes = places_restantes + 1 WHERE id = $1`,
		seanceID,
	); err != nil {
		log.Printf("handlers.Annuler: update places : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// Restitution du quota UNIQUEMENT pour les formules limitées
	// (seances_restantes non NULL). Le WHERE fait le tri tout seul.
	if _, err := tx.Exec(ctx, `
		UPDATE abonnements
		SET seances_restantes = seances_restantes + 1
		WHERE adherent_id = $1 AND statut = 'en cours'
		  AND seances_restantes IS NOT NULL`,
		adherentID,
	); err != nil {
		log.Printf("handlers.Annuler: restitution quota : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.Annuler: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "réservation annulée, la place est libérée"})
}
