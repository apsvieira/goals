-- Add sync timestamps to goals
-- Note: SQLite doesn't support non-constant defaults in ALTER TABLE
-- So we add as NULL, then update existing rows
ALTER TABLE goals ADD COLUMN updated_at DATETIME;
ALTER TABLE goals ADD COLUMN deleted_at DATETIME;

-- Set updated_at for existing rows to current time
UPDATE goals SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL;

-- Add sync timestamps to completions
ALTER TABLE completions ADD COLUMN updated_at DATETIME;
ALTER TABLE completions ADD COLUMN deleted_at DATETIME;

-- Set updated_at for existing rows to current time
UPDATE completions SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL;

-- Index for sync queries
CREATE INDEX idx_goals_updated_at ON goals(updated_at);
CREATE INDEX idx_completions_updated_at ON completions(updated_at);
