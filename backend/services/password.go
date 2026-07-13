package services

import "golang.org/x/crypto/bcrypt"

// HashPassword hache un mot de passe en clair avec bcrypt (coût par défaut 10).
// Le mot de passe n'est JAMAIS stocké en clair (cahier des charges 6.2).
func HashPassword(clair string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(clair), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword compare un mot de passe en clair au hash stocké en base.
// Retourne true si ça correspond.
func CheckPassword(hash, clair string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(clair)) == nil
}
