package network

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"

	"btc-federation/internal/types"
)

// HostWrapper wraps libp2p host with additional functionality
type HostWrapper struct {
	host   host.Host
	config *types.NetworkConfig
}

// NewHostWrapper creates a new host wrapper with the given configuration
func NewHostWrapper(config *types.NetworkConfig, privateKey crypto.PrivKey) (*HostWrapper, error) {
	var opts []libp2p.Option

	// Set private key if provided
	if privateKey != nil {
		opts = append(opts, libp2p.Identity(privateKey))
	}

	// Enable TCP transport
	opts = append(opts, libp2p.Transport(tcp.NewTCPTransport))

	// Use default security
	opts = append(opts, libp2p.DefaultSecurity)

	// Parse and set listen addresses
	var listenAddrs []multiaddr.Multiaddr
	for _, addrStr := range config.Addresses {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			return nil, err
		}
		listenAddrs = append(listenAddrs, addr)
	}

	if len(listenAddrs) > 0 {
		opts = append(opts, libp2p.ListenAddrs(listenAddrs...))
	}

	// Create libp2p host
	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}

	return &HostWrapper{
		host:   h,
		config: config,
	}, nil
}

// Host returns the underlying libp2p host
func (hw *HostWrapper) Host() host.Host {
	return hw.host
}

// Close closes the host
func (hw *HostWrapper) Close() error {
	return hw.host.Close()
}

// Connect connects to a peer at the given address
func (hw *HostWrapper) Connect(ctx context.Context, addr multiaddr.Multiaddr) error {
	addrInfo, err := parseMultiaddr(addr)
	if err != nil {
		return err
	}

	return hw.host.Connect(ctx, *addrInfo)
}

// parseMultiaddr parses a multiaddr and extracts peer info
func parseMultiaddr(addr multiaddr.Multiaddr) (*peer.AddrInfo, error) {
	// This is a simplified parser - in production we'd use proper libp2p utilities
	// For now, just return basic structure
	return &peer.AddrInfo{
		Addrs: []multiaddr.Multiaddr{addr},
	}, nil
}

// ConvertPrivateKeyFromBase64 converts a base64 Ed25519 private key (from KeyManager)
// to a libp2p crypto.PrivKey for use with libp2p
func ConvertPrivateKeyFromBase64(privateKeyBase64 string) (crypto.PrivKey, error) {
	if privateKeyBase64 == "" {
		return nil, fmt.Errorf("private key cannot be empty")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid private key base64: %w", err)
	}

	if len(keyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: expected %d, got %d", ed25519.PrivateKeySize, len(keyBytes))
	}

	ed25519Key := ed25519.PrivateKey(keyBytes)
	libp2pKey, err := crypto.UnmarshalEd25519PrivateKey(ed25519Key)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Ed25519 private key: %w", err)
	}

	return libp2pKey, nil
}
