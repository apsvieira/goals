package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
	"github.com/google/uuid"
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
	now := time.Now().UTC()
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = now
	}

	var position int
	if g.UserID == nil {
		err := d.QueryRow(
			`INSERT INTO goals (id, name, color, position, target_count, target_period, user_id, created_at, updated_at)
			 VALUES (?, ?, ?, COALESCE((SELECT MAX(position) FROM goals WHERE user_id IS NULL AND deleted_at IS NULL), -1) + 1, ?, ?, ?, ?, ?)
			 RETURNING position`,
			g.ID, g.Name, g.Color, g.TargetCount, g.TargetPeriod, g.UserID, g.CreatedAt, g.UpdatedAt,
		).Scan(&position)
		if err != nil {
			return fmt.Errorf("insert goal: %w", err)
		}
	} else {
		err := d.QueryRow(
			`INSERT INTO goals (id, name, color, position, target_count, target_period, user_id, created_at, updated_at)
			 VALUES (?, ?, ?, COALESCE((SELECT MAX(position) FROM goals WHERE user_id = ? AND deleted_at IS NULL), -1) + 1, ?, ?, ?, ?, ?)
			 RETURNING position`,
			g.ID, g.Name, g.Color, *g.UserID, g.TargetCount, g.TargetPeriod, g.UserID, g.CreatedAt, g.UpdatedAt,
		).Scan(&position)
		if err != nil {
			return fmt.Errorf("insert goal: %w", err)
		}
	}
	g.Position = position
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

func (d *SQLiteDB) ListCompletions(userID *string, from, to string, goalID *string) ([]models.Completion, error) {
	// Join with goals to filter by user ownership
	query := `SELECT c.id, c.goal_id, c.date, c.created_at, c.updated_at, c.deleted_at
		FROM completions c
		INNER JOIN goals g ON c.goal_id = g.id
		WHERE c.date >= ? AND c.date <= ?`
	args := []any{from, to}

	// Filter by user ownership
	if userID == nil {
		query += ` AND g.user_id IS NULL`
	} else {
		query += ` AND g.user_id = ?`
		args = append(args, *userID)
	}

	if goalID != nil {
		query += ` AND c.goal_id = ?`
		args = append(args, *goalID)
	}
	// Exclude soft-deleted completions
	query += ` AND c.deleted_at IS NULL`
	query += ` ORDER BY c.date ASC`

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

func (d *SQLiteDB) GetCompletionByID(id string) (*models.Completion, error) {
	var c models.Completion
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, goal_id, date, created_at, updated_at, deleted_at FROM completions WHERE id = ? AND deleted_at IS NULL`,
		id,
	).Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt, &updatedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query completion by id: %w", err)
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

func (d *SQLiteDB) GetCompletionByGoalAndDateIncludingDeleted(goalID, date string) (*models.Completion, error) {
	var c models.Completion
	var deletedAt sql.NullTime
	var updatedAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, goal_id, date, created_at, updated_at, deleted_at FROM completions WHERE goal_id = ? AND date = ?`,
		goalID, date,
	).Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt, &updatedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query completion including deleted: %w", err)
	}
	if updatedAt.Valid {
		c.UpdatedAt = updatedAt.Time
	}
	if deletedAt.Valid {
		c.DeletedAt = &deletedAt.Time
	}
	return &c, nil
}

func (d *SQLiteDB) CreateCompletion(c *models.Completion) error {
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = time.Now().UTC()
	}

	_, err := d.Exec(
		`INSERT INTO completions (id, goal_id, date, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (goal_id, date) DO UPDATE SET deleted_at = NULL, updated_at = excluded.updated_at`,
		c.ID, c.GoalID, c.Date, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert completion: %w", err)
	}
	return nil
}

func (d *SQLiteDB) DeleteCompletion(id string) error {
	now := time.Now().UTC()
	_, err := d.Exec(
		`UPDATE completions SET deleted_at = ?, updated_at = ? WHERE id = ?`,
		now, now, id,
	)
	if err != nil {
		return fmt.Errorf("soft delete completion: %w", err)
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

// generateUUID creates a UUID v4 for IDs
func generateUUID() string {
	return uuid.New().String()
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

	// Multi-step upsert to handle SQLite's limitation with multiple unique
	// constraints (PK on id + UNIQUE on goal_id,date).

	// Step 1: Try to update by PK (most common path: same id for same row).
	res, err := d.Exec(`
		UPDATE completions SET
			updated_at = ?,
			deleted_at = ?
		WHERE id = ? AND ? > updated_at
	`, c.UpdatedAt, c.DeletedAt, c.ID, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert completion (update by id): %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected > 0 {
		return nil
	}

	// Check if the row exists by id but wasn't updated (server is newer).
	var existsByID int
	if err := d.QueryRow(`SELECT COUNT(*) FROM completions WHERE id = ?`, c.ID).Scan(&existsByID); err != nil {
		return fmt.Errorf("upsert completion (check id): %w", err)
	}
	if existsByID > 0 {
		return nil // Row exists but server is newer; LWW keeps server version
	}

	// Step 2: Try to update by (goal_id, date) — handles the case where a
	// different id maps to the same (goal_id, date) pair (sync race).
	res, err = d.Exec(`
		UPDATE completions SET
			id = ?,
			updated_at = ?,
			deleted_at = ?
		WHERE goal_id = ? AND date = ? AND ? > updated_at
	`, c.ID, c.UpdatedAt, c.DeletedAt, c.GoalID, c.Date, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert completion (update by goal_date): %w", err)
	}
	rowsAffected, _ = res.RowsAffected()
	if rowsAffected > 0 {
		return nil
	}

	// Step 3: No existing row; insert new.
	_, err = d.Exec(`
		INSERT OR IGNORE INTO completions (id, goal_id, date, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, c.ID, c.GoalID, c.Date, c.CreatedAt, c.UpdatedAt, c.DeletedAt)
	if err != nil {
		return fmt.Errorf("upsert completion (insert): %w", err)
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

func (d *SQLiteDB) SoftDeleteCompletion(userID *string, goalID, date string) error {
	now := time.Now().UTC()
	query := `UPDATE completions SET deleted_at = ?, updated_at = ?
		WHERE goal_id = ? AND date = ?
		AND goal_id IN (SELECT id FROM goals WHERE `
	args := []any{now, now, goalID, date}

	if userID == nil {
		query += `user_id IS NULL)`
	} else {
		query += `user_id = ?)`
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
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

// Device Tokens (Push Notifications)

func (d *SQLiteDB) CreateDeviceToken(userID, token, platform string) (*models.DeviceToken, error) {
	now := time.Now().UTC()
	dt := &models.DeviceToken{
		ID:        generateUUID(),
		UserID:    userID,
		Token:     token,
		Platform:  platform,
		CreatedAt: now,
	}

	_, err := d.Exec(
		`INSERT INTO device_tokens (id, user_id, token, platform, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(token) DO UPDATE SET user_id = excluded.user_id, platform = excluded.platform`,
		dt.ID, dt.UserID, dt.Token, dt.Platform, dt.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert device token: %w", err)
	}

	// If token already existed, fetch the actual record
	var existing models.DeviceToken
	var lastUsedAt sql.NullTime
	err = d.QueryRow(
		`SELECT id, user_id, token, platform, created_at, last_used_at FROM device_tokens WHERE token = ?`,
		token,
	).Scan(&existing.ID, &existing.UserID, &existing.Token, &existing.Platform, &existing.CreatedAt, &lastUsedAt)
	if err != nil {
		return nil, fmt.Errorf("fetch device token: %w", err)
	}
	if lastUsedAt.Valid {
		existing.LastUsedAt = &lastUsedAt.Time
	}
	return &existing, nil
}

func (d *SQLiteDB) GetDeviceTokensByUserID(userID string) ([]models.DeviceToken, error) {
	rows, err := d.Query(
		`SELECT id, user_id, token, platform, created_at, last_used_at FROM device_tokens WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query device tokens: %w", err)
	}
	defer rows.Close()

	var tokens []models.DeviceToken
	for rows.Next() {
		var dt models.DeviceToken
		var lastUsedAt sql.NullTime
		if err := rows.Scan(&dt.ID, &dt.UserID, &dt.Token, &dt.Platform, &dt.CreatedAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("scan device token: %w", err)
		}
		if lastUsedAt.Valid {
			dt.LastUsedAt = &lastUsedAt.Time
		}
		tokens = append(tokens, dt)
	}
	return tokens, rows.Err()
}

func (d *SQLiteDB) DeleteDeviceToken(tokenID string) error {
	_, err := d.Exec(`DELETE FROM device_tokens WHERE id = ?`, tokenID)
	if err != nil {
		return fmt.Errorf("delete device token: %w", err)
	}
	return nil
}

func (d *SQLiteDB) UpdateDeviceTokenLastUsed(tokenID string) error {
	_, err := d.Exec(
		`UPDATE device_tokens SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC(), tokenID,
	)
	if err != nil {
		return fmt.Errorf("update device token last used: %w", err)
	}
	return nil
}

func (d *SQLiteDB) IsEventProcessed(eventID string) (bool, error) {
	var count int
	err := d.QueryRow(`SELECT COUNT(*) FROM processed_events WHERE event_id = ?`, eventID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check processed event: %w", err)
	}
	return count > 0, nil
}

func (d *SQLiteDB) MarkEventProcessed(eventID string) error {
	_, err := d.Exec(
		`INSERT OR IGNORE INTO processed_events (event_id, processed_at) VALUES (?, ?)`,
		eventID, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("mark event processed: %w", err)
	}
	return nil
}

func (d *SQLiteDB) PruneProcessedEvents(olderThan time.Time) error {
	_, err := d.Exec(`DELETE FROM processed_events WHERE processed_at < ?`, olderThan)
	if err != nil {
		return fmt.Errorf("prune processed events: %w", err)
	}
	return nil
}

// Debug reports

func (d *SQLiteDB) CreateDebugReport(r *models.DebugReport) error {
	if r.ID == "" {
		r.ID = generateUUID()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	device := r.Device
	if len(device) == 0 {
		device = []byte(`null`)
	}
	state := r.State
	if len(state) == 0 {
		state = []byte(`null`)
	}
	breadcrumbs := r.Breadcrumbs
	if len(breadcrumbs) == 0 {
		breadcrumbs = []byte(`[]`)
	}
	var desc sql.NullString
	if r.Description != "" {
		desc = sql.NullString{String: r.Description, Valid: true}
	}
	_, err := d.Exec(
		`INSERT INTO debug_reports (id, user_id, client_id, created_at, trigger, app_version, platform, device, state, description, breadcrumbs)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.UserID, r.ClientID, r.CreatedAt, r.Trigger, r.AppVersion, r.Platform,
		string(device), string(state), desc, string(breadcrumbs),
	)
	if err != nil {
		return fmt.Errorf("insert debug report: %w", err)
	}
	return nil
}

func (d *SQLiteDB) ListDebugReports(filter DebugReportFilter) ([]models.DebugReport, error) {
	query := `SELECT id, user_id, client_id, created_at, trigger, app_version, platform, device, state, description, breadcrumbs FROM debug_reports WHERE 1=1`
	var args []any
	if filter.UserID != nil {
		query += ` AND user_id = ?`
		args = append(args, *filter.UserID)
	}
	if filter.Since != nil {
		query += ` AND created_at >= ?`
		args = append(args, *filter.Since)
	}
	query += ` ORDER BY created_at DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query debug reports: %w", err)
	}
	defer rows.Close()

	var reports []models.DebugReport
	for rows.Next() {
		var r models.DebugReport
		var desc sql.NullString
		var device, state, breadcrumbs string
		if err := rows.Scan(&r.ID, &r.UserID, &r.ClientID, &r.CreatedAt, &r.Trigger, &r.AppVersion, &r.Platform, &device, &state, &desc, &breadcrumbs); err != nil {
			return nil, fmt.Errorf("scan debug report: %w", err)
		}
		r.Device = []byte(device)
		r.State = []byte(state)
		r.Breadcrumbs = []byte(breadcrumbs)
		if desc.Valid {
			r.Description = desc.String
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (d *SQLiteDB) GetDebugReport(id string) (*models.DebugReport, error) {
	var r models.DebugReport
	var desc sql.NullString
	var device, state, breadcrumbs string
	err := d.QueryRow(
		`SELECT id, user_id, client_id, created_at, trigger, app_version, platform, device, state, description, breadcrumbs FROM debug_reports WHERE id = ?`,
		id,
	).Scan(&r.ID, &r.UserID, &r.ClientID, &r.CreatedAt, &r.Trigger, &r.AppVersion, &r.Platform, &device, &state, &desc, &breadcrumbs)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query debug report: %w", err)
	}
	r.Device = []byte(device)
	r.State = []byte(state)
	r.Breadcrumbs = []byte(breadcrumbs)
	if desc.Valid {
		r.Description = desc.String
	}
	return &r, nil
}

func (d *SQLiteDB) DeleteOldDebugReports(olderThan time.Time) (int64, error) {
	res, err := d.Exec(`DELETE FROM debug_reports WHERE created_at < ?`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("delete old debug reports: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}

func (d *SQLiteDB) DeleteAccount(userID string) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete completions for all goals owned by this user
	if _, err := tx.Exec(`DELETE FROM completions WHERE goal_id IN (SELECT id FROM goals WHERE user_id = ?)`, userID); err != nil {
		return fmt.Errorf("delete completions: %w", err)
	}
	// Delete goals
	if _, err := tx.Exec(`DELETE FROM goals WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete goals: %w", err)
	}
	// Delete device tokens
	if _, err := tx.Exec(`DELETE FROM device_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete device tokens: %w", err)
	}
	// Delete auth providers
	if _, err := tx.Exec(`DELETE FROM auth_providers WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete auth providers: %w", err)
	}
	// Delete sessions
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	// Delete debug reports
	if _, err := tx.Exec(`DELETE FROM debug_reports WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete debug reports: %w", err)
	}
	// Delete user
	if _, err := tx.Exec(`DELETE FROM users WHERE id = ?`, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return tx.Commit()
}
