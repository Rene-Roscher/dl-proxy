package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the database connection
type DB struct {
	*sql.DB
	driver string
}

// New creates a new database connection
func New(dsn, dbType string) (*DB, error) {
	// Auto-detect driver if not specified
	driver := dbType
	if driver == "" {
		if strings.Contains(dsn, ".db") || strings.HasPrefix(dsn, "file:") {
			driver = "sqlite3"
		} else {
			driver = "mysql"
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Connected to %s database", driver)
	return &DB{DB: db, driver: driver}, nil
}

// RunMigrations executes all SQL migration files in the migrations directory
func (db *DB) RunMigrations(migrationsPath string) error {
	files, err := filepath.Glob(filepath.Join(migrationsPath, "*.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	for _, file := range files {
		log.Printf("Running migration: %s", filepath.Base(file))

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}

	log.Println("All migrations completed successfully")
	return nil
}

// GetFileByCacheKey retrieves a file by its cache key
func (db *DB) GetFileByCacheKey(cacheKey string) (*File, error) {
	query := `SELECT id, cache_key, file_name, normalized_url, original_url,
	          s3_path, file_size, cached, request_count, last_requested_at,
	          first_cached_at, error_reason, created_at, updated_at
	          FROM files WHERE cache_key = ?`

	var file File
	var errorReason sql.NullString
	var lastRequested sql.NullTime
	var firstCached sql.NullTime

	err := db.QueryRow(query, cacheKey).Scan(
		&file.ID,
		&file.CacheKey,
		&file.FileName,
		&file.NormalizedURL,
		&file.OriginalURL,
		&file.S3Path,
		&file.FileSize,
		&file.Cached,
		&file.RequestCount,
		&lastRequested,
		&firstCached,
		&errorReason,
		&file.CreatedAt,
		&file.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query file: %w", err)
	}

	if errorReason.Valid {
		file.ErrorReason = &errorReason.String
	}
	if lastRequested.Valid {
		file.LastRequestedAt = &lastRequested.Time
	}
	if firstCached.Valid {
		file.FirstCachedAt = &firstCached.Time
	}

	return &file, nil
}

// CreateFile inserts a new file record
func (db *DB) CreateFile(file *File) error {
	query := `INSERT INTO files (cache_key, file_name, normalized_url, original_url,
	          s3_path, file_size, cached, error_reason)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	var errorReason sql.NullString
	if file.ErrorReason != nil {
		errorReason = sql.NullString{String: *file.ErrorReason, Valid: true}
	}

	result, err := db.Exec(query,
		file.CacheKey,
		file.FileName,
		file.NormalizedURL,
		file.OriginalURL,
		file.S3Path,
		file.FileSize,
		file.Cached,
		errorReason,
	)
	if err != nil {
		return fmt.Errorf("failed to insert file: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	file.ID = id
	return nil
}

// UpdateFile updates an existing file record
func (db *DB) UpdateFile(file *File) error {
	query := `UPDATE files SET file_name = ?, file_size = ?, cached = ?,
	          first_cached_at = ?, error_reason = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	var errorReason sql.NullString
	if file.ErrorReason != nil {
		errorReason = sql.NullString{String: *file.ErrorReason, Valid: true}
	}

	var firstCached sql.NullTime
	if file.FirstCachedAt != nil {
		firstCached = sql.NullTime{Time: *file.FirstCachedAt, Valid: true}
	}

	_, err := db.Exec(query,
		file.FileName,
		file.FileSize,
		file.Cached,
		firstCached,
		errorReason,
		file.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}

	return nil
}

// CreateRequest inserts a new request log entry
func (db *DB) CreateRequest(req *Request) error {
	query := `INSERT INTO requests (file_id, user_agent, ip_address, http_status,
	          bytes_served, duration_ms) VALUES (?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query,
		req.FileID,
		req.UserAgent,
		req.IPAddress,
		req.HTTPStatus,
		req.BytesServed,
		req.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("failed to insert request: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	req.ID = id
	return nil
}

// IncrementRequestCount increments the request counter and updates last_requested_at
func (db *DB) IncrementRequestCount(fileID int64) error {
	query := `UPDATE files
	          SET request_count = request_count + 1,
	              last_requested_at = CURRENT_TIMESTAMP,
	              updated_at = CURRENT_TIMESTAMP
	          WHERE id = ?`

	_, err := db.Exec(query, fileID)
	if err != nil {
		return fmt.Errorf("failed to increment request count: %w", err)
	}

	return nil
}

// GetRequestCountInWindow counts requests for a file within a time window
func (db *DB) GetRequestCountInWindow(cacheKey string, windowDuration time.Duration) (int, error) {
	var query string
	if db.driver == "sqlite3" {
		query = `SELECT COUNT(*)
		          FROM requests r
		          JOIN files f ON r.file_id = f.id
		          WHERE f.cache_key = ?
		          AND r.created_at >= datetime('now', '-' || ? || ' seconds')`
	} else {
		query = `SELECT COUNT(*)
		          FROM requests r
		          JOIN files f ON r.file_id = f.id
		          WHERE f.cache_key = ?
		          AND r.created_at >= DATE_SUB(NOW(), INTERVAL ? SECOND)`
	}

	var count int
	err := db.QueryRow(query, cacheKey, int(windowDuration.Seconds())).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count requests in window: %w", err)
	}

	return count, nil
}

// GetUnusedCachedFiles finds cached files not requested within retention period
func (db *DB) GetUnusedCachedFiles(retentionDays int) ([]File, error) {
	var query string
	if db.driver == "sqlite3" {
		query = `SELECT id, cache_key, file_name, s3_path, file_size,
		          last_requested_at, first_cached_at
		          FROM files
		          WHERE cached = 1
		          AND last_requested_at < datetime('now', '-' || ? || ' days')
		          ORDER BY last_requested_at ASC`
	} else {
		query = `SELECT id, cache_key, file_name, s3_path, file_size,
		          last_requested_at, first_cached_at
		          FROM files
		          WHERE cached = TRUE
		          AND last_requested_at < DATE_SUB(NOW(), INTERVAL ? DAY)
		          ORDER BY last_requested_at ASC`
	}

	rows, err := db.Query(query, retentionDays)
	if err != nil {
		return nil, fmt.Errorf("failed to query unused files: %w", err)
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		var lastRequested sql.NullTime
		var firstCached sql.NullTime

		err := rows.Scan(
			&f.ID,
			&f.CacheKey,
			&f.FileName,
			&f.S3Path,
			&f.FileSize,
			&lastRequested,
			&firstCached,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file row: %w", err)
		}

		if lastRequested.Valid {
			f.LastRequestedAt = &lastRequested.Time
		}
		if firstCached.Valid {
			f.FirstCachedAt = &firstCached.Time
		}

		files = append(files, f)
	}

	return files, nil
}

// DeleteFile deletes a file record from database
func (db *DB) DeleteFile(fileID int64) error {
	query := `DELETE FROM files WHERE id = ?`

	_, err := db.Exec(query, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
