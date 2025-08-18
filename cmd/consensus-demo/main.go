// consensus-demo demonstrates the core HotStuff consensus data structures.
package main

import (
	"crypto/rand"
	"fmt"
	"log"

	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/types"
)

// newConsensusConfigForTesting creates a consensus configuration with mock public keys for testing.
func newConsensusConfigForTesting(nodeCount int) (*types.ConsensusConfig, error) {
	if nodeCount < 4 {
		return nil, fmt.Errorf("BFT consensus requires at least 4 nodes for meaningful fault tolerance, got %d", nodeCount)
	}

	if nodeCount > 512 {
		return nil, fmt.Errorf("node count cannot exceed 512, got %d", nodeCount)
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

func main() {
	fmt.Println("=== HotStuff Consensus Foundation Demo ===")
	
	// Create consensus configuration for 5 nodes (can tolerate 1 Byzantine)
	config, err := newConsensusConfigForTesting(5)
	if err != nil {
		log.Fatalf("Failed to create consensus config: %v", err)
	}
	fmt.Printf("Consensus Configuration: %s\n", config.String())
	
	// Show public key mapping
	fmt.Println("\nPublic Key Mapping:")
	for i := 0; i < config.TotalNodes(); i++ {
		nodeID := types.NodeID(i)
		pubKey, err := config.GetPublicKey(nodeID)
		if err != nil {
			log.Fatalf("Failed to get public key for node %d: %v", nodeID, err)
		}
		fmt.Printf("  Node %d: %x...%x (%d bytes)\n", nodeID, pubKey[:4], pubKey[len(pubKey)-4:], len(pubKey))
	}
	
	// Demo 1: Block creation and hash calculation
	fmt.Println("\n1. Block Creation and Hash Calculation")
	demoBlockCreation(config)
	
	// Demo 2: Vote creation for different phases
	fmt.Println("\n2. Vote Creation for Different Phases")
	demoVoteCreation(config)
	
	// Demo 3: QC formation from multiple votes
	fmt.Println("\n3. QuorumCertificate Formation")
	demoQCFormation(config)
	
	// Demo 4: Message type discrimination
	fmt.Println("\n4. Message Type Discrimination")
	demoMessageTypes(config)
	
	// Demo 5: Basic consensus data flow
	fmt.Println("\n5. Basic Consensus Data Flow")
	demoConsensusFlow(config)
	
	fmt.Println("\n=== Demo completed successfully! ===")
}

func demoBlockCreation(config *types.ConsensusConfig) {
	// Create genesis block
	genesisPayload := []byte("Genesis Block")
	genesisBlock := types.NewBlock(
		types.BlockHash{}, // Empty parent for genesis
		0,                 // Height 0
		0,                 // View 0
		0,                 // Proposer (NodeID 0)
		genesisPayload,
	)
	
	fmt.Printf("Genesis Block: %s\n", genesisBlock.String())
	fmt.Printf("  Hash: %x\n", genesisBlock.Hash)
	fmt.Printf("  IsGenesis: %t\n", genesisBlock.IsGenesis())
	
	// Validate genesis block
	if err := genesisBlock.Validate(config); err != nil {
		log.Fatalf("Genesis block validation failed: %v", err)
	}
	fmt.Printf("  Validation: PASSED\n")
	
	// Create next block
	nextPayload := []byte("Block 1 - Transaction Data")
	nextBlock := types.NewBlock(
		genesisBlock.Hash,
		1,
		1,
		1, // NodeID 1
		nextPayload,
	)
	
	fmt.Printf("Next Block: %s\n", nextBlock.String())
	fmt.Printf("  ParentHash: %x\n", nextBlock.ParentHash)
	fmt.Printf("  IsGenesis: %t\n", nextBlock.IsGenesis())
	
	if err := nextBlock.Validate(config); err != nil {
		log.Fatalf("Next block validation failed: %v", err)
	}
	fmt.Printf("  Validation: PASSED\n")
}

func demoVoteCreation(config *types.ConsensusConfig) {
	// Create a block to vote on
	blockPayload := []byte("Block to vote on")
	block := types.NewBlock(
		types.BlockHash{},
		1,
		1,
		0, // NodeID 0 as leader
		blockPayload,
	)
	
	// Create votes for different phases
	phases := []types.ConsensusPhase{
		types.PhasePrepare,
		types.PhasePreCommit,
		types.PhaseCommit,
	}
	
	for _, phase := range phases {
		// Generate mock signature
		signature := make([]byte, 32)
		rand.Read(signature)
		
		vote := types.NewVote(
			block.Hash,
			1, // View 1
			phase,
			1, // NodeID 1 as voter
			signature,
		)
		
		fmt.Printf("Vote for %s: %s\n", phase.String(), vote.String())
		
		if err := vote.Validate(config); err != nil {
			log.Fatalf("Vote validation failed: %v", err)
		}
		fmt.Printf("  Validation: PASSED\n")
	}
}

func demoQCFormation(config *types.ConsensusConfig) {
	// Create a block
	blockPayload := []byte("QC Block")
	block := types.NewBlock(
		types.BlockHash{},
		2,
		2,
		0, // NodeID 0 as leader
		blockPayload,
	)
	
	// Create multiple votes for the same block and phase
	voterIDs := []types.NodeID{1, 2, 3, 4} // Use enough for quorum (need 3 for 5 nodes)
	votes := make([]types.Vote, len(voterIDs))
	
	for i, voterID := range voterIDs {
		signature := make([]byte, 32)
		rand.Read(signature)
		
		votes[i] = *types.NewVote(
			block.Hash,
			2, // View 2
			types.PhasePrepare,
			voterID,
			signature,
		)
	}
	
	// Create QuorumCertificate
	qc, err := types.NewQuorumCertificate(
		block.Hash,
		2, // View 2
		types.PhasePrepare,
		votes,
		config,
	)
	if err != nil {
		log.Fatalf("Failed to create QC: %v", err)
	}
	
	fmt.Printf("QuorumCertificate: %s\n", qc.String())
	fmt.Printf("  Vote Count: %d\n", len(qc.Votes))
	fmt.Printf("  VoterBitmap: %x\n", qc.VoterBitmap)
	
	// Test quorum threshold using config
	fmt.Printf("  HasQuorum: %t\n", qc.HasQuorum(config)) // Should be true (4 >= 3 for 5-node network)
	
	// Get voters from vote list and bitmap
	voters_list := qc.GetVoters()
	fmt.Printf("  Voters from votes: %v\n", voters_list)
	voters_bitmap := qc.GetVotersFromBitmap(config)
	fmt.Printf("  Voters from bitmap: %v\n", voters_bitmap)
	
	if err := qc.Validate(config); err != nil {
		log.Fatalf("QC validation failed: %v", err)
	}
	fmt.Printf("  Validation: PASSED\n")
}

func demoMessageTypes(config *types.ConsensusConfig) {
	// Create a block and QC for messages
	blockPayload := []byte("Message Demo Block")
	block := types.NewBlock(
		types.BlockHash{},
		3,
		3,
		0, // NodeID 0 as leader
		blockPayload,
	)
	
	// Create a simple QC with enough votes for quorum
	signature := make([]byte, 32)
	rand.Read(signature)
	// Create enough votes for quorum (need 3 votes for 5-node network)
	votes := make([]types.Vote, 3)
	for i := 0; i < 3; i++ {
		signature := make([]byte, 32)
		rand.Read(signature)
		votes[i] = *types.NewVote(block.Hash, 3, types.PhasePrepare, types.NodeID(i+1), signature)
	}
	qc, err := types.NewQuorumCertificate(block.Hash, 3, types.PhasePrepare, votes, config)
	if err != nil {
		log.Fatalf("Failed to create QC for message demo: %v", err)
	}
	// Use the first vote for vote message demo
	vote := &votes[0]
	
	// Create different message types
	messageTests := []struct {
		name    string
		message messages.ConsensusMessage
	}{
		{
			name:    "ProposalMsg",
			message: messages.NewProposalMsg(block, qc, 3, 0), // NodeID 0 as leader
		},
		{
			name:    "VoteMsg", 
			message: messages.NewVoteMsg(vote, 1), // NodeID 1 as voter
		},
		{
			name:    "TimeoutMsg",
			message: messages.NewTimeoutMsg(3, qc, 2, signature), // NodeID 2 sends timeout
		},
		{
			name:    "NewViewMsg",
			message: messages.NewNewViewMsg(4, qc, []messages.TimeoutMsg{}, 3), // NodeID 3 as new leader
		},
	}
	
	for _, test := range messageTests {
		fmt.Printf("%s:\n", test.name)
		fmt.Printf("  Type: %s\n", test.message.Type().String())
		fmt.Printf("  View: %d\n", test.message.View())
		fmt.Printf("  Sender: %s\n", test.message.Sender())
		
		if err := test.message.Validate(config); err != nil {
			log.Fatalf("%s validation failed: %v", test.name, err)
		}
		fmt.Printf("  Validation: PASSED\n")
	}
}

func demoConsensusFlow(config *types.ConsensusConfig) {
	fmt.Println("Simulating basic HotStuff consensus flow...")
	
	// Phase 1: Leader proposes a block
	fmt.Println("\nPhase 1: Block Proposal")
	proposalPayload := []byte("Consensus Flow Block")
	// Get leader for view 1
	leader, err := config.GetLeaderForView(1)
	if err != nil {
		log.Fatalf("Failed to get leader for view 1: %v", err)
	}

	proposedBlock := types.NewBlock(
		types.BlockHash{}, // Genesis parent
		1,
		1,
		leader, // Leader for view 1
		proposalPayload,
	)
	
	// Leader creates proposal message
	proposal := messages.NewProposalMsg(proposedBlock, nil, 1, leader)
	fmt.Printf("  Leader proposes: %s\n", proposedBlock.String())
	fmt.Printf("  Proposal message: Type=%s, View=%d, Sender=%s\n", 
		proposal.Type().String(), proposal.View(), proposal.Sender())
	
	// Phase 2: Nodes vote in Prepare phase (need quorum = 3 votes)
	fmt.Println("\nPhase 2: Prepare Phase Voting")
	prepareVotes := make([]types.Vote, 0)
	// Use first 3 nodes as voters (excluding leader if it's node 0)
	voterIDs := []types.NodeID{1, 2, 3} // Enough for quorum
	
	for _, voterID := range voterIDs {
		signature := make([]byte, 32)
		rand.Read(signature)
		
		vote := types.NewVote(
			proposedBlock.Hash,
			1,
			types.PhasePrepare,
			voterID,
			signature,
		)
		
		prepareVotes = append(prepareVotes, *vote)
		voteMsg := messages.NewVoteMsg(vote, voterID)
		fmt.Printf("  Node %d votes: %s\n", voterID, vote.String())
		fmt.Printf("    Vote message: Type=%s, View=%d, Sender=%d\n", 
			voteMsg.Type().String(), voteMsg.View(), voteMsg.Sender())
	}
	
	// Phase 3: Form Prepare QC
	fmt.Println("\nPhase 3: Prepare QC Formation")
	prepareQC, err := types.NewQuorumCertificate(
		proposedBlock.Hash,
		1,
		types.PhasePrepare,
		prepareVotes,
		config,
	)
	if err != nil {
		log.Fatalf("Failed to create prepare QC: %v", err)
	}
	fmt.Printf("  Prepare QC: %s\n", prepareQC.String())
	fmt.Printf("  Quorum achieved: %t (threshold=%d)\n", prepareQC.HasQuorum(config), config.QuorumThreshold())
	
	// Phase 4: PreCommit phase (simplified)
	fmt.Println("\nPhase 4: PreCommit Phase")
	precommitVotes := make([]types.Vote, 0)
	for _, voterID := range voterIDs {
		signature := make([]byte, 32)
		rand.Read(signature)
		
		vote := types.NewVote(
			proposedBlock.Hash,
			1,
			types.PhasePreCommit,
			voterID,
			signature,
		)
		precommitVotes = append(precommitVotes, *vote)
		fmt.Printf("  Node %d precommits: %s\n", voterID, vote.String())
	}
	
	precommitQC, err := types.NewQuorumCertificate(
		proposedBlock.Hash,
		1,
		types.PhasePreCommit,
		precommitVotes,
		config,
	)
	if err != nil {
		log.Fatalf("Failed to create precommit QC: %v", err)
	}
	fmt.Printf("  PreCommit QC: %s\n", precommitQC.String())
	
	// Phase 5: Commit phase
	fmt.Println("\nPhase 5: Commit Phase")
	commitVotes := make([]types.Vote, 0)
	for _, voterID := range voterIDs {
		signature := make([]byte, 32)
		rand.Read(signature)
		
		vote := types.NewVote(
			proposedBlock.Hash,
			1,
			types.PhaseCommit,
			voterID,
			signature,
		)
		commitVotes = append(commitVotes, *vote)
		fmt.Printf("  Node %d commits: %s\n", voterID, vote.String())
	}
	
	commitQC, err := types.NewQuorumCertificate(
		proposedBlock.Hash,
		1,
		types.PhaseCommit,
		commitVotes,
		config,
	)
	if err != nil {
		log.Fatalf("Failed to create commit QC: %v", err)
	}
	fmt.Printf("  Commit QC: %s\n", commitQC.String())
	fmt.Printf("  Block finalized! ✓\n")
	
	fmt.Println("\nConsensus flow complete - Block committed to chain!")
}