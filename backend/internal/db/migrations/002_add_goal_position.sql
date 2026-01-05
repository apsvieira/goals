-- Add position column for goal ordering
ALTER TABLE goals ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

-- Set initial positions based on creation order
UPDATE goals SET position = (
    SELECT COUNT(*) FROM goals g2 WHERE g2.created_at <= goals.created_at
);
