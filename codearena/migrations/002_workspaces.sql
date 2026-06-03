-- Workspaces: a persistent, per-user project (the "Repl"). Each workspace maps
-- to a set of Kubernetes objects (Deployment + PVC + Service + Ingress) managed
-- by internal/workspace. The row is the source of truth for desired state;
-- `status` tracks running vs stopped (hibernated).
CREATE TABLE IF NOT EXISTS workspaces (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- pgcrypto enabled in 001_init
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT   NOT NULL,
    image      TEXT   NOT NULL DEFAULT '',   -- workspace base image (empty => server default)
    status     TEXT   NOT NULL DEFAULT 'stopped', -- creating | running | stopped | error
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workspaces_user ON workspaces (user_id, created_at DESC);
