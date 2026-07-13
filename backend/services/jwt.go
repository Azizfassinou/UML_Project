// Package services contient la logique métier réutilisable.
// jwt.go : génération et validation des tokens d'accès et de rafraîchissement.
package services

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims représente le contenu (payload) de nos tokens JWT.
// UserID = id dans la table utilisateurs, Role = 'adherent' | 'gestionnaire' | 'coach'.
type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	// Type distingue access token et refresh token pour empêcher
	// qu'un refresh token soit utilisé comme access token (et inversement).
	Type string `json:"typ"` // "access" ou "refresh"
	jwt.RegisteredClaims
}

var (
	ErrTokenInvalide = errors.New("token invalide ou expiré")
	ErrMauvaisType   = errors.New("type de token inattendu")
)

func secret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

// ttlMinutes lit une durée dans l'env avec une valeur par défaut.
func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

// GenerateAccessToken crée un access token courte durée (60 min par défaut,
// conforme au cahier des charges : durée de vie limitée à 1 heure).
func GenerateAccessToken(userID int, role string) (string, error) {
	ttl := time.Duration(envInt("JWT_TTL_MINUTES", 60)) * time.Minute
	return generate(userID, role, "access", ttl)
}

// GenerateRefreshToken crée un refresh token longue durée (7 jours par défaut),
// utilisé uniquement sur POST /auth/refresh pour obtenir un nouvel access token.
func GenerateRefreshToken(userID int, role string) (string, error) {
	ttl := time.Duration(envInt("JWT_REFRESH_TTL_HOURS", 168)) * time.Hour
	return generate(userID, role, "refresh", ttl)
}

// GenerateEmailVerifyToken crée un token de validation d'email (24h).
// Il est envoyé par mail sous forme de lien : GET /auth/verify/:token.
// Avantage vs table en base : auto-porteur, expire tout seul, aucune
// modification du schéma PostgreSQL nécessaire.
func GenerateEmailVerifyToken(userID int) (string, error) {
	return generate(userID, "", "email_verify", 24*time.Hour)
}

func generate(userID int, role, typ string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		Type:   typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "favfit-api",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret())
}

// ValidateToken vérifie la signature et l'expiration d'un token,
// et contrôle que son type correspond à l'usage attendu.
func ValidateToken(tokenStr, expectedType string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Refuse tout algorithme autre que HMAC (protection contre
		// l'attaque classique de substitution d'algorithme, ex: "none").
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalide
		}
		return secret(), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrTokenInvalide
	}
	if claims.Type != expectedType {
		return nil, ErrMauvaisType
	}
	return claims, nil
}
