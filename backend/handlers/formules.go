// formules.go : consultation des formules d'abonnement.
package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/favfit/backend/db"
	"github.com/favfit/backend/models"
)

// ListeFormules gère GET /formules (route publique : un visiteur doit
// pouvoir consulter les tarifs avant de créer un compte).
//
// Ne renvoie que les formules actives — les formules archivées restent
// en base (les abonnements existants y sont liés) mais ne sont plus
// proposées à la souscription.
func ListeFormules(c *gin.Context) {
	ctx := c.Request.Context()

	rows, err := db.Pool.Query(ctx, `
		SELECT id, nom, description, tarif_ht, taux_tva, tarif_ttc,
		       duree_engagement, nombre_seances, acces_multi_salles
		FROM formules
		WHERE statut = 'active'
		ORDER BY tarif_ttc ASC`)
	if err != nil {
		log.Printf("handlers.ListeFormules: query : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer rows.Close()

	formules := []models.Formule{}
	for rows.Next() {
		var f models.Formule
		if err := rows.Scan(
			&f.ID, &f.Nom, &f.Description, &f.TarifHT, &f.TauxTVA, &f.TarifTTC,
			&f.DureeEngagement, &f.NombreSeances, &f.AccesMultiSalles,
		); err != nil {
			log.Printf("handlers.ListeFormules: scan : %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
			return
		}
		formules = append(formules, f)
	}
	if rows.Err() != nil {
		log.Printf("handlers.ListeFormules: rows : %v", rows.Err())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"formules": formules})
}
