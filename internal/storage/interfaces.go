// BTC-FED-2-3 - Peer Storage & File Management
// Package storage contains interfaces and implementations for peer storage
package storage

import (
	"btc-federation/internal/types"
)

// PeerStorage defines the interface for peer storage operations
type PeerStorage interface {
	// GetPeers returns all currently loaded peers
	GetPeers() []types.Peer

	// LoadPeers loads peers from the storage and returns them
	// This method also refreshes the internal peer list
	LoadPeers() ([]types.Peer, error)

	// SavePeers saves the provided peers to storage (for completeness)
	SavePeers(peers []types.Peer) error

	// AddPeer adds a single peer to the storage
	AddPeer(peer types.Peer) error

	// RemovePeer removes a peer by public key
	RemovePeer(publicKey string) error

	// Close closes the storage and releases any resources
	Close() error
}