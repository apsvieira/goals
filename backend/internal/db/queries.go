package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

// Goals

func (d *SQLiteDB) ListGoals(userID *string, includeArchived bool) ([]models.Goal, error) {
	query := `SELECT id, name, color, position, user_id, created_at, archived_at FROM goals WHERE `
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
	query += ` ORDER BY position ASC, created_at ASC`

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query goals: %w", err)
	}
	defer rows.Close()

	var goals []models.Goal
	for rows.Next() {
		var g models.Goal
		var archivedAt sql.NullTime
		var goalUserID sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &g.Color, &g.Position, &goalUserID, &g.CreatedAt, &archivedAt); err != nil {
			return nil, fmt.Errorf("scan goal: %w", err)
		}
		if archivedAt.Valid {
			g.ArchivedAt = &archivedAt.Time
		}
		if goalUserID.Valid {
			g.UserID = &goalUserID.String
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (d *SQLiteDB) GetGoal(userID *string, id string) (*models.Goal, error) {
	var g models.Goal
	var archivedAt sql.NullTime
	var goalUserID sql.NullString

	query := `SELECT id, name, color, position, user_id, created_at, archived_at FROM goals WHERE id = ?`
	args := []any{id}

	// Add user_id filter
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += ` AND user_id = ?`
		args = append(args, *userID)
	}

	err := d.QueryRow(query, args...).Scan(&g.ID, &g.Name, &g.Color, &g.Position, &goalUserID, &g.CreatedAt, &archivedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query goal: %w", err)
	}
	if archivedAt.Valid {
		g.ArchivedAt = &archivedAt.Time
	}
	if goalUserID.Valid {
		g.UserID = &goalUserID.String
	}
	return &g, nil
}

func (d *SQLiteDB) CreateGoal(g *models.Goal) error {
	// Get next position for this user's goals
	var maxPos sql.NullInt64
	if g.UserID == nil {
		d.QueryRow(`SELECT MAX(position) FROM goals WHERE user_id IS NULL`).Scan(&maxPos)
	} else {
		d.QueryRow(`SELECT MAX(position) FROM goals WHERE user_id = ?`, *g.UserID).Scan(&maxPos)
	}
	nextPos := 0
	if maxPos.Valid {
		nextPos = int(maxPos.Int64) + 1
	}
	g.Position = nextPos

	_, err := d.Exec(
		`INSERT INTO goals (id, name, color, position, user_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Color, g.Position, g.UserID, g.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert goal: %w", err)
	}
	return nil
}

func (d *SQLiteDB) UpdateGoal(userID *string, id string, name, color *string) error {
	if name == nil && color == nil {
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
	query := `UPDATE goals SET archived_at = ? WHERE id = ?`
	args := []any{time.Now().UTC(), id}

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
	query := `SELECT id, goal_id, date, created_at FROM completions WHERE date >= ? AND date <= ?`
	args := []any{from, to}

	if goalID != nil {
		query += ` AND goal_id = ?`
		args = append(args, *goalID)
	}
	query += ` ORDER BY date ASC`

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query completions: %w", err)
	}
	defer rows.Close()

	var completions []models.Completion
	for rows.Next() {
		var c models.Completion
		if err := rows.Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan completion: %w", err)
		}
		completions = append(completions, c)
	}
	return completions, rows.Err()
}

func (d *SQLiteDB) GetCompletionByGoalAndDate(goalID, date string) (*models.Completion, error) {
	var c models.Completion
	err := d.QueryRow(
		`SELECT id, goal_id, date, created_at FROM completions WHERE goal_id = ? AND date = ?`,
		goalID, date,
	).Scan(&c.ID, &c.GoalID, &c.Date, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query completion: %w", err)
	}
	return &c, nil
}

func (d *SQLiteDB) CreateCompletion(c *models.Completion) error {
	_, err := d.Exec(
		`INSERT INTO completions (id, goal_id, date, created_at) VALUES (?, ?, ?, ?)`,
		c.ID, c.GoalID, c.Date, c.CreatedAt,
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
