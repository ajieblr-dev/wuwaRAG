package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// UserQueryLog represents a single user question recorded in the development audit database.
type UserQueryLog struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	SessionID    string    `json:"session_id,omitempty"`
	Question     string    `json:"question"`
	Tokens       int       `json:"tokens"`
	LatencyMs    int       `json:"latency_ms"`
	Status       string    `json:"status"` // "success" or "fail"
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuditStats represents aggregated summary metrics for website development and analysis.
type AuditStats struct {
	TotalQueries int `json:"total_queries"`
	SuccessCount int `json:"success_count"`
	FailCount    int `json:"fail_count"`
	AvgLatencyMs int `json:"avg_latency_ms"`
	TotalTokens  int `json:"total_tokens"`
}

// AuditDB wraps a dedicated SQLite database holding development audit query history.
// This database is completely separate from the user chat database so user actions
// like clearing chat history do not affect development telemetry.
type AuditDB struct {
	db *sql.DB
}

// NewAuditDB opens or creates the development audit database at the specified path.
func NewAuditDB(dbPath string) (*AuditDB, error) {
	if dbPath == "" {
		dbPath = "./data/dev_audit.db"
	}

	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create audit database directory %s: %w", dir, err)
		}
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit sqlite database at %s: %w", dbPath, err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(time.Hour)

	audit := &AuditDB{db: conn}
	if err := audit.initPragmas(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := audit.migrate(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return audit, nil
}

// Close closes the underlying audit database connection.
func (a *AuditDB) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// RawDB returns the raw *sql.DB handle.
func (a *AuditDB) RawDB() *sql.DB {
	return a.db
}

func (a *AuditDB) initPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
	}
	for _, p := range pragmas {
		if _, err := a.db.Exec(p); err != nil {
			return fmt.Errorf("audit db pragma %s failed: %w", p, err)
		}
	}
	return nil
}

func (a *AuditDB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS user_queries (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		username TEXT NOT NULL,
		session_id TEXT,
		question TEXT NOT NULL,
		tokens INTEGER NOT NULL DEFAULT 0,
		latency_ms INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL,
		error_message TEXT,
		created_at TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_user_queries_created_at ON user_queries(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_user_queries_user_id ON user_queries(user_id);
	CREATE INDEX IF NOT EXISTS idx_user_queries_username ON user_queries(username);
	CREATE INDEX IF NOT EXISTS idx_user_queries_status ON user_queries(status);
	`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := a.db.ExecContext(ctx, schema)
	return err
}

// LogQuery inserts a record of a user question into the development audit database.
func (a *AuditDB) LogQuery(ctx context.Context, log UserQueryLog) (*UserQueryLog, error) {
	if log.ID == "" {
		log.ID = "uq-" + uuid.New().String()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	if log.Status == "" {
		log.Status = "success"
	}

	query := `
	INSERT INTO user_queries (id, user_id, username, session_id, question, tokens, latency_ms, status, error_message, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := a.db.ExecContext(
		ctx,
		query,
		log.ID,
		log.UserID,
		log.Username,
		log.SessionID,
		log.Question,
		log.Tokens,
		log.LatencyMs,
		log.Status,
		log.ErrorMessage,
		log.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to log user query: %w", err)
	}

	return &log, nil
}

// ListQueryLogs retrieves question logs ordered by time descending with optional filtering and pagination.
func (a *AuditDB) ListQueryLogs(ctx context.Context, limit, offset int, statusFilter, search string) ([]UserQueryLog, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []interface{}

	statusFilter = strings.TrimSpace(strings.ToLower(statusFilter))
	if statusFilter != "" && statusFilter != "all" {
		conditions = append(conditions, "status = ?")
		args = append(args, statusFilter)
	}

	search = strings.TrimSpace(search)
	if search != "" {
		likeTerm := "%" + search + "%"
		conditions = append(conditions, "(question LIKE ? OR username LIKE ? OR session_id LIKE ?)")
		args = append(args, likeTerm, likeTerm, likeTerm)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 1. Get total count matching filter
	countQuery := "SELECT COUNT(*) FROM user_queries " + whereClause
	var total int
	if err := a.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count user queries: %w", err)
	}

	// 2. Fetch records
	dataQuery := fmt.Sprintf(`
	SELECT id, user_id, username, COALESCE(session_id, ''), question, tokens, latency_ms, status, COALESCE(error_message, ''), created_at
	FROM user_queries
	%s
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`, whereClause)

	queryArgs := append(args, limit, offset)
	rows, err := a.db.QueryContext(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list user queries: %w", err)
	}
	defer rows.Close()

	logs := make([]UserQueryLog, 0)
	for rows.Next() {
		var l UserQueryLog
		if err := rows.Scan(
			&l.ID,
			&l.UserID,
			&l.Username,
			&l.SessionID,
			&l.Question,
			&l.Tokens,
			&l.LatencyMs,
			&l.Status,
			&l.ErrorMessage,
			&l.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user query row: %w", err)
		}
		logs = append(logs, l)
	}

	return logs, total, nil
}

// GetStats returns summary counts and averages across the audit logs.
func (a *AuditDB) GetStats(ctx context.Context) (*AuditStats, error) {
	query := `
	SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END), 0),
		COALESCE(AVG(latency_ms), 0),
		COALESCE(SUM(tokens), 0)
	FROM user_queries
	`
	var stats AuditStats
	var avgLatency float64
	err := a.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalQueries,
		&stats.SuccessCount,
		&stats.FailCount,
		&avgLatency,
		&stats.TotalTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate audit stats: %w", err)
	}
	stats.AvgLatencyMs = int(avgLatency)
	return &stats, nil
}
