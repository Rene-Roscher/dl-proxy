package cache

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"download-proxy/db"
	"download-proxy/security"
)

// Handler handles proxy requests
type Handler struct {
	db             *db.DB
	s3             *S3Client
	downloader     *Downloader
	validator      *security.Validator
	urlExpiry      time.Duration
	cacheThreshold int
	cacheWindow    time.Duration
}

// NewHandler creates a new proxy handler
func NewHandler(database *db.DB, s3Client *S3Client, validator *security.Validator, urlExpiry time.Duration, downloadTimeout time.Duration, cacheThreshold int, cacheWindow time.Duration) *Handler {
	return &Handler{
		db:             database,
		s3:             s3Client,
		downloader:     NewDownloader(downloadTimeout),
		validator:      validator,
		urlExpiry:      urlExpiry,
		cacheThreshold: cacheThreshold,
		cacheWindow:    cacheWindow,
	}
}

// ServeHTTP handles the /proxy endpoint
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate client IP
	clientIP := getClientIP(r)
	if err := h.validator.ValidateClientIP(clientIP); err != nil {
		log.Printf("IP validation failed for %s: %v", clientIP, err)
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Get URL parameter
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}

	// Validate target URL for security (SSRF protection)
	if err := h.validator.ValidateTargetURL(targetURL); err != nil {
		log.Printf("URL validation failed for %s: %v", targetURL, err)
		http.Error(w, "URL not allowed", http.StatusForbidden)
		return
	}

	// Normalize URL and generate cache key
	normalizedURL, err := NormalizeURL(targetURL)
	if err != nil {
		log.Printf("Failed to normalize URL %s: %v", targetURL, err)
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	cacheKey := GenerateCacheKey(normalizedURL)
	filename := ExtractFileName(targetURL)

	log.Printf("Request for URL: %s (cache_key: %s)", targetURL, cacheKey)

	// Check if file exists in database
	file, err := h.db.GetFileByCacheKey(cacheKey)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If file is cached, serve from R2
	if file != nil && file.Cached {
		h.handleCachedFile(w, r, file, startTime)
		return
	}

	// File exists but not cached yet - check if we should cache it
	if file != nil && !file.Cached {
		h.handleUncachedExistingFile(w, r, file, targetURL, startTime)
		return
	}

	// File doesn't exist yet - create tracking entry and serve directly
	h.handleNewFile(w, r, targetURL, normalizedURL, cacheKey, filename, startTime)
}

// handleCachedFile handles requests for already cached files
func (h *Handler) handleCachedFile(w http.ResponseWriter, r *http.Request, file *db.File, startTime time.Time) {
	log.Printf("Cache HIT for %s (requested %d times)", file.CacheKey, file.RequestCount)

	// Increment request counter
	if err := h.db.IncrementRequestCount(file.ID); err != nil {
		log.Printf("Failed to increment request count: %v", err)
	}

	// Generate presigned URL
	presignedURL, err := h.s3.GeneratePresignedURL(file.S3Path, h.urlExpiry)
	if err != nil {
		log.Printf("Failed to generate presigned URL: %v", err)
		http.Error(w, "Failed to generate download URL", http.StatusInternalServerError)
		h.logRequest(file.ID, r, http.StatusInternalServerError, 0, startTime)
		return
	}

	// Log request
	h.logRequest(file.ID, r, http.StatusFound, 0, startTime)

	// Redirect to presigned URL
	http.Redirect(w, r, presignedURL, http.StatusFound)
}

// handleUncachedExistingFile handles files that exist but aren't cached yet
// Checks if request threshold is met to trigger caching
func (h *Handler) handleUncachedExistingFile(w http.ResponseWriter, r *http.Request, file *db.File, targetURL string, startTime time.Time) {
	// Increment request counter first
	if err := h.db.IncrementRequestCount(file.ID); err != nil {
		log.Printf("Failed to increment request count: %v", err)
	}

	// Check request count in time window
	requestCount, err := h.db.GetRequestCountInWindow(file.CacheKey, h.cacheWindow)
	if err != nil {
		log.Printf("Failed to get request count: %v", err)
		requestCount = file.RequestCount // Fallback to total count
	}

	log.Printf("File %s not cached - %d requests in last %v (threshold: %d)",
		file.CacheKey, requestCount, h.cacheWindow, h.cacheThreshold)

	// Check if we've hit the threshold to start caching
	shouldCache := requestCount >= h.cacheThreshold

	if shouldCache {
		log.Printf("Threshold reached! Caching file %s to R2", file.CacheKey)
		h.downloadAndCache(w, r, file, targetURL, startTime)
	} else {
		log.Printf("Threshold not reached - proxying without caching (%d/%d)",
			requestCount, h.cacheThreshold)
		h.proxyWithoutCaching(w, r, file, targetURL, startTime)
	}
}

// handleNewFile handles first-time requests for unknown files
func (h *Handler) handleNewFile(w http.ResponseWriter, r *http.Request, targetURL, normalizedURL, cacheKey, filename string, startTime time.Time) {
	log.Printf("New file %s - creating tracking entry", cacheKey)

	// Create file entry in database (not cached)
	file := &db.File{
		CacheKey:      cacheKey,
		FileName:      filename,
		NormalizedURL: normalizedURL,
		OriginalURL:   targetURL,
		S3Path:        fmt.Sprintf("%s/%s/%s", h.s3.prefix, cacheKey, filename),
		Cached:        false,
		RequestCount:  1,
	}

	now := time.Now()
	file.LastRequestedAt = &now

	if err := h.db.CreateFile(file); err != nil {
		log.Printf("Failed to create file record: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("First request for file - proxying without caching (1/%d)", h.cacheThreshold)

	// Proxy without caching (not enough requests yet)
	h.proxyWithoutCaching(w, r, file, targetURL, startTime)
}

// proxyWithoutCaching downloads and serves the file without caching to R2
func (h *Handler) proxyWithoutCaching(w http.ResponseWriter, r *http.Request, file *db.File, targetURL string, startTime time.Time) {
	// Download from source
	downloadInfo, err := h.downloader.Download(targetURL)
	if err != nil {
		log.Printf("Failed to download from source: %v", err)
		http.Error(w, "Failed to download from source", http.StatusBadGateway)
		h.logRequest(file.ID, r, http.StatusBadGateway, 0, startTime)
		return
	}
	defer downloadInfo.Reader.Close()

	// Set response headers
	contentType := downloadInfo.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	if downloadInfo.ContentLength > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", downloadInfo.ContentLength))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.FileName))
	w.Header().Set("X-Cache-Status", "MISS-PASSTHROUGH")

	// Stream directly to client (no R2 upload)
	bytesServed, err := io.Copy(w, downloadInfo.Reader)
	if err != nil {
		log.Printf("Failed to stream to client: %v", err)
		h.logRequest(file.ID, r, http.StatusInternalServerError, bytesServed, startTime)
		return
	}

	// Log request
	h.logRequest(file.ID, r, http.StatusOK, bytesServed, startTime)
}

// downloadAndCache downloads the file and caches it to R2 while streaming to client
func (h *Handler) downloadAndCache(w http.ResponseWriter, r *http.Request, file *db.File, targetURL string, startTime time.Time) {
	log.Printf("Downloading and caching file %s", file.CacheKey)

	// Download from source
	downloadInfo, err := h.downloader.Download(targetURL)
	if err != nil {
		log.Printf("Failed to download from source: %v", err)
		errorMsg := fmt.Sprintf("Failed to download: %v", err)
		file.ErrorReason = &errorMsg
		h.db.UpdateFile(file)
		http.Error(w, "Failed to download from source", http.StatusBadGateway)
		h.logRequest(file.ID, r, http.StatusBadGateway, 0, startTime)
		return
	}
	defer downloadInfo.Reader.Close()

	// Update content type if we got it from the source
	contentType := downloadInfo.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Create a pipe for streaming to S3
	pipeReader, pipeWriter := io.Pipe()

	// Upload to S3 in background goroutine
	uploadErrChan := make(chan error, 1)
	go func() {
		defer pipeReader.Close()
		s3Path, err := h.s3.Upload(pipeReader, file.CacheKey, file.FileName, contentType)
		if err != nil {
			log.Printf("Failed to upload to S3: %v", err)
			uploadErrChan <- err
			return
		}
		log.Printf("Successfully uploaded to S3: %s", s3Path)
		uploadErrChan <- nil
	}()

	// Set response headers BEFORE streaming
	w.Header().Set("Content-Type", contentType)
	if downloadInfo.ContentLength > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", downloadInfo.ContentLength))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.FileName))
	w.Header().Set("X-Cache-Status", "MISS-CACHING")

	// Create multi-writer: stream to BOTH client AND S3 simultaneously
	multiWriter := io.MultiWriter(w, pipeWriter)

	// Stream to client and S3 simultaneously
	bytesServed, copyErr := io.Copy(multiWriter, downloadInfo.Reader)
	pipeWriter.Close() // Signal completion to S3 uploader

	// Wait for S3 upload to complete
	uploadErr := <-uploadErrChan

	// Update file record based on results
	if copyErr == nil && uploadErr == nil {
		file.Cached = true
		file.FileSize = bytesServed
		now := time.Now()
		file.FirstCachedAt = &now
		log.Printf("File %s cached successfully (%d bytes)", file.CacheKey, bytesServed)
	} else {
		errorMsg := fmt.Sprintf("copy: %v, upload: %v", copyErr, uploadErr)
		file.ErrorReason = &errorMsg
		log.Printf("Failed to cache file %s: %s", file.CacheKey, errorMsg)
	}

	h.db.UpdateFile(file)

	// Log request
	status := http.StatusOK
	if copyErr != nil {
		status = http.StatusInternalServerError
	}
	h.logRequest(file.ID, r, status, bytesServed, startTime)
}

// logRequest logs a request to the database
func (h *Handler) logRequest(fileID int64, r *http.Request, status int, bytesServed int64, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	req := &db.Request{
		FileID:      fileID,
		UserAgent:   r.UserAgent(),
		IPAddress:   getClientIP(r),
		HTTPStatus:  status,
		BytesServed: bytesServed,
		DurationMs:  duration,
	}

	if err := h.db.CreateRequest(req); err != nil {
		log.Printf("Failed to log request: %v", err)
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies/load balancers)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP in the chain
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// countingWriter counts bytes written
type countingWriter struct {
	count *int64
}

func (cw *countingWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	*cw.count += int64(n)
	return n, nil
}
