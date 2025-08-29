// Package main demonstrates the usage patterns of consensus abstraction interfaces.
// This demo shows how the NetworkInterface, StorageInterface, and CryptoInterface
// can be used with mock implementations for testing and development.
package main

import (
	"context"
	"errors"
	"time"

	"btc-federation/pkg/consensus/crypto"
	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/network"
	"btc-federation/pkg/consensus/storage"
	"btc-federation/pkg/consensus/types"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"
)

// newConsensusConfigForTesting creates a consensus configuration with mock public keys for testing.
func newConsensusConfigForTesting(nodeCount int) (*types.ConsensusConfig, error) {
	if nodeCount < 4 {
		return nil, fmt.Errorf("BFT consensus requires at least 4 nodes for meaningful fault tolerance, got %d", nodeCount)
	}

	// Generate mock public keys for testing
	mockKeys := make([]types.PublicKey, nodeCount)
	for i := 0; i < nodeCount; i++ {
		// Create a simple mock public key (32 bytes with node index pattern)
		mockKey := make([]byte, 32)
		mockKey[0] = byte(i) // First byte is the node index
		for j := 1; j < 32; j++ {
			mockKey[j] = byte((i + j) % 256) // Fill with deterministic pattern
		}
		mockKeys[i] = types.PublicKey(mockKey)
	}

	return types.NewConsensusConfig(mockKeys)
}

// MockNetwork implements NetworkInterface for demonstration
type MockNetwork struct {
	nodeID   types.NodeID
	messages chan network.ReceivedMessage
}

func NewMockNetwork(nodeID types.NodeID) *MockNetwork {
	return &MockNetwork{
		nodeID:   nodeID,
		messages: make(chan network.ReceivedMessage, 100),
	}
}

func (mn *MockNetwork) Send(ctx context.Context, nodeID types.NodeID, message messages.ConsensusMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		fmt.Printf("Node %d: Sending %s message to node %d\n", mn.nodeID, message.Type(), nodeID)
		return nil
	}
}

func (mn *MockNetwork) Broadcast(ctx context.Context, message messages.ConsensusMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		fmt.Printf("Node %d: Broadcasting %s message to all nodes\n", mn.nodeID, message.Type())
		return nil
	}
}

func (mn *MockNetwork) Receive() <-chan network.ReceivedMessage {
	return mn.messages
}

// Close is handled by the provider - not part of the interface
func (mn *MockNetwork) close() {
	close(mn.messages)
	fmt.Printf("Network: Node %d closed\n", mn.nodeID)
}

// MockStorage implements StorageInterface for demonstration
type MockStorage struct {
	blocks map[types.BlockHash]*types.Block
	qcs    map[string]*types.QuorumCertificate
	view   types.ViewNumber
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		blocks: make(map[types.BlockHash]*types.Block),
		qcs:    make(map[string]*types.QuorumCertificate),
		view:   0,
	}
}

func (ms *MockStorage) StoreBlock(block *types.Block) error {
	ms.blocks[block.Hash] = block
	fmt.Printf("Storage: Stored block %x at height %d\n", block.Hash[:8], block.Height)
	return nil
}

func (ms *MockStorage) GetBlock(hash types.BlockHash) (*types.Block, error) {
	if block, exists := ms.blocks[hash]; exists {
		return block, nil
	}
	return nil, fmt.Errorf("block not found %x: %w", hash[:8], storage.ErrNotFound)
}

func (ms *MockStorage) StoreQC(qc *types.QuorumCertificate) error {
	key := fmt.Sprintf("%x-%d", qc.BlockHash[:8], qc.Phase)
	ms.qcs[key] = qc
	fmt.Printf("Storage: Stored QC for block %x, phase %s\n", qc.BlockHash[:8], qc.Phase)
	return nil
}

func (ms *MockStorage) GetQC(blockHash types.BlockHash, phase types.ConsensusPhase) (*types.QuorumCertificate, error) {
	key := fmt.Sprintf("%x-%d", blockHash[:8], phase)
	if qc, exists := ms.qcs[key]; exists {
		return qc, nil
	}
	return nil, fmt.Errorf("QC not found for block %x, phase %s: %w", blockHash[:8], phase, storage.ErrNotFound)
}

func (ms *MockStorage) StoreView(view types.ViewNumber) error {
	ms.view = view
	fmt.Printf("Storage: Updated view to %d\n", view)
	return nil
}

func (ms *MockStorage) GetCurrentView() (types.ViewNumber, error) {
	return ms.view, nil
}

func (ms *MockStorage) GetBlocksByHeight(height types.Height) ([]*types.Block, error) {
	var blocks []*types.Block
	for _, block := range ms.blocks {
		if block.Height == height {
			blocks = append(blocks, block)
		}
	}
	return blocks, nil
}

func (ms *MockStorage) GetHighestQC() (*types.QuorumCertificate, error) {
	// Return the first QC found (simplified for demo)
	for _, qc := range ms.qcs {
		return qc, nil
	}
	return nil, fmt.Errorf("no QC found: %w", storage.ErrNotFound)
}

// Close is handled by the provider - not part of the interface
func (ms *MockStorage) close() {
	fmt.Println("Storage: Closed")
}

// MockCrypto implements CryptoInterface for demonstration
type MockCrypto struct {
	nodeID types.NodeID
}

func NewMockCrypto(nodeID types.NodeID) *MockCrypto {
	return &MockCrypto{nodeID: nodeID}
}

func (mc *MockCrypto) Sign(data []byte) ([]byte, error) {
	// Mock signature - in real implementation this would use actual cryptography
	signature := fmt.Sprintf("signature-node-%d-%x", mc.nodeID, data[:8])
	fmt.Printf("Crypto: Node %d signed data %x\n", mc.nodeID, data[:8])
	return []byte(signature), nil
}

func (mc *MockCrypto) Verify(data []byte, signature []byte, nodeID types.NodeID) error {
	expectedSig := fmt.Sprintf("signature-node-%d-%x", nodeID, data[:8])
	if string(signature) == expectedSig {
		fmt.Printf("Crypto: Verified signature from node %d\n", nodeID)
		return nil
	}
	return fmt.Errorf("signature verification failed: %w", crypto.ErrVerification)
}

func (mc *MockCrypto) Hash(data []byte) types.BlockHash {
	hash := sha256.Sum256(data)
	return hash
}

func main() {
	fmt.Println("=== Consensus Interfaces Demo ===\n")

	// Create consensus configuration with 5 nodes
	config, err := newConsensusConfigForTesting(5)
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	fmt.Printf("Configuration: %s\n", config.String())

	// Initialize mock implementations
	networkIntf := NewMockNetwork(0)
	storageIntf := NewMockStorage()
	cryptoIntf := NewMockCrypto(0)
	// Cleanup handled by providers in real implementation
	defer networkIntf.close()
	defer storageIntf.close()

	// Create context for operations
	ctx := context.Background()

	fmt.Println("\n1. Creating and storing a block...")

	// Create a sample block using NewBlock constructor
	blockData := []byte("sample block data")
	block := types.NewBlock(
		types.BlockHash{}, // Genesis parent
		1,                 // Height
		1,                 // View
		0,                 // Proposer
		blockData,         // Payload
	)

	// Store the block
	if err := storageIntf.StoreBlock(block); err != nil {
		log.Fatalf("Failed to store block: %v", err)
	}

	fmt.Println("\n2. Creating and storing votes...")

	// Create votes for the QC (need 3 for quorum in 5-node network)
	voterIDs := []types.NodeID{1, 2, 3}
	votes := make([]types.Vote, len(voterIDs))

	for i, voterID := range voterIDs {
		signature := make([]byte, 32)
		rand.Read(signature)
		
		vote := types.NewVote(
			block.Hash,
			block.View,
			types.PhasePrepare,
			voterID,
			signature,
		)
		votes[i] = *vote
		fmt.Printf("  Created vote from Node %d: %s\n", voterID, vote.String())
	}

	fmt.Println("\n3. Creating and storing a QuorumCertificate...")

	// Create QC from votes
	qc, err := types.NewQuorumCertificate(block.Hash, block.View, types.PhasePrepare, votes, config)
	if err != nil {
		log.Fatalf("Failed to create QC: %v", err)
	}

	if err := qc.Validate(config); err != nil {
		log.Fatalf("QC validation failed: %v", err)
	}

	if err := storageIntf.StoreQC(qc); err != nil {
		log.Fatalf("Failed to store QC: %v", err)
	}

	fmt.Println("\n4. Creating and broadcasting messages...")

	// Create a proposal message
	proposalMsg := messages.NewProposalMsg(block, nil, block.View, block.Proposer)
	if err := proposalMsg.Validate(config); err != nil {
		log.Fatalf("Proposal validation failed: %v", err)
	}

	// Broadcast the proposal
	if err := networkIntf.Broadcast(ctx, proposalMsg); err != nil {
		log.Fatalf("Failed to broadcast proposal: %v", err)
	}

	// Create a vote message
	voteMsg := messages.NewVoteMsg(&votes[0], votes[0].Voter)
	if err := voteMsg.Validate(config); err != nil {
		log.Fatalf("Vote message validation failed: %v", err)
	}

	// Send the vote to leader
	if err := networkIntf.Send(ctx, 0, voteMsg); err != nil {
		log.Fatalf("Failed to send vote: %v", err)
	}

	fmt.Println("\n5. Demonstrating signature operations...")

	// Sign some data
	testData := []byte("test consensus message")
	signature, err := cryptoIntf.Sign(testData)
	if err != nil {
		log.Fatalf("Failed to sign data: %v", err)
	}

	// Verify the signature
	if err := cryptoIntf.Verify(testData, signature, 0); err != nil {
		log.Fatalf("Failed to verify signature: %v", err)
	}

	fmt.Println("\n6. Demonstrating data retrieval...")

	// Retrieve the stored block
	retrievedBlock, err := storageIntf.GetBlock(block.Hash)
	if err != nil {
		log.Fatalf("Failed to retrieve block: %v", err)
	}
	fmt.Printf("Retrieved block height: %d\n", retrievedBlock.Height)

	// Retrieve the stored QC
	retrievedQC, err := storageIntf.GetQC(block.Hash, types.PhasePrepare)
	if err != nil {
		log.Fatalf("Failed to retrieve QC: %v", err)
	}
	fmt.Printf("Retrieved QC phase: %s\n", retrievedQC.Phase)

	fmt.Println("\n7. Demonstrating view management...")

	// Update view
	newView := types.ViewNumber(2)
	if err := storageIntf.StoreView(newView); err != nil {
		log.Fatalf("Failed to store view: %v", err)
	}

	// Retrieve current view
	currentView, err := storageIntf.GetCurrentView()
	if err != nil {
		log.Fatalf("Failed to get current view: %v", err)
	}
	fmt.Printf("Current view: %d\n", currentView)

	fmt.Println("\n8. Demonstrating error handling...")

	// Try to retrieve non-existent block
	nonExistentHash := types.BlockHash{0x99, 0x88, 0x77}
	_, err = storageIntf.GetBlock(nonExistentHash)
	if err != nil {
		fmt.Printf("Expected error for non-existent block: %v\n", err)
		
		// Check if it's the right error type using errors.Is
		if errors.Is(err, storage.ErrNotFound) {
			fmt.Println("  Correct error type: ErrNotFound")
		}
	}

	// Try to verify invalid signature
	invalidSig := []byte("invalid-signature")
	err = cryptoIntf.Verify(testData, invalidSig, 0)
	if err != nil {
		fmt.Printf("Expected error for invalid signature: %v\n", err)
		
		// Check if it's the right error type using errors.Is
		if errors.Is(err, crypto.ErrVerification) {
			fmt.Println("  Correct error type: ErrVerification")
		}
	}

	fmt.Println("\n9. Demonstrating context cancellation...")

	// Create a context with timeout to demonstrate cancellation
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	
	// This should be cancelled immediately due to the short timeout
	time.Sleep(1 * time.Millisecond) // Ensure context times out
	err = networkIntf.Send(ctxWithTimeout, 1, proposalMsg)
	if err != nil {
		fmt.Printf("Expected timeout error: %v\n", err)
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("  Correct error type: context.DeadlineExceeded")
		}
	}

	fmt.Println("\n=== Demo completed successfully! ===")
	fmt.Println("\nThis demo showed:")
	fmt.Println("- NetworkInterface: Send/Broadcast with context cancellation support")
	fmt.Println("- StorageInterface: Block/QC/View persistence operations")
	fmt.Println("- CryptoInterface: Sign/Verify/Hash operations")
	fmt.Println("- Error handling with errors.Is for sentinel errors")
	fmt.Println("- Provider-based lifecycle management (no Close methods)")
	fmt.Println("- Integration with existing consensus types and messages")
}