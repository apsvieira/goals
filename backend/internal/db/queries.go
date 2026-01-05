package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/apsv/goal-tracker/backend/internal/models"
)

// Goals

func (d *DB) ListGoals(includeArchived bool) ([]models.Goal, error) {
	query := `SELECT id, name, color, position, created_at, archived_at FROM goals`
	if !includeArchived {
		query += ` WHERE archived_at IS NULL`
	}
	query += ` ORDER BY position ASC, created_at ASC`

	rows, err := d.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query goals: %w", err)
	}
	defer rows.Close()

	var goals []models.Goal
	for rows.Next() {
		var g models.Goal
		var archivedAt sql.NullTime
		if err := rows.Scan(&g.ID, &g.Name, &g.Color, &g.Position, &g.CreatedAt, &archivedAt); err != nil {
			return nil, fmt.Errorf("scan goal: %w", err)
		}
		if archivedAt.Valid {
			g.ArchivedAt = &archivedAt.Time
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (d *DB) GetGoal(id string) (*models.Goal, error) {
	var g models.Goal
	var archivedAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, name, color, position, created_at, archived_at FROM goals WHERE id = ?`,
		id,
	).Scan(&g.ID, &g.Name, &g.Color, &g.Position, &g.CreatedAt, &archivedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query goal: %w", err)
	}
	if archivedAt.Valid {
		g.ArchivedAt = &archivedAt.Time
	}
	return &g, nil
}

func (d *DB) CreateGoal(g *models.Goal) error {
	// Get next position
	var maxPos sql.NullInt64
	d.QueryRow(`SELECT MAX(position) FROM goals`).Scan(&maxPos)
	nextPos := 0
	if maxPos.Valid {
		nextPos = int(maxPos.Int64) + 1
	}
	g.Position = nextPos

	_, err := d.Exec(
		`INSERT INTO goals (id, name, color, position, created_at) VALUES (?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Color, g.Position, g.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert goal: %w", err)
	}
	return nil
}

func (d *DB) UpdateGoal(id string, name, color *string) error {
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

	_, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update goal: %w", err)
	}
	return nil
}

func (d *DB) ArchiveGoal(id string) error {
	_, err := d.Exec(
		`UPDATE goals SET archived_at = ? WHERE id = ?`,
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("archive goal: %w", err)
	}
	return nil
}

// Completions

func (d *DB) ListCompletions(from, to string, goalID *string) ([]models.Completion, error) {
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

func (d *DB) GetCompletionByGoalAndDate(goalID, date string) (*models.Completion, error) {
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

func (d *DB) CreateCompletion(c *models.Completion) error {
	_, err := d.Exec(
		`INSERT INTO completions (id, goal_id, date, created_at) VALUES (?, ?, ?, ?)`,
		c.ID, c.GoalID, c.Date, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert completion: %w", err)
	}
	return nil
}

func (d *DB) DeleteCompletion(id string) error {
	_, err := d.Exec(`DELETE FROM completions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete completion: %w", err)
	}
	return nil
}

func (d *DB) ReorderGoals(goalIDs []string) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i, id := range goalIDs {
		_, err := tx.Exec(`UPDATE goals SET position = ? WHERE id = ?`, i, id)
		if err != nil {
			return fmt.Errorf("update position for %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
