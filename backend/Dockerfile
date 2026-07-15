# ---------- Étape 1 : build ----------
# Image complète avec la toolchain Go, uniquement pour compiler.
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copier d'abord go.mod/go.sum seuls : le téléchargement des dépendances
# est mis en cache par Docker tant que ces fichiers ne changent pas.
COPY go.mod go.sum ./
RUN go mod download

# Puis le code source, et compilation statique (pas de libc requise).
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o favfit-api .

# ---------- Étape 2 : image finale ----------
# Alpine minimale : ~10 Mo au lieu de ~800 Mo avec la toolchain.
FROM alpine:3.20

# Certificats TLS (nécessaires pour appeler l'API Stripe en HTTPS).
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/favfit-api .

EXPOSE 8080

CMD ["./favfit-api"]
