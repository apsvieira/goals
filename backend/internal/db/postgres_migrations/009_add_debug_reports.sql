-- Debug reports for on-device diagnostics collection (shake-to-report + auto-capture).
-- Retention: 90 days (enforced by StartDebugReportsCleanup goroutine).
-- Note: user_id references users.id which is TEXT in this schema (not UUID),
-- so user_id and client_id are stored as TEXT matching the existing convention.
CREATE TABLE IF NOT EXISTS debug_reports (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id   TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    trigger     TEXT NOT NULL CHECK (trigger IN ('shake','auto')),
    app_version TEXT NOT NULL,
    platform    TEXT NOT NULL,
    device      JSONB NOT NULL,
    state       JSONB NOT NULL,
    description TEXT,
    breadcrumbs JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_debug_reports_user_created ON debug_reports (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_debug_reports_created      ON debug_reports (created_at);
