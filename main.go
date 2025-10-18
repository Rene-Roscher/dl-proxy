package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"download-proxy/cache"
	"download-proxy/config"
	"download-proxy/db"
	"download-proxy/security"
)

func main() {
	log.Println("Starting Download Proxy...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded successfully")
	log.Printf("Server will listen on port: %s", cfg.Port)

	// Get database type
	dbType := cfg.GetDBType()

	if dbType == "sqlite3" {
		log.Printf("Database: SQLite (%s)", cfg.DBPath)
		// Ensure data directory exists for SQLite
		if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
			log.Fatalf("Failed to create data directory: %v", err)
		}
	} else {
		log.Printf("Database: MySQL %s@%s:%s/%s", cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)
	}
	log.Printf("S3 Bucket: %s (region: %s)", cfg.S3Bucket, cfg.AWSRegion)

	// Connect to database
	database, err := db.New(cfg.DSN(), dbType)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	log.Println("Database connection established")

	// Run migrations
	var migrationPattern string
	if dbType == "sqlite3" {
		migrationPattern = filepath.Join("db", "migrations", "*_sqlite.sql")
	} else {
		migrationPattern = filepath.Join("db", "migrations", "001_init.sql")
	}

	// Find and run migration files
	migrationFiles, err := filepath.Glob(migrationPattern)
	if err != nil {
		log.Fatalf("Failed to find migration files: %v", err)
	}
	if len(migrationFiles) == 0 {
		log.Fatalf("No migration files found matching pattern: %s", migrationPattern)
	}

	for _, migFile := range migrationFiles {
		log.Printf("Running migration: %s", filepath.Base(migFile))
		content, err := os.ReadFile(migFile)
		if err != nil {
			log.Fatalf("Failed to read migration file %s: %v", migFile, err)
		}
		if _, err := database.Exec(string(content)); err != nil {
			log.Fatalf("Failed to execute migration %s: %v", filepath.Base(migFile), err)
		}
		log.Printf("✓ Applied migration: %s", filepath.Base(migFile))
	}

	// Initialize S3 client
	s3Client, err := cache.NewS3Client(
		cfg.AWSRegion,
		cfg.AWSAccessKeyID,
		cfg.AWSSecretAccessKey,
		cfg.S3Bucket,
		cfg.S3Prefix,
		cfg.S3Endpoint,
	)
	if err != nil {
		log.Fatalf("Failed to initialize S3 client: %v", err)
	}

	log.Println("S3 client initialized")

	// Initialize security validator
	validator, err := security.NewValidator(cfg.AllowedIPNets)
	if err != nil {
		log.Fatalf("Failed to initialize security validator: %v", err)
	}

	if len(cfg.AllowedIPNets) > 0 {
		log.Printf("IP subnet filtering enabled: %v", cfg.AllowedIPNets)
	} else {
		log.Println("WARNING: No IP subnet filtering - all IPs allowed")
	}

	// Create handler with smart caching configuration
	handler := cache.NewHandler(
		database,
		s3Client,
		validator,
		cfg.PresignedURLExpiry,
		cfg.DownloadTimeout,
		cfg.CacheThreshold,
		cfg.CacheWindow,
	)

	log.Printf("Smart caching enabled: %d requests in %v triggers caching", cfg.CacheThreshold, cfg.CacheWindow)
	log.Printf("Download timeout: %v", cfg.DownloadTimeout)
	log.Printf("Cleanup: Files unused for %d days will be deleted", cfg.CacheRetentionDays)

	// Start cleanup goroutine
	go runCleanupJob(database, s3Client, cfg.CacheRetentionDays)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.Handle("/proxy", handler)
	mux.HandleFunc("/health", healthCheckHandler)
	mux.HandleFunc("/", rootHandler)

	// Create HTTP server with timeouts
	addr := fmt.Sprintf(":%s", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.DownloadTimeout + (1 * time.Minute), // Download timeout + buffer
		IdleTimeout:  120 * time.Second,
	}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down gracefully...")
		if err := server.Close(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Start server
	log.Printf("Server listening on %s", addr)
	log.Printf("Ready to accept requests at http://localhost%s/proxy?url=...", addr)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	log.Println("Server stopped")
}

// runCleanupJob periodically cleans up unused cached files from R2
func runCleanupJob(database *db.DB, s3Client *cache.S3Client, retentionDays int) {
	ticker := time.NewTicker(24 * time.Hour) // Run daily
	defer ticker.Stop()

	// Run once immediately on startup (after 5 minutes)
	time.Sleep(5 * time.Minute)
	performCleanup(database, s3Client, retentionDays)

	for range ticker.C {
		performCleanup(database, s3Client, retentionDays)
	}
}

// performCleanup executes the cleanup logic
func performCleanup(database *db.DB, s3Client *cache.S3Client, retentionDays int) {
	log.Printf("Starting cleanup job (retention: %d days)", retentionDays)

	// Get unused files
	unusedFiles, err := database.GetUnusedCachedFiles(retentionDays)
	if err != nil {
		log.Printf("Failed to get unused files: %v", err)
		return
	}

	if len(unusedFiles) == 0 {
		log.Println("No files to clean up")
		return
	}

	log.Printf("Found %d files to clean up", len(unusedFiles))

	var deletedCount int
	var deletedBytes int64

	for _, file := range unusedFiles {
		log.Printf("Deleting unused file: %s (last used: %v, size: %d bytes)",
			file.CacheKey, file.LastRequestedAt, file.FileSize)

		// Delete from R2
		if err := s3Client.DeleteObject(file.S3Path); err != nil {
			log.Printf("Failed to delete from R2: %v", err)
			continue
		}

		// Delete from database
		if err := database.DeleteFile(file.ID); err != nil {
			log.Printf("Failed to delete from database: %v", err)
			continue
		}

		deletedCount++
		deletedBytes += file.FileSize
	}

	log.Printf("Cleanup completed: deleted %d files, freed %.2f GB",
		deletedCount, float64(deletedBytes)/1024/1024/1024)
}

// healthCheckHandler handles health check requests
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","service":"download-proxy"}`)
}

// rootHandler handles requests to the root path
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Download Proxy</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
        .endpoint { background: #e8f4f8; padding: 15px; margin: 15px 0; border-radius: 5px; }
        .example { background: #f9f9f9; padding: 10px; margin: 10px 0; border-left: 3px solid #4CAF50; }
    </style>
</head>
<body>
    <h1>Download Proxy Service</h1>
    <p>A lightweight, high-performance download proxy that caches files to S3.</p>

    <div class="endpoint">
        <h2>Endpoint</h2>
        <p><code>GET /proxy?url={target_url}</code></p>
    </div>

    <div class="example">
        <h3>Example</h3>
        <code>curl "http://localhost:8080/proxy?url=https://example.com/file.zip"</code>
    </div>

    <h3>How it works</h3>
    <ul>
        <li><strong>First request:</strong> Downloads from source, streams to S3, serves to client (HTTP 200)</li>
        <li><strong>Cached request:</strong> Redirects to S3 presigned URL (HTTP 302)</li>
    </ul>

    <h3>Health Check</h3>
    <p><code>GET /health</code> - Returns service health status</p>
</body>
</html>`)
}
