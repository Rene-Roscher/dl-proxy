package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "valid config with MySQL",
			config: &Config{
				DBType:             "mysql",
				DBUser:             "user",
				DBPassword:         "pass",
				AWSAccessKeyID:     "key",
				AWSSecretAccessKey: "secret",
				S3Bucket:           "bucket",
			},
			wantError: false,
		},
		{
			name: "valid config with SQLite",
			config: &Config{
				DBPath:             "./data/test.db",
				AWSAccessKeyID:     "key",
				AWSSecretAccessKey: "secret",
				S3Bucket:           "bucket",
			},
			wantError: false,
		},
		{
			name: "missing DB user for MySQL",
			config: &Config{
				DBType:             "mysql",
				DBPassword:         "pass",
				AWSAccessKeyID:     "key",
				AWSSecretAccessKey: "secret",
				S3Bucket:           "bucket",
			},
			wantError: true,
		},
		{
			name: "missing DB password for MySQL",
			config: &Config{
				DBType:             "mysql",
				DBUser:             "user",
				AWSAccessKeyID:     "key",
				AWSSecretAccessKey: "secret",
				S3Bucket:           "bucket",
			},
			wantError: true,
		},
		{
			name: "missing AWS access key",
			config: &Config{
				DBPath:             "./data/test.db",
				AWSSecretAccessKey: "secret",
				S3Bucket:           "bucket",
			},
			wantError: true,
		},
		{
			name: "missing AWS secret key",
			config: &Config{
				DBPath:         "./data/test.db",
				AWSAccessKeyID: "key",
				S3Bucket:       "bucket",
			},
			wantError: true,
		},
		{
			name: "missing S3 bucket",
			config: &Config{
				DBPath:             "./data/test.db",
				AWSAccessKeyID:     "key",
				AWSSecretAccessKey: "secret",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestDSNGeneration(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "MySQL DSN",
			config: &Config{
				DBType:     "mysql",
				DBUser:     "testuser",
				DBPassword: "testpass",
				DBHost:     "localhost",
				DBPort:     "3306",
				DBName:     "testdb",
			},
			expected: "testuser:testpass@tcp(localhost:3306)/testdb?parseTime=true&multiStatements=true",
		},
		{
			name: "SQLite DSN",
			config: &Config{
				DBPath: "./data/test.db",
			},
			expected: "./data/test.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.config.DSN()
			if dsn != tt.expected {
				t.Errorf("DSN mismatch:\ngot:  %s\nwant: %s", dsn, tt.expected)
			}
		})
	}
}

func TestConfigLoadWithEnv(t *testing.T) {
	// Set test environment variables
	os.Setenv("PORT", "9090")
	os.Setenv("DB_USER", "envuser")
	os.Setenv("DB_PASSWORD", "envpass")
	os.Setenv("AWS_ACCESS_KEY_ID", "envkey")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")
	os.Setenv("S3_BUCKET", "envbucket")
	os.Setenv("CACHE_THRESHOLD", "20")
	os.Setenv("CACHE_WINDOW", "2h")
	os.Setenv("CACHE_RETENTION_DAYS", "60")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("S3_BUCKET")
		os.Unsetenv("CACHE_THRESHOLD")
		os.Unsetenv("CACHE_WINDOW")
		os.Unsetenv("CACHE_RETENTION_DAYS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port: got %s, want 9090", cfg.Port)
	}

	if cfg.DBUser != "envuser" {
		t.Errorf("DBUser: got %s, want envuser", cfg.DBUser)
	}

	if cfg.CacheThreshold != 20 {
		t.Errorf("CacheThreshold: got %d, want 20", cfg.CacheThreshold)
	}

	if cfg.CacheWindow != 2*time.Hour {
		t.Errorf("CacheWindow: got %v, want 2h", cfg.CacheWindow)
	}

	if cfg.CacheRetentionDays != 60 {
		t.Errorf("CacheRetentionDays: got %d, want 60", cfg.CacheRetentionDays)
	}
}

func TestInvalidDurationParsing(t *testing.T) {
	os.Setenv("PRESIGNED_URL_EXPIRY", "invalid")
	os.Setenv("DB_USER", "test")
	os.Setenv("DB_PASSWORD", "test")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("S3_BUCKET", "test")

	defer func() {
		os.Unsetenv("PRESIGNED_URL_EXPIRY")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("S3_BUCKET")
	}()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for invalid PRESIGNED_URL_EXPIRY")
	}
}

func TestCacheWindowInvalidDuration(t *testing.T) {
	os.Setenv("CACHE_WINDOW", "notaduration")
	os.Setenv("DB_USER", "test")
	os.Setenv("DB_PASSWORD", "test")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("S3_BUCKET", "test")

	defer func() {
		os.Unsetenv("CACHE_WINDOW")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("S3_BUCKET")
	}()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for invalid CACHE_WINDOW")
	}
}
