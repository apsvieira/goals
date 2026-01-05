-- Add sync timestamps to goals
ALTER TABLE goals ADD COLUMN updated_at TIMESTAMPTZ DEFAULT NOW();
ALTER TABLE goals ADD COLUMN deleted_at TIMESTAMPTZ;

-- Add sync timestamps to completions
ALTER TABLE completions ADD COLUMN updated_at TIMESTAMPTZ DEFAULT NOW();
ALTER TABLE completions ADD COLUMN deleted_at TIMESTAMPTZ;

-- Index for sync queries
CREATE INDEX idx_goals_updated_at ON goals(updated_at);
CREATE INDEX idx_completions_updated_at ON completions(updated_at);
