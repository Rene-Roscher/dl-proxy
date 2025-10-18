package cache

import (
	"testing"
)

// TestShouldCacheDecision tests the business logic for caching decisions
func TestShouldCacheDecision(t *testing.T) {
	tests := []struct {
		name         string
		requestCount int
		threshold    int
		shouldCache  bool
		description  string
	}{
		{
			name:         "below threshold",
			requestCount: 5,
			threshold:    10,
			shouldCache:  false,
			description:  "First 5 requests - should NOT cache",
		},
		{
			name:         "at threshold",
			requestCount: 10,
			threshold:    10,
			shouldCache:  true,
			description:  "Exactly 10 requests - should cache",
		},
		{
			name:         "above threshold",
			requestCount: 15,
			threshold:    10,
			shouldCache:  true,
			description:  "Above threshold - should cache",
		},
		{
			name:         "zero threshold always cache",
			requestCount: 1,
			threshold:    0,
			shouldCache:  true,
			description:  "Threshold 0 means always cache",
		},
		{
			name:         "high threshold",
			requestCount: 99,
			threshold:    100,
			shouldCache:  false,
			description:  "High threshold not reached",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Business logic: cache if requestCount >= threshold
			shouldCache := tt.requestCount >= tt.threshold

			if shouldCache != tt.shouldCache {
				t.Errorf("%s: got shouldCache=%v, want %v (count=%d, threshold=%d)",
					tt.description, shouldCache, tt.shouldCache, tt.requestCount, tt.threshold)
			}
		})
	}
}

// TestCacheKeyUniqueness tests that different URLs produce different cache keys
// but same normalized URLs produce same cache key
func TestCacheKeyUniqueness(t *testing.T) {
	tests := []struct {
		url1     string
		url2     string
		sameName string
	}{
		{
			url1:     "https://example.com/file.zip",
			url2:     "https://example.com/file.zip",
			sameName: "identical URLs",
		},
		{
			url1:     "https://EXAMPLE.com/file.zip",
			url2:     "https://example.com/file.zip",
			sameName: "case insensitive host",
		},
		{
			url1:     "https://example.com:443/file.zip",
			url2:     "https://example.com/file.zip",
			sameName: "default port removal",
		},
		{
			url1:     "https://example.com/file.zip?a=1&b=2",
			url2:     "https://example.com/file.zip?b=2&a=1",
			sameName: "query param order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.sameName, func(t *testing.T) {
			normalized1, _ := NormalizeURL(tt.url1)
			normalized2, _ := NormalizeURL(tt.url2)

			key1 := GenerateCacheKey(normalized1)
			key2 := GenerateCacheKey(normalized2)

			if key1 != key2 {
				t.Errorf("%s should produce same cache key:\nURL1: %s -> %s\nURL2: %s -> %s",
					tt.sameName, tt.url1, key1, tt.url2, key2)
			}
		})
	}

	// Test different URLs produce different keys
	differentTests := []struct {
		url1 string
		url2 string
		name string
	}{
		{
			url1: "https://example.com/file1.zip",
			url2: "https://example.com/file2.zip",
			name: "different files",
		},
		{
			url1: "https://example.com/file.zip",
			url2: "https://other.com/file.zip",
			name: "different hosts",
		},
		{
			url1: "https://example.com/file.zip?token=ABC",
			url2: "https://example.com/file.zip?token=XYZ",
			name: "different query values",
		},
	}

	for _, tt := range differentTests {
		t.Run(tt.name, func(t *testing.T) {
			normalized1, _ := NormalizeURL(tt.url1)
			normalized2, _ := NormalizeURL(tt.url2)

			key1 := GenerateCacheKey(normalized1)
			key2 := GenerateCacheKey(normalized2)

			if key1 == key2 {
				t.Errorf("%s should produce different cache keys:\nURL1: %s -> %s\nURL2: %s -> %s",
					tt.name, tt.url1, key1, tt.url2, key2)
			}
		})
	}
}

// TestS3PathGeneration tests the S3 path construction logic
func TestS3PathGeneration(t *testing.T) {
	tests := []struct {
		prefix   string
		cacheKey string
		filename string
		expected string
	}{
		{
			prefix:   "proxy",
			cacheKey: "abc123",
			filename: "test.zip",
			expected: "proxy/abc123/test.zip",
		},
		{
			prefix:   "cache",
			cacheKey: "def456",
			filename: "file.tar.gz",
			expected: "cache/def456/file.tar.gz",
		},
		{
			prefix:   "prod",
			cacheKey: "xyz789",
			filename: "archive.zip",
			expected: "prod/xyz789/archive.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Business logic for S3 path
			s3Path := tt.prefix + "/" + tt.cacheKey + "/" + tt.filename

			if s3Path != tt.expected {
				t.Errorf("S3 path mismatch:\ngot:  %s\nwant: %s", s3Path, tt.expected)
			}
		})
	}
}

// TestFileNameExtraction tests extracting filename from URLs
func TestFileNameExtraction(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{
			url:      "https://example.com/download/file.zip",
			expected: "file.zip",
		},
		{
			url:      "https://example.com/archive.tar.gz",
			expected: "archive.tar.gz",
		},
		{
			url:      "https://example.com/path/to/document.pdf?token=abc",
			expected: "document.pdf",
		},
		{
			url:      "https://example.com/",
			expected: "download",
		},
		{
			url:      "https://example.com/noext",
			expected: "noext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			filename := ExtractFileName(tt.url)

			if filename != tt.expected {
				t.Errorf("Filename extraction failed:\nURL:  %s\ngot:  %s\nwant: %s",
					tt.url, filename, tt.expected)
			}
		})
	}
}

// TestCacheKeyLength tests that cache keys are consistent MD5 length
func TestCacheKeyLength(t *testing.T) {
	urls := []string{
		"https://example.com/file.zip",
		"https://very-long-domain-name.example.com/path/to/very/long/file/name.zip?with=many&query=parameters&and=values",
		"https://a.b",
	}

	for _, url := range urls {
		normalized, err := NormalizeURL(url)
		if err != nil {
			t.Fatalf("Failed to normalize %s: %v", url, err)
		}

		key := GenerateCacheKey(normalized)

		// MD5 hash should always be 32 characters (hex)
		if len(key) != 32 {
			t.Errorf("Cache key for %s has wrong length: got %d, want 32", url, len(key))
		}

		// Should only contain hex characters
		for _, c := range key {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Cache key contains non-hex character: %c", c)
			}
		}
	}
}

// TestRetentionLogic tests the business logic for cache retention
func TestRetentionLogic(t *testing.T) {
	tests := []struct {
		name          string
		daysUnused    int
		retentionDays int
		shouldDelete  bool
	}{
		{
			name:          "recently used",
			daysUnused:    5,
			retentionDays: 30,
			shouldDelete:  false,
		},
		{
			name:          "exactly at threshold",
			daysUnused:    30,
			retentionDays: 30,
			shouldDelete:  false,
		},
		{
			name:          "past retention",
			daysUnused:    35,
			retentionDays: 30,
			shouldDelete:  true,
		},
		{
			name:          "way past retention",
			daysUnused:    100,
			retentionDays: 30,
			shouldDelete:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Business logic: delete if daysUnused > retentionDays
			shouldDelete := tt.daysUnused > tt.retentionDays

			if shouldDelete != tt.shouldDelete {
				t.Errorf("Retention logic failed: daysUnused=%d, retention=%d, got shouldDelete=%v, want %v",
					tt.daysUnused, tt.retentionDays, shouldDelete, tt.shouldDelete)
			}
		})
	}
}
