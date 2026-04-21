-- Debug reports for on-device diagnostics collection (shake-to-report + auto-capture).
-- Retention: 90 days (enforced by StartDebugReportsCleanup goroutine).
-- SQLite mirror of the Postgres migration; JSONB becomes TEXT here.
CREATE TABLE IF NOT EXISTS debug_reports (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id   TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    trigger     TEXT NOT NULL CHECK (trigger IN ('shake','auto')),
    app_version TEXT NOT NULL,
    platform    TEXT NOT NULL,
    device      TEXT NOT NULL,
    state       TEXT NOT NULL,
    description TEXT,
    breadcrumbs TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_debug_reports_user_created ON debug_reports (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_debug_reports_created      ON debug_reports (created_at);
