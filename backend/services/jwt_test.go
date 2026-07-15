package services

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// setupSecret injecte un secret de test dans l'environnement.
// t.Setenv restaure automatiquement la valeur d'origine après le test.
func setupSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret-de-test-uniquement")
}

func TestAccessToken_AllerRetour(t *testing.T) {
	setupSecret(t)

	token, err := GenerateAccessToken(42, "adherent")
	if err != nil {
		t.Fatalf("génération : %v", err)
	}

	claims, err := ValidateToken(token, "access")
	if err != nil {
		t.Fatalf("validation : %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("UserID = %d, attendu 42", claims.UserID)
	}
	if claims.Role != "adherent" {
		t.Errorf("Role = %q, attendu \"adherent\"", claims.Role)
	}
}

// Un refresh token ne doit JAMAIS passer sur une route protégée
// (qui attend un token de type "access").
func TestRefreshToken_RefuseCommeAccess(t *testing.T) {
	setupSecret(t)

	refresh, err := GenerateRefreshToken(42, "adherent")
	if err != nil {
		t.Fatalf("génération : %v", err)
	}

	if _, err := ValidateToken(refresh, "access"); err == nil {
		t.Fatal("un refresh token a été accepté comme access token")
	}
}

func TestEmailVerifyToken_AllerRetour(t *testing.T) {
	setupSecret(t)

	token, err := GenerateEmailVerifyToken(7)
	if err != nil {
		t.Fatalf("génération : %v", err)
	}

	claims, err := ValidateToken(token, "email_verify")
	if err != nil {
		t.Fatalf("validation : %v", err)
	}
	if claims.UserID != 7 {
		t.Errorf("UserID = %d, attendu 7", claims.UserID)
	}

	// Et il ne doit pas être utilisable comme access token.
	if _, err := ValidateToken(token, "access"); err == nil {
		t.Fatal("un token email_verify a été accepté comme access token")
	}
}

func TestToken_Expire(t *testing.T) {
	setupSecret(t)

	// On fabrique un token déjà expiré (le helper generate est privé
	// mais on est dans le même package, donc accessible).
	token, err := generate(1, "adherent", "access", -time.Minute)
	if err != nil {
		t.Fatalf("génération : %v", err)
	}

	if _, err := ValidateToken(token, "access"); err == nil {
		t.Fatal("un token expiré a été accepté")
	}
}

func TestToken_SignatureInvalide(t *testing.T) {
	setupSecret(t)

	// Token signé avec un AUTRE secret -> doit être rejeté.
	claims := Claims{UserID: 1, Role: "adherent", Type: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
	intrus := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := intrus.SignedString([]byte("mauvais-secret"))

	if _, err := ValidateToken(tokenStr, "access"); err == nil {
		t.Fatal("un token signé avec un mauvais secret a été accepté")
	}
}

func TestToken_Falsifie(t *testing.T) {
	setupSecret(t)

	token, _ := GenerateAccessToken(1, "adherent")
	// On corrompt le payload : la signature ne correspond plus.
	falsifie := token[:len(token)-4] + "XXXX"

	if _, err := ValidateToken(falsifie, "access"); err == nil {
		t.Fatal("un token falsifié a été accepté")
	}
}

func TestBcrypt_AllerRetour(t *testing.T) {
	hash, err := HashPassword("S3curePass!")
	if err != nil {
		t.Fatalf("hachage : %v", err)
	}
	if hash == "S3curePass!" {
		t.Fatal("le mot de passe est stocké en clair")
	}
	if !CheckPassword(hash, "S3curePass!") {
		t.Error("le bon mot de passe est refusé")
	}
	if CheckPassword(hash, "MauvaisPass") {
		t.Error("un mauvais mot de passe est accepté")
	}
}
