package db

import (
	"time"
)

// File represents a cached file entry
type File struct {
	ID              int64      `db:"id"`
	CacheKey        string     `db:"cache_key"`
	FileName        string     `db:"file_name"`
	NormalizedURL   string     `db:"normalized_url"`
	OriginalURL     string     `db:"original_url"`
	S3Path          string     `db:"s3_path"`
	FileSize        int64      `db:"file_size"`
	Cached          bool       `db:"cached"`
	RequestCount    int        `db:"request_count"`
	LastRequestedAt *time.Time `db:"last_requested_at"`
	FirstCachedAt   *time.Time `db:"first_cached_at"`
	ErrorReason     *string    `db:"error_reason"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

// Request represents a download request log entry
type Request struct {
	ID          int64     `db:"id"`
	FileID      int64     `db:"file_id"`
	UserAgent   string    `db:"user_agent"`
	IPAddress   string    `db:"ip_address"`
	HTTPStatus  int       `db:"http_status"`
	BytesServed int64     `db:"bytes_served"`
	DurationMs  int64     `db:"duration_ms"`
	CreatedAt   time.Time `db:"created_at"`
}
