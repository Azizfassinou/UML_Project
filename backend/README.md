# FAV FIT — Backend API (Go + Gin)

API REST de la plateforme de gestion de salles de sport FAV FIT.
Stack : Go 1.22, Gin, PostgreSQL (pgx), JWT (golang-jwt/v5), Stripe.

## Démarrage rapide

```bash
# 1. Config
cp .env.example .env
# éditer .env : DB_*, JWT_SECRET (openssl rand -base64 48)

# 2. Base de données (via docker-compose du repo, ou en local)
psql -U favfit -d favfit -f ../db/init.sql

# 3. Dépendances + run
go mod tidy
go run .
```

L'API écoute sur `http://localhost:8080`. Test rapide :

```bash
curl http://localhost:8080/health
# {"status":"ok"}

curl http://localhost:8080/adherents/1
# 401 {"error":"header Authorization manquant"}  <- le middleware JWT fonctionne
```

## Structure

```
main.go        Point d'entrée : env, DB, middlewares, routes
routes/        Déclaration des routes + protections (JWT, rôles)
handlers/      Logique de chaque endpoint (à venir)
models/        Structs Go <-> tables PostgreSQL (à venir)
middleware/    AuthRequired (JWT), RequireRole (RBAC), CORS
services/      Logique métier : JWT, calcul TTC, quotas... 
db/            Pool de connexions pgx
```

## Authentification

- Access token : HS256, 60 min (`JWT_TTL_MINUTES`)
- Refresh token : 7 jours (`JWT_REFRESH_TTL_HOURS`), utilisable uniquement sur `POST /auth/refresh`
- Header attendu : `Authorization: Bearer <access_token>`
- RBAC : rôles `adherent`, `gestionnaire`, `coach` portés dans les claims
