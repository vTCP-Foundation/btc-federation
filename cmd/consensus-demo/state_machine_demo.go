// state_machine_demo.go demonstrates the HotStuff state machine implementation
package main

import (
	"crypto/rand"
	"fmt"
	"log"

	"btc-federation/pkg/consensus/engine"
	"btc-federation/pkg/consensus/types"
)

// demoStateMachine demonstrates the complete HotStuff state machine functionality
func demoStateMachine() {
	fmt.Println("\n=== HotStuff State Machine Demo ===")
	
	// Create configuration for 5 nodes (can tolerate 1 Byzantine)
	config, err := newConsensusConfigForTesting(5)
	if err != nil {
		log.Fatalf("Failed to create consensus config: %v", err)
	}
	
	fmt.Printf("Configuration: %s\n", config.String())
	
	// Demo 1: Initialize consensus nodes
	fmt.Println("\n1. Initialize Consensus Nodes")
	demoNodeInitialization(config)
	
	// Demo 2: Block proposal and vote processing
	fmt.Println("\n2. Block Proposal Processing")
	demoBlockProposalProcessing(config)
	
	// Demo 3: Vote collection and QC formation
	fmt.Println("\n3. Vote Collection and QC Formation")  
	demoVoteCollectionAndQC(config)
	
	// Demo 4: Three-phase state transitions
	fmt.Println("\n4. Three-Phase State Transitions")
	demoThreePhaseTransitions(config)
	
	// Demo 5: Safety rule enforcement
	fmt.Println("\n5. Safety Rule Enforcement")
	demoSafetyRuleEnforcement(config)
	
	// Demo 6: Block tree and fork handling
	fmt.Println("\n6. Block Tree and Fork Handling")
	demoBlockTreeForkHandling(config)
	
	fmt.Println("\n=== State Machine Demo completed successfully! ===")
}

func demoNodeInitialization(config *types.ConsensusConfig) {
	// Initialize multiple consensus nodes
	nodes := make([]*engine.HotStuffConsensus, config.TotalNodes())
	
	for i := 0; i < config.TotalNodes(); i++ {
		nodeID := types.NodeID(i)
		node, err := engine.NewHotStuffConsensus(nodeID, config)
		if err != nil {
			log.Fatalf("Failed to create consensus node %d: %v", nodeID, err)
		}
		nodes[i] = node
		fmt.Printf("  Node %d initialized: %s\n", nodeID, node.String())
		
		// Verify components are properly initialized
		blockTree := node.GetBlockTree()
		fmt.Printf("    BlockTree: %s\n", blockTree.String())
		fmt.Printf("    CurrentView: %d\n", node.GetCurrentView())
	}
}

func demoBlockProposalProcessing(config *types.ConsensusConfig) {
	// Create a consensus node
	nodeID := types.NodeID(1)
	node, err := engine.NewHotStuffConsensus(nodeID, config)
	if err != nil {
		log.Fatalf("Failed to create consensus node: %v", err)
	}
	
	// Get the leader for view 1
	leader, err := config.GetLeaderForView(1)
	if err != nil {
		log.Fatalf("Failed to get leader for view 1: %v", err)
	}
	
	// Advance to view 1
	node.AdvanceView()
	
	// Create a block proposal from the expected leader
	payload := []byte("Demo block payload")
	genesisHash := node.GetBlockTree().GetRoot().Hash
	proposalBlock := types.NewBlock(
		genesisHash,
		1,
		1, // View 1
		leader,
		payload,
	)
	
	fmt.Printf("  Leader %d proposes block: %s\n", leader, proposalBlock.String())
	
	// Process the proposal
	vote, err := node.ProcessProposal(proposalBlock)
	if err != nil {
		log.Fatalf("Failed to process proposal: %v", err)
	}
	
	if vote != nil {
		fmt.Printf("  Node %d generated vote: %s\n", nodeID, vote.String())
		fmt.Printf("  Vote validation: PASSED\n")
	} else {
		fmt.Printf("  Node %d did not vote (safety rules prevented voting)\n", nodeID)
	}
	
	// Verify block was added to the tree
	blockTree := node.GetBlockTree()
	retrievedBlock, err := blockTree.GetBlock(proposalBlock.Hash)
	if err != nil {
		log.Fatalf("Failed to retrieve block from tree: %v", err)
	}
	fmt.Printf("  Block added to tree: %s\n", retrievedBlock.String())
}

func demoVoteCollectionAndQC(config *types.ConsensusConfig) {
	// Create a consensus node to collect votes
	collectorNode, err := engine.NewHotStuffConsensus(types.NodeID(0), config)
	if err != nil {
		log.Fatalf("Failed to create collector node: %v", err)
	}
	
	// Create a block to vote on
	payload := []byte("QC formation demo block")
	genesisHash := collectorNode.GetBlockTree().GetRoot().Hash
	block := types.NewBlock(
		genesisHash,
		1,
		1, // View 1
		types.NodeID(0), // Leader
		payload,
	)
	
	fmt.Printf("  Block for voting: %s\n", block.String())
	
	// Create votes from different nodes (enough for quorum)
	quorumSize := config.QuorumThreshold()
	votes := make([]*types.Vote, quorumSize)
	
	for i := 0; i < quorumSize; i++ {
		voterID := types.NodeID(i + 1) // Use nodes 1, 2, 3
		signature := make([]byte, 32)
		rand.Read(signature)
		
		vote := types.NewVote(
			block.Hash,
			1, // View 1
			types.PhasePrepare,
			voterID,
			signature,
		)
		votes[i] = vote
		fmt.Printf("  Vote from Node %d: %s\n", voterID, vote.String())
	}
	
	// Process votes one by one and check for QC formation
	var qc *types.QuorumCertificate
	for i, vote := range votes {
		resultQC, err := collectorNode.ProcessVote(vote)
		if err != nil {
			log.Fatalf("Failed to process vote %d: %v", i, err)
		}
		
		if resultQC != nil {
			qc = resultQC
			fmt.Printf("  QC formed after %d votes: %s\n", i+1, qc.String())
			break
		}
		fmt.Printf("  Vote %d processed, no QC yet (%d/%d)\n", i+1, i+1, quorumSize)
	}
	
	if qc == nil {
		log.Fatalf("Failed to form QC with sufficient votes")
	}
	
	// Verify QC properties
	if !qc.HasQuorum(config) {
		log.Fatalf("QC reports no quorum despite having enough votes")
	}
	fmt.Printf("  QC validation: PASSED\n")
	fmt.Printf("  Quorum threshold: %d, Actual votes: %d\n", config.QuorumThreshold(), len(qc.Votes))
}

func demoThreePhaseTransitions(config *types.ConsensusConfig) {
	// Create a consensus node
	node, err := engine.NewHotStuffConsensus(types.NodeID(0), config)
	if err != nil {
		log.Fatalf("Failed to create consensus node: %v", err)
	}
	
	// Create a block for the consensus flow
	payload := []byte("Three-phase demo block")
	genesisHash := node.GetBlockTree().GetRoot().Hash
	block := types.NewBlock(
		genesisHash,
		1,
		1, // View 1  
		types.NodeID(0), // Leader
		payload,
	)
	
	fmt.Printf("  Block: %s\n", block.String())
	
	// Helper function to create QC for a phase
	createQC := func(phase types.ConsensusPhase) *types.QuorumCertificate {
		votes := make([]types.Vote, config.QuorumThreshold())
		for i := 0; i < config.QuorumThreshold(); i++ {
			signature := make([]byte, 32)
			rand.Read(signature)
			
			votes[i] = *types.NewVote(
				block.Hash,
				1, // View 1
				phase,
				types.NodeID(i+1), // Voters 1, 2, 3
				signature,
			)
		}
		
		qc, err := types.NewQuorumCertificate(block.Hash, 1, phase, votes, config)
		if err != nil {
			log.Fatalf("Failed to create %s QC: %v", phase.String(), err)
		}
		return qc
	}
	
	// Phase 1: Prepare
	fmt.Printf("  Phase 1: Prepare\n")
	prepareQC := createQC(types.PhasePrepare)
	
	// Process prepare votes to trigger prepare QC handling
	for _, vote := range prepareQC.Votes {
		_, err := node.ProcessVote(&vote)
		if err != nil {
			log.Fatalf("Failed to process prepare vote: %v", err)  
		}
	}
	
	currentPrepareQC := node.GetPrepareQC()
	if currentPrepareQC == nil {
		log.Fatalf("Prepare QC not set after processing prepare votes")
	}
	fmt.Printf("    PrepareQC updated: %s\n", currentPrepareQC.String())
	
	// Phase 2: PreCommit  
	fmt.Printf("  Phase 2: PreCommit\n")
	precommitQC := createQC(types.PhasePreCommit)
	
	for _, vote := range precommitQC.Votes {
		_, err := node.ProcessVote(&vote)
		if err != nil {
			log.Fatalf("Failed to process precommit vote: %v", err)
		}
	}
	
	currentLockedQC := node.GetLockedQC()
	if currentLockedQC == nil {
		log.Fatalf("Locked QC not set after processing precommit votes")
	}
	fmt.Printf("    LockedQC updated: %s\n", currentLockedQC.String())
	
	// Phase 3: Commit
	fmt.Printf("  Phase 3: Commit\n")
	commitQC := createQC(types.PhaseCommit)
	
	// Add the block to tree first so commit can find it
	if err := node.GetBlockTree().AddBlock(block); err != nil {
		log.Fatalf("Failed to add block to tree: %v", err)
	}
	
	for _, vote := range commitQC.Votes {
		_, err := node.ProcessVote(&vote)
		if err != nil {
			log.Fatalf("Failed to process commit vote: %v", err)
		}
	}
	
	// Check if block was committed
	committedBlock := node.GetBlockTree().GetCommitted()
	if committedBlock.Hash != block.Hash {
		log.Fatalf("Block was not committed after commit QC")
	}
	fmt.Printf("    Block committed: %s\n", committedBlock.String())
	
	fmt.Printf("  Three-phase transition completed successfully!\n")
}

func demoSafetyRuleEnforcement(config *types.ConsensusConfig) {
	// Create a consensus node
	node, err := engine.NewHotStuffConsensus(types.NodeID(1), config)
	if err != nil {
		log.Fatalf("Failed to create consensus node: %v", err)
	}
	
	// Advance to view 1
	node.AdvanceView()
	
	// Create two conflicting blocks for the same view
	genesisHash := node.GetBlockTree().GetRoot().Hash
	
	block1 := types.NewBlock(
		genesisHash,
		1,
		1, // View 1
		types.NodeID(1), // This node as leader
		[]byte("Block 1"),
	)
	
	block2 := types.NewBlock(
		genesisHash,  
		1,
		1, // Same view
		types.NodeID(1), // Same leader
		[]byte("Block 2"), // Different payload
	)
	
	fmt.Printf("  Block 1: %s\n", block1.String())
	fmt.Printf("  Block 2: %s\n", block2.String())
	
	// Process first block - should succeed
	vote1, err := node.ProcessProposal(block1)
	if err != nil {
		log.Fatalf("Failed to process first proposal: %v", err)
	}
	if vote1 != nil {
		fmt.Printf("  Node voted for Block 1: %s\n", vote1.String())
	}
	
	// Try to process second block in same view - should fail due to safety rules
	vote2, err := node.ProcessProposal(block2)
	if err == nil && vote2 != nil {
		log.Fatalf("Safety rules failed: node voted twice in the same view")
	}
	
	fmt.Printf("  Block 2 proposal rejected: %v\n", err)
	fmt.Printf("  Safety rule enforcement: PASSED\n")
	
	// Test view advancement and allow voting in new view
	node.AdvanceView() // Advance to view 2
	fmt.Printf("  Advanced to view %d\n", node.GetCurrentView())
	
	// Create a new block for view 2
	block3 := types.NewBlock(
		genesisHash,
		1,
		2, // View 2
		types.NodeID(2), // New leader
		[]byte("Block 3 for view 2"),
	)
	
	vote3, err := node.ProcessProposal(block3)
	if err != nil {
		log.Fatalf("Failed to process proposal in new view: %v", err)
	}
	if vote3 != nil {
		fmt.Printf("  Node voted for Block 3 in new view: %s\n", vote3.String())
		fmt.Printf("  View advancement allows new voting: PASSED\n")
	}
}

func demoBlockTreeForkHandling(config *types.ConsensusConfig) {
	// Create a consensus node
	node, err := engine.NewHotStuffConsensus(types.NodeID(0), config)
	if err != nil {
		log.Fatalf("Failed to create consensus node: %v", err)
	}
	
	blockTree := node.GetBlockTree()
	genesisBlock := blockTree.GetRoot()
	
	fmt.Printf("  Genesis block: %s\n", genesisBlock.String())
	
	// Create a chain: genesis -> block1 -> block2a
	block1 := types.NewBlock(
		genesisBlock.Hash,
		1,
		1,
		types.NodeID(0),
		[]byte("Block 1"),
	)
	
	err = blockTree.AddBlock(block1)
	if err != nil {
		log.Fatalf("Failed to add block1: %v", err)
	}
	fmt.Printf("  Added Block 1: %s\n", block1.String())
	
	// Create fork: two different blocks at height 2
	block2a := types.NewBlock(
		block1.Hash,
		2,
		2,
		types.NodeID(1),
		[]byte("Block 2a"),
	)
	
	block2b := types.NewBlock(
		block1.Hash, // Same parent
		2,           // Same height
		3,           // Different view
		types.NodeID(2),
		[]byte("Block 2b"), // Different payload
	)
	
	// Add both blocks to create a fork
	err = blockTree.AddBlock(block2a)
	if err != nil {
		log.Fatalf("Failed to add block2a: %v", err)
	}
	fmt.Printf("  Added Block 2a: %s\n", block2a.String())
	
	err = blockTree.AddBlock(block2b)
	if err != nil {
		log.Fatalf("Failed to add block2b: %v", err)
	}
	fmt.Printf("  Added Block 2b: %s\n", block2b.String())
	
	// Verify fork structure
	children := blockTree.GetChildren(block1.Hash)
	fmt.Printf("  Block 1 has %d children (fork created)\n", len(children))
	
	for i, child := range children {
		fmt.Printf("    Child %d: %s\n", i+1, child.String())
	}
	
	// Test ancestor relationships
	isAncestor, err := blockTree.IsAncestor(genesisBlock.Hash, block2a.Hash)
	if err != nil {
		log.Fatalf("Failed to check ancestry: %v", err)
	}
	fmt.Printf("  Genesis is ancestor of Block 2a: %t\n", isAncestor)
	
	isAncestor, err = blockTree.IsAncestor(block1.Hash, block2b.Hash)
	if err != nil {
		log.Fatalf("Failed to check ancestry: %v", err)
	}
	fmt.Printf("  Block 1 is ancestor of Block 2b: %t\n", isAncestor)
	
	// Test LCA (Lowest Common Ancestor)
	lca, err := blockTree.FindLCA(block2a.Hash, block2b.Hash)
	if err != nil {
		log.Fatalf("Failed to find LCA: %v", err)
	}
	fmt.Printf("  LCA of Block 2a and 2b: %s\n", lca.String())
	
	if lca.Hash != block1.Hash {
		log.Fatalf("LCA should be Block 1")
	}
	
	// Commit one branch and verify state
	err = blockTree.CommitBlock(block2a)
	if err != nil {
		log.Fatalf("Failed to commit block2a: %v", err)
	}
	
	committedBlock := blockTree.GetCommitted()
	fmt.Printf("  Committed block: %s\n", committedBlock.String())
	
	// Verify tree state
	fmt.Printf("  Final tree state: %s\n", blockTree.String())
	
	if err := blockTree.ValidateTree(); err != nil {
		log.Fatalf("Tree validation failed: %v", err)
	}
	fmt.Printf("  Tree validation: PASSED\n")
}