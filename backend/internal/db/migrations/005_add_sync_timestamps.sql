-- Add sync timestamps to goals
ALTER TABLE goals ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE goals ADD COLUMN deleted_at DATETIME;

-- Add sync timestamps to completions
ALTER TABLE completions ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE completions ADD COLUMN deleted_at DATETIME;

-- Index for sync queries
CREATE INDEX idx_goals_updated_at ON goals(updated_at);
CREATE INDEX idx_completions_updated_at ON completions(updated_at);
