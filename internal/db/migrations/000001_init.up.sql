-- Mirrors the MongoDB collections of the Kotlin service (pub.edholm.db.Swosh
-- and pub.edholm.db.User). There are no CHECK constraints on amount or
-- description on purpose: the old admin UI never validated edits, so imported
-- rows may break the rules the API enforces on new links.

CREATE TABLE swosh (
    id          text        PRIMARY KEY,
    payee       text        NOT NULL,
    amount      integer     NOT NULL,
    description text,
    expires_on  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX swosh_expires_on_idx ON swosh (expires_on) WHERE expires_on IS NOT NULL;

CREATE TABLE users (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    username    text        NOT NULL UNIQUE,
    -- Spring Security DelegatingPasswordEncoder format, e.g. "{bcrypt}$2a$10$…".
    password    text        NOT NULL,
    enabled     boolean     NOT NULL DEFAULT true,
    authorities text[]      NOT NULL DEFAULT '{USER}',
    created_at  timestamptz NOT NULL DEFAULT now()
);
