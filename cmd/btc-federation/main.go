// BTC-FED-2-3 - Peer Storage & File Management
// Main demonstration program for peer storage functionality
package main

import (
	"btc-federation/internal/storage"
	"btc-federation/internal/types"
	"encoding/base64"
	"fmt"
	"log"
	"os"
)

func main() {
	fmt.Println("BTC Federation - Peer Storage Demo")
	fmt.Println("==================================")

	// Create storage instance
	peerStorage, err := storage.NewFileStorage("demo_peers.yaml")
	if err != nil {
		log.Fatalf("Failed to create peer storage: %v", err)
	}
	defer peerStorage.Close()

	// Demonstrate GetPeers() - initially empty
	fmt.Println("\n1. Initial peers (should be empty):")
	peers := peerStorage.GetPeers()
	fmt.Printf("   Found %d peers\n", len(peers))

	// Create and add some demo peers
	fmt.Println("\n2. Adding demo peers...")
	
	// Create demo peer 1
	key1 := make([]byte, 32)
	copy(key1, []byte("demo-peer-1-key"))
	peer1 := types.Peer{
		PublicKey: base64.StdEncoding.EncodeToString(key1),
		Addresses: []string{
			"/ip4/192.168.1.100/tcp/9000",
			"/dns4/peer1.example.com/tcp/9000",
		},
	}

	if err := peerStorage.AddPeer(peer1); err != nil {
		log.Printf("   Error adding peer1: %v", err)
	} else {
		fmt.Println("   ✓ Added peer1")
	}

	// Create demo peer 2
	key2 := make([]byte, 32)
	copy(key2, []byte("demo-peer-2-key"))
	peer2 := types.Peer{
		PublicKey: base64.StdEncoding.EncodeToString(key2),
		Addresses: []string{
			"/ip4/192.168.1.200/tcp/9000",
			"/ip6/2001:db8::1/tcp/9000",
		},
	}

	if err := peerStorage.AddPeer(peer2); err != nil {
		log.Printf("   Error adding peer2: %v", err)
	} else {
		fmt.Println("   ✓ Added peer2")
	}

	// Demonstrate GetPeers() after adding
	fmt.Println("\n3. Peers after adding (using GetPeers()):")
	peers = peerStorage.GetPeers()
	for i, peer := range peers {
		fmt.Printf("   Peer %d:\n", i+1)
		fmt.Printf("     Public Key: %s...\n", peer.PublicKey[:16])
		fmt.Printf("     Addresses: %v\n", peer.Addresses)
	}

	// Demonstrate LoadPeers() 
	fmt.Println("\n4. Reloading peers from file (using LoadPeers()):")
	loadedPeers, err := peerStorage.LoadPeers()
	if err != nil {
		log.Printf("   Error loading peers: %v", err)
	} else {
		fmt.Printf("   Loaded %d peers from file\n", len(loadedPeers))
		for i, peer := range loadedPeers {
			fmt.Printf("   Peer %d: %s...\n", i+1, peer.PublicKey[:16])
		}
	}

	// Demonstrate duplicate handling
	fmt.Println("\n5. Testing duplicate peer handling:")
	duplicatePeer := peer1 // Same public key as peer1
	if err := peerStorage.AddPeer(duplicatePeer); err != nil {
		fmt.Printf("   ✓ Correctly rejected duplicate: %v\n", err)
	}

	// Demonstrate validation
	fmt.Println("\n6. Testing validation:")
	invalidPeer := types.Peer{
		PublicKey: "invalid-key",
		Addresses: []string{"/ip4/192.168.1.100/tcp/9000"},
	}
	if err := peerStorage.AddPeer(invalidPeer); err != nil {
		fmt.Printf("   ✓ Correctly rejected invalid peer: %v\n", err)
	}

	// Show final state
	fmt.Println("\n7. Final peer count:")
	finalPeers := peerStorage.GetPeers()
	fmt.Printf("   Total peers: %d\n", len(finalPeers))

	// Show file contents
	fmt.Println("\n8. Generated peers.yaml file:")
	if data, err := os.ReadFile("demo_peers.yaml"); err == nil {
		fmt.Printf("%s\n", string(data))
	} else {
		fmt.Printf("   Error reading file: %v\n", err)
	}

	fmt.Println("\nDemo completed successfully!")
	fmt.Println("File 'demo_peers.yaml' contains the persisted peer data.")
}