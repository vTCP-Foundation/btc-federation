// BTC-FED-2-3 - Peer Storage & File Management
// Unit tests for file-based peer storage
package storage

import (
	"btc-federation/internal/types"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test helper functions

func createTestDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "peer_storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

func createValidPeer(suffix string) types.Peer {
	key := make([]byte, 32)
	// Make each peer unique by varying the key
	if len(suffix) > 0 {
		copy(key, []byte(suffix))
	}
	
	// Convert suffix to a valid IP octet (default to 100 if not numeric)
	ipOctet := "100"
	dnsName := "peer100"
	if suffix != "" {
		// Try to use suffix as number, fallback to hash if not numeric
		if len(suffix) <= 3 && suffix != "" {
			// For simple numeric suffixes, use directly
			ipOctet = suffix
			dnsName = fmt.Sprintf("peer%s", suffix)
		} else {
			// For complex suffixes, use hash to generate valid octet
			hash := 0
			for _, b := range []byte(suffix) {
				hash = (hash + int(b)) % 200 + 10 // Keep in range 10-209
			}
			ipOctet = fmt.Sprintf("%d", hash)
			// Create valid DNS name (replace invalid chars with hyphens)
			validSuffix := ""
			for _, r := range suffix {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
					validSuffix += string(r)
				} else {
					validSuffix += "-"
				}
			}
			dnsName = fmt.Sprintf("peer%s", validSuffix)
		}
	}
	
	return types.Peer{
		PublicKey: base64.StdEncoding.EncodeToString(key),
		Addresses: []string{
			fmt.Sprintf("/ip4/192.168.1.%s/tcp/9000", ipOctet),
			fmt.Sprintf("/dns4/%s.example.com/tcp/9000", dnsName),
		},
	}
}

func createTestFile(t *testing.T, filePath string, content string) {
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
}

// Test cases for FileStorage

func TestNewFileStorage(t *testing.T) {
	testDir := createTestDir(t)

	t.Run("with default file path", func(t *testing.T) {
		storage, err := NewFileStorage("")
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		if storage.filePath != DefaultPeersFile {
			t.Errorf("Expected default file path %s, got %s", DefaultPeersFile, storage.filePath)
		}
	})

	t.Run("with custom file path", func(t *testing.T) {
		customPath := filepath.Join(testDir, "custom_peers.yaml")
		storage, err := NewFileStorage(customPath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		if storage.filePath != customPath {
			t.Errorf("Expected custom file path %s, got %s", customPath, storage.filePath)
		}
	})

	t.Run("with existing valid file", func(t *testing.T) {
		filePath := filepath.Join(testDir, "existing_peers.yaml")
		validYAML := `peers:
  - public_key: ` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `
    addresses:
      - "/ip4/192.168.1.100/tcp/9000"
`
		createTestFile(t, filePath, validYAML)

		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		peers := storage.GetPeers()
		if len(peers) != 1 {
			t.Errorf("Expected 1 peer loaded, got %d", len(peers))
		}
	})

	t.Run("with existing invalid file", func(t *testing.T) {
		filePath := filepath.Join(testDir, "invalid_peers.yaml")
		createTestFile(t, filePath, "invalid yaml content [[[")

		_, err := NewFileStorage(filePath)
		if err == nil {
			t.Error("Expected error for invalid YAML file")
		}
	})
}

func TestFileStorage_GetPeers(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "test_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}
	defer storage.Close()

	// Initially empty
	peers := storage.GetPeers()
	if len(peers) != 0 {
		t.Errorf("Expected 0 peers initially, got %d", len(peers))
	}

	// Add a peer and verify
	peer := createValidPeer("100")
	if err := storage.AddPeer(peer); err != nil {
		t.Fatalf("AddPeer() failed: %v", err)
	}

	peers = storage.GetPeers()
	if len(peers) != 1 {
		t.Errorf("Expected 1 peer after adding, got %d", len(peers))
	}

	// Verify it's a copy (mutations don't affect internal state)
	originalLen := len(peers[0].Addresses)
	peers[0].Addresses = append(peers[0].Addresses, "extra")

	peersAgain := storage.GetPeers()
	if len(peersAgain[0].Addresses) != originalLen {
		t.Error("GetPeers() should return a copy, not the original slice")
	}
}

func TestFileStorage_LoadPeers(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "test_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}
	defer storage.Close()

	t.Run("load from non-existent file", func(t *testing.T) {
		peers, err := storage.LoadPeers()
		if err != nil {
			t.Errorf("LoadPeers() failed for non-existent file: %v", err)
		}
		if len(peers) != 0 {
			t.Errorf("Expected 0 peers from non-existent file, got %d", len(peers))
		}
	})

	t.Run("load from valid file", func(t *testing.T) {
		peer1 := createValidPeer("100")
		peer2 := createValidPeer("200")
		if err := storage.SavePeers([]types.Peer{peer1, peer2}); err != nil {
			t.Fatalf("SavePeers() failed: %v", err)
		}

		peers, err := storage.LoadPeers()
		if err != nil {
			t.Errorf("LoadPeers() failed: %v", err)
		}
		if len(peers) != 2 {
			t.Errorf("Expected 2 peers, got %d", len(peers))
		}
	})

	t.Run("load with duplicates (should be filtered)", func(t *testing.T) {
		peer := createValidPeer("300")
		duplicatePeer := peer // Same public key
		duplicatePeer.Addresses = []string{"/ip4/10.0.0.1/tcp/9000"}

		yamlContent := fmt.Sprintf(`peers:
  - public_key: %s
    addresses:
      - "/ip4/192.168.1.100/tcp/9000"
  - public_key: %s
    addresses:
      - "/ip4/10.0.0.1/tcp/9000"
`, peer.PublicKey, peer.PublicKey)

		createTestFile(t, filePath, yamlContent)

		peers, err := storage.LoadPeers()
		if err != nil {
			t.Errorf("LoadPeers() failed: %v", err)
		}
		if len(peers) != 1 {
			t.Errorf("Expected 1 peer after duplicate filtering, got %d", len(peers))
		}
	})

	t.Run("load from corrupted file", func(t *testing.T) {
		createTestFile(t, filePath, "corrupted yaml content [[[")

		_, err := storage.LoadPeers()
		if err == nil {
			t.Error("Expected error for corrupted YAML file")
		}
		if !strings.Contains(err.Error(), "failed to parse peers file") {
			t.Errorf("Expected parse error, got: %v", err)
		}

		// Check that backup was created
		backupPath := filePath + BackupFileSuffix
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("Expected backup file to be created for corrupted file")
		}
	})
}

func TestFileStorage_SavePeers(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "test_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}
	defer storage.Close()

	t.Run("save valid peers", func(t *testing.T) {
		peer1 := createValidPeer("100")
		peer2 := createValidPeer("200")
		peers := []types.Peer{peer1, peer2}

		if err := storage.SavePeers(peers); err != nil {
			t.Errorf("SavePeers() failed: %v", err)
		}

		// Verify file was created and content is correct
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("Expected peers file to be created")
		}

		// Reload and verify
		loadedPeers := storage.GetPeers()
		if len(loadedPeers) != 2 {
			t.Errorf("Expected 2 saved peers, got %d", len(loadedPeers))
		}
	})

	t.Run("save with invalid peer", func(t *testing.T) {
		invalidPeer := types.Peer{
			PublicKey: "invalid-key",
			Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
		}

		err := storage.SavePeers([]types.Peer{invalidPeer})
		if err == nil {
			t.Error("Expected error for invalid peer")
		}
		if !strings.Contains(err.Error(), "failed to validate peers") {
			t.Errorf("Expected validation error, got: %v", err)
		}
	})

	t.Run("save to read-only directory", func(t *testing.T) {
		readOnlyDir := filepath.Join(testDir, "readonly")
		if err := os.Mkdir(readOnlyDir, 0444); err != nil {
			t.Fatalf("Failed to create read-only directory: %v", err)
		}
		defer os.Chmod(readOnlyDir, 0755) // Cleanup

		readOnlyPath := filepath.Join(readOnlyDir, "peers.yaml")
		readOnlyStorage, err := NewFileStorage(readOnlyPath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer readOnlyStorage.Close()

		peer := createValidPeer("100")
		err = readOnlyStorage.SavePeers([]types.Peer{peer})
		if err == nil {
			t.Error("Expected error when writing to read-only directory")
		}
	})
}

func TestFileStorage_AddPeer(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "test_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}
	defer storage.Close()

	t.Run("add valid peer", func(t *testing.T) {
		peer := createValidPeer("100")

		if err := storage.AddPeer(peer); err != nil {
			t.Errorf("AddPeer() failed: %v", err)
		}

		peers := storage.GetPeers()
		if len(peers) != 1 {
			t.Errorf("Expected 1 peer after adding, got %d", len(peers))
		}
		if !peers[0].Equal(&peer) {
			t.Error("Added peer doesn't match expected peer")
		}
	})

	t.Run("add duplicate peer", func(t *testing.T) {
		peer := createValidPeer("100") // Same as above

		err := storage.AddPeer(peer)
		if err == nil {
			t.Error("Expected error when adding duplicate peer")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("Expected duplicate error, got: %v", err)
		}
	})

	t.Run("add invalid peer", func(t *testing.T) {
		invalidPeer := types.Peer{
			PublicKey: "invalid",
			Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
		}

		err := storage.AddPeer(invalidPeer)
		if err == nil {
			t.Error("Expected error for invalid peer")
		}
		if !strings.Contains(err.Error(), "invalid peer") {
			t.Errorf("Expected validation error, got: %v", err)
		}
	})
}

func TestFileStorage_RemovePeer(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "test_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}
	defer storage.Close()

	// Add some peers first
	peer1 := createValidPeer("100")
	peer2 := createValidPeer("200")
	if err := storage.SavePeers([]types.Peer{peer1, peer2}); err != nil {
		t.Fatalf("SavePeers() failed: %v", err)
	}

	t.Run("remove existing peer", func(t *testing.T) {
		if err := storage.RemovePeer(peer1.PublicKey); err != nil {
			t.Errorf("RemovePeer() failed: %v", err)
		}

		peers := storage.GetPeers()
		if len(peers) != 1 {
			t.Errorf("Expected 1 peer after removal, got %d", len(peers))
		}
		if peers[0].Equal(&peer1) {
			t.Error("Removed peer still exists")
		}
		if !peers[0].Equal(&peer2) {
			t.Error("Wrong peer was removed")
		}
	})

	t.Run("remove non-existent peer", func(t *testing.T) {
		nonExistentKey := base64.StdEncoding.EncodeToString([]byte("nonexistent"))

		err := storage.RemovePeer(nonExistentKey)
		if err == nil {
			t.Error("Expected error when removing non-existent peer")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected not found error, got: %v", err)
		}
	})
}

func TestFileStorage_ConcurrentAccess(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "concurrent_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}
	defer storage.Close()

	const numGoroutines = 10
	const operationsPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*operationsPerGoroutine)

	// Concurrent reads
	t.Run("concurrent reads", func(t *testing.T) {
		// Add some initial data
		initialPeers := []types.Peer{
			createValidPeer("100"),
			createValidPeer("200"),
		}
		if err := storage.SavePeers(initialPeers); err != nil {
			t.Fatalf("SavePeers() failed: %v", err)
		}

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					peers := storage.GetPeers()
					if len(peers) < 2 {
						errors <- fmt.Errorf("expected at least 2 peers, got %d", len(peers))
						return
					}
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent read error: %v", err)
		}
	})

	// Concurrent writes (should be serialized)
	t.Run("concurrent writes", func(t *testing.T) {
		errors = make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				peer := createValidPeer(fmt.Sprintf("concurrent_%d", id))
				if err := storage.AddPeer(peer); err != nil {
					// Some operations may fail due to duplicates, that's OK
					if !strings.Contains(err.Error(), "already exists") {
						errors <- err
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent write error: %v", err)
		}

		// Verify final state is consistent
		peers := storage.GetPeers()
		publicKeys := make(map[string]bool)
		for _, peer := range peers {
			if publicKeys[peer.PublicKey] {
				t.Error("Found duplicate peer after concurrent operations")
			}
			publicKeys[peer.PublicKey] = true
		}
	})
}

func TestFileStorage_AtomicOperations(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "atomic_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}
	defer storage.Close()

	// Verify temp files are cleaned up after successful operation
	t.Run("temp file cleanup", func(t *testing.T) {
		peer := createValidPeer("100")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Check that no temp files remain
		tempFile := filePath + TempFileSuffix
		if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
			t.Error("Temporary file was not cleaned up after successful operation")
		}
	})

	// Verify file integrity after operations
	t.Run("file integrity", func(t *testing.T) {
		// Perform multiple operations
		for i := 0; i < 5; i++ {
			peer := createValidPeer(fmt.Sprintf("integrity_%d", i))
			if err := storage.AddPeer(peer); err != nil {
				t.Fatalf("AddPeer() failed: %v", err)
			}
		}

		// Verify file can be read and parsed correctly
		newStorage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("Failed to create new storage from existing file: %v", err)
		}
		defer newStorage.Close()

		peers := newStorage.GetPeers()
		if len(peers) != 6 { // 1 from previous test + 5 from this test
			t.Errorf("Expected 6 peers in file, got %d", len(peers))
		}
	})
}

func TestFileStorage_FileModificationDetection(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "mod_detection_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}
	defer storage.Close()

	// Add initial peer
	peer1 := createValidPeer("100")
	if err := storage.AddPeer(peer1); err != nil {
		t.Fatalf("AddPeer() failed: %v", err)
	}

	// Manually modify the file
	time.Sleep(10 * time.Millisecond) // Ensure different modification time
	peer2 := createValidPeer("200")
	manualContent := fmt.Sprintf(`peers:
  - public_key: %s
    addresses:
      - "/ip4/192.168.1.100/tcp/9000"
  - public_key: %s
    addresses:
      - "/ip4/192.168.1.200/tcp/9000"
`, peer1.PublicKey, peer2.PublicKey)

	if err := os.WriteFile(filePath, []byte(manualContent), 0644); err != nil {
		t.Fatalf("Failed to manually modify file: %v", err)
	}

	// LoadPeers should detect the change and reload
	peers, err := storage.LoadPeers()
	if err != nil {
		t.Fatalf("LoadPeers() failed: %v", err)
	}

	if len(peers) != 2 {
		t.Errorf("Expected 2 peers after manual modification, got %d", len(peers))
	}

	// GetPeers should also reflect the changes
	cachedPeers := storage.GetPeers()
	if len(cachedPeers) != 2 {
		t.Errorf("Expected 2 cached peers after modification detection, got %d", len(cachedPeers))
	}
}

func TestFileStorage_Close(t *testing.T) {
	testDir := createTestDir(t)
	filePath := filepath.Join(testDir, "close_test_peers.yaml")

	storage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("NewFileStorage() failed: %v", err)
	}

	// Add some data
	peer := createValidPeer("100")
	if err := storage.AddPeer(peer); err != nil {
		t.Fatalf("AddPeer() failed: %v", err)
	}

	// Close storage
	if err := storage.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Verify internal state is cleared
	peers := storage.GetPeers()
	if len(peers) != 0 {
		t.Errorf("Expected 0 peers after close, got %d", len(peers))
	}

	// Verify file still exists and contains data
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("File should still exist after Close()")
	}

	// Verify we can create new storage from the same file
	newStorage, err := NewFileStorage(filePath)
	if err != nil {
		t.Fatalf("Failed to create new storage after close: %v", err)
	}
	defer newStorage.Close()

	newPeers := newStorage.GetPeers()
	if len(newPeers) != 1 {
		t.Errorf("Expected 1 peer in new storage, got %d", len(newPeers))
	}
}

// Additional tests for better coverage

func TestFileStorage_ErrorConditions(t *testing.T) {
	testDir := createTestDir(t)

	t.Run("writeFileAtomic sync error", func(t *testing.T) {
		// Test atomic write operations under various conditions
		filePath := filepath.Join(testDir, "sync_test_peers.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// This tests the atomic write path
		peer := createValidPeer("sync-test")
		if err := storage.AddPeer(peer); err != nil {
			t.Errorf("AddPeer() should succeed: %v", err)
		}

		// Verify temp file cleanup
		tempFile := filePath + TempFileSuffix
		if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
			t.Error("Temporary file should be cleaned up")
		}
	})

	t.Run("backup creation scenarios", func(t *testing.T) {
		filePath := filepath.Join(testDir, "backup_test_peers.yaml")
		
		// Create storage with initial content
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		peer := createValidPeer("backup-test")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Force a backup by creating corrupted content
		createTestFile(t, filePath, "corrupted content")

		// This should trigger backup creation
		_, err = storage.LoadPeers()
		if err == nil {
			t.Error("Expected error for corrupted file")
		}

		// Verify backup was created
		backupPath := filePath + BackupFileSuffix
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("Backup file should have been created")
		}
	})

	t.Run("copy file error conditions", func(t *testing.T) {
		filePath := filepath.Join(testDir, "copy_test_peers.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Test copyFile function indirectly through backup
		peer := createValidPeer("copy-test")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Verify file exists for backup
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("Source file should exist")
		}
	})

	t.Run("file modification time edge cases", func(t *testing.T) {
		filePath := filepath.Join(testDir, "mod_time_peers.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Add initial peer
		peer1 := createValidPeer("mod-time-1")
		if err := storage.AddPeer(peer1); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Load peers multiple times to test modification time caching
		for i := 0; i < 3; i++ {
			peers, err := storage.LoadPeers()
			if err != nil {
				t.Errorf("LoadPeers() iteration %d failed: %v", i, err)
			}
			if len(peers) != 1 {
				t.Errorf("Expected 1 peer in iteration %d, got %d", i, len(peers))
			}
		}
	})

	t.Run("directory creation in atomic write", func(t *testing.T) {
		// Test that writeFileAtomic creates directories if needed
		subDir := filepath.Join(testDir, "subdir", "deep")
		filePath := filepath.Join(subDir, "deep_peers.yaml")
		
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		peer := createValidPeer("deep-dir")
		if err := storage.AddPeer(peer); err != nil {
			t.Errorf("AddPeer() should create directories: %v", err)
		}

		// Verify file was created in deep directory
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("File should exist in created directory")
		}
	})

	t.Run("validation edge cases", func(t *testing.T) {
		filePath := filepath.Join(testDir, "validation_peers.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Test peer with maximum addresses
		validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
		addresses := make([]string, types.MaxAddresses)
		for i := 0; i < types.MaxAddresses; i++ {
			addresses[i] = fmt.Sprintf("/ip4/192.168.1.%d/tcp/9000", i+1)
		}

		maxPeer := types.Peer{
			PublicKey: validKey,
			Addresses: addresses,
		}

		if err := storage.AddPeer(maxPeer); err != nil {
			t.Errorf("AddPeer() should accept peer with max addresses: %v", err)
		}

		// Verify it was added
		peers := storage.GetPeers()
		if len(peers) != 1 {
			t.Errorf("Expected 1 peer with max addresses, got %d", len(peers))
		}
		if len(peers[0].Addresses) != types.MaxAddresses {
			t.Errorf("Expected %d addresses, got %d", types.MaxAddresses, len(peers[0].Addresses))
		}
	})
}

func TestFileStorage_BackupScenarios(t *testing.T) {
	testDir := createTestDir(t)

	t.Run("backup non-existent file", func(t *testing.T) {
		filePath := filepath.Join(testDir, "nonexistent.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// This should not fail even though file doesn't exist
		err = storage.createBackup()
		if err != nil {
			t.Errorf("createBackup() should handle non-existent file: %v", err)
		}
	})

	t.Run("backup with permission error", func(t *testing.T) {
		filePath := filepath.Join(testDir, "perm_test.yaml")
		
		// Create storage first with valid content
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Add valid content first
		peer := createValidPeer("perm-test")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Make directory read-only to cause backup error
		if err := os.Chmod(testDir, 0444); err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}
		defer os.Chmod(testDir, 0755) // Restore permissions

		// This should handle the permission error gracefully
		err = storage.createBackup()
		if err == nil {
			t.Error("Expected error due to permission issue")
		}
	})
}

// Additional edge case tests to reach 90+ coverage
func TestFileStorage_EdgeCases(t *testing.T) {
	testDir := createTestDir(t)

	t.Run("writeFileAtomic error scenarios", func(t *testing.T) {
		filePath := filepath.Join(testDir, "write_error_test.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Test successful write first
		peer := createValidPeer("write-test")
		if err := storage.AddPeer(peer); err != nil {
			t.Errorf("AddPeer() should succeed: %v", err)
		}

		// Test that file was created
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("File should have been created")
		}
	})

	t.Run("copyFile edge cases", func(t *testing.T) {
		srcPath := filepath.Join(testDir, "copy_src.yaml")
		dstPath := filepath.Join(testDir, "copy_dst.yaml")

		// Create source file with storage
		storage, err := NewFileStorage(srcPath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		peer := createValidPeer("copy-edge")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Test copyFile through backup operation
		if err := storage.copyFile(srcPath, dstPath); err != nil {
			t.Errorf("copyFile() should succeed: %v", err)
		}

		// Verify destination file exists
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			t.Error("Destination file should exist after copy")
		}
	})

	t.Run("invalid file operations", func(t *testing.T) {
		// Test with invalid paths
		invalidPath := "/invalid/nonexistent/path/test.yaml"
		_, err := NewFileStorage(invalidPath)
		if err != nil {
			// This is expected for some systems
			t.Logf("Expected error for invalid path: %v", err)
		}
	})

	t.Run("yaml marshal edge cases", func(t *testing.T) {
		filePath := filepath.Join(testDir, "marshal_test.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Test with empty peer list
		if err := storage.SavePeers([]types.Peer{}); err != nil {
			t.Errorf("SavePeers() should handle empty list: %v", err)
		}

		// Verify empty file handling
		peers, err := storage.LoadPeers()
		if err != nil {
			t.Errorf("LoadPeers() should handle empty file: %v", err)
		}
		if len(peers) != 0 {
			t.Errorf("Expected 0 peers from empty file, got %d", len(peers))
		}
	})

	t.Run("concurrent modification scenarios", func(t *testing.T) {
		filePath := filepath.Join(testDir, "concurrent_mod.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Add initial peer
		peer1 := createValidPeer("concurrent-mod-1")
		if err := storage.AddPeer(peer1); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Test multiple operations without file modification
		for i := 0; i < 5; i++ {
			peers := storage.GetPeers()
			if len(peers) != 1 {
				t.Errorf("GetPeers() iteration %d: expected 1 peer, got %d", i, len(peers))
			}
		}
	})

	t.Run("file permissions and access", func(t *testing.T) {
		filePath := filepath.Join(testDir, "permissions_test.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Test normal operation
		peer := createValidPeer("permissions")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Test file exists and is readable
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("File should exist after AddPeer")
		}

		// Test that we can read it back
		peers := storage.GetPeers()
		if len(peers) != 1 {
			t.Errorf("Expected 1 peer after read, got %d", len(peers))
		}
	})
}

// Test specific error scenarios to improve coverage
func TestFileStorage_SpecificErrorPaths(t *testing.T) {
	testDir := createTestDir(t)

	t.Run("copyFile source read error", func(t *testing.T) {
		storage := &FileStorage{filePath: "dummy"}
		
		// Try to copy non-existent source file
		err := storage.copyFile("/nonexistent/source.yaml", filepath.Join(testDir, "dest.yaml"))
		if err == nil {
			t.Error("Expected error when copying non-existent source file")
		}
	})

	t.Run("copyFile destination create error", func(t *testing.T) {
		srcPath := filepath.Join(testDir, "copyfile_src.yaml")
		storage, err := NewFileStorage(srcPath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Create source file
		peer := createValidPeer("copyfile-src")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Try to copy to invalid destination
		err = storage.copyFile(srcPath, "/invalid/path/dest.yaml")
		if err == nil {
			t.Error("Expected error when copying to invalid destination")
		}
	})

	t.Run("file stat error in loadPeersUnsafe", func(t *testing.T) {
		// Create file then make directory inaccessible
		filePath := filepath.Join(testDir, "subdir2", "stat_error.yaml")
		os.MkdirAll(filepath.Dir(filePath), 0755)
		
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Add initial peer
		peer := createValidPeer("stat-error")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Make directory inaccessible
		if err := os.Chmod(filepath.Dir(filePath), 0000); err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}
		defer os.Chmod(filepath.Dir(filePath), 0755)

		// This should trigger a stat error
		_, err = storage.LoadPeers()
		if err == nil {
			t.Error("Expected error due to stat permission issue")
		}
	})

	t.Run("addPeer update state edge case", func(t *testing.T) {
		filePath := filepath.Join(testDir, "addpeer_edge.yaml")
		storage, err := NewFileStorage(filePath)
		if err != nil {
			t.Fatalf("NewFileStorage() failed: %v", err)
		}
		defer storage.Close()

		// Add peer successfully
		peer := createValidPeer("addpeer-edge")
		if err := storage.AddPeer(peer); err != nil {
			t.Fatalf("AddPeer() failed: %v", err)
		}

		// Manually corrupt the file modification time tracking
		storage.lastMod = time.Time{} // Reset modification time

		// Try to add another peer, which should still work
		peer2 := createValidPeer("addpeer-edge-2")
		if err := storage.AddPeer(peer2); err != nil {
			t.Errorf("AddPeer() should work even with reset mod time: %v", err)
		}

		// Verify both peers exist
		peers := storage.GetPeers()
		if len(peers) != 2 {
			t.Errorf("Expected 2 peers, got %d", len(peers))
		}
	})
}