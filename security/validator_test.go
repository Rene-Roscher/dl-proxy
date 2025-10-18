package security

import (
	"net"
	"testing"
)

func TestValidateClientIP(t *testing.T) {
	tests := []struct {
		name          string
		allowedNets   []string
		clientIP      string
		shouldSucceed bool
	}{
		{
			name:          "no filtering - allow all",
			allowedNets:   []string{},
			clientIP:      "1.2.3.4",
			shouldSucceed: true,
		},
		{
			name:          "IP in allowed subnet",
			allowedNets:   []string{"109.71.254.0/24"},
			clientIP:      "109.71.254.100",
			shouldSucceed: true,
		},
		{
			name:          "IP not in allowed subnet",
			allowedNets:   []string{"109.71.254.0/24"},
			clientIP:      "109.71.255.100",
			shouldSucceed: false,
		},
		{
			name:          "IP in one of multiple subnets",
			allowedNets:   []string{"109.71.254.0/24", "192.168.1.0/24"},
			clientIP:      "192.168.1.50",
			shouldSucceed: true,
		},
		{
			name:          "IP with port notation",
			allowedNets:   []string{"109.71.254.0/24"},
			clientIP:      "109.71.254.100:54321",
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator(tt.allowedNets)
			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}

			err = validator.ValidateClientIP(tt.clientIP)
			if tt.shouldSucceed && err != nil {
				t.Errorf("Expected validation to succeed, got error: %v", err)
			}
			if !tt.shouldSucceed && err == nil {
				t.Error("Expected validation to fail, but it succeeded")
			}
		})
	}
}

func TestValidateTargetURL(t *testing.T) {
	validator, _ := NewValidator([]string{})

	tests := []struct {
		name          string
		url           string
		shouldSucceed bool
	}{
		{
			name:          "valid https URL",
			url:           "https://example.com/file.zip",
			shouldSucceed: true,
		},
		{
			name:          "valid http URL",
			url:           "http://example.com/file.zip",
			shouldSucceed: true,
		},
		{
			name:          "file scheme not allowed",
			url:           "file:///etc/passwd",
			shouldSucceed: false,
		},
		{
			name:          "ftp scheme not allowed",
			url:           "ftp://example.com/file.zip",
			shouldSucceed: false,
		},
		{
			name:          "localhost not allowed",
			url:           "http://localhost:8080/file",
			shouldSucceed: false,
		},
		{
			name:          "127.0.0.1 not allowed",
			url:           "http://127.0.0.1:8080/file",
			shouldSucceed: false,
		},
		{
			name:          "0.0.0.0 not allowed",
			url:           "http://0.0.0.0/file",
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTargetURL(tt.url)
			if tt.shouldSucceed && err != nil {
				t.Errorf("Expected validation to succeed, got error: %v", err)
			}
			if !tt.shouldSucceed && err == nil {
				t.Error("Expected validation to fail, but it succeeded")
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip        string
		isPrivate bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // AWS metadata
		{"8.8.8.8", false},         // Google DNS
		{"1.1.1.1", false},         // Cloudflare DNS
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := mustParseIP(tt.ip)
			result := isPrivateIP(ip)
			if result != tt.isPrivate {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.isPrivate)
			}
		})
	}
}

func mustParseIP(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		panic("invalid IP: " + s)
	}
	return ip
}
