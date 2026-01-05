package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
	_ "github.com/lib/pq"
)

//go:embed postgres_migrations/*.sql
var postgresMigrationsFS embed.FS

// PostgresDB implements the Database interface for PostgreSQL.
type PostgresDB struct {
	*sql.DB
}

// Ensure PostgresDB implements Database interface
var _ Database = (*PostgresDB)(nil)

// NewPostgres creates a new PostgreSQL database connection.
func NewPostgres(connStr string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &PostgresDB{db}, nil
}

func (d *PostgresDB) Migrate() error {
	// Create migrations tracking table
	_, err := d.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		name TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get list of migration files
	entries, err := fs.ReadDir(postgresMigrationsFS, "postgres_migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Sort migrations by name
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	// Apply each migration if not already applied
	for _, name := range names {
		// Check if already applied
		var count int
		err := d.QueryRow("SELECT COUNT(*) FROM _migrations WHERE name = $1", name).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if count > 0 {
			continue // Already applied
		}

		// Read and execute migration
		content, err := postgresMigrationsFS.ReadFile("postgres_migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := d.Exec(string(content)); err != nil {
			return fmt.Errorf("execute migration %s: %w", name, err)
		}

		// Record migration
		if _, err := d.Exec("INSERT INTO _migrations (name) VALUES ($1)", name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	return nil
}

// Goals

func (d *PostgresDB) ListGoals(userID *string, includeArchived bool) ([]models.Goal, error) {
	query := `SELECT id, name, color, position, user_id, created_at, updated_at, archived_at, deleted_at FROM goals WHERE `
	var args []any
	paramNum := 1

	// Filter by user_id
	if userID == nil {
		query += `user_id IS NULL`
	} else {
		query += fmt.Sprintf(`user_id = $%d`, paramNum)
		args = append(args, *userID)
		paramNum++
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
		if err := rows.Scan(&g.ID, &g.Name, &g.Color, &g.Position, &goalUserID, &g.CreatedAt, &updatedAt, &archivedAt, &deletedAt); err != nil {
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
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (d *PostgresDB) GetGoal(userID *string, id string) (*models.Goal, error) {
	var g models.Goal
	var archivedAt, deletedAt sql.NullTime
	var updatedAt sql.NullTime
	var goalUserID sql.NullString

	query := `SELECT id, name, color, position, user_id, created_at, updated_at, archived_at, deleted_at FROM goals WHERE id = $1`
	args := []any{id}

	// Add user_id filter
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += ` AND user_id = $2`
		args = append(args, *userID)
	}

	err := d.QueryRow(query, args...).Scan(&g.ID, &g.Name, &g.Color, &g.Position, &goalUserID, &g.CreatedAt, &updatedAt, &archivedAt, &deletedAt)
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
	return &g, nil
}

func (d *PostgresDB) CreateGoal(g *models.Goal) error {
	// Get next position for this user's goals
	var maxPos sql.NullInt64
	if g.UserID == nil {
		d.QueryRow(`SELECT MAX(position) FROM goals WHERE user_id IS NULL AND deleted_at IS NULL`).Scan(&maxPos)
	} else {
		d.QueryRow(`SELECT MAX(position) FROM goals WHERE user_id = $1 AND deleted_at IS NULL`, *g.UserID).Scan(&maxPos)
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
		`INSERT INTO goals (id, name, color, position, user_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		g.ID, g.Name, g.Color, g.Position, g.UserID, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert goal: %w", err)
	}
	return nil
}

func (d *PostgresDB) UpdateGoal(userID *string, id string, name, color *string) error {
	if name == nil && color == nil {
		return nil
	}

	query := `UPDATE goals SET `
	var args []any
	var updates []string
	paramNum := 1

	if name != nil {
		updates = append(updates, fmt.Sprintf(`name = $%d`, paramNum))
		args = append(args, *name)
		paramNum++
	}
	if color != nil {
		updates = append(updates, fmt.Sprintf(`color = $%d`, paramNum))
		args = append(args, *color)
		paramNum++
	}

	// Always update updated_at
	updates = append(updates, fmt.Sprintf(`updated_at = $%d`, paramNum))
	args = append(args, time.Now().UTC())
	paramNum++

	query += strings.Join(updates, ", ")
	query += fmt.Sprintf(` WHERE id = $%d`, paramNum)
	args = append(args, id)
	paramNum++

	// Add user_id filter for ownership verification
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += fmt.Sprintf(` AND user_id = $%d`, paramNum)
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update goal: %w", err)
	}
	return nil
}

func (d *PostgresDB) ArchiveGoal(userID *string, id string) error {
	now := time.Now().UTC()
	query := `UPDATE goals SET archived_at = $1, updated_at = $2 WHERE id = $3`
	args := []any{now, now, id}

	// Add user_id filter for ownership verification
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += ` AND user_id = $4`
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("archive goal: %w", err)
	}
	return nil
}

func (d *PostgresDB) ReorderGoals(userID *string, goalIDs []string) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i, id := range goalIDs {
		query := `UPDATE goals SET position = $1 WHERE id = $2`
		args := []any{i, id}

		// Add user_id filter for ownership verification
		if userID == nil {
			query += ` AND user_id IS NULL`
		} else {
			query += ` AND user_id = $3`
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

// Completions

func (d *PostgresDB) ListCompletions(from, to string, goalID *string) ([]models.Completion, error) {
	query := `SELECT id, goal_id, date, created_at, updated_at, deleted_at FROM completions WHERE date >= $1 AND date <= $2`
	args := []any{from, to}

	if goalID != nil {
		query += ` AND goal_id = $3`
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

func (d *PostgresDB) GetCompletionByGoalAndDate(goalID, date string) (*models.Completion, error) {
	var c models.Completion
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, goal_id, date, created_at, updated_at, deleted_at FROM completions WHERE goal_id = $1 AND date = $2 AND deleted_at IS NULL`,
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

func (d *PostgresDB) CreateCompletion(c *models.Completion) error {
	now := time.Now().UTC()
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}

	_, err := d.Exec(
		`INSERT INTO completions (id, goal_id, date, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
		c.ID, c.GoalID, c.Date, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert completion: %w", err)
	}
	return nil
}

func (d *PostgresDB) DeleteCompletion(id string) error {
	_, err := d.Exec(`DELETE FROM completions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete completion: %w", err)
	}
	return nil
}

func (d *PostgresDB) Ping() error {
	return d.DB.Ping()
}

// Users

func (d *PostgresDB) GetUserByID(id string) (*models.User, error) {
	var u models.User
	var lastLoginAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, email, name, avatar_url, created_at, last_login_at FROM users WHERE id = $1`,
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

func (d *PostgresDB) GetUserByEmail(email string) (*models.User, error) {
	var u models.User
	var lastLoginAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, email, name, avatar_url, created_at, last_login_at FROM users WHERE email = $1`,
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

func (d *PostgresDB) CreateUser(u *models.User) error {
	_, err := d.Exec(
		`INSERT INTO users (id, email, name, avatar_url, created_at) VALUES ($1, $2, $3, $4, $5)`,
		u.ID, u.Email, u.Name, u.AvatarURL, u.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (d *PostgresDB) UpdateUserLastLogin(id string) error {
	_, err := d.Exec(
		`UPDATE users SET last_login_at = $1 WHERE id = $2`,
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

func (d *PostgresDB) GetOrCreateUserByProvider(provider, providerUserID, email, name, avatarURL string) (*models.User, error) {
	tx, err := d.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if auth provider exists
	var userID string
	err = tx.QueryRow(
		`SELECT user_id FROM auth_providers WHERE provider = $1 AND provider_user_id = $2`,
		provider, providerUserID,
	).Scan(&userID)

	if err == nil {
		// Provider exists, get user and update last login
		var u models.User
		var lastLoginAt sql.NullTime
		err = tx.QueryRow(
			`SELECT id, email, name, avatar_url, created_at, last_login_at FROM users WHERE id = $1`,
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
		_, err = tx.Exec(`UPDATE users SET last_login_at = $1 WHERE id = $2`, now, u.ID)
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
		`SELECT id, email, name, avatar_url, created_at, last_login_at FROM users WHERE email = $1`,
		email,
	).Scan(&existingUser.ID, &existingUser.Email, &existingUser.Name, &existingUser.AvatarURL, &existingUser.CreatedAt, &lastLoginAt)

	if err == nil {
		// User exists, add auth provider
		if lastLoginAt.Valid {
			existingUser.LastLoginAt = &lastLoginAt.Time
		}
		providerID := generatePostgresUUID()
		_, err = tx.Exec(
			`INSERT INTO auth_providers (id, user_id, provider, provider_user_id) VALUES ($1, $2, $3, $4)`,
			providerID, existingUser.ID, provider, providerUserID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert auth provider: %w", err)
		}

		// Update last login
		now := time.Now().UTC()
		_, err = tx.Exec(`UPDATE users SET last_login_at = $1 WHERE id = $2`, now, existingUser.ID)
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
		ID:          generatePostgresUUID(),
		Email:       email,
		Name:        name,
		AvatarURL:   avatarURL,
		CreatedAt:   now,
		LastLoginAt: &now,
	}

	_, err = tx.Exec(
		`INSERT INTO users (id, email, name, avatar_url, created_at, last_login_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		u.ID, u.Email, u.Name, u.AvatarURL, u.CreatedAt, u.LastLoginAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	// Add auth provider
	providerID := generatePostgresUUID()
	_, err = tx.Exec(
		`INSERT INTO auth_providers (id, user_id, provider, provider_user_id) VALUES ($1, $2, $3, $4)`,
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

func (d *PostgresDB) CreateSession(s *models.Session) error {
	_, err := d.Exec(
		`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)`,
		s.ID, s.UserID, s.TokenHash, s.ExpiresAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (d *PostgresDB) GetSessionByTokenHash(tokenHash string) (*models.Session, error) {
	var s models.Session
	err := d.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, created_at FROM sessions WHERE token_hash = $1`,
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

func (d *PostgresDB) DeleteSession(id string) error {
	_, err := d.Exec(`DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (d *PostgresDB) DeleteExpiredSessions() error {
	_, err := d.Exec(`DELETE FROM sessions WHERE expires_at < $1`, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

// generatePostgresUUID creates a simple UUID for IDs
func generatePostgresUUID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000000)
}

// Sync operations

func (d *PostgresDB) GetGoalChangesSince(userID *string, since *time.Time) ([]models.Goal, error) {
	query := `SELECT id, name, color, position, user_id, created_at, updated_at, archived_at, deleted_at FROM goals WHERE `
	var args []any
	paramNum := 1

	// Filter by user_id
	if userID == nil {
		query += `user_id IS NULL`
	} else {
		query += fmt.Sprintf(`user_id = $%d`, paramNum)
		args = append(args, *userID)
		paramNum++
	}

	// Filter by updated_at if since is provided
	if since != nil {
		query += fmt.Sprintf(` AND updated_at > $%d`, paramNum)
		args = append(args, *since)
		paramNum++
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
		if err := rows.Scan(&g.ID, &g.Name, &g.Color, &g.Position, &goalUserID, &g.CreatedAt, &updatedAt, &archivedAt, &deletedAt); err != nil {
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
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (d *PostgresDB) GetCompletionChangesSince(userID *string, since *time.Time) ([]models.Completion, error) {
	// For completions, we need to join with goals to filter by user_id
	query := `SELECT c.id, c.goal_id, c.date, c.created_at, c.updated_at, c.deleted_at
		FROM completions c
		INNER JOIN goals g ON c.goal_id = g.id
		WHERE `
	var args []any
	paramNum := 1

	// Filter by user_id through goals
	if userID == nil {
		query += `g.user_id IS NULL`
	} else {
		query += fmt.Sprintf(`g.user_id = $%d`, paramNum)
		args = append(args, *userID)
		paramNum++
	}

	// Filter by updated_at if since is provided
	if since != nil {
		query += fmt.Sprintf(` AND c.updated_at > $%d`, paramNum)
		args = append(args, *since)
		paramNum++
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

func (d *PostgresDB) UpsertGoal(goal *models.Goal) error {
	now := time.Now().UTC()
	if goal.UpdatedAt.IsZero() {
		goal.UpdatedAt = now
	}

	_, err := d.Exec(`
		INSERT INTO goals (id, name, color, position, user_id, created_at, updated_at, archived_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT(id) DO UPDATE SET
			name = EXCLUDED.name,
			color = EXCLUDED.color,
			position = EXCLUDED.position,
			updated_at = EXCLUDED.updated_at,
			archived_at = EXCLUDED.archived_at,
			deleted_at = EXCLUDED.deleted_at
		WHERE EXCLUDED.updated_at > goals.updated_at
	`, goal.ID, goal.Name, goal.Color, goal.Position, goal.UserID, goal.CreatedAt, goal.UpdatedAt, goal.ArchivedAt, goal.DeletedAt)

	if err != nil {
		return fmt.Errorf("upsert goal: %w", err)
	}
	return nil
}

func (d *PostgresDB) UpsertCompletion(c *models.Completion) error {
	now := time.Now().UTC()
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}

	_, err := d.Exec(`
		INSERT INTO completions (id, goal_id, date, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(id) DO UPDATE SET
			updated_at = EXCLUDED.updated_at,
			deleted_at = EXCLUDED.deleted_at
		WHERE EXCLUDED.updated_at > completions.updated_at
	`, c.ID, c.GoalID, c.Date, c.CreatedAt, c.UpdatedAt, c.DeletedAt)

	if err != nil {
		return fmt.Errorf("upsert completion: %w", err)
	}
	return nil
}

func (d *PostgresDB) SoftDeleteGoal(userID *string, id string) error {
	now := time.Now().UTC()
	query := `UPDATE goals SET deleted_at = $1, updated_at = $2 WHERE id = $3`
	args := []any{now, now, id}

	// Add user_id filter for ownership verification
	if userID == nil {
		query += ` AND user_id IS NULL`
	} else {
		query += ` AND user_id = $4`
		args = append(args, *userID)
	}

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("soft delete goal: %w", err)
	}
	return nil
}

func (d *PostgresDB) SoftDeleteCompletion(goalID, date string) error {
	now := time.Now().UTC()
	_, err := d.Exec(
		`UPDATE completions SET deleted_at = $1, updated_at = $2 WHERE goal_id = $3 AND date = $4`,
		now, now, goalID, date,
	)
	if err != nil {
		return fmt.Errorf("soft delete completion: %w", err)
	}
	return nil
}

func (d *PostgresDB) GetGoalByID(id string) (*models.Goal, error) {
	var g models.Goal
	var archivedAt, deletedAt sql.NullTime
	var updatedAt sql.NullTime
	var goalUserID sql.NullString

	err := d.QueryRow(
		`SELECT id, name, color, position, user_id, created_at, updated_at, archived_at, deleted_at FROM goals WHERE id = $1`,
		id,
	).Scan(&g.ID, &g.Name, &g.Color, &g.Position, &goalUserID, &g.CreatedAt, &updatedAt, &archivedAt, &deletedAt)
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
	return &g, nil
}
