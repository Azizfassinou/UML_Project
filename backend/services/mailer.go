package services

import (
	"fmt"
	"log"
	"os"
)

// SendVerificationEmail envoie le lien de validation de compte.
//
// En mode dev (pas de SMTP configuré), le lien est simplement loggué
// dans la console : suffisant pour la démo et les tests.
// Si vous voulez brancher un vrai envoi plus tard (bonus), remplacez
// le log par un appel SMTP (net/smtp) ou une API type Resend/Brevo.
func SendVerificationEmail(email, token string) error {
	base := os.Getenv("FRONTEND_URL")
	if base == "" {
		base = "http://localhost:3000"
	}
	// Le frontend affichera une page "validation en cours" qui appelle
	// GET /auth/verify/:token côté API.
	lien := fmt.Sprintf("%s/verify/%s", base, token)

	log.Printf("mailer: [DEV] email de validation pour %s -> %s", email, lien)
	return nil
}

// SendSeanceAnnuleeEmail notifie un adhérent qu'une séance qu'il avait
// réservée a été annulée (règle 4.4, extends "Notifier les adhérents").
// Même logique dev que la validation : loggué en console.
func SendSeanceAnnuleeEmail(email, intituleSeance string) error {
	log.Printf("mailer: [DEV] notification pour %s -> votre séance %q a été annulée, votre place et votre quota ont été restitués", email, intituleSeance)
	return nil
}
