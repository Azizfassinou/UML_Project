package handlers

import (
	"testing"
	"time"
)

// date construit une time.Time à partir de AAAA, MM, JJ (lisibilité des tests).
func date(annee int, mois time.Month, jour int) time.Time {
	return time.Date(annee, mois, jour, 0, 0, 0, 0, time.UTC)
}

// Table-driven test : le style idiomatique Go, parfait à montrer en soutenance.
func TestAgeEnAnnees(t *testing.T) {
	// Date de référence fixe pour des tests reproductibles :
	// on fait comme si on était le 15 juin 2026.
	now := date(2026, time.June, 15)

	cas := []struct {
		nom       string
		naissance time.Time
		attendu   int
	}{
		{"anniversaire déjà passé cette année", date(2000, time.March, 10), 26},
		{"anniversaire pas encore passé", date(2000, time.December, 25), 25},
		{"anniversaire aujourd'hui même", date(2010, time.June, 15), 16},
		{"anniversaire demain (encore 15 ans)", date(2010, time.June, 16), 15},
		{"anniversaire hier (16 ans révolus)", date(2010, time.June, 14), 16},
		{"tout juste majeur", date(2008, time.June, 15), 18},
		{"mineur 17 ans", date(2008, time.June, 16), 17},
		// Cas piège : né un 29 février (année bissextile).
		{"né un 29 février", date(2008, time.February, 29), 18},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := ageEnAnnees(c.naissance, now); got != c.attendu {
				t.Errorf("ageEnAnnees(%s) = %d, attendu %d",
					c.naissance.Format("2006-01-02"), got, c.attendu)
			}
		})
	}
}

// Vérifie les seuils métier du cahier des charges (règle 4.1) :
// < 16 refusé, 16-17 accord parental, >= 18 libre.
func TestSeuilsInscription(t *testing.T) {
	now := date(2026, time.June, 15)

	if age := ageEnAnnees(date(2010, time.June, 16), now); age >= 16 {
		t.Errorf("un adhérent de %d ans ne devrait pas passer le seuil des 16 ans", age)
	}
	if age := ageEnAnnees(date(2010, time.June, 15), now); age < 16 {
		t.Errorf("16 ans pile devrait passer le seuil, âge calculé : %d", age)
	}
	if age := ageEnAnnees(date(2008, time.June, 15), now); age < 18 {
		t.Errorf("18 ans pile ne devrait plus exiger d'accord parental, âge calculé : %d", age)
	}
}
