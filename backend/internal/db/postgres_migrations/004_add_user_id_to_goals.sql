-- Add user_id column to goals (nullable for backward compatibility with existing data)
ALTER TABLE goals ADD COLUMN user_id TEXT REFERENCES users(id) ON DELETE CASCADE;

-- Add index for user queries
CREATE INDEX idx_goals_user_id ON goals(user_id);
