// seances.go : consultation du planning des séances.
package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/favfit/backend/db"
)

// seanceItem = une ligne du planning renvoyée au frontend,
// avec les infos jointes (nom de salle, nom du coach) pour éviter
// à Aziz de faire 3 appels API par séance.
type seanceItem struct {
	ID              int       `json:"id"`
	Intitule        string    `json:"intitule"`
	Description     *string   `json:"description"`
	SalleID         int       `json:"salle_id"`
	SalleNom        string    `json:"salle_nom"`
	CoachID         int       `json:"coach_id"`
	CoachNom        string    `json:"coach_nom"`
	DateHeure       time.Time `json:"date_heure"`
	Duree           *int      `json:"duree"`
	CapaciteMax     int       `json:"capacite_max"`
	PlacesRestantes int       `json:"places_restantes"`
	Statut          string    `json:"statut"`
}

// ListeSeances gère GET /seances (page planning).
//
// Filtres optionnels en query string, cumulables :
//
//	?salle=1        -> séances d'une salle
//	?coach=2        -> séances d'un coach
//	?date=2026-07-20 -> séances d'un jour précis
//
// Par défaut : uniquement les séances programmées et à venir
// (l'historique passe par GET /adherents/:id/reservations).
func ListeSeances(c *gin.Context) {
	ctx := c.Request.Context()

	// Construction dynamique du WHERE avec paramètres positionnels.
	// JAMAIS de concaténation de valeurs dans le SQL (injection).
	query := `
		SELECT s.id, s.intitule, s.description,
		       s.salle_id, sa.nom,
		       s.coach_id, u.prenom || ' ' || u.nom,
		       s.date_heure, s.duree, s.capacite_max, s.places_restantes, s.statut
		FROM seances s
		JOIN salles sa ON sa.id = s.salle_id
		JOIN coachs co ON co.id = s.coach_id
		JOIN utilisateurs u ON u.id = co.utilisateur_id
		WHERE s.statut = 'programmee' AND s.date_heure > NOW()`

	args := []interface{}{}
	pos := 1

	if salle := c.Query("salle"); salle != "" {
		query += ` AND s.salle_id = $` + strconv.Itoa(pos)
		args = append(args, salle)
		pos++
	}
	if coach := c.Query("coach"); coach != "" {
		query += ` AND s.coach_id = $` + strconv.Itoa(pos)
		args = append(args, coach)
		pos++
	}
	if date := c.Query("date"); date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date invalide (format attendu : AAAA-MM-JJ)"})
			return
		}
		query += ` AND s.date_heure::date = $` + strconv.Itoa(pos)
		args = append(args, date)
		pos++
	}

	query += ` ORDER BY s.date_heure ASC`

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		log.Printf("handlers.ListeSeances: query : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer rows.Close()

	seances := []seanceItem{}
	for rows.Next() {
		var s seanceItem
		if err := rows.Scan(
			&s.ID, &s.Intitule, &s.Description,
			&s.SalleID, &s.SalleNom,
			&s.CoachID, &s.CoachNom,
			&s.DateHeure, &s.Duree, &s.CapaciteMax, &s.PlacesRestantes, &s.Statut,
		); err != nil {
			log.Printf("handlers.ListeSeances: scan : %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
			return
		}
		seances = append(seances, s)
	}
	if rows.Err() != nil {
		log.Printf("handlers.ListeSeances: rows : %v", rows.Err())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"seances": seances})
}
