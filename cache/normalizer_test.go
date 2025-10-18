package cache

import (
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "basic URL with protocol removal",
			input:    "https://example.com/file.zip",
			expected: "example.com/file.zip",
			wantErr:  false,
		},
		{
			name:     "lowercase host and path",
			input:    "https://EXAMPLE.com:443/FILE.zip",
			expected: "example.com/file.zip",
			wantErr:  false,
		},
		{
			name:     "query params sorted with case-sensitive values",
			input:    "https://EXAMPLE.com:443/FILE.zip?token=ABC&v=2",
			expected: "example.com/file.zip?token=ABC&v=2",
			wantErr:  false,
		},
		{
			name:     "query params sorted alphabetically",
			input:    "https://example.com/file.zip?z=1&a=2&m=3",
			expected: "example.com/file.zip?a=2&m=3&z=1",
			wantErr:  false,
		},
		{
			name:     "preserve non-standard port",
			input:    "https://example.com:8080/file.zip",
			expected: "example.com:8080/file.zip",
			wantErr:  false,
		},
		{
			name:     "remove default https port",
			input:    "https://example.com:443/file.zip",
			expected: "example.com/file.zip",
			wantErr:  false,
		},
		{
			name:     "remove default http port",
			input:    "http://example.com:80/file.zip",
			expected: "example.com/file.zip",
			wantErr:  false,
		},
		{
			name:     "remove trailing slash",
			input:    "https://example.com/path/",
			expected: "example.com/path",
			wantErr:  false,
		},
		{
			name:     "remove fragment",
			input:    "https://example.com/file.zip#section",
			expected: "example.com/file.zip",
			wantErr:  false,
		},
		{
			name:     "complex URL with all features",
			input:    "https://DOWNLOAD.Example.COM:443/Path/To/FILE.zip?Token=XYZ&version=2&auth=ABC#fragment",
			expected: "download.example.com/path/to/file.zip?auth=ABC&token=XYZ&version=2",
			wantErr:  false,
		},
		{
			name:     "URL with multiple query params same key",
			input:    "https://example.com/file?tag=a&tag=b&tag=c",
			expected: "example.com/file?tag=a&tag=b&tag=c",
			wantErr:  false,
		},
		{
			name:     "root path",
			input:    "https://example.com/",
			expected: "example.com/",
			wantErr:  false,
		},
		{
			name:     "path with dots",
			input:    "https://example.com/path/./to/../file.zip",
			expected: "example.com/path/./to/../file.zip",
			wantErr:  false,
		},
		{
			name:     "query params with special characters in values",
			input:    "https://example.com/file?url=https://other.com/path&token=ABC_123",
			expected: "example.com/file?token=ABC_123&url=https://other.com/path",
			wantErr:  false,
		},
		{
			name:     "lowercased query keys but preserve value case",
			input:    "https://example.com/file?TOKEN=AbCdEf&Version=2",
			expected: "example.com/file?token=AbCdEf&version=2",
			wantErr:  false,
		},
		{
			name:    "invalid URL",
			input:   "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple URL",
			input: "example.com/file.zip",
		},
		{
			name:  "URL with query params",
			input: "example.com/file.zip?token=abc&v=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := GenerateCacheKey(tt.input)
			// MD5 hash should be 32 characters (hex)
			if len(key) != 32 {
				t.Errorf("cache key length = %d, expected 32", len(key))
			}

			// Same input should produce same key
			key2 := GenerateCacheKey(tt.input)
			if key != key2 {
				t.Errorf("same input produced different keys: %s vs %s", key, key2)
			}
		})
	}
}

func TestGenerateCacheKeyConsistency(t *testing.T) {
	// Test that the same normalized URL always produces the same cache key
	url1 := "example.com/file.zip?a=1&b=2"
	url2 := "example.com/file.zip?a=1&b=2"

	key1 := GenerateCacheKey(url1)
	key2 := GenerateCacheKey(url2)

	if key1 != key2 {
		t.Errorf("identical URLs produced different cache keys: %s vs %s", key1, key2)
	}
}

func TestExtractFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple filename",
			input:    "https://example.com/file.zip",
			expected: "file.zip",
		},
		{
			name:     "nested path",
			input:    "https://example.com/path/to/document.pdf",
			expected: "document.pdf",
		},
		{
			name:     "filename with query params",
			input:    "https://example.com/download/file.zip?token=abc",
			expected: "file.zip",
		},
		{
			name:     "root path",
			input:    "https://example.com/",
			expected: "download",
		},
		{
			name:     "no extension",
			input:    "https://example.com/myfile",
			expected: "myfile",
		},
		{
			name:     "multiple dots in filename",
			input:    "https://example.com/archive.tar.gz",
			expected: "archive.tar.gz",
		},
		{
			name:     "invalid URL",
			input:    "://invalid",
			expected: "download",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractFileName(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}
