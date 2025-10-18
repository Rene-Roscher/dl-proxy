package cache

import (
	"fmt"
	"io"
	"net/http"
)

// Downloader handles downloading files from remote URLs
type Downloader struct {
	client *http.Client
}

// NewDownloader creates a new downloader with a configured HTTP client
func NewDownloader() *Downloader {
	return &Downloader{
		client: &http.Client{
			// Don't follow redirects automatically, let caller decide
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// DownloadInfo contains metadata about a download
type DownloadInfo struct {
	ContentType   string
	ContentLength int64
	Reader        io.ReadCloser
}

// Download initiates a download from a URL and returns a reader
// The caller is responsible for closing the reader
func (d *Downloader) Download(url string) (*DownloadInfo, error) {
	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "DownloadProxy/1.0")

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Extract metadata
	info := &DownloadInfo{
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.ContentLength,
		Reader:        resp.Body,
	}

	// Default content type if not specified
	if info.ContentType == "" {
		info.ContentType = "application/octet-stream"
	}

	return info, nil
}

// TeeReader creates a reader that writes to multiple writers simultaneously
// This is used to stream data to both the client and S3 at the same time
type TeeReader struct {
	reader  io.Reader
	writers []io.Writer
}

// NewTeeReader creates a new TeeReader
func NewTeeReader(reader io.Reader, writers ...io.Writer) *TeeReader {
	return &TeeReader{
		reader:  reader,
		writers: writers,
	}
}

// Read implements io.Reader, writing to all configured writers
func (t *TeeReader) Read(p []byte) (n int, err error) {
	n, err = t.reader.Read(p)
	if n > 0 {
		// Write to all writers
		for _, w := range t.writers {
			if _, writeErr := w.Write(p[:n]); writeErr != nil {
				return n, writeErr
			}
		}
	}
	return n, err
}
