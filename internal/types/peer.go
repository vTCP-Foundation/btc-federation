// BTC-FED-2-3 - Peer Storage & File Management
// Package types contains data structures for peer management
package types

import (
	"encoding/base64"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// Constants for validation
const (
	// MaxAddresses defines the maximum number of addresses per peer
	MaxAddresses = 10
	// MinAddresses defines the minimum number of addresses per peer
	MinAddresses = 1
	// PublicKeyLength defines the expected length of base64-encoded public keys
	PublicKeyLength = 44 // 32 bytes base64 encoded
)

// Peer represents a network peer with public key and addresses
type Peer struct {
	PublicKey string   `yaml:"public_key" json:"public_key"`
	Addresses []string `yaml:"addresses" json:"addresses"`
}

// PeerList represents the root structure for peers.yaml file
type PeerList struct {
	Peers []Peer `yaml:"peers" json:"peers"`
}

// Validate validates the peer data format and constraints
func (p *Peer) Validate() error {
	if err := p.validatePublicKey(); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	if err := p.validateAddresses(); err != nil {
		return fmt.Errorf("invalid addresses: %w", err)
	}

	return nil
}

// validatePublicKey validates the public key format
func (p *Peer) validatePublicKey() error {
	if p.PublicKey == "" {
		return fmt.Errorf("public key cannot be empty")
	}

	// Validate base64 encoding
	decoded, err := base64.StdEncoding.DecodeString(p.PublicKey)
	if err != nil {
		return fmt.Errorf("public key must be valid base64: %w", err)
	}

	// Validate expected length (32 bytes for typical public keys)
	if len(decoded) != 32 {
		return fmt.Errorf("public key must be 32 bytes when decoded, got %d bytes", len(decoded))
	}

	return nil
}

// validateAddresses validates the address format and constraints
func (p *Peer) validateAddresses() error {
	if len(p.Addresses) < MinAddresses {
		return fmt.Errorf("peer must have at least %d address", MinAddresses)
	}

	if len(p.Addresses) > MaxAddresses {
		return fmt.Errorf("peer cannot have more than %d addresses", MaxAddresses)
	}

	for i, addr := range p.Addresses {
		if err := validateAddress(addr); err != nil {
			return fmt.Errorf("address %d is invalid: %w", i, err)
		}
	}

	return nil
}

// validateAddress validates a single multiaddr format
func validateAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("address cannot be empty")
	}

	// Basic multiaddr validation patterns
	ipv4Pattern := regexp.MustCompile(`^/ip4/(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/tcp/(\d+)$`)
	ipv6Pattern := regexp.MustCompile(`^/ip6/([^/]+)/tcp/(\d+)$`) // More permissive pattern for IPv6
	dnsPattern := regexp.MustCompile(`^/dns4?/([a-zA-Z0-9.-]+)/tcp/(\d+)$`)

	switch {
	case ipv4Pattern.MatchString(addr):
		return validateIPv4Address(addr, ipv4Pattern)
	case ipv6Pattern.MatchString(addr):
		return validateIPv6Address(addr, ipv6Pattern)
	case dnsPattern.MatchString(addr):
		return validateDNSAddress(addr, dnsPattern)
	default:
		return fmt.Errorf("unsupported address format: %s", addr)
	}
}

// validateIPv4Address validates IPv4 multiaddr format
func validateIPv4Address(addr string, pattern *regexp.Regexp) error {
	matches := pattern.FindStringSubmatch(addr)
	if len(matches) != 3 {
		return fmt.Errorf("invalid IPv4 format")
	}

	ip := net.ParseIP(matches[1])
	if ip == nil {
		return fmt.Errorf("invalid IPv4 address: %s", matches[1])
	}

	return validatePort(matches[2])
}

// validateIPv6Address validates IPv6 multiaddr format
func validateIPv6Address(addr string, pattern *regexp.Regexp) error {
	matches := pattern.FindStringSubmatch(addr)
	if len(matches) != 3 {
		return fmt.Errorf("invalid IPv6 format")
	}

	ip := net.ParseIP(matches[1])
	if ip == nil {
		return fmt.Errorf("invalid IPv6 address: %s", matches[1])
	}

	return validatePort(matches[2])
}

// validateDNSAddress validates DNS multiaddr format
func validateDNSAddress(addr string, pattern *regexp.Regexp) error {
	matches := pattern.FindStringSubmatch(addr)
	if len(matches) != 3 {
		return fmt.Errorf("invalid DNS format")
	}

	hostname := matches[1]
	if len(hostname) == 0 || len(hostname) > 253 {
		return fmt.Errorf("invalid hostname length: %s", hostname)
	}

	// Basic hostname validation
	if strings.Contains(hostname, "..") || strings.HasPrefix(hostname, ".") || strings.HasSuffix(hostname, ".") {
		return fmt.Errorf("invalid hostname format: %s", hostname)
	}

	return validatePort(matches[2])
}

// validatePort validates port number
func validatePort(portStr string) error {
	if portStr == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Convert and validate port range
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return fmt.Errorf("invalid port number: %s", portStr)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}

	return nil
}

// String returns a string representation of the peer
func (p *Peer) String() string {
	return fmt.Sprintf("Peer{PublicKey: %s..., Addresses: %v}", 
		p.PublicKey[:8], p.Addresses)
}

// Equal checks if two peers are equal (same public key)
func (p *Peer) Equal(other *Peer) bool {
	return p.PublicKey == other.PublicKey
}

// HasAddress checks if the peer has a specific address
func (p *Peer) HasAddress(address string) bool {
	for _, addr := range p.Addresses {
		if addr == address {
			return true
		}
	}
	return false
}