-- 001_init.sql — CodeArena playground schema.
-- pgcrypto provides gen_random_uuid() for the runs primary key.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per "Run" click. status lifecycle:
--   queued -> running -> success | compile_error | runtime_error
--                        | time_limit_exceeded | internal_error
CREATE TABLE IF NOT EXISTS runs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    BIGINT NOT NULL REFERENCES users(id),
    language   TEXT NOT NULL DEFAULT 'go',
    code       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'queued',
    output     TEXT NOT NULL DEFAULT '',
    error      TEXT NOT NULL DEFAULT '',
    exit_code  INT NOT NULL DEFAULT 0,
    runtime_ms INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- "My recent runs", newest first, is the hot query.
CREATE INDEX IF NOT EXISTS idx_runs_user_created
    ON runs (user_id, created_at DESC);
