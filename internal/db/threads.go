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

// ErrThreadNotFound is returned when a requested conversation thread is not found.
var ErrThreadNotFound = errors.New("thread not found")

// Thread represents a multi-turn conversation thread.
type Thread struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a single message in a thread.
type Message struct {
	ID        string    `json:"id"`
	ThreadID  string    `json:"thread_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Sources   string    `json:"sources,omitempty"` // JSON stringified array of snippets
	Meta      string    `json:"meta,omitempty"`    // JSON stringified metadata (model, latency, tokens)
	CreatedAt time.Time `json:"created_at"`
}

// CreateThread creates a new conversation thread for the given user.
func (d *DB) CreateThread(ctx context.Context, userID, title string) (*Thread, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New Conversation"
	}
	if len(title) > 80 {
		title = title[:80] + "…"
	}

	now := time.Now().UTC()
	t := &Thread{
		ID:        "th-" + uuid.New().String()[:8],
		UserID:    userID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}

	query := `
	INSERT INTO threads (id, user_id, title, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?)
	`
	_, err := d.db.ExecContext(ctx, query, t.ID, t.UserID, t.Title, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}

	return t, nil
}

// ListThreads returns all conversation threads for a user, sorted newest first.
func (d *DB) ListThreads(ctx context.Context, userID string) ([]Thread, error) {
	query := `
	SELECT id, user_id, title, created_at, updated_at
	FROM threads
	WHERE user_id = ?
	ORDER BY updated_at DESC
	`
	rows, err := d.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list threads: %w", err)
	}
	defer rows.Close()

	threads := make([]Thread, 0)
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan thread row: %w", err)
		}
		threads = append(threads, t)
	}
	return threads, nil
}

// GetThreadWithMessages returns a thread and all of its messages in chronological order.
func (d *DB) GetThreadWithMessages(ctx context.Context, userID, threadID string) (*Thread, []Message, error) {
	var t Thread
	queryThread := `
	SELECT id, user_id, title, created_at, updated_at
	FROM threads
	WHERE id = ? AND user_id = ?
	`
	err := d.db.QueryRowContext(ctx, queryThread, threadID, userID).Scan(&t.ID, &t.UserID, &t.Title, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrThreadNotFound
		}
		return nil, nil, fmt.Errorf("failed to query thread: %w", err)
	}

	queryMessages := `
	SELECT id, thread_id, user_id, role, content, COALESCE(sources, ''), COALESCE(meta, ''), created_at
	FROM messages
	WHERE thread_id = ?
	ORDER BY created_at ASC
	`
	rows, err := d.db.QueryContext(ctx, queryMessages, threadID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.UserID, &m.Role, &m.Content, &m.Sources, &m.Meta, &m.CreatedAt); err != nil {
			return nil, nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, m)
	}

	return &t, messages, nil
}

// DeleteThread removes a thread and cascades deletion to all contained messages.
func (d *DB) DeleteThread(ctx context.Context, userID, threadID string) error {
	query := `DELETE FROM threads WHERE id = ? AND user_id = ?`
	res, err := d.db.ExecContext(ctx, query, threadID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete thread: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrThreadNotFound
	}
	return nil
}

// DeleteAllThreads removes all conversation threads for a user and cascades deletion to messages.
func (d *DB) DeleteAllThreads(ctx context.Context, userID string) error {
	query := `DELETE FROM threads WHERE user_id = ?`
	_, err := d.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete all threads: %w", err)
	}
	return nil
}

// AddMessage inserts a new message into a thread and touches the thread's updated_at timestamp.
func (d *DB) AddMessage(ctx context.Context, threadID, userID, role, content, sourcesJSON, metaJSON string) (*Message, error) {
	now := time.Now().UTC()
	m := &Message{
		ID:        uuid.New().String(),
		ThreadID:  threadID,
		UserID:    userID,
		Role:      role,
		Content:   content,
		Sources:   sourcesJSON,
		Meta:      metaJSON,
		CreatedAt: now,
	}

	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin message insert tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	queryInsert := `
	INSERT INTO messages (id, thread_id, user_id, role, content, sources, meta, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, queryInsert, m.ID, m.ThreadID, m.UserID, m.Role, m.Content, m.Sources, m.Meta, m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert message: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE threads SET updated_at = ? WHERE id = ?`, now, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to update thread timestamp: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit message insert: %w", err)
	}

	return m, nil
}
