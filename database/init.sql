CREATE TABLE utilisateurs (
    id               SERIAL PRIMARY KEY,
    nom              VARCHAR(100) NOT NULL,
    prenom           VARCHAR(100) NOT NULL,
    email            VARCHAR(255) NOT NULL UNIQUE,
    mot_de_passe     VARCHAR(255) NOT NULL,
    telephone        VARCHAR(20),
    date_inscription DATE NOT NULL DEFAULT CURRENT_DATE,
    statut           VARCHAR(20) NOT NULL DEFAULT 'actif',
    role             VARCHAR(20) NOT NULL
);
 
CREATE TABLE salles (
    id               SERIAL PRIMARY KEY,
    nom              VARCHAR(100) NOT NULL,
    adresse          VARCHAR(255),
    telephone        VARCHAR(20),
    heures_ouverture VARCHAR(100),
    capacite_max     INTEGER,
    statut           VARCHAR(20) NOT NULL DEFAULT 'ouverte'
);
 
CREATE TABLE adherents (
    id             SERIAL PRIMARY KEY,
    utilisateur_id INTEGER NOT NULL,
    date_naissance DATE,
    adresse        VARCHAR(255),
    salle_id       INTEGER,
    FOREIGN KEY (utilisateur_id) REFERENCES utilisateurs(id) ON DELETE CASCADE,
    FOREIGN KEY (salle_id) REFERENCES salles(id) ON DELETE RESTRICT
);
 
CREATE TABLE gestionnaires (
    id             SERIAL PRIMARY KEY,
    utilisateur_id INTEGER NOT NULL,
    role           VARCHAR(50),
    FOREIGN KEY (utilisateur_id) REFERENCES utilisateurs(id) ON DELETE CASCADE
);
 
CREATE TABLE coachs (
    id             SERIAL PRIMARY KEY,
    utilisateur_id INTEGER NOT NULL,
    specialites    VARCHAR(255),
    FOREIGN KEY (utilisateur_id) REFERENCES utilisateurs(id) ON DELETE CASCADE
);
 
CREATE TABLE formules (
    id                 SERIAL PRIMARY KEY,
    nom                VARCHAR(100) NOT NULL,
    description        TEXT,
    tarif_ht           NUMERIC(10,2) NOT NULL,
    taux_tva           NUMERIC(5,2) NOT NULL DEFAULT 20.00,
    tarif_ttc          NUMERIC(10,2) NOT NULL,
    duree_engagement   INTEGER,
    nombre_seances     INTEGER,
    acces_multi_salles BOOLEAN NOT NULL DEFAULT FALSE,
    statut             VARCHAR(20) NOT NULL DEFAULT 'active'
);
 
CREATE TABLE abonnements (
    id                SERIAL PRIMARY KEY,
    adherent_id       INTEGER NOT NULL,
    formule_id        INTEGER NOT NULL,
    date_debut        DATE NOT NULL,
    date_fin          DATE,
    statut            VARCHAR(20) NOT NULL DEFAULT 'en cours',
    date_resiliation  DATE,
    motif_resiliation VARCHAR(255),
    seances_restantes INTEGER,
    FOREIGN KEY (adherent_id) REFERENCES adherents(id) ON DELETE CASCADE,
    FOREIGN KEY (formule_id) REFERENCES formules(id) ON DELETE RESTRICT
);
 
CREATE TABLE paiements (
    id                    SERIAL PRIMARY KEY,
    abonnement_id         INTEGER NOT NULL,
    date_paiement         DATE NOT NULL DEFAULT CURRENT_DATE,
    montant_ttc           NUMERIC(10,2) NOT NULL,
    mode_paiement         VARCHAR(30),
    statut                VARCHAR(20) NOT NULL DEFAULT 'en attente',
    reference_transaction VARCHAR(255),
    FOREIGN KEY (abonnement_id) REFERENCES abonnements(id) ON DELETE CASCADE
);
 
CREATE TABLE coach_salle (
    coach_id INTEGER NOT NULL,
    salle_id INTEGER NOT NULL,
    PRIMARY KEY (coach_id, salle_id),
    FOREIGN KEY (coach_id) REFERENCES coachs(id) ON DELETE CASCADE,
    FOREIGN KEY (salle_id) REFERENCES salles(id) ON DELETE CASCADE
);
 
CREATE TABLE seances (
    id               SERIAL PRIMARY KEY,
    intitule         VARCHAR(150) NOT NULL,
    description      TEXT,
    salle_id         INTEGER NOT NULL,
    coach_id         INTEGER NOT NULL,
    date_heure       TIMESTAMP NOT NULL,
    duree            INTEGER,
    capacite_max     INTEGER NOT NULL,
    places_restantes INTEGER NOT NULL,
    statut           VARCHAR(20) NOT NULL DEFAULT 'programmee',
    FOREIGN KEY (salle_id) REFERENCES salles(id) ON DELETE RESTRICT,
    FOREIGN KEY (coach_id) REFERENCES coachs(id) ON DELETE RESTRICT
);
 
CREATE TABLE reservations (
    id               SERIAL PRIMARY KEY,
    adherent_id      INTEGER NOT NULL,
    seance_id        INTEGER NOT NULL,
    date_reservation TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    statut           VARCHAR(20) NOT NULL DEFAULT 'confirmee',
    FOREIGN KEY (adherent_id) REFERENCES adherents(id) ON DELETE CASCADE,
    FOREIGN KEY (seance_id) REFERENCES seances(id) ON DELETE CASCADE
);
 
CREATE TABLE historique_salles (
    id                SERIAL PRIMARY KEY,
    adherent_id       INTEGER NOT NULL,
    ancienne_salle_id INTEGER,
    nouvelle_salle_id INTEGER NOT NULL,
    date_changement   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (adherent_id) REFERENCES adherents(id) ON DELETE CASCADE,
    FOREIGN KEY (ancienne_salle_id) REFERENCES salles(id) ON DELETE RESTRICT,
    FOREIGN KEY (nouvelle_salle_id) REFERENCES salles(id) ON DELETE RESTRICT
);