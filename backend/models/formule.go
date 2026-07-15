package models

// Statuts d'une formule d'abonnement.
const (
	FormuleActive   = "active"
	FormuleArchivee = "archivee"
)

// Formule = table formules.
//
// Pointeurs sur les champs nullables en base :
//   - NombreSeances : NULL = séances illimitées (cahier des charges 3.2)
//   - DureeEngagement : NULL traité comme 0 = sans engagement
type Formule struct {
	ID               int     `json:"id"`
	Nom              string  `json:"nom"`
	Description      *string `json:"description"`
	TarifHT          float64 `json:"tarif_ht"`
	TauxTVA          float64 `json:"taux_tva"`
	TarifTTC         float64 `json:"tarif_ttc"`
	DureeEngagement  *int    `json:"duree_engagement"`
	NombreSeances    *int    `json:"nombre_seances"` // null = illimité
	AccesMultiSalles bool    `json:"acces_multi_salles"`
	Statut           string  `json:"statut,omitempty"`
}
