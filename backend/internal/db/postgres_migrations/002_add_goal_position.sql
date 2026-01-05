-- Add position column for goal ordering
ALTER TABLE goals ADD COLUMN IF NOT EXISTS position INTEGER NOT NULL DEFAULT 0;

-- Set initial positions based on creation order
UPDATE goals SET position = subquery.row_num - 1
FROM (
    SELECT id, ROW_NUMBER() OVER (ORDER BY created_at) as row_num
    FROM goals
) AS subquery
WHERE goals.id = subquery.id;
