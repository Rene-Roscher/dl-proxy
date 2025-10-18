package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
)

// NormalizeURL converts a URL to a canonical form for caching
// Rules:
// 1. Remove protocol (http/https)
// 2. Lowercase host and path
// 3. Remove default ports (:80, :443)
// 4. Sort query parameters by key (lowercased keys)
// 5. Preserve query parameter VALUES (case-sensitive)
// 6. Remove fragments (#section)
// 7. Remove trailing slashes
// 8. Keep non-standard ports
func NormalizeURL(rawURL string) (string, error) {
	// Parse the URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Extract host (lowercase)
	host := strings.ToLower(u.Host)

	// Remove default ports
	if strings.HasSuffix(host, ":80") || strings.HasSuffix(host, ":443") {
		// Only remove if it's actually the default port for the scheme
		if (strings.HasSuffix(host, ":80") && u.Scheme == "http") ||
			(strings.HasSuffix(host, ":443") && u.Scheme == "https") {
			host = host[:strings.LastIndex(host, ":")]
		}
	}

	// Get path (lowercase, remove trailing slash)
	pathStr := strings.ToLower(u.Path)
	pathStr = strings.TrimRight(pathStr, "/")
	if pathStr == "" {
		pathStr = "/"
	}

	// Sort query parameters by lowercased key, preserve value case
	queryStr := normalizeQuery(u.Query())

	// Build normalized URL: host + path + query
	normalized := host + pathStr
	if queryStr != "" {
		normalized += "?" + queryStr
	}

	return normalized, nil
}

// normalizeQuery sorts query parameters by lowercased key
// while preserving the case of the values
func normalizeQuery(query url.Values) string {
	if len(query) == 0 {
		return ""
	}

	// Create a slice of key-value pairs with lowercased keys
	type param struct {
		key   string
		value string
	}

	var params []param
	for key, values := range query {
		lowerKey := strings.ToLower(key)
		for _, value := range values {
			params = append(params, param{key: lowerKey, value: value})
		}
	}

	// Sort by key (already lowercased)
	sort.Slice(params, func(i, j int) bool {
		if params[i].key == params[j].key {
			return params[i].value < params[j].value
		}
		return params[i].key < params[j].key
	})

	// Build query string
	var parts []string
	for _, p := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", p.key, p.value))
	}

	return strings.Join(parts, "&")
}

// GenerateCacheKey creates an MD5 hash from a normalized URL
func GenerateCacheKey(normalizedURL string) string {
	hash := md5.Sum([]byte(normalizedURL))
	return hex.EncodeToString(hash[:])
}

// ExtractFileName extracts the filename from a URL path
func ExtractFileName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "download"
	}

	filename := path.Base(u.Path)
	if filename == "/" || filename == "." {
		return "download"
	}

	return filename
}
