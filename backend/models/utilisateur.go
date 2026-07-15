// Package models contient les structures Go correspondant aux tables PostgreSQL.
package models

import "time"

// Statuts possibles d'un compte utilisateur.
// 'en_attente' = inscrit mais email non validé (cahier des charges 4.1 :
// pas de souscription possible tant que le compte n'est pas validé).
const (
	StatutEnAttente = "en_attente"
	StatutActif     = "actif"
	StatutSuspendu  = "suspendu"
	StatutResilie   = "resilie"
)

// Rôles portés dans la colonne utilisateurs.role et dans les claims JWT.
const (
	RoleAdherent     = "adherent"
	RoleGestionnaire = "gestionnaire"
	RoleCoach        = "coach"
)

// Utilisateur = table utilisateurs (classe abstraite du diagramme de classes).
type Utilisateur struct {
	ID              int       `json:"id"`
	Nom             string    `json:"nom"`
	Prenom          string    `json:"prenom"`
	Email           string    `json:"email"`
	MotDePasse      string    `json:"-"` // jamais sérialisé dans les réponses JSON
	Telephone       string    `json:"telephone,omitempty"`
	DateInscription time.Time `json:"date_inscription"`
	Statut          string    `json:"statut"`
	Role            string    `json:"role"`
}

// Adherent = table adherents (spécialisation d'Utilisateur).
type Adherent struct {
	ID            int        `json:"id"`
	UtilisateurID int        `json:"utilisateur_id"`
	DateNaissance *time.Time `json:"date_naissance,omitempty"`
	Adresse       string     `json:"adresse,omitempty"`
	SalleID       *int       `json:"salle_id,omitempty"`
}
