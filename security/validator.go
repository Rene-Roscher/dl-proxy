package security

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Validator handles security validation for requests
type Validator struct {
	allowedNets []*net.IPNet
}

// NewValidator creates a new security validator
func NewValidator(allowedIPNets []string) (*Validator, error) {
	var nets []*net.IPNet

	for _, cidr := range allowedIPNets {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %s: %w", cidr, err)
		}
		nets = append(nets, ipNet)
	}

	return &Validator{
		allowedNets: nets,
	}, nil
}

// ValidateClientIP checks if the client IP is in allowed subnets
func (v *Validator) ValidateClientIP(clientIP string) error {
	// If no subnets configured, allow all
	if len(v.allowedNets) == 0 {
		return nil
	}

	// Parse client IP
	ip := net.ParseIP(clientIP)
	if ip == nil {
		// Try to extract IP from "IP:port" format
		host, _, err := net.SplitHostPort(clientIP)
		if err != nil {
			return fmt.Errorf("invalid client IP: %s", clientIP)
		}
		ip = net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("invalid client IP after parsing: %s", host)
		}
	}

	// Check if IP is in any allowed subnet
	for _, ipNet := range v.allowedNets {
		if ipNet.Contains(ip) {
			return nil
		}
	}

	return fmt.Errorf("client IP %s not in allowed subnets", ip.String())
}

// ValidateTargetURL checks if the target URL is safe to download from
func (v *Validator) ValidateTargetURL(targetURL string) error {
	u, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTP and HTTPS
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only http and https schemes allowed, got: %s", u.Scheme)
	}

	// Block localhost and loopback addresses
	hostname := strings.ToLower(u.Hostname())
	if isLocalhost(hostname) {
		return fmt.Errorf("localhost URLs not allowed")
	}

	// Resolve hostname to IP and check for private networks
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// DNS lookup failed - allow it (might be temporary)
		// In production you might want to block this
		return nil
	}

	// Check if any resolved IP is private/loopback
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("private IP addresses not allowed: %s resolves to %s", hostname, ip.String())
		}
	}

	return nil
}

// isLocalhost checks if hostname is localhost variant
func isLocalhost(hostname string) bool {
	localhost := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
		"::",
	}

	for _, local := range localhost {
		if hostname == local {
			return true
		}
	}

	return false
}

// isPrivateIP checks if IP is in private/reserved range
func isPrivateIP(ip net.IP) bool {
	// Check for loopback
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check for private ranges
	privateRanges := []string{
		"10.0.0.0/8",      // Private network
		"172.16.0.0/12",   // Private network
		"192.168.0.0/16",  // Private network
		"169.254.0.0/16",  // AWS metadata, link-local
		"fc00::/7",        // IPv6 private
		"fe80::/10",       // IPv6 link-local
	}

	for _, cidr := range privateRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}
