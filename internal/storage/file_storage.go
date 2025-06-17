// BTC-FED-2-3 - Peer Storage & File Management
// Package storage implements file-based peer storage with atomic operations
package storage

import (
	"btc-federation/internal/types"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Constants for file operations
const (
	// DefaultPeersFile is the default filename for peer storage
	DefaultPeersFile = "peers.yaml"
	// TempFileSuffix is the suffix for temporary files during atomic operations
	TempFileSuffix = ".tmp"
	// BackupFileSuffix is the suffix for backup files
	BackupFileSuffix = ".backup"
	// FilePermissions defines the file permissions for peer files
	FilePermissions = 0644
	// MaxBackupFiles defines the maximum number of backup files to keep
	MaxBackupFiles = 3
)

// FileStorage implements PeerStorage interface with file-based storage
type FileStorage struct {
	mu       sync.RWMutex // Protects concurrent access to peers and file operations
	filePath string       // Path to the peers.yaml file
	peers    []types.Peer // In-memory cache of peers
	lastMod  time.Time    // Last modification time of the file
}

// NewFileStorage creates a new file-based peer storage
func NewFileStorage(filePath string) (*FileStorage, error) {
	if filePath == "" {
		filePath = DefaultPeersFile
	}

	storage := &FileStorage{
		filePath: filePath,
		peers:    make([]types.Peer, 0),
	}

	// Load initial peers if file exists
	if _, err := os.Stat(filePath); err == nil {
		if _, err := storage.LoadPeers(); err != nil {
			return nil, fmt.Errorf("failed to load initial peers: %w", err)
		}
	}

	return storage, nil
}

// GetPeers returns all currently loaded peers (thread-safe read)
func (fs *FileStorage) GetPeers() []types.Peer {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]types.Peer, len(fs.peers))
	copy(result, fs.peers)
	return result
}

// LoadPeers loads peers from the file and returns them
func (fs *FileStorage) LoadPeers() ([]types.Peer, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.loadPeersUnsafe()
}

// loadPeersUnsafe loads peers without locking (internal use only)
func (fs *FileStorage) loadPeersUnsafe() ([]types.Peer, error) {
	// Check if file exists
	fileInfo, err := os.Stat(fs.filePath)
	if os.IsNotExist(err) {
		// File doesn't exist, return empty list
		fs.peers = make([]types.Peer, 0)
		fs.lastMod = time.Time{}
		return fs.peers, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat peers file: %w", err)
	}

	// Check if file was modified since last load
	if !fs.lastMod.IsZero() && fileInfo.ModTime().Equal(fs.lastMod) {
		// File hasn't changed, return cached peers
		result := make([]types.Peer, len(fs.peers))
		copy(result, fs.peers)
		return result, nil
	}

	// Read and parse file
	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read peers file: %w", err)
	}

	var peerList types.PeerList
	if err := yaml.Unmarshal(data, &peerList); err != nil {
		// File is corrupted, try to create backup and return error
		if backupErr := fs.createBackup(); backupErr != nil {
			return nil, fmt.Errorf("failed to parse peers file and backup failed: %w, backup error: %v", err, backupErr)
		}
		return nil, fmt.Errorf("failed to parse peers file (backup created): %w", err)
	}

	// Validate and filter peers
	validPeers, err := fs.validateAndFilterPeers(peerList.Peers)
	if err != nil {
		return nil, fmt.Errorf("failed to validate peers: %w", err)
	}

	// Update internal state
	fs.peers = validPeers
	fs.lastMod = fileInfo.ModTime()

	// Return a copy
	result := make([]types.Peer, len(fs.peers))
	copy(result, fs.peers)
	return result, nil
}

// SavePeers saves the provided peers to storage with atomic operations
func (fs *FileStorage) SavePeers(peers []types.Peer) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Validate peers before saving
	validPeers, err := fs.validateAndFilterPeers(peers)
	if err != nil {
		return fmt.Errorf("failed to validate peers before saving: %w", err)
	}

	// Prepare data structure
	peerList := types.PeerList{
		Peers: validPeers,
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&peerList)
	if err != nil {
		return fmt.Errorf("failed to marshal peers to YAML: %w", err)
	}

	// Write atomically
	if err := fs.writeFileAtomic(data); err != nil {
		return fmt.Errorf("failed to write peers file: %w", err)
	}

	// Update internal state
	fs.peers = validPeers
	if fileInfo, err := os.Stat(fs.filePath); err == nil {
		fs.lastMod = fileInfo.ModTime()
	}

	return nil
}

// AddPeer adds a single peer to the storage
func (fs *FileStorage) AddPeer(peer types.Peer) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Validate the peer
	if err := peer.Validate(); err != nil {
		return fmt.Errorf("invalid peer: %w", err)
	}

	// Check for duplicates
	for _, existingPeer := range fs.peers {
		if existingPeer.Equal(&peer) {
			return fmt.Errorf("peer with public key %s already exists", peer.PublicKey)
		}
	}

	// Add peer to the list
	updatedPeers := append(fs.peers, peer)

	// Save updated list
	peerList := types.PeerList{
		Peers: updatedPeers,
	}

	data, err := yaml.Marshal(&peerList)
	if err != nil {
		return fmt.Errorf("failed to marshal peers to YAML: %w", err)
	}

	if err := fs.writeFileAtomic(data); err != nil {
		return fmt.Errorf("failed to write peers file: %w", err)
	}

	// Update internal state
	fs.peers = updatedPeers
	if fileInfo, err := os.Stat(fs.filePath); err == nil {
		fs.lastMod = fileInfo.ModTime()
	}

	return nil
}

// RemovePeer removes a peer by public key
func (fs *FileStorage) RemovePeer(publicKey string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Find and remove peer
	var updatedPeers []types.Peer
	found := false

	for _, peer := range fs.peers {
		if peer.PublicKey != publicKey {
			updatedPeers = append(updatedPeers, peer)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("peer with public key %s not found", publicKey)
	}

	// Save updated list
	peerList := types.PeerList{
		Peers: updatedPeers,
	}

	data, err := yaml.Marshal(&peerList)
	if err != nil {
		return fmt.Errorf("failed to marshal peers to YAML: %w", err)
	}

	if err := fs.writeFileAtomic(data); err != nil {
		return fmt.Errorf("failed to write peers file: %w", err)
	}

	// Update internal state
	fs.peers = updatedPeers
	if fileInfo, err := os.Stat(fs.filePath); err == nil {
		fs.lastMod = fileInfo.ModTime()
	}

	return nil
}

// Close closes the storage and releases any resources
func (fs *FileStorage) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Clear internal state
	fs.peers = nil
	fs.lastMod = time.Time{}

	return nil
}

// validateAndFilterPeers validates peers and removes duplicates
func (fs *FileStorage) validateAndFilterPeers(peers []types.Peer) ([]types.Peer, error) {
	var validPeers []types.Peer
	seenKeys := make(map[string]bool)

	for i, peer := range peers {
		// Validate peer
		if err := peer.Validate(); err != nil {
			return nil, fmt.Errorf("peer %d is invalid: %w", i, err)
		}

		// Check for duplicates
		if seenKeys[peer.PublicKey] {
			// Skip duplicate peer, but continue processing
			continue
		}

		seenKeys[peer.PublicKey] = true
		validPeers = append(validPeers, peer)
	}

	return validPeers, nil
}

// writeFileAtomic writes data to file atomically using temp file + rename
func (fs *FileStorage) writeFileAtomic(data []byte) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temporary file in the same directory
	tempFile := fs.filePath + TempFileSuffix
	file, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, FilePermissions)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Ensure temp file is cleaned up on error
	defer func() {
		if file != nil {
			file.Close()
			os.Remove(tempFile)
		}
	}()

	// Write data
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close file
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	file = nil // Mark as closed

	// Atomic rename
	if err := os.Rename(tempFile, fs.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// createBackup creates a backup of the current peers file
func (fs *FileStorage) createBackup() error {
	if _, err := os.Stat(fs.filePath); os.IsNotExist(err) {
		return nil // No file to backup
	}

	backupPath := fs.filePath + BackupFileSuffix
	return fs.copyFile(fs.filePath, backupPath)
}

// copyFile copies a file from src to dst
func (fs *FileStorage) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return destFile.Sync()
}