// Package main demonstrates the mock infrastructure components for consensus testing.
// This demo shows how to use MockNetwork, MockStorage, and MockCrypto together
// in a multi-node test environment with failure injection capabilities.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/mocks"
	"btc-federation/pkg/consensus/types"
)

func main() {
	fmt.Println("🚀 Mock Infrastructure Demo")
	fmt.Println("============================")

	// Run different demo scenarios
	if err := runBasicNetworkDemo(); err != nil {
		log.Fatalf("Basic network demo failed: %v", err)
	}

	if err := runStorageDemo(); err != nil {
		log.Fatalf("Storage demo failed: %v", err)
	}

	if err := runCryptoDemo(); err != nil {
		log.Fatalf("Crypto demo failed: %v", err)
	}

	if err := runIntegratedScenarioDemo(); err != nil {
		log.Fatalf("Integrated scenario demo failed: %v", err)
	}

	if err := runFailureInjectionDemo(); err != nil {
		log.Fatalf("Failure injection demo failed: %v", err)
	}

	fmt.Println("\n✅ All demos completed successfully!")
}

// runBasicNetworkDemo demonstrates basic message passing between nodes.
func runBasicNetworkDemo() error {
	fmt.Println("\n📡 Demo 1: Basic Network Message Passing")
	fmt.Println("----------------------------------------")

	// Create network configuration
	config := mocks.DefaultNetworkConfig()
	config.BaseDelay = 10 * time.Millisecond
	failures := mocks.DefaultNetworkFailureConfig()

	// Create 3 mock networks
	network1 := mocks.NewMockNetwork(0, config, failures)
	network2 := mocks.NewMockNetwork(1, config, failures)
	network3 := mocks.NewMockNetwork(2, config, failures)

	// Connect networks
	peers1 := map[types.NodeID]*mocks.MockNetwork{1: network2, 2: network3}
	peers2 := map[types.NodeID]*mocks.MockNetwork{0: network1, 2: network3}
	peers3 := map[types.NodeID]*mocks.MockNetwork{0: network1, 1: network2}

	network1.SetPeers(peers1)
	network2.SetPeers(peers2)
	network3.SetPeers(peers3)

	// Create test message
	ctx := context.Background()
	testBlock := &types.Block{
		Hash:     [32]byte{1, 2, 3, 4, 5},
		Height:   1,
		View:     1,
		Proposer: 0,
	}
	proposal := messages.NewProposalMsg(testBlock, nil, 1, 0)

	// Send message from node 0 to node 1
	fmt.Printf("   Sending proposal from node 0 to node 1...")
	err := network1.Send(ctx, 1, proposal)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	fmt.Println(" ✓")

	// Broadcast message from node 2
	fmt.Printf("   Broadcasting proposal from node 2...")
	err = network3.Broadcast(ctx, proposal)
	if err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}
	fmt.Println(" ✓")

	// Wait for message delivery
	time.Sleep(50 * time.Millisecond)

	// Check if node 1 received messages
	select {
	case received := <-network2.Receive():
		fmt.Printf("   Node 1 received message from node %d: %s ✓\n", 
			received.Sender, received.Message.Type())
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("node 1 did not receive message in time")
	}

	// Check if node 1 received broadcast
	select {
	case received := <-network2.Receive():
		fmt.Printf("   Node 1 received broadcast from node %d: %s ✓\n", 
			received.Sender, received.Message.Type())
	case <-time.After(100 * time.Millisecond):
		fmt.Printf("   Node 1 did not receive broadcast (expected in some cases)\n")
	}

	// Get network stats
	stats := network1.GetStats()
	fmt.Printf("   Node 0 stats: %d peers, queue: %d/%d ✓\n", 
		stats.ConnectedPeers, stats.QueueSize, stats.QueueCapacity)

	return nil
}

// runStorageDemo demonstrates block and QC storage operations.
func runStorageDemo() error {
	fmt.Println("\n💾 Demo 2: Storage Operations")
	fmt.Println("----------------------------")

	config := mocks.DefaultStorageConfig()
	config.PersistenceDelay = 2 * time.Millisecond
	failures := mocks.DefaultStorageFailureConfig()

	storage := mocks.NewMockStorage(config, failures)

	// Create test block
	testBlock := &types.Block{
		Hash:     [32]byte{1, 2, 3, 4, 5, 6, 7, 8},
		Height:   1,
		View:     1,
		Proposer: 0,
	}

	// Store block
	fmt.Printf("   Storing block with height %d...", testBlock.Height)
	err := storage.StoreBlock(testBlock)
	if err != nil {
		return fmt.Errorf("failed to store block: %w", err)
	}
	fmt.Println(" ✓")

	// Retrieve block
	fmt.Printf("   Retrieving block by hash...")
	retrievedBlock, err := storage.GetBlock(testBlock.Hash)
	if err != nil {
		return fmt.Errorf("failed to retrieve block: %w", err)
	}
	if retrievedBlock.Hash != testBlock.Hash {
		return fmt.Errorf("retrieved block hash mismatch")
	}
	fmt.Println(" ✓")

	// Store view number
	fmt.Printf("   Storing view number 5...")
	err = storage.StoreView(5)
	if err != nil {
		return fmt.Errorf("failed to store view: %w", err)
	}
	fmt.Println(" ✓")

	// Retrieve view number
	fmt.Printf("   Retrieving current view...")
	currentView, err := storage.GetCurrentView()
	if err != nil {
		return fmt.Errorf("failed to get current view: %w", err)
	}
	if currentView != 5 {
		return fmt.Errorf("view number mismatch: got %d, expected 5", currentView)
	}
	fmt.Printf(" got view %d ✓\n", currentView)

	// Get storage stats
	stats := storage.GetStats()
	fmt.Printf("   Storage stats: %d blocks, %d QCs, view %d ✓\n", 
		stats.BlockCount, stats.QCCount, stats.CurrentView)

	return nil
}

// runCryptoDemo demonstrates signature and verification operations.
func runCryptoDemo() error {
	fmt.Println("\n🔐 Demo 3: Cryptographic Operations")
	fmt.Println("----------------------------------")

	config := mocks.DefaultCryptoConfig()
	failures := mocks.DefaultCryptoFailureConfig()

	crypto1 := mocks.NewMockCrypto(0, config, failures)
	crypto2 := mocks.NewMockCrypto(1, config, failures)

	// Add nodes to each other's known keys
	crypto1.AddNode(1)
	crypto2.AddNode(0)

	testData := []byte("hello consensus world")

	// Sign data with node 0
	fmt.Printf("   Signing data with node 0...")
	signature, err := crypto1.Sign(testData)
	if err != nil {
		return fmt.Errorf("failed to sign data: %w", err)
	}
	fmt.Printf(" signature length: %d ✓\n", len(signature))

	// Verify signature with node 1
	fmt.Printf("   Verifying signature with node 1...")
	err = crypto2.Verify(testData, signature, 0)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}
	fmt.Println(" ✓")

	// Test hash function
	fmt.Printf("   Computing hash of data...")
	hash1 := crypto1.Hash(testData)
	hash2 := crypto2.Hash(testData)
	if hash1 != hash2 {
		return fmt.Errorf("hash mismatch between nodes")
	}
	fmt.Printf(" hash: %x ✓\n", hash1[:8])

	// Test invalid signature verification
	fmt.Printf("   Testing invalid signature rejection...")
	invalidSignature := make([]byte, len(signature))
	copy(invalidSignature, signature)
	invalidSignature[0] ^= 0xFF // Corrupt first byte
	
	err = crypto2.Verify(testData, invalidSignature, 0)
	if err == nil {
		return fmt.Errorf("invalid signature was accepted")
	}
	fmt.Println(" correctly rejected ✓")

	// Get crypto stats
	stats := crypto1.GetStats()
	fmt.Printf("   Node 0 crypto stats: %d signs, %d verifies, %d hashes ✓\n", 
		stats.SignCount, stats.VerifyCount, stats.HashCount)

	return nil
}

// runIntegratedScenarioDemo demonstrates using TestEnvironment for orchestrated testing.
func runIntegratedScenarioDemo() error {
	fmt.Println("\n🌐 Demo 4: Integrated Test Environment")
	fmt.Println("------------------------------------")

	// Create happy path scenario
	scenario := mocks.HappyPathScenario()
	scenario.NodeCount = 5
	scenario.Duration = 2 * time.Second

	fmt.Printf("   Creating test environment with %d nodes...", scenario.NodeCount)
	env := mocks.NewTestEnvironment(scenario)
	fmt.Println(" ✓")

	// Create and connect nodes
	fmt.Printf("   Creating consensus nodes...")
	err := env.CreateNodes()
	if err != nil {
		return fmt.Errorf("failed to create nodes: %w", err)
	}
	fmt.Println(" ✓")

	fmt.Printf("   Establishing network connections...")
	err = env.ConnectNodes()
	if err != nil {
		return fmt.Errorf("failed to connect nodes: %w", err)
	}
	fmt.Println(" ✓")

	// Start nodes
	fmt.Printf("   Starting all nodes...")
	env.StartAllNodes()
	fmt.Println(" ✓")

	// Let the environment run briefly
	time.Sleep(500 * time.Millisecond)

	// Get environment statistics
	fmt.Printf("   Collecting environment statistics...")
	stats := env.GetEnvironmentStats()
	fmt.Printf("\n      - Total nodes: %d\n", stats.NodeCount)
	fmt.Printf("      - Running nodes: %d\n", stats.RunningNodes)
	fmt.Printf("      - Byzantine nodes: %d\n", stats.ByzantineNodes)
	fmt.Printf("      - Runtime: %v ✓\n", stats.RunTime)

	// Cleanup
	fmt.Printf("   Cleaning up environment...")
	env.Cleanup()
	fmt.Println(" ✓")

	return nil
}

// runFailureInjectionDemo demonstrates failure injection capabilities.
func runFailureInjectionDemo() error {
	fmt.Println("\n💥 Demo 5: Failure Injection")
	fmt.Println("---------------------------")

	// Create scenario with failures
	scenario := mocks.StorageFailureScenario()
	scenario.NodeCount = 4 // Minimum for BFT
	scenario.Duration = 1 * time.Second

	fmt.Printf("   Creating environment with storage failures...")
	env := mocks.NewTestEnvironment(scenario)
	
	err := env.CreateNodes()
	if err != nil {
		return fmt.Errorf("failed to create nodes: %w", err)
	}
	
	err = env.ConnectNodes()
	if err != nil {
		return fmt.Errorf("failed to connect nodes: %w", err)
	}
	fmt.Println(" ✓")

	// Start nodes
	env.StartAllNodes()

	// Simulate network partition
	fmt.Printf("   Simulating network partition...")
	err = env.SimulatePartition([]types.NodeID{0})
	if err != nil {
		return fmt.Errorf("failed to simulate partition: %w", err)
	}
	fmt.Println(" ✓")

	time.Sleep(200 * time.Millisecond)

	// End partition
	fmt.Printf("   Ending network partition...")
	env.EndPartition()
	fmt.Println(" ✓")

	// Set Byzantine behavior
	fmt.Printf("   Configuring Byzantine node...")
	env.SetByzantineNodes([]types.NodeID{1})
	fmt.Println(" ✓")

	time.Sleep(200 * time.Millisecond)

	// Get final stats
	stats := env.GetEnvironmentStats()
	fmt.Printf("   Final stats: %d Byzantine nodes, partition: %t ✓\n", 
		stats.ByzantineNodes, stats.PartitionActive)

	// Cleanup
	env.Cleanup()

	return nil
}