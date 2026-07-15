// abonnements.go : souscription (seq03), résiliation et changement de formule.
// Règles métier : cahier des charges 4.2.
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
	"github.com/favfit/backend/services"
)

type souscrireRequest struct {
	FormuleID     int    `json:"formule_id" binding:"required"`
	ModePaiement  string `json:"mode_paiement" binding:"omitempty,oneof=carte sepa"`
	PaymentMethod string `json:"payment_method"` // id Stripe, optionnel (test : pm_card_visa)
}

// Souscrire gère POST /abonnements (diagramme seq03).
//
// Règles 4.1/4.2 :
//   - compte validé obligatoire (statut utilisateur 'actif')
//   - un seul abonnement actif à la fois            <- bloc alt seq03
//   - paiement immédiat du premier mois              <- include "Appel API Stripe"
//   - date_fin = date_debut + engagement (NULL si sans engagement)
//   - seances_restantes initialisé au quota de la formule (NULL = illimité)
func Souscrire(c *gin.Context) {
	var req souscrireRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "données invalides : " + err.Error()})
		return
	}
	if req.ModePaiement == "" {
		req.ModePaiement = "carte"
	}

	userID := c.GetInt(middleware.CtxUserID)
	ctx := c.Request.Context()

	// --- Adhérent + compte validé (règle 4.1) ---
	var adherentID int
	var statutCompte string
	err := db.Pool.QueryRow(ctx, `
		SELECT a.id, u.statut
		FROM adherents a JOIN utilisateurs u ON u.id = a.utilisateur_id
		WHERE a.utilisateur_id = $1`, userID,
	).Scan(&adherentID, &statutCompte)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusForbidden, gin.H{"error": "réservé aux adhérents"})
		return
	}
	if err != nil {
		log.Printf("handlers.Souscrire: adherent : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if statutCompte != "actif" {
		c.JSON(http.StatusForbidden, gin.H{"error": "compte non validé ou suspendu : souscription impossible"})
		return
	}

	// --- Formule active + tarif ---
	var (
		tarifTTC        float64
		dureeEngagement *int
		nombreSeances   *int
	)
	err = db.Pool.QueryRow(ctx, `
		SELECT tarif_ttc, duree_engagement, nombre_seances
		FROM formules WHERE id = $1 AND statut = 'active'`,
		req.FormuleID,
	).Scan(&tarifTTC, &dureeEngagement, &nombreSeances)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "formule introuvable ou archivée"})
		return
	}
	if err != nil {
		log.Printf("handlers.Souscrire: formule : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	defer tx.Rollback(ctx)

	// --- Bloc alt seq03 : un seul abonnement actif à la fois ---
	// FOR UPDATE sur les abonnements de l'adhérent : deux souscriptions
	// simultanées ne peuvent pas passer ensemble.
	var dejaActif bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM abonnements
			WHERE adherent_id = $1 AND statut = 'en cours'
			FOR UPDATE
		)`, adherentID,
	).Scan(&dejaActif)
	if err != nil {
		log.Printf("handlers.Souscrire: verif actif : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if dejaActif {
		c.JSON(http.StatusConflict, gin.H{"error": "vous avez déjà un abonnement actif : résiliez-le ou changez de formule"})
		return
	}

	// --- Include seq03 : paiement immédiat du premier mois ---
	// AVANT la création de l'abonnement : pas de paiement = pas d'abonnement
	// (plus simple et plus sûr que créer puis annuler sous 48h).
	paiement, err := services.EncaisserPremierMois(tarifTTC, req.PaymentMethod)
	if err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{ // 402
			"error":     "le paiement a été refusé, l'abonnement n'a pas été créé",
			"reference": paiement.Reference,
		})
		return
	}

	// --- Création de l'abonnement ---
	dateDebut := time.Now()
	var dateFin *time.Time
	if dureeEngagement != nil && *dureeEngagement > 0 {
		fin := dateDebut.AddDate(0, *dureeEngagement, 0)
		dateFin = &fin
	}

	var abonnementID int
	err = tx.QueryRow(ctx, `
		INSERT INTO abonnements (adherent_id, formule_id, date_debut, date_fin, statut, seances_restantes)
		VALUES ($1, $2, $3, $4, 'en cours', $5)
		RETURNING id`,
		adherentID, req.FormuleID, dateDebut, dateFin, nombreSeances,
	).Scan(&abonnementID)
	if err != nil {
		log.Printf("handlers.Souscrire: insert abonnement : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	// --- Trace du paiement ---
	if _, err := tx.Exec(ctx, `
		INSERT INTO paiements (abonnement_id, montant_ttc, mode_paiement, statut, reference_transaction)
		VALUES ($1, $2, $3, $4, $5)`,
		abonnementID, tarifTTC, req.ModePaiement, paiement.Statut, paiement.Reference,
	); err != nil {
		log.Printf("handlers.Souscrire: insert paiement : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.Souscrire: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "abonnement souscrit, premier mois encaissé",
		"abonnement_id":  abonnementID,
		"montant_ttc":    tarifTTC,
		"reference":      paiement.Reference,
		"date_fin":       dateFin, // null = sans engagement
	})
}

// Motifs légitimes de résiliation anticipée (règle 4.2).
var motifsLegitimes = map[string]bool{
	"demenagement":       true,
	"incapacite_medicale": true,
}

type resilierRequest struct {
	Motif string `json:"motif" binding:"required"`
}

// Resilier gère PUT /abonnements/:id/resilier.
//
// Règles 4.2 :
//   - sans engagement (ou engagement échu) : résiliation libre,
//     l'abonnement reste actif jusqu'à la fin du mois en cours
//   - avec engagement en cours : uniquement motif légitime
//     (déménagement, incapacité médicale)
func Resilier(c *gin.Context) {
	abonnementID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id invalide"})
		return
	}

	var req resilierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "motif requis"})
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

	var (
		proprietaire int
		statut       string
		dateFin      *time.Time
		engagement   *int
	)
	err = tx.QueryRow(ctx, `
		SELECT a.adherent_id, a.statut, a.date_fin, f.duree_engagement
		FROM abonnements a JOIN formules f ON f.id = a.formule_id
		WHERE a.id = $1
		FOR UPDATE`, abonnementID,
	).Scan(&proprietaire, &statut, &dateFin, &engagement)

	if err == pgx.ErrNoRows || (err == nil && proprietaire != adherentID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "abonnement introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.Resilier: select : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if statut != "en cours" {
		c.JSON(http.StatusConflict, gin.H{"error": "cet abonnement n'est pas actif (statut : " + statut + ")"})
		return
	}

	// --- Engagement en cours ? -> motif légitime obligatoire ---
	engagementEnCours := engagement != nil && *engagement > 0 &&
		dateFin != nil && time.Now().Before(*dateFin)
	if engagementEnCours && !motifsLegitimes[req.Motif] {
		c.JSON(http.StatusForbidden, gin.H{
			"error":            "résiliation anticipée impossible : votre engagement court jusqu'au " + dateFin.Format("2006-01-02"),
			"motifs_acceptes":  []string{"demenagement", "incapacite_medicale"},
		})
		return
	}

	// --- Résiliation : actif jusqu'à la fin du mois en cours (règle 4.2) ---
	now := time.Now()
	finDeMois := time.Date(now.Year(), now.Month(), 1, 23, 59, 59, 0, now.Location()).
		AddDate(0, 1, -1)

	if _, err := tx.Exec(ctx, `
		UPDATE abonnements
		SET date_resiliation = $1, motif_resiliation = $2, date_fin = $3
		WHERE id = $4`,
		now, req.Motif, finDeMois, abonnementID,
	); err != nil {
		log.Printf("handlers.Resilier: update : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.Resilier: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "résiliation enregistrée, votre abonnement reste actif jusqu'à la fin du mois",
		"date_fin": finDeMois.Format("2006-01-02"),
	})
}

type changerFormuleRequest struct {
	FormuleID int `json:"formule_id" binding:"required"`
}

// ChangerFormule gère PUT /abonnements/:id/changer-formule.
//
// Règle 4.2 : possible à tout moment, prend effet à la prochaine échéance.
// Simplification assumée (documentée dans le dossier) : le changement est
// appliqué immédiatement en base, la facturation au nouveau tarif se fera
// naturellement à la prochaine échéance puisque le paiement mensuel lit
// la formule courante. Le quota de séances est réaligné sur la nouvelle formule.
func ChangerFormule(c *gin.Context) {
	abonnementID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id invalide"})
		return
	}

	var req changerFormuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "formule_id requis"})
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

	var proprietaire int
	var statut string
	var formuleActuelle int
	err = tx.QueryRow(ctx, `
		SELECT adherent_id, statut, formule_id FROM abonnements
		WHERE id = $1 FOR UPDATE`, abonnementID,
	).Scan(&proprietaire, &statut, &formuleActuelle)

	if err == pgx.ErrNoRows || (err == nil && proprietaire != adherentID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "abonnement introuvable"})
		return
	}
	if err != nil {
		log.Printf("handlers.ChangerFormule: select : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}
	if statut != "en cours" {
		c.JSON(http.StatusConflict, gin.H{"error": "cet abonnement n'est pas actif"})
		return
	}
	if formuleActuelle == req.FormuleID {
		c.JSON(http.StatusConflict, gin.H{"error": "c'est déjà votre formule actuelle"})
		return
	}

	// Nouvelle formule active + son quota.
	var nouveauQuota *int
	err = tx.QueryRow(ctx,
		`SELECT nombre_seances FROM formules WHERE id = $1 AND statut = 'active'`,
		req.FormuleID,
	).Scan(&nouveauQuota)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "formule introuvable ou archivée"})
		return
	}
	if err != nil {
		log.Printf("handlers.ChangerFormule: formule : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if _, err := tx.Exec(ctx, `
		UPDATE abonnements SET formule_id = $1, seances_restantes = $2
		WHERE id = $3`,
		req.FormuleID, nouveauQuota, abonnementID,
	); err != nil {
		log.Printf("handlers.ChangerFormule: update : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("handlers.ChangerFormule: commit : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur base de données"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "changement de formule enregistré, effectif à la prochaine échéance de facturation",
		"formule_id": req.FormuleID,
	})
}