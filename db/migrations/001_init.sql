-- Create files table
CREATE TABLE IF NOT EXISTS files (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    cache_key VARCHAR(32) NOT NULL UNIQUE,
    file_name VARCHAR(255) NOT NULL,
    normalized_url TEXT NOT NULL,
    original_url TEXT NOT NULL,
    s3_path VARCHAR(512) NOT NULL,
    file_size BIGINT DEFAULT 0,
    cached BOOLEAN DEFAULT FALSE,
    request_count INT DEFAULT 0,
    last_requested_at TIMESTAMP NULL,
    first_cached_at TIMESTAMP NULL,
    error_reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_cache_key (cache_key),
    INDEX idx_cached (cached),
    INDEX idx_last_requested (last_requested_at),
    INDEX idx_request_count (request_count)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create requests table
CREATE TABLE IF NOT EXISTS requests (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    file_id BIGINT NOT NULL,
    user_agent VARCHAR(512),
    ip_address VARCHAR(45) NOT NULL,
    http_status INT NOT NULL,
    bytes_served BIGINT DEFAULT 0,
    duration_ms BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
    INDEX idx_file_id (file_id),
    INDEX idx_created_at (created_at),
    INDEX idx_ip_address (ip_address)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
