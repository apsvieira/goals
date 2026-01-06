package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

// Goals

func (d *SQLiteDB) ListGoals(userID *string, includeArchived bool) ([]models.Goal, error) {
	query := `SELECT id, name, color, position, target_count, target_period, user_id, created_at, updated_at, archived_at, deleted_at FROM goals WHERE `
	var args []any

	// Filter by user_id
	if userID == nil {
		query += `user_id IS NULL`
	} else {
		query += `user_id = ?`
		args = append(args, *userID)
	}

	if !includeArchived {
		query += ` AND archived_at IS NULL`
	}
	// Always exclude soft-deleted goals
	query += ` AND deleted_at IS NULL`
	query += ` ORDER BY position ASC, created_at ASC`

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query goals: %w", err)
	}
	defer rows.Close()

	var goals []models.Goal
	for rows.Next() {
		var g models.Goal
		var archivedAt, deletedAt sql.NullTime
		var updatedAt sql.NullTime
		var goalUserID sql.NullString
		var targetCount sql.NullInt64
		var targetPeriod sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &g.Color, &g.Position, &targetCount, &targetPeriod, &goalUserID, &g.CreatedAt, &updatedAt, &archivedAt, &deletedAt); err != nil {
			return nil, fmt.Errorf("scan goal: %w", err)
		}
		if archivedAt.Valid {
			g.ArchivedAt = &archivedAt.Time
		}
		if deletedAt.Valid {
			g.DeletedAt = &deletedAt.Time
		}
		if updatedAt.Valid {
			g.UpdatedAt = updatedAt.Time
		} else {
			g.UpdatedAt = g.CreatedAt
		}
		if goalUserID.Valid {
			g.UserID = &goalUserID.String
		}
		if targetCount.Valid {
			tc := int(targetCount.Int64)
			g.TargetCount = &tc
		}
		if targetPeriod.Valid {
			g.TargetPeriod = &targetPeriod.String
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (d *SQLiteDB) GetGoal(userID *string, id string) (*models.Goal, error) {
	var g models.Goal
	var archivedAt, deletedAt sql.NullTime
	var updatedAt sql.NullTime
	var goalUserID sql.NullString
	var targetCount sql.NullInt64
	var targetPeriod sql.NullString

	query := `SELECT id, name, color, position, target_count, target_period, user_id, created_at, updated_at, archived_at, deleted_at FROM goals WHERE id = ?`
	args := []any{id}

	// Add user_id filter
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += ` AND user_id = ?`
		args = append(args, *userID)
	}

	err := d.QueryRow(query, args...).Scan(&g.ID, &g.Name, &g.Color, &g.Position, &targetCount, &targetPeriod, &goalUserID, &g.CreatedAt, &updatedAt, &archivedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query goal: %w", err)
	}
	if archivedAt.Valid {
		g.ArchivedAt = &archivedAt.Time
	}
	if deletedAt.Valid {
		g.DeletedAt = &deletedAt.Time
	}
	if updatedAt.Valid {
		g.UpdatedAt = updatedAt.Time
	} else {
		g.UpdatedAt = g.CreatedAt
	}
	if goalUserID.Valid {
		g.UserID = &goalUserID.String
	}
	if targetCount.Valid {
		tc := int(targetCount.Int64)
		g.TargetCount = &tc
	}
	if targetPeriod.Valid {
		g.TargetPeriod = &targetPeriod.String
	}
	return &g, nil
}

func (d *SQLiteDB) CreateGoal(g *models.Goal) error {
	// Get next position for this user's goals
	var maxPos sql.NullInt64
	if g.UserID == nil {
		d.QueryRow(`SELECT MAX(position) FROM goals WHERE user_id IS NULL AND deleted_at IS NULL`).Scan(&maxPos)
	} else {
		d.QueryRow(`SELECT MAX(position) FROM goals WHERE user_id = ? AND deleted_at IS NULL`, *g.UserID).Scan(&maxPos)
	}
	nextPos := 0
	if maxPos.Valid {
		nextPos = int(maxPos.Int64) + 1
	}
	g.Position = nextPos

	now := time.Now().UTC()
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = now
	}

	_, err := d.Exec(
		`INSERT INTO goals (id, name, color, position, target_count, target_period, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Color, g.Position, g.TargetCount, g.TargetPeriod, g.UserID, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert goal: %w", err)
	}
	return nil
}

func (d *SQLiteDB) UpdateGoal(userID *string, id string, name, color *string, targetCount *int, targetPeriod *string) error {
	if name == nil && color == nil && targetCount == nil && targetPeriod == nil {
		return nil
	}

	query := `UPDATE goals SET `
	var args []any
	var updates []string

	if name != nil {
		updates = append(updates, `name = ?`)
		args = append(args, *name)
	}
	if color != nil {
		updates = append(updates, `color = ?`)
		args = append(args, *color)
	}
	if targetCount != nil {
		updates = append(updates, `target_count = ?`)
		args = append(args, *targetCount)
	}
	if targetPeriod != nil {
		updates = append(updates, `target_period = ?`)
		args = append(args, *targetPeriod)
	}

	// Always update updated_at
	updates = append(updates, `updated_at = ?`)
	args = append(args, time.Now().UTC())

	for i, u := range updates {
		if i > 0 {
			query += ", "
		}
		query += u
	}
	query += ` WHERE id = ?`
	args = append(args, id)

	// Add user_id filter for ownership verification
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += ` AND user_id = ?`
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update goal: %w", err)
	}
	return nil
}

func (d *SQLiteDB) ArchiveGoal(userID *string, id string) error {
	now := time.Now().UTC()
	query := `UPDATE goals SET archived_at = ?, updated_at = ? WHERE id = ?`
	args := []any{now, now, id}

	// Add user_id filter for ownership verification
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += ` AND user_id = ?`
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("archive goal: %w", err)
	}
	return nil
}

// Completions

func (d *SQLiteDB) ListCompletions(from, to string, goalID *string) ([]models.Completion, error) {
	query := `SELECT id, goal_id, date, created_at, updated_at, deleted_at FROM completions WHERE date >= ? AND date <= ?`
	args := []any{from, to}

	if goalID != nil {
		query += ` AND goal_id = ?`
		args = append(args, *goalID)
	}
	// Exclude soft-deleted completions
	query += ` AND deleted_at IS NULL`
	query += ` ORDER BY date ASC`

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query completions: %w", err)
	}
	defer rows.Close()

	var completions []models.Completion
	for rows.Next() {
		var c models.Completion
		var updatedAt sql.NullTime
		var deletedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt, &updatedAt, &deletedAt); err != nil {
			return nil, fmt.Errorf("scan completion: %w", err)
		}
		if updatedAt.Valid {
			c.UpdatedAt = updatedAt.Time
		} else {
			c.UpdatedAt = c.CreatedAt
		}
		if deletedAt.Valid {
			c.DeletedAt = &deletedAt.Time
		}
		completions = append(completions, c)
	}
	return completions, rows.Err()
}

func (d *SQLiteDB) GetCompletionByGoalAndDate(goalID, date string) (*models.Completion, error) {
	var c models.Completion
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, goal_id, date, created_at, updated_at, deleted_at FROM completions WHERE goal_id = ? AND date = ? AND deleted_at IS NULL`,
		goalID, date,
	).Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt, &updatedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query completion: %w", err)
	}
	if updatedAt.Valid {
		c.UpdatedAt = updatedAt.Time
	} else {
		c.UpdatedAt = c.CreatedAt
	}
	if deletedAt.Valid {
		c.DeletedAt = &deletedAt.Time
	}
	return &c, nil
}

func (d *SQLiteDB) CreateCompletion(c *models.Completion) error {
	now := time.Now().UTC()
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}

	_, err := d.Exec(
		`INSERT INTO completions (id, goal_id, date, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.GoalID, c.Date, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert completion: %w", err)
	}
	return nil
}

func (d *SQLiteDB) DeleteCompletion(id string) error {
	_, err := d.Exec(`DELETE FROM completions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete completion: %w", err)
	}
	return nil
}

func (d *SQLiteDB) ReorderGoals(userID *string, goalIDs []string) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i, id := range goalIDs {
		query := `UPDATE goals SET position = ? WHERE id = ?`
		args := []any{i, id}

		// Add user_id filter for ownership verification
		if userID == nil {
			query += ` AND user_id IS NULL`
		} else {
			query += ` AND user_id = ?`
			args = append(args, *userID)
		}

		_, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("update position for %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// Users

func (d *SQLiteDB) GetUserByID(id string) (*models.User, error) {
	var u models.User
	var lastLoginAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, email, name, avatar_url, created_at, last_login_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &lastLoginAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}
	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	return &u, nil
}

func (d *SQLiteDB) GetUserByEmail(email string) (*models.User, error) {
	var u models.User
	var lastLoginAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, email, name, avatar_url, created_at, last_login_at FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &lastLoginAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}
	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	return &u, nil
}

func (d *SQLiteDB) CreateUser(u *models.User) error {
	_, err := d.Exec(
		`INSERT INTO users (id, email, name, avatar_url, created_at) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.AvatarURL, u.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (d *SQLiteDB) UpdateUserLastLogin(id string) error {
	_, err := d.Exec(
		`UPDATE users SET last_login_at = ? WHERE id = ?`,
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

func (d *SQLiteDB) GetOrCreateUserByProvider(provider, providerUserID, email, name, avatarURL string) (*models.User, error) {
	tx, err := d.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if auth provider exists
	var userID string
	err = tx.QueryRow(
		`SELECT user_id FROM auth_providers WHERE provider = ? AND provider_user_id = ?`,
		provider, providerUserID,
	).Scan(&userID)

	if err == nil {
		// Provider exists, get user and update last login
		var u models.User
		var lastLoginAt sql.NullTime
		err = tx.QueryRow(
			`SELECT id, email, name, avatar_url, created_at, last_login_at FROM users WHERE id = ?`,
			userID,
		).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &lastLoginAt)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		if lastLoginAt.Valid {
			u.LastLoginAt = &lastLoginAt.Time
		}

		// Update last login
		now := time.Now().UTC()
		_, err = tx.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, now, u.ID)
		if err != nil {
			return nil, fmt.Errorf("update last login: %w", err)
		}
		u.LastLoginAt = &now

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit: %w", err)
		}
		return &u, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check auth provider: %w", err)
	}

	// Check if user exists by email
	var existingUser models.User
	var lastLoginAt sql.NullTime
	err = tx.QueryRow(
		`SELECT id, email, name, avatar_url, created_at, last_login_at FROM users WHERE email = ?`,
		email,
	).Scan(&existingUser.ID, &existingUser.Email, &existingUser.Name, &existingUser.AvatarURL, &existingUser.CreatedAt, &lastLoginAt)

	if err == nil {
		// User exists, add auth provider
		if lastLoginAt.Valid {
			existingUser.LastLoginAt = &lastLoginAt.Time
		}
		providerID := generateUUID()
		_, err = tx.Exec(
			`INSERT INTO auth_providers (id, user_id, provider, provider_user_id) VALUES (?, ?, ?, ?)`,
			providerID, existingUser.ID, provider, providerUserID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert auth provider: %w", err)
		}

		// Update last login
		now := time.Now().UTC()
		_, err = tx.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, now, existingUser.ID)
		if err != nil {
			return nil, fmt.Errorf("update last login: %w", err)
		}
		existingUser.LastLoginAt = &now

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit: %w", err)
		}
		return &existingUser, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check user by email: %w", err)
	}

	// Create new user
	now := time.Now().UTC()
	u := &models.User{
		ID:          generateUUID(),
		Email:       email,
		Name:        name,
		AvatarURL:   avatarURL,
		CreatedAt:   now,
		LastLoginAt: &now,
	}

	_, err = tx.Exec(
		`INSERT INTO users (id, email, name, avatar_url, created_at, last_login_at) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.AvatarURL, u.CreatedAt, u.LastLoginAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	// Add auth provider
	providerID := generateUUID()
	_, err = tx.Exec(
		`INSERT INTO auth_providers (id, user_id, provider, provider_user_id) VALUES (?, ?, ?, ?)`,
		providerID, u.ID, provider, providerUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert auth provider: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return u, nil
}

// Sessions

func (d *SQLiteDB) CreateSession(s *models.Session) error {
	_, err := d.Exec(
		`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.UserID, s.TokenHash, s.ExpiresAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (d *SQLiteDB) GetSessionByTokenHash(tokenHash string) (*models.Session, error) {
	var s models.Session
	err := d.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, created_at FROM sessions WHERE token_hash = ?`,
		tokenHash,
	).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	return &s, nil
}

func (d *SQLiteDB) DeleteSession(id string) error {
	_, err := d.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (d *SQLiteDB) DeleteExpiredSessions() error {
	_, err := d.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

// generateUUID creates a simple UUID for IDs
func generateUUID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000000)
}

// Sync operations

func (d *SQLiteDB) GetGoalChangesSince(userID *string, since *time.Time) ([]models.Goal, error) {
	query := `SELECT id, name, color, position, target_count, target_period, user_id, created_at, updated_at, archived_at, deleted_at FROM goals WHERE `
	var args []any

	// Filter by user_id
	if userID == nil {
		query += `user_id IS NULL`
	} else {
		query += `user_id = ?`
		args = append(args, *userID)
	}

	// Filter by updated_at if since is provided
	if since != nil {
		query += ` AND updated_at > ?`
		args = append(args, *since)
	}

	query += ` ORDER BY updated_at ASC`

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query goals: %w", err)
	}
	defer rows.Close()

	var goals []models.Goal
	for rows.Next() {
		var g models.Goal
		var archivedAt, deletedAt sql.NullTime
		var updatedAt sql.NullTime
		var goalUserID sql.NullString
		var targetCount sql.NullInt64
		var targetPeriod sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &g.Color, &g.Position, &targetCount, &targetPeriod, &goalUserID, &g.CreatedAt, &updatedAt, &archivedAt, &deletedAt); err != nil {
			return nil, fmt.Errorf("scan goal: %w", err)
		}
		if archivedAt.Valid {
			g.ArchivedAt = &archivedAt.Time
		}
		if deletedAt.Valid {
			g.DeletedAt = &deletedAt.Time
		}
		if updatedAt.Valid {
			g.UpdatedAt = updatedAt.Time
		} else {
			g.UpdatedAt = g.CreatedAt
		}
		if goalUserID.Valid {
			g.UserID = &goalUserID.String
		}
		if targetCount.Valid {
			tc := int(targetCount.Int64)
			g.TargetCount = &tc
		}
		if targetPeriod.Valid {
			g.TargetPeriod = &targetPeriod.String
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (d *SQLiteDB) GetCompletionChangesSince(userID *string, since *time.Time) ([]models.Completion, error) {
	// For completions, we need to join with goals to filter by user_id
	query := `SELECT c.id, c.goal_id, c.date, c.created_at, c.updated_at, c.deleted_at
		FROM completions c
		INNER JOIN goals g ON c.goal_id = g.id
		WHERE `
	var args []any

	// Filter by user_id through goals
	if userID == nil {
		query += `g.user_id IS NULL`
	} else {
		query += `g.user_id = ?`
		args = append(args, *userID)
	}

	// Filter by updated_at if since is provided
	if since != nil {
		query += ` AND c.updated_at > ?`
		args = append(args, *since)
	}

	query += ` ORDER BY c.updated_at ASC`

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query completions: %w", err)
	}
	defer rows.Close()

	var completions []models.Completion
	for rows.Next() {
		var c models.Completion
		var updatedAt sql.NullTime
		var deletedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt, &updatedAt, &deletedAt); err != nil {
			return nil, fmt.Errorf("scan completion: %w", err)
		}
		if updatedAt.Valid {
			c.UpdatedAt = updatedAt.Time
		} else {
			c.UpdatedAt = c.CreatedAt
		}
		if deletedAt.Valid {
			c.DeletedAt = &deletedAt.Time
		}
		completions = append(completions, c)
	}
	return completions, rows.Err()
}

func (d *SQLiteDB) UpsertGoal(goal *models.Goal) error {
	now := time.Now().UTC()
	if goal.UpdatedAt.IsZero() {
		goal.UpdatedAt = now
	}

	_, err := d.Exec(`
		INSERT INTO goals (id, name, color, position, target_count, target_period, user_id, created_at, updated_at, archived_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			color = excluded.color,
			position = excluded.position,
			target_count = excluded.target_count,
			target_period = excluded.target_period,
			updated_at = excluded.updated_at,
			archived_at = excluded.archived_at,
			deleted_at = excluded.deleted_at
		WHERE excluded.updated_at > goals.updated_at
	`, goal.ID, goal.Name, goal.Color, goal.Position, goal.TargetCount, goal.TargetPeriod, goal.UserID, goal.CreatedAt, goal.UpdatedAt, goal.ArchivedAt, goal.DeletedAt)

	if err != nil {
		return fmt.Errorf("upsert goal: %w", err)
	}
	return nil
}

func (d *SQLiteDB) UpsertCompletion(c *models.Completion) error {
	now := time.Now().UTC()
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}

	_, err := d.Exec(`
		INSERT INTO completions (id, goal_id, date, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			updated_at = excluded.updated_at,
			deleted_at = excluded.deleted_at
		WHERE excluded.updated_at > completions.updated_at
	`, c.ID, c.GoalID, c.Date, c.CreatedAt, c.UpdatedAt, c.DeletedAt)

	if err != nil {
		return fmt.Errorf("upsert completion: %w", err)
	}
	return nil
}

func (d *SQLiteDB) SoftDeleteGoal(userID *string, id string) error {
	now := time.Now().UTC()
	query := `UPDATE goals SET deleted_at = ?, updated_at = ? WHERE id = ?`
	args := []any{now, now, id}

	// Add user_id filter for ownership verification
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += ` AND user_id = ?`
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("soft delete goal: %w", err)
	}
	return nil
}

func (d *SQLiteDB) SoftDeleteCompletion(goalID, date string) error {
	now := time.Now().UTC()
	_, err := d.Exec(
		`UPDATE completions SET deleted_at = ?, updated_at = ? WHERE goal_id = ? AND date = ?`,
		now, now, goalID, date,
	)
	if err != nil {
		return fmt.Errorf("soft delete completion: %w", err)
	}
	return nil
}

func (d *SQLiteDB) GetGoalByID(id string) (*models.Goal, error) {
	var g models.Goal
	var archivedAt, deletedAt sql.NullTime
	var updatedAt sql.NullTime
	var goalUserID sql.NullString
	var targetCount sql.NullInt64
	var targetPeriod sql.NullString

	err := d.QueryRow(
		`SELECT id, name, color, position, target_count, target_period, user_id, created_at, updated_at, archived_at, deleted_at FROM goals WHERE id = ?`,
		id,
	).Scan(&g.ID, &g.Name, &g.Color, &g.Position, &targetCount, &targetPeriod, &goalUserID, &g.CreatedAt, &updatedAt, &archivedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query goal: %w", err)
	}
	if archivedAt.Valid {
		g.ArchivedAt = &archivedAt.Time
	}
	if deletedAt.Valid {
		g.DeletedAt = &deletedAt.Time
	}
	if updatedAt.Valid {
		g.UpdatedAt = updatedAt.Time
	} else {
		g.UpdatedAt = g.CreatedAt
	}
	if goalUserID.Valid {
		g.UserID = &goalUserID.String
	}
	if targetCount.Valid {
		tc := int(targetCount.Int64)
		g.TargetCount = &tc
	}
	if targetPeriod.Valid {
		g.TargetPeriod = &targetPeriod.String
	}
	return &g, nil
}
