# FAV FIT — Contrat d'API Backend

**Base URL (dev) :** `http://localhost:8080`
**Dernière mise à jour :** juillet 2026 — 13 endpoints opérationnels.

## Authentification

Toutes les routes sont protégées **sauf** `/health`, `/auth/*` et `GET /formules`.

Header requis sur les routes protégées :

```
Authorization: Bearer <access_token>
```

- L'**access token** expire après **1 heure** → au 401, utiliser le refresh token puis rejouer la requête.
- Le **refresh token** expire après **7 jours** → au-delà, redirection vers la page de connexion.
- Après un refresh, **remplacer les deux tokens** (rotation) : l'ancien refresh ne doit plus être utilisé.

### Codes d'erreur globaux

| Code | Signification | Action côté front |
|---|---|---|
| 400 | Corps/paramètre invalide | Afficher le message `error` |
| 401 | Token absent/invalide/expiré | Tenter un refresh, sinon rediriger vers login |
| 403 | Authentifié mais pas le droit | Afficher le message `error` |
| 404 | Ressource introuvable (ou pas à toi) | Afficher "introuvable" |
| 409 | Conflit avec l'état actuel (doublon, complet, délai...) | Afficher le message `error` |
| 500 | Erreur serveur | Message générique "réessayez plus tard" |

Toutes les erreurs ont le même format : `{"error": "message en français affichable tel quel"}`

---

## 1. Auth

### POST /auth/register — Inscription

```json
// Requête
{
  "nom": "Martin",
  "prenom": "Lucas",
  "email": "lucas.martin@mail.fr",
  "mot_de_passe": "S3curePass!",        // min 8 caractères
  "telephone": "0612345678",             // optionnel
  "date_naissance": "2003-04-12",        // AAAA-MM-JJ
  "adresse": "12 rue de la Paix, 75002 Paris",
  "salle_id": 1,
  "accord_parental": false               // requis à true si 16-17 ans
}

// 201
{
  "message": "compte créé, un email de validation a été envoyé",
  "user_id": 1,
  "adherent_id": 1,
  "statut": "en_attente"
}
```

Erreurs : `409` email déjà utilisé · `403` moins de 16 ans, ou 16-17 ans sans accord parental · `400` salle inconnue.

⚠️ Le compte est créé **non validé** : le login renverra 403 tant que l'email n'est pas confirmé.

### GET /auth/verify/:token — Validation email

Le lien reçu par email pointe vers `{FRONTEND_URL}/verify/:token` : la page front appelle cet endpoint avec le token.

`200` `{"message": "email validé, votre compte est actif"}` — idempotent (re-clic = 200 aussi).
`401` lien invalide ou expiré (validité : 24h).

### POST /auth/login — Connexion

```json
// Requête
{ "email": "lucas.martin@mail.fr", "mot_de_passe": "S3curePass!" }

// 200
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "user": { "id": 1, "prenom": "Lucas", "nom": "Martin", "role": "adherent" }
}
```

Erreurs : `401` "identifiants incorrects" (email inconnu OU mauvais mot de passe, volontairement indistincts) · `403` compte non validé / suspendu / résilié.

### POST /auth/refresh — Rafraîchir les tokens

```json
// Requête
{ "refresh_token": "eyJ..." }

// 200 : même format que le login (sans "user"), NOUVELLE paire de tokens
```

Erreurs : `401` refresh invalide/expiré → rediriger vers login · `403` compte suspendu entre-temps.

---

## 2. Formules

### GET /formules — Liste des formules actives (PUBLIC)

```json
// 200
{
  "formules": [
    {
      "id": 1,
      "nom": "Découverte",
      "description": "Pour tester en douceur",
      "tarif_ht": 24.99,
      "taux_tva": 20,
      "tarif_ttc": 29.99,
      "duree_engagement": 0,        // 0 ou null = sans engagement
      "nombre_seances": 4,          // null = ILLIMITÉ
      "acces_multi_salles": false
    }
  ]
}
```

Triées par prix croissant. `nombre_seances: null` → afficher « Illimité ».

---

## 3. Séances (planning)

### GET /seances — Planning des séances à venir

Filtres optionnels et cumulables : `?salle=1&coach=2&date=2026-07-20`

```json
// 200
{
  "seances": [
    {
      "id": 1,
      "intitule": "Yoga Flow",
      "description": "Séance tous niveaux",
      "salle_id": 1,
      "salle_nom": "FitFlow République",
      "coach_id": 1,
      "coach_nom": "Sofia Diallo",
      "date_heure": "2026-07-20T18:00:00Z",
      "duree": 60,
      "capacite_max": 12,
      "places_restantes": 8,
      "statut": "programmee"
    }
  ]
}
```

Ne renvoie que les séances **programmées et futures**, triées chronologiquement.

---

## 4. Adhérent

Un adhérent n'accède qu'à **ses propres** données (`:id` = son `adherent_id`, renvoyé au register et dans `GET /adherents/:id`). Accéder à un autre id → 404.

### GET /adherents/:id — Profil + abonnement (le dashboard en 1 appel)

```json
// 200
{
  "profil": {
    "adherent_id": 1,
    "nom": "Martin",
    "prenom": "Lucas",
    "email": "lucas.martin@mail.fr",
    "telephone": "0612345678",
    "date_naissance": "2003-04-12T00:00:00Z",
    "adresse": "12 rue de la Paix, 75002 Paris",
    "salle_id": 1,
    "salle_nom": "FitFlow République",
    "statut": "actif"
  },
  "abonnement": {                    // null si aucun abonnement actif
    "id": 1,
    "formule": "Standard",
    "date_debut": "2026-07-01T00:00:00Z",
    "date_fin": "2027-07-01T00:00:00Z",
    "seances_restantes": 7,          // null = illimité
    "acces_multi_salles": false
  }
}
```

### PUT /adherents/:id — Modifier le profil

Champs modifiables (tous optionnels, envoyer seulement ceux qui changent) :

```json
{ "nom": "...", "prenom": "...", "telephone": "...", "adresse": "..." }
```

`200` `{"message": "profil mis à jour"}` · `400` si aucun champ fourni.
(Email et mot de passe : parcours dédiés, pas encore disponibles.)

### PUT /adherents/:id/salle — Changer de salle principale

```json
// Requête
{ "salle_id": 2 }

// 200
{ "message": "salle principale mise à jour", "salle_id": 2 }

// 409 si formule sans multi-salles et changement < 30 jours :
{
  "error": "votre formule ne permet qu'un changement de salle tous les 30 jours",
  "prochain_changement": "2026-08-14"    // à afficher à l'utilisateur
}
```

Autres erreurs : `404` salle inconnue · `409` salle fermée, ou déjà la salle actuelle.

### GET /adherents/:id/reservations — Historique réservations

```json
// 200
{
  "reservations": [
    {
      "id": 1,
      "date_reservation": "2026-07-15T14:30:00Z",
      "statut": "confirmee",          // confirmee | annulee | present | absent
      "seance_id": 1,
      "intitule": "Yoga Flow",
      "date_heure": "2026-07-20T18:00:00Z",
      "salle": "FitFlow République"
    }
  ]
}
```

Triées de la plus récente à la plus ancienne. Filtrer côté front : `date_heure > now && statut == "confirmee"` = "à venir".

### GET /adherents/:id/paiements — Historique paiements

```json
// 200
{
  "paiements": [
    {
      "id": 1,
      "date_paiement": "2026-07-01T00:00:00Z",
      "montant_ttc": 49.99,
      "mode_paiement": "carte",
      "statut": "valide",             // en attente | valide | echoue | rembourse
      "reference_transaction": "pi_3Nxxx",
      "formule": "Standard"
    }
  ]
}
```

---

## 5. Réservations

### POST /reservations — Réserver une séance

```json
// Requête
{ "seance_id": 1 }

// 201
{
  "message": "réservation confirmée",
  "reservation_id": 1,
  "seance_id": 1,
  "places_restantes": 7
}
```

Erreurs (chaque message est affichable tel quel) :

| Code | Cas |
|---|---|
| 403 | Pas d'abonnement actif |
| 403 | Formule sans multi-salles + séance hors salle principale |
| 403 | Quota mensuel de séances épuisé |
| 409 | Séance complète |
| 409 | Moins d'1h avant le début |
| 409 | Déjà réservé cette séance |
| 409 | Séance annulée/terminée |
| 404 | Séance inconnue |

### DELETE /reservations/:id — Annuler une réservation

`200` `{"message": "réservation annulée, la place est libérée"}`

Erreurs : `409` moins de 2h avant le début · `409` déjà annulée · `404` réservation inconnue (ou pas la tienne). La séance est restituée au quota si la formule est limitée.

---

## 6. À venir (pas encore disponibles → 501)

`POST /abonnements` (souscription + Stripe) · `PUT /abonnements/:id/resilier` · `PUT /abonnements/:id/changer-formule` · `POST /seances` et `DELETE /seances/:id` (gestionnaire/coach) · `PUT /reservations/:id/presence` (coach).

En attendant la souscription, créer les abonnements de test directement en base (voir `seed.sql`).
