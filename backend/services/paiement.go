// paiement.go : encaissement du premier mois d'abonnement.
//
// Deux modes (adapter pattern) :
//   - STRIPE_SECRET_KEY renseignée -> vrai appel à l'API Stripe
//     (PaymentIntent créé et confirmé immédiatement, mode test)
//   - variable vide -> mode SIMULATION : paiement toujours accepté,
//     référence factice. Permet de développer, tester et faire la démo
//     sans dépendre d'un service externe.
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ResultatPaiement est renvoyé au handler quel que soit le mode.
type ResultatPaiement struct {
	Reference string // reference_transaction en base (pi_xxx ou SIMU-xxx)
	Statut    string // 'valide' ou 'echoue'
}

var ErrPaiementRefuse = errors.New("paiement refusé")

// EncaisserPremierMois débite le montant TTC (en euros) du premier mois.
// paymentMethod : id de moyen de paiement Stripe (ex : pm_card_visa en test).
func EncaisserPremierMois(montantTTC float64, paymentMethod string) (*ResultatPaiement, error) {
	key := os.Getenv("STRIPE_SECRET_KEY")

	// ---------- Mode SIMULATION ----------
	if key == "" || key == "sk_test_xxx" {
		ref := fmt.Sprintf("SIMU-%d-%04d", time.Now().Unix(), rand.Intn(10000))
		log.Printf("paiement: [SIMULATION] %.2f EUR encaissés (ref %s)", montantTTC, ref)
		return &ResultatPaiement{Reference: ref, Statut: "valide"}, nil
	}

	// ---------- Mode STRIPE réel ----------
	// Appel direct à l'API REST (pas besoin du SDK complet pour un
	// PaymentIntent). Montant en CENTIMES, obligatoire chez Stripe.
	if paymentMethod == "" {
		paymentMethod = "pm_card_visa" // carte de test Stripe par défaut
	}

	form := url.Values{}
	form.Set("amount", strconv.Itoa(int(montantTTC*100+0.5)))
	form.Set("currency", "eur")
	form.Set("payment_method", paymentMethod)
	form.Set("confirm", "true")
	// Pas de redirection 3DS dans ce flux serveur-à-serveur simplifié.
	form.Set("automatic_payment_methods[enabled]", "true")
	form.Set("automatic_payment_methods[allow_redirects]", "never")

	req, err := http.NewRequest("POST",
		"https://api.stripe.com/v1/payment_intents",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(key, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe injoignable : %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var pi struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &pi); err != nil {
		return nil, fmt.Errorf("réponse stripe illisible : %w", err)
	}

	if pi.Error != nil {
		log.Printf("paiement: stripe a refusé : %s", pi.Error.Message)
		return &ResultatPaiement{Reference: pi.ID, Statut: "echoue"}, ErrPaiementRefuse
	}
	if pi.Status != "succeeded" {
		log.Printf("paiement: statut stripe inattendu : %s", pi.Status)
		return &ResultatPaiement{Reference: pi.ID, Statut: "echoue"}, ErrPaiementRefuse
	}

	log.Printf("paiement: [STRIPE] %.2f EUR encaissés (ref %s)", montantTTC, pi.ID)
	return &ResultatPaiement{Reference: pi.ID, Statut: "valide"}, nil
}
