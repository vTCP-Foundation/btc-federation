// BTC-FED-2-3 - Peer Storage & File Management
// Unit tests for peer types and validation
package types

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestPeer_Validate(t *testing.T) {
	validPublicKey := base64.StdEncoding.EncodeToString(make([]byte, 32))

	tests := []struct {
		name    string
		peer    Peer
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid peer with IPv4",
			peer: Peer{
				PublicKey: validPublicKey,
				Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
			},
			wantErr: false,
		},
		{
			name: "valid peer with IPv6",
			peer: Peer{
				PublicKey: validPublicKey,
				Addresses: []string{"/ip6/2001:db8::1/tcp/9000"},
			},
			wantErr: false,
		},
		{
			name: "valid peer with DNS",
			peer: Peer{
				PublicKey: validPublicKey,
				Addresses: []string{"/dns4/example.com/tcp/9000"},
			},
			wantErr: false,
		},
		{
			name: "valid peer with multiple addresses",
			peer: Peer{
				PublicKey: validPublicKey,
				Addresses: []string{
					"/ip4/192.168.1.100/tcp/9000",
					"/dns4/example.com/tcp/9000",
				},
			},
			wantErr: false,
		},
		{
			name: "empty public key",
			peer: Peer{
				PublicKey: "",
				Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
			},
			wantErr: true,
			errMsg:  "public key cannot be empty",
		},
		{
			name: "invalid base64 public key",
			peer: Peer{
				PublicKey: "invalid-base64!",
				Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
			},
			wantErr: true,
			errMsg:  "public key must be valid base64",
		},
		{
			name: "wrong length public key",
			peer: Peer{
				PublicKey: base64.StdEncoding.EncodeToString(make([]byte, 16)),
				Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
			},
			wantErr: true,
			errMsg:  "public key must be 32 bytes when decoded",
		},
		{
			name: "no addresses",
			peer: Peer{
				PublicKey: validPublicKey,
				Addresses: []string{},
			},
			wantErr: true,
			errMsg:  "peer must have at least 1 address",
		},
		{
			name: "too many addresses",
			peer: Peer{
				PublicKey: validPublicKey,
				Addresses: func() []string {
					addrs := make([]string, MaxAddresses+1)
					for i := range addrs {
						addrs[i] = "/ip4/192.168.1.100/tcp/9000"
					}
					return addrs
				}(),
			},
			wantErr: true,
			errMsg:  "peer cannot have more than 10 addresses",
		},
		{
			name: "invalid address format",
			peer: Peer{
				PublicKey: validPublicKey,
				Addresses: []string{"invalid-address"},
			},
			wantErr: true,
			errMsg:  "unsupported address format",
		},
		{
			name: "empty address",
			peer: Peer{
				PublicKey: validPublicKey,
				Addresses: []string{""},
			},
			wantErr: true,
			errMsg:  "address cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.peer.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Peer.Validate() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Peer.Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Peer.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
		errMsg  string
	}{
		// IPv4 tests
		{
			name:    "valid IPv4",
			addr:    "/ip4/192.168.1.100/tcp/9000",
			wantErr: false,
		},
		{
			name:    "valid IPv4 with port 1",
			addr:    "/ip4/192.168.1.100/tcp/1",
			wantErr: false,
		},
		{
			name:    "valid IPv4 with port 65535",
			addr:    "/ip4/192.168.1.100/tcp/65535",
			wantErr: false,
		},
		{
			name:    "invalid IPv4 address",
			addr:    "/ip4/999.999.999.999/tcp/9000",
			wantErr: true,
			errMsg:  "invalid IPv4 address",
		},
		{
			name:    "IPv4 with port 0",
			addr:    "/ip4/192.168.1.100/tcp/0",
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name:    "IPv4 with port too high",
			addr:    "/ip4/192.168.1.100/tcp/65536",
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},

		// IPv6 tests
		{
			name:    "valid IPv6",
			addr:    "/ip6/2001:db8::1/tcp/9000",
			wantErr: false,
		},
		{
			name:    "valid IPv6 full",
			addr:    "/ip6/2001:0db8:85a3:0000:0000:8a2e:0370:7334/tcp/9000",
			wantErr: false,
		},
		{
			name:    "valid IPv6 localhost",
			addr:    "/ip6/::1/tcp/9000",
			wantErr: false,
		},
		{
			name:    "invalid IPv6",
			addr:    "/ip6/invalid-ipv6/tcp/9000",
			wantErr: true,
			errMsg:  "invalid IPv6 address",
		},

		// DNS tests
		{
			name:    "valid DNS4",
			addr:    "/dns4/example.com/tcp/9000",
			wantErr: false,
		},
		{
			name:    "valid DNS",
			addr:    "/dns/example.com/tcp/9000",
			wantErr: false,
		},
		{
			name:    "valid DNS with subdomain",
			addr:    "/dns4/sub.example.com/tcp/9000",
			wantErr: false,
		},
		{
			name:    "DNS with invalid hostname (double dots)",
			addr:    "/dns4/example..com/tcp/9000",
			wantErr: true,
			errMsg:  "invalid hostname format",
		},
		{
			name:    "DNS with invalid hostname (starts with dot)",
			addr:    "/dns4/.example.com/tcp/9000",
			wantErr: true,
			errMsg:  "invalid hostname format",
		},
		{
			name:    "DNS with invalid hostname (ends with dot)",
			addr:    "/dns4/example.com./tcp/9000",
			wantErr: true,
			errMsg:  "invalid hostname format",
		},

		// General format tests
		{
			name:    "unsupported protocol",
			addr:    "/udp/192.168.1.100/port/9000",
			wantErr: true,
			errMsg:  "unsupported address format",
		},
		{
			name:    "missing tcp",
			addr:    "/ip4/192.168.1.100/9000",
			wantErr: true,
			errMsg:  "unsupported address format",
		},
		{
			name:    "wrong order",
			addr:    "/tcp/9000/ip4/192.168.1.100",
			wantErr: true,
			errMsg:  "unsupported address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAddress(tt.addr)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateAddress() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateAddress() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateAddress() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestPeer_Equal(t *testing.T) {
	publicKey1 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	publicKey2 := base64.StdEncoding.EncodeToString(append(make([]byte, 31), 1))

	peer1 := Peer{
		PublicKey: publicKey1,
		Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
	}

	peer2 := Peer{
		PublicKey: publicKey1,
		Addresses: []string{"/ip4/192.168.1.200/tcp/9000"}, // Different address
	}

	peer3 := Peer{
		PublicKey: publicKey2,
		Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
	}

	if !peer1.Equal(&peer2) {
		t.Error("Peers with same public key should be equal")
	}

	if peer1.Equal(&peer3) {
		t.Error("Peers with different public keys should not be equal")
	}
}

func TestPeer_HasAddress(t *testing.T) {
	peer := Peer{
		PublicKey: base64.StdEncoding.EncodeToString(make([]byte, 32)),
		Addresses: []string{
			"/ip4/192.168.1.100/tcp/9000",
			"/dns4/example.com/tcp/9000",
		},
	}

	if !peer.HasAddress("/ip4/192.168.1.100/tcp/9000") {
		t.Error("Peer should have the IPv4 address")
	}

	if !peer.HasAddress("/dns4/example.com/tcp/9000") {
		t.Error("Peer should have the DNS address")
	}

	if peer.HasAddress("/ip4/192.168.1.200/tcp/9000") {
		t.Error("Peer should not have the non-existent address")
	}
}

func TestPeer_String(t *testing.T) {
	peer := Peer{
		PublicKey: base64.StdEncoding.EncodeToString(make([]byte, 32)),
		Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
	}

	str := peer.String()
	if !strings.Contains(str, "Peer{") {
		t.Error("String representation should contain 'Peer{'")
	}
	if !strings.Contains(str, peer.PublicKey[:8]) {
		t.Error("String representation should contain truncated public key")
	}
}