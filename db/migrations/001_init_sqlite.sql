-- SQLite Migration

CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key VARCHAR(32) NOT NULL UNIQUE,
    file_name VARCHAR(255) NOT NULL,
    normalized_url TEXT NOT NULL,
    original_url TEXT NOT NULL,
    s3_path VARCHAR(512) NOT NULL,
    file_size INTEGER DEFAULT 0,
    cached INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    last_requested_at DATETIME NULL,
    first_cached_at DATETIME NULL,
    error_reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cache_key ON files(cache_key);
CREATE INDEX IF NOT EXISTS idx_cached ON files(cached);
CREATE INDEX IF NOT EXISTS idx_last_requested ON files(last_requested_at);
CREATE INDEX IF NOT EXISTS idx_request_count ON files(request_count);

CREATE TABLE IF NOT EXISTS requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,
    user_agent TEXT,
    ip_address VARCHAR(45),
    http_status INTEGER,
    bytes_served INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_file_id ON requests(file_id);
CREATE INDEX IF NOT EXISTS idx_created_at ON requests(created_at);

-- Trigger to update updated_at timestamp
CREATE TRIGGER IF NOT EXISTS update_files_timestamp
AFTER UPDATE ON files
FOR EACH ROW
BEGIN
    UPDATE files SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;
