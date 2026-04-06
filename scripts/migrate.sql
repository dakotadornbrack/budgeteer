-- Run this once to set up the schema.
-- In production, use a migration tool like golang-migrate.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS transactions (
    id           TEXT        PRIMARY KEY,
    description  TEXT        NOT NULL,
    amount_cents BIGINT      NOT NULL CHECK (amount_cents > 0),
    category     TEXT        NOT NULL,
    date         DATE        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions (category);
CREATE INDEX IF NOT EXISTS idx_transactions_date     ON transactions (date DESC);

CREATE TABLE IF NOT EXISTS budgets (
    id           TEXT        PRIMARY KEY,
    category     TEXT        NOT NULL,
    limit_cents  BIGINT      NOT NULL CHECK (limit_cents > 0),
    month        SMALLINT    NOT NULL CHECK (month BETWEEN 1 AND 12),
    year         SMALLINT    NOT NULL CHECK (year BETWEEN 2000 AND 2100),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (category, month, year)
);
