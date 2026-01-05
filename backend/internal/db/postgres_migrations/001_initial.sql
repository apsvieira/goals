-- Goals table
CREATE TABLE IF NOT EXISTS goals (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    color TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ
);

-- Completions table
CREATE TABLE IF NOT EXISTS completions (
    id UUID PRIMARY KEY,
    goal_id UUID NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(goal_id, date)
);

-- Index for faster date range queries
CREATE INDEX IF NOT EXISTS idx_completions_date ON completions(date);
CREATE INDEX IF NOT EXISTS idx_completions_goal_id ON completions(goal_id);
