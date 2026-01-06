-- Add target columns for weekly/monthly goals
ALTER TABLE goals ADD COLUMN target_count INTEGER;
ALTER TABLE goals ADD COLUMN target_period TEXT;
