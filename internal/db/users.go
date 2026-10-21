package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrUserNotFound is returned when a requested user does not exist.
var ErrUserNotFound = errors.New("user not found")

// ErrUserAlreadyExists is returned when username or email is already taken.
var ErrUserAlreadyExists = errors.New("username or email already exists")

// User represents an authenticated account in the system.
type User struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
	Role          string    `json:"role"` // "admin" or "user"
	QuestionCount int       `json:"question_count"`
	QuestionLimit int       `json:"question_limit"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateUser inserts a new user with the specified credentials and quota.
func (d *DB) CreateUser(ctx context.Context, username, email, passwordHash, role string, questionLimit int) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))
	role = strings.ToLower(strings.TrimSpace(role))

	if role == "" {
		role = "user"
	}
	if questionLimit <= 0 {
		questionLimit = 5
	}

	now := time.Now().UTC()
	u := &User{
		ID:            uuid.New().String(),
		Username:      username,
		Email:         email,
		PasswordHash:  passwordHash,
		Role:          role,
		QuestionCount: 0,
		QuestionLimit: questionLimit,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	query := `
	INSERT INTO users (id, username, email, password_hash, role, question_count, question_limit, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.ExecContext(ctx, query, u.ID, u.Username, u.Email, u.PasswordHash, u.Role, u.QuestionCount, u.QuestionLimit, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	return u, nil
}

// GetUserByID queries a user by their UUID primary key.
func (d *DB) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
	SELECT id, username, email, password_hash, role, question_count, question_limit, created_at, updated_at
	FROM users
	WHERE id = ?
	`
	row := d.db.QueryRowContext(ctx, query, id)
	return scanUser(row)
}

// GetUserByUsernameOrEmail retrieves a user matching either username or email.
func (d *DB) GetUserByUsernameOrEmail(ctx context.Context, identifier string) (*User, error) {
	identClean := strings.TrimSpace(identifier)
	identLower := strings.ToLower(identClean)

	query := `
	SELECT id, username, email, password_hash, role, question_count, question_limit, created_at, updated_at
	FROM users
	WHERE username = ? OR LOWER(email) = ?
	LIMIT 1
	`
	row := d.db.QueryRowContext(ctx, query, identClean, identLower)
	return scanUser(row)
}

// CheckAndIncrementQuestionCount checks whether the user is permitted to ask a question.
// For admin users, allowed is always true and remaining is returned as -1 (unlimited).
// For regular users, if question_count < question_limit, it increments count and returns remaining quota.
// If question_count >= question_limit, it returns allowed=false and remaining=0 without incrementing.
func (d *DB) CheckAndIncrementQuestionCount(ctx context.Context, userID string) (allowed bool, remaining int, limit int, err error) {
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return false, 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var role string
	var currentCount, qLimit int

	querySelect := `SELECT role, question_count, question_limit FROM users WHERE id = ?`
	err = tx.QueryRowContext(ctx, querySelect, userID).Scan(&role, &currentCount, &qLimit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, 0, ErrUserNotFound
		}
		return false, 0, 0, fmt.Errorf("failed to query user quota: %w", err)
	}

	now := time.Now().UTC()

	// Admin account is exempt from question limits
	if strings.ToLower(role) == "admin" {
		// Increment count for audit trail, but never block
		_, err = tx.ExecContext(ctx, `UPDATE users SET question_count = question_count + 1, updated_at = ? WHERE id = ?`, now, userID)
		if err != nil {
			return false, 0, 0, fmt.Errorf("failed to update admin question count: %w", err)
		}
		if err = tx.Commit(); err != nil {
			return false, 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
		}
		return true, -1, -1, nil
	}

	// Standard user check
	if currentCount >= qLimit {
		_ = tx.Rollback()
		return false, 0, qLimit, nil
	}

	newCount := currentCount + 1
	_, err = tx.ExecContext(ctx, `UPDATE users SET question_count = ?, updated_at = ? WHERE id = ?`, newCount, now, userID)
	if err != nil {
		return false, 0, 0, fmt.Errorf("failed to increment question count: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return false, 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	remaining = qLimit - newCount
	if remaining < 0 {
		remaining = 0
	}

	return true, remaining, qLimit, nil
}

// ResetUserQuestionCount updates a user's question count and optionally their limit.
func (d *DB) ResetUserQuestionCount(ctx context.Context, userID string, newCount int, newLimit int) error {
	now := time.Now().UTC()
	var err error
	if newLimit > 0 {
		query := `UPDATE users SET question_count = ?, question_limit = ?, updated_at = ? WHERE id = ?`
		_, err = d.db.ExecContext(ctx, query, newCount, newLimit, now, userID)
	} else {
		query := `UPDATE users SET question_count = ?, updated_at = ? WHERE id = ?`
		_, err = d.db.ExecContext(ctx, query, newCount, now, userID)
	}
	if err != nil {
		return fmt.Errorf("failed to reset user quota: %w", err)
	}
	return nil
}

// ListUsers returns a list of all registered users (for admin management).
func (d *DB) ListUsers(ctx context.Context) ([]User, error) {
	query := `
	SELECT id, username, email, password_hash, role, question_count, question_limit, created_at, updated_at
	FROM users
	ORDER BY created_at DESC
	`
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.QuestionCount, &u.QuestionLimit, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.QuestionCount, &u.QuestionLimit, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	return &u, nil
}
