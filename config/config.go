package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	// HTTP Server
	Port string

	// Database
	DBType     string // "sqlite3" or "mysql" (auto-detected if empty)
	DBPath     string // For SQLite: path to .db file
	DBHost     string // For MySQL
	DBPort     string // For MySQL
	DBUser     string // For MySQL
	DBPassword string // For MySQL
	DBName     string // For MySQL

	// AWS S3 / R2
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	S3Bucket           string
	S3Prefix           string
	S3Endpoint         string // Custom endpoint for R2 or other S3-compatible services

	// Presigned URL settings
	PresignedURLExpiry time.Duration

	// Smart Caching settings
	CacheThreshold     int           // Min requests before caching (e.g. 10)
	CacheWindow        time.Duration // Time window for threshold (e.g. 1 hour)
	CacheRetentionDays int           // Days to keep unused cached files (e.g. 30)

	// Logging
	LogLevel string
}

// Load reads configuration from environment variables
// It attempts to load from .env file first, then falls back to system environment
func Load() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DBType:             getEnv("DB_TYPE", ""),
		DBPath:             getEnv("DB_PATH", "./data/proxy.db"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "3306"),
		DBUser:             getEnv("DB_USER", ""),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "proxy_db"),
		AWSRegion:          getEnv("AWS_REGION", "auto"),
		AWSAccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		S3Bucket:           getEnv("S3_BUCKET", ""),
		S3Prefix:           getEnv("S3_PREFIX", "proxy"),
		S3Endpoint:         getEnv("S3_ENDPOINT", ""),
		CacheThreshold:     getEnvAsInt("CACHE_THRESHOLD", 10),
		CacheRetentionDays: getEnvAsInt("CACHE_RETENTION_DAYS", 30),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
	}

	// Parse presigned URL expiry
	expiryStr := getEnv("PRESIGNED_URL_EXPIRY", "1h")
	expiry, err := time.ParseDuration(expiryStr)
	if err != nil {
		return nil, fmt.Errorf("invalid PRESIGNED_URL_EXPIRY: %w", err)
	}
	cfg.PresignedURLExpiry = expiry

	// Parse cache window
	cacheWindowStr := getEnv("CACHE_WINDOW", "1h")
	cacheWindow, err := time.ParseDuration(cacheWindowStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_WINDOW: %w", err)
	}
	cfg.CacheWindow = cacheWindow

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration values are set
func (c *Config) Validate() error {
	// Determine database type
	dbType := c.DBType
	if dbType == "" {
		if c.DBPath != "" {
			dbType = "sqlite3"
		} else if c.DBUser != "" {
			dbType = "mysql"
		} else {
			dbType = "sqlite3" // Default to SQLite
		}
	}

	// Validate database config
	if dbType == "mysql" {
		required := map[string]string{
			"DB_USER":     c.DBUser,
			"DB_PASSWORD": c.DBPassword,
		}
		for name, value := range required {
			if value == "" {
				return fmt.Errorf("required environment variable %s is not set for MySQL", name)
			}
		}
	}

	// Validate S3 config
	required := map[string]string{
		"AWS_ACCESS_KEY_ID":     c.AWSAccessKeyID,
		"AWS_SECRET_ACCESS_KEY": c.AWSSecretAccessKey,
		"S3_BUCKET":             c.S3Bucket,
	}

	for name, value := range required {
		if value == "" {
			return fmt.Errorf("required environment variable %s is not set", name)
		}
	}

	return nil
}

// DSN returns the database connection string
func (c *Config) DSN() string {
	dbType := c.GetDBType()

	if dbType == "sqlite3" {
		return c.DBPath
	}

	// MySQL DSN
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		c.DBUser,
		c.DBPassword,
		c.DBHost,
		c.DBPort,
		c.DBName,
	)
}

// GetDBType returns the database type (auto-detects if not explicitly set)
func (c *Config) GetDBType() string {
	// Use explicit type if set
	if c.DBType != "" {
		return c.DBType
	}

	// Auto-detect based on config
	if c.DBUser != "" || c.DBPassword != "" {
		return "mysql"
	}

	return "sqlite3" // Default to SQLite
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
