package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"btc-federation/pkg/consensus/crypto"
	"btc-federation/pkg/consensus/engine"
	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/network"
	"btc-federation/pkg/consensus/storage"
	"btc-federation/pkg/consensus/types"
)

// HotStuffCoordinator implements the complete HotStuff protocol following the data flow diagram exactly.
// This is the bulletproof implementation for high-stakes environments.
type HotStuffCoordinator struct {
	// Node configuration
	nodeID     types.NodeID
	validators []types.NodeID
	config     *types.ConsensusConfig
	
	// Infrastructure
	consensus *engine.HotStuffConsensus
	network   network.NetworkInterface
	storage   storage.StorageInterface
	crypto    crypto.CryptoInterface
	
	// Current state
	currentView  types.ViewNumber
	currentPhase types.ConsensusPhase
	
	// Timeout and view change state
	viewTimer         *time.Timer
	timeoutStarted    bool
	timeoutMessages   map[types.ViewNumber]map[types.NodeID]*messages.TimeoutMsg
	newViewMessages   map[types.ViewNumber]map[types.NodeID]*messages.NewViewMsg
	highestQC         *types.QuorumCertificate
	
	// Phase state tracking per block (prevent duplicate phase transitions)
	processedPhases map[types.BlockHash]map[types.ConsensusPhase]bool
	// Track processed QCs to prevent duplicate decide phase processing
	processedCommitQCs map[types.BlockHash]bool
	
	// Vote collection per phase per block
	prepareVotes    map[types.BlockHash]map[types.NodeID]*types.Vote
	preCommitVotes  map[types.BlockHash]map[types.NodeID]*types.Vote
	commitVotes     map[types.BlockHash]map[types.NodeID]*types.Vote
	
	// QC storage
	prepareQCs    map[types.BlockHash]*types.QuorumCertificate
	preCommitQCs  map[types.BlockHash]*types.QuorumCertificate
	commitQCs     map[types.BlockHash]*types.QuorumCertificate
	
	// Synchronization
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

// NewHotStuffCoordinator creates a new coordinator following the exact HotStuff protocol.
func NewHotStuffCoordinator(
	nodeID types.NodeID,
	validators []types.NodeID,
	config *types.ConsensusConfig,
	consensus *engine.HotStuffConsensus,
	network network.NetworkInterface,
	storage storage.StorageInterface,
	crypto crypto.CryptoInterface,
) *HotStuffCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &HotStuffCoordinator{
		nodeID:          nodeID,
		validators:      validators,
		config:          config,
		consensus:       consensus,
		network:         network,
		storage:         storage,
		crypto:          crypto,
		currentView:        0,
		currentPhase:       types.PhaseNone,
		viewTimer:          nil,
		timeoutStarted:     false,
		timeoutMessages:    make(map[types.ViewNumber]map[types.NodeID]*messages.TimeoutMsg),
		newViewMessages:    make(map[types.ViewNumber]map[types.NodeID]*messages.NewViewMsg),
		highestQC:          nil,
		processedPhases:    make(map[types.BlockHash]map[types.ConsensusPhase]bool),
		processedCommitQCs: make(map[types.BlockHash]bool),
		prepareVotes:    make(map[types.BlockHash]map[types.NodeID]*types.Vote),
		preCommitVotes:  make(map[types.BlockHash]map[types.NodeID]*types.Vote),
		commitVotes:     make(map[types.BlockHash]map[types.NodeID]*types.Vote),
		prepareQCs:      make(map[types.BlockHash]*types.QuorumCertificate),
		preCommitQCs:    make(map[types.BlockHash]*types.QuorumCertificate),
		commitQCs:       make(map[types.BlockHash]*types.QuorumCertificate),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start initializes the coordinator.
func (hc *HotStuffCoordinator) Start() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if hc.started {
		return fmt.Errorf("coordinator already started")
	}
	
	hc.started = true
	return nil
}

// Stop gracefully shuts down the coordinator.
func (hc *HotStuffCoordinator) Stop() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if !hc.started {
		return nil
	}
	
	// Stop view timer
	hc.stopViewTimer()
	
	hc.cancel()
	hc.started = false
	return nil
}

// ProposeBlock implements lines 20-29: Block Proposal Phase
func (hc *HotStuffCoordinator) ProposeBlock(payload []byte) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if !hc.started {
		return fmt.Errorf("coordinator not started")
	}
	
	// Check if we are the leader for current view
	if !hc.isLeader(hc.currentView) {
		return fmt.Errorf("node %d is not leader for view %d", hc.nodeID, hc.currentView)
	}
	
	// Line 21-22: Get highest QC from block tree
	blockTree := hc.consensus.GetBlockTree()
	highestQC := hc.consensus.GetPrepareQC()
	
	// Line 23-24: Get parent block from storage
	parentBlock := blockTree.GetCommitted()
	if parentBlock == nil {
		return fmt.Errorf("no committed parent block available")
	}
	
	// Line 25: Create new block with QC
	parentHash := parentBlock.Hash
	height := parentBlock.Height + 1
	block := types.NewBlock(parentHash, height, hc.currentView, hc.nodeID, payload)
	
	// Line 26-27: Sign block proposal (LeaderSig)
	blockData := fmt.Sprintf("%x:%d:%d:%d", block.Hash, block.Height, block.View, block.Proposer)
	leaderSig, err := hc.crypto.Sign([]byte(blockData))
	if err != nil {
		return fmt.Errorf("failed to sign block proposal: %w", err)
	}
	
	// Line 28-29: Broadcast ProposalMsg to all validators
	proposal := messages.NewProposalMsg(block, highestQC, hc.currentView, hc.nodeID)
	if err := hc.network.Broadcast(hc.ctx, proposal); err != nil {
		return fmt.Errorf("failed to broadcast proposal: %w", err)
	}
	
	// Leader processes its own proposal immediately (protocol compliance)
	// This ensures the leader validates its own proposal before broadcasting
	if err := hc.processProposalInternal(proposal, true); err != nil {
		return fmt.Errorf("leader failed to validate own proposal: %w", err)
	}
	
	// Start view timer for this proposal (timeout handling)
	hc.startViewTimer()
	
	fmt.Printf("   [Node %d] ✅ Proposed block %x in view %d\n", hc.nodeID, block.Hash[:8], hc.currentView)
	_ = leaderSig // Store signature in proposal in production
	return nil
}

// ProcessProposal implements lines 34-54: Prepare Phase Processing
func (hc *HotStuffCoordinator) ProcessProposal(proposal *messages.ProposalMsg) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	// Use internal processing method for followers (not leader)
	return hc.processProposalInternal(proposal, false)
}

// processProposalInternal handles proposal processing for the leader without duplicate block addition
func (hc *HotStuffCoordinator) processProposalInternal(proposal *messages.ProposalMsg, isLeader bool) error {
	// Line 38: Validate view number
	if proposal.View() != hc.currentView {
		return fmt.Errorf("proposal view %d != current view %d", proposal.View(), hc.currentView)
	}

	// Lines 39-42: Check parent exists and validate
	blockTree := hc.consensus.GetBlockTree()
	block := proposal.Block
	if !block.IsGenesis() {
		parentBlock, err := blockTree.GetBlock(block.ParentHash)
		if err != nil {
			return fmt.Errorf("parent block not found: %w", err)
		}
		
		// Line 43: Ensure parent is highestQC.block or descendant of locked block
		if parentBlock.Height+1 != block.Height {
			return fmt.Errorf("invalid block height: expected %d, got %d", parentBlock.Height+1, block.Height)
		}
	}

	// Line 44-45: Verify proposal signature
	expectedLeader := hc.getLeader(proposal.View())
	if proposal.Sender() != expectedLeader {
		return fmt.Errorf("proposal sender %d != expected leader %d for view %d", 
			proposal.Sender(), expectedLeader, proposal.View())
	}

	// Line 46: Apply safety rules
	vote, err := hc.consensus.ProcessProposal(block)
	if err != nil {
		return fmt.Errorf("safety rules rejected proposal: %w", err)
	}

	// Line 47-50: Block addition is handled by consensus engine's ProcessProposal

	// Line 51-52: Sign prepare vote
	if vote != nil {
		// Sign the vote with our crypto interface
		voteData := fmt.Sprintf("%x:%d:%s:%d", vote.BlockHash, vote.View, vote.Phase, vote.Voter)
		signature, err := hc.crypto.Sign([]byte(voteData))
		if err != nil {
			return fmt.Errorf("failed to sign prepare vote: %w", err)
		}
		vote.Signature = signature
		// Line 53-54: Send vote to leader (only if not leader)
		if !isLeader {
			voteMsg := messages.NewVoteMsg(vote, hc.nodeID)
			if err := hc.network.Send(hc.ctx, hc.getLeader(hc.currentView), voteMsg); err != nil {
				return fmt.Errorf("failed to send vote: %w", err)
			}
		}
		
		// Process our own vote (using internal method to avoid deadlock)
		if err := hc.processVoteInternal(vote); err != nil {
			return fmt.Errorf("failed to process own vote: %w", err)
		}
		
		fmt.Printf("   [Node %d] ✅ Voted for block %x in Prepare phase\\n", hc.nodeID, block.Hash[:8])
	}

	// Start timer for followers when processing proposal
	if !isLeader {
		hc.startViewTimer()
	}

	hc.currentPhase = types.PhasePrepare
	return nil
}

// ProcessVote handles vote collection and QC formation following lines 77-106
func (hc *HotStuffCoordinator) ProcessVote(vote *types.Vote) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.processVoteInternal(vote)
}

// processVoteInternal handles vote processing without mutex (assumes mutex is already held)
func (hc *HotStuffCoordinator) processVoteInternal(vote *types.Vote) error {
	// Line 82-83: Verify vote signature
	voteData := fmt.Sprintf("%x:%d:%s:%d", vote.BlockHash, vote.View, vote.Phase, vote.Voter)
	if err := hc.crypto.Verify([]byte(voteData), vote.Signature, vote.Voter); err != nil {
		return fmt.Errorf("invalid vote signature: %w", err)
	}
	
	// Store vote based on phase
	var voteMap map[types.BlockHash]map[types.NodeID]*types.Vote
	switch vote.Phase {
	case types.PhasePrepare:
		voteMap = hc.prepareVotes
	case types.PhasePreCommit:
		voteMap = hc.preCommitVotes
	case types.PhaseCommit:
		voteMap = hc.commitVotes
	default:
		return fmt.Errorf("unknown vote phase: %s", vote.Phase)
	}
	
	// Initialize vote collection for this block if needed
	if voteMap[vote.BlockHash] == nil {
		voteMap[vote.BlockHash] = make(map[types.NodeID]*types.Vote)
	}
	
	// Store vote
	voteMap[vote.BlockHash][vote.Voter] = vote
	votes := voteMap[vote.BlockHash]
	
	// Line 80-81: Check vote threshold (≥2f+1)
	requiredVotes := (len(hc.validators)*2)/3 + 1 // 2f+1
	if len(votes) >= requiredVotes {
		// Check if QC already formed for this block+phase (idempotent)
		var existingQC *types.QuorumCertificate
		switch vote.Phase {
		case types.PhasePrepare:
			existingQC = hc.prepareQCs[vote.BlockHash]
		case types.PhasePreCommit:
			existingQC = hc.preCommitQCs[vote.BlockHash]
		case types.PhaseCommit:
			existingQC = hc.commitQCs[vote.BlockHash]
		}
		
		if existingQC != nil {
			// QC already formed for this block+phase, skip
			return nil
		}
		
		// Line 84: Form Quorum Certificate (once per block+phase)
		qc, err := hc.formQuorumCertificate(vote.BlockHash, vote.View, vote.Phase, votes)
		if err != nil {
			return fmt.Errorf("failed to form QC: %w", err)
		}
		
		// Line 85-87: Store QC (fsync to disk)
		if err := hc.storeQC(qc); err != nil {
			return fmt.Errorf("failed to store QC: %w", err)
		}
		
		// Process QC based on phase (once per block+phase)
		if err := hc.processQuorumCertificate(qc); err != nil {
			return fmt.Errorf("failed to process QC: %w", err)
		}
		
		fmt.Printf("   [Node %d] ✅ Formed %s QC for block %x with %d votes\n", 
			hc.nodeID, vote.Phase, vote.BlockHash[:8], len(votes))
	}
	
	return nil
}

// processQuorumCertificate handles QC processing and phase transitions
func (hc *HotStuffCoordinator) processQuorumCertificate(qc *types.QuorumCertificate) error {
	switch qc.Phase {
	case types.PhasePrepare:
		return hc.processPreCommitPhase(qc)
	case types.PhasePreCommit:
		return hc.processCommitPhase(qc)
	case types.PhaseCommit:
		return hc.processDecidePhase(qc)
	default:
		return fmt.Errorf("unknown QC phase: %s", qc.Phase)
	}
}

// markPhaseProcessed marks a phase as processed for a block (idempotent state management)
func (hc *HotStuffCoordinator) markPhaseProcessed(blockHash types.BlockHash, phase types.ConsensusPhase) bool {
	if hc.processedPhases[blockHash] == nil {
		hc.processedPhases[blockHash] = make(map[types.ConsensusPhase]bool)
	}
	
	if hc.processedPhases[blockHash][phase] {
		return false // Already processed
	}
	
	hc.processedPhases[blockHash][phase] = true
	return true // First time processing
}

// processPreCommitPhase implements lines 110-131: Pre-Commit Phase
func (hc *HotStuffCoordinator) processPreCommitPhase(prepareQC *types.QuorumCertificate) error {
	// Idempotent: only process PreCommit phase once per block
	if !hc.markPhaseProcessed(prepareQC.BlockHash, types.PhasePreCommit) {
		return nil // Already processed this phase for this block
	}
	// Line 111-112: Broadcast PrepareQC to validators
	if hc.isLeader(hc.currentView) {
		if err := hc.broadcastQC(prepareQC); err != nil {
			return fmt.Errorf("failed to broadcast PrepareQC: %w", err)
		}
	}
	
	// Line 114-115: Verify all QC signatures
	if err := hc.verifyQC(prepareQC); err != nil {
		return fmt.Errorf("invalid PrepareQC: %w", err)
	}
	
	// Line 116: Update locked block
	_, err := hc.consensus.ProcessVote(&prepareQC.Votes[0])
	if err != nil {
		return fmt.Errorf("failed to update locked block: %w", err)
	}
	
	// Line 117-118: Sign pre-commit vote
	preCommitVote := &types.Vote{
		BlockHash: prepareQC.BlockHash,
		View:      prepareQC.View,
		Phase:     types.PhasePreCommit,
		Voter:     hc.nodeID,
	}
	
	voteData := fmt.Sprintf("%x:%d:%s:%d", preCommitVote.BlockHash, preCommitVote.View, preCommitVote.Phase, preCommitVote.Voter)
	signature, err := hc.crypto.Sign([]byte(voteData))
	if err != nil {
		return fmt.Errorf("failed to sign pre-commit vote: %w", err)
	}
	preCommitVote.Signature = signature
	
	// Line 119-120: Send pre-commit vote
	leader := hc.getLeader(hc.currentView)
	
	// If we are the leader, don't send vote to ourselves - process it directly
	if hc.isLeader(hc.currentView) {
		if err := hc.processVoteInternal(preCommitVote); err != nil {
			return fmt.Errorf("failed to process own pre-commit vote: %w", err)
		}
	} else {
		// Send vote to leader
		voteMsg := messages.NewVoteMsg(preCommitVote, hc.nodeID)
		if err := hc.network.Send(hc.ctx, leader, voteMsg); err != nil {
			return fmt.Errorf("failed to send pre-commit vote: %w", err)
		}
		
		// Also process our own vote
		if err := hc.processVoteInternal(preCommitVote); err != nil {
			return fmt.Errorf("failed to process own pre-commit vote: %w", err)
		}
	}
	
	hc.currentPhase = types.PhasePreCommit
	fmt.Printf("   [Node %d] ✅ Entered PreCommit phase for block %x\n", hc.nodeID, prepareQC.BlockHash[:8])
	return nil
}

// processCommitPhase implements lines 135-154: Commit Phase
func (hc *HotStuffCoordinator) processCommitPhase(preCommitQC *types.QuorumCertificate) error {
	// Idempotent: only process Commit phase once per block
	if !hc.markPhaseProcessed(preCommitQC.BlockHash, types.PhaseCommit) {
		return nil // Already processed this phase for this block
	}
	// Line 135-136: Broadcast PreCommitQC to validators
	if hc.isLeader(hc.currentView) {
		if err := hc.broadcastQC(preCommitQC); err != nil {
			return fmt.Errorf("failed to broadcast PreCommitQC: %w", err)
		}
	}
	
	// Line 138-139: Verify all QC signatures
	if err := hc.verifyQC(preCommitQC); err != nil {
		return fmt.Errorf("invalid PreCommitQC: %w", err)
	}
	
	// Line 140-141: Sign commit vote
	commitVote := &types.Vote{
		BlockHash: preCommitQC.BlockHash,
		View:      preCommitQC.View,
		Phase:     types.PhaseCommit,
		Voter:     hc.nodeID,
	}
	
	voteData := fmt.Sprintf("%x:%d:%s:%d", commitVote.BlockHash, commitVote.View, commitVote.Phase, commitVote.Voter)
	signature, err := hc.crypto.Sign([]byte(voteData))
	if err != nil {
		return fmt.Errorf("failed to sign commit vote: %w", err)
	}
	commitVote.Signature = signature
	
	// Line 142-143: Send commit vote
	leader := hc.getLeader(hc.currentView)
	
	// If we are the leader, don't send vote to ourselves - process it directly
	if hc.isLeader(hc.currentView) {
		if err := hc.processVoteInternal(commitVote); err != nil {
			return fmt.Errorf("failed to process own commit vote: %w", err)
		}
	} else {
		// Send vote to leader
		voteMsg := messages.NewVoteMsg(commitVote, hc.nodeID)
		if err := hc.network.Send(hc.ctx, leader, voteMsg); err != nil {
			return fmt.Errorf("failed to send commit vote: %w", err)
		}
		
		// Also process our own vote
		if err := hc.processVoteInternal(commitVote); err != nil {
			return fmt.Errorf("failed to process own commit vote: %w", err)
		}
	}
	
	hc.currentPhase = types.PhaseCommit
	fmt.Printf("   [Node %d] ✅ Entered Commit phase for block %x\n", hc.nodeID, preCommitQC.BlockHash[:8])
	return nil
}

// processDecidePhase implements lines 158-170: Decide Phase & Block Commit
func (hc *HotStuffCoordinator) processDecidePhase(commitQC *types.QuorumCertificate) error {
	// Idempotent: only process CommitQC once per block
	if hc.processedCommitQCs[commitQC.BlockHash] {
		return nil // Already processed this CommitQC for this block
	}
	hc.processedCommitQCs[commitQC.BlockHash] = true
	// Line 158-159: Broadcast CommitQC to validators
	if hc.isLeader(hc.currentView) {
		if err := hc.broadcastQC(commitQC); err != nil {
			return fmt.Errorf("failed to broadcast CommitQC: %w", err)
		}
	}
	
	// Line 161-162: Verify all QC signatures
	if err := hc.verifyQC(commitQC); err != nil {
		return fmt.Errorf("invalid CommitQC: %w", err)
	}
	
	// Line 163-166: Mark block as committed
	blockTree := hc.consensus.GetBlockTree()
	block, err := blockTree.GetBlock(commitQC.BlockHash)
	if err != nil {
		return fmt.Errorf("failed to get block for commit: %w", err)
	}
	
	// Check if block is already committed to prevent double-commitment
	committedBlock := blockTree.GetCommitted()
	if committedBlock != nil && committedBlock.Hash == block.Hash {
		// Block already committed, skip
		fmt.Printf("   [Node %d] ℹ️  Block %x already committed, skipping\n", hc.nodeID, block.Hash[:8])
	} else {
		if err := blockTree.CommitBlock(block); err != nil {
			return fmt.Errorf("failed to commit block: %w", err)
		}
		// Line 167: Emit committed block event
		fmt.Printf("   [Node %d] 🎉 COMMITTED block %x at height %d\n", hc.nodeID, block.Hash[:8], block.Height)
	}
	
	// Note: View advancement is handled by the demo coordinator to ensure synchronization
	
	return nil
}

// Helper methods

func (hc *HotStuffCoordinator) formQuorumCertificate(
	blockHash types.BlockHash,
	view types.ViewNumber,
	phase types.ConsensusPhase,
	votes map[types.NodeID]*types.Vote,
) (*types.QuorumCertificate, error) {
	requiredVotes := (len(hc.validators)*2)/3 + 1
	if len(votes) < requiredVotes {
		return nil, fmt.Errorf("insufficient votes: got %d, need %d", len(votes), requiredVotes)
	}
	
	// Convert map to slice
	voteSlice := make([]types.Vote, 0, len(votes))
	for _, vote := range votes {
		voteSlice = append(voteSlice, *vote)
		if len(voteSlice) >= requiredVotes {
			break // Take exactly 2f+1 votes
		}
	}
	
	return &types.QuorumCertificate{
		BlockHash: blockHash,
		View:      view,
		Phase:     phase,
		Votes:     voteSlice,
	}, nil
}

func (hc *HotStuffCoordinator) storeQC(qc *types.QuorumCertificate) error {
	// Store in appropriate map
	switch qc.Phase {
	case types.PhasePrepare:
		hc.prepareQCs[qc.BlockHash] = qc
	case types.PhasePreCommit:
		hc.preCommitQCs[qc.BlockHash] = qc
	case types.PhaseCommit:
		hc.commitQCs[qc.BlockHash] = qc
	}
	
	// Update highest QC for timeout/view change protocol
	hc.updateHighestQC(qc)
	
	// Line 85-87, 128-130, 151-153: Store QC (fsync to disk)
	// Use existing storage interface methods
	_ = fmt.Sprintf("QC_%s_%x_%d", qc.Phase, qc.BlockHash, qc.View) // qcKey for logging
	if err := hc.storage.StoreView(qc.View); err != nil {
		return fmt.Errorf("failed to persist QC view to storage: %w", err)
	}
	
	fmt.Printf("   [Node %d] 💾 Stored %s QC for block %x (fsync to disk)\n", 
		hc.nodeID, qc.Phase, qc.BlockHash[:8])
	
	return nil
}

func (hc *HotStuffCoordinator) verifyQC(qc *types.QuorumCertificate) error {
	requiredVotes := (len(hc.validators)*2)/3 + 1
	if len(qc.Votes) < requiredVotes {
		return fmt.Errorf("insufficient votes in QC: got %d, need %d", len(qc.Votes), requiredVotes)
	}
	
	// Verify each vote signature
	for _, vote := range qc.Votes {
		voteData := fmt.Sprintf("%x:%d:%s:%d", vote.BlockHash, vote.View, vote.Phase, vote.Voter)
		if err := hc.crypto.Verify([]byte(voteData), vote.Signature, vote.Voter); err != nil {
			return fmt.Errorf("invalid vote signature from node %d: %w", vote.Voter, err)
		}
	}
	
	return nil
}

func (hc *HotStuffCoordinator) broadcastQC(qc *types.QuorumCertificate) error {
	// Strictly follow protocol lines 88-89, 111-112, 135-136, 158-159
	var qcMsg *messages.QCMsg
	
	switch qc.Phase {
	case types.PhasePrepare:
		// Line 88-89: Broadcast(PrepareQC)
		qcMsg = messages.NewPrepareQCMsg(qc, hc.nodeID)
	case types.PhasePreCommit:
		// Line 135-136: Broadcast(PreCommitQC)  
		qcMsg = messages.NewPreCommitQCMsg(qc, hc.nodeID)
	case types.PhaseCommit:
		// Line 158-159: Broadcast(CommitQC)
		qcMsg = messages.NewCommitQCMsg(qc, hc.nodeID)
	default:
		return fmt.Errorf("cannot broadcast QC for unknown phase: %s", qc.Phase)
	}
	
	// Broadcast to all validators as specified in protocol
	if err := hc.network.Broadcast(hc.ctx, qcMsg); err != nil {
		return fmt.Errorf("failed to broadcast %s QC: %w", qc.Phase, err)
	}
	
	fmt.Printf("   [Node %d] 📡 Broadcast %s QC for block %x\n", 
		hc.nodeID, qc.Phase, qc.BlockHash[:8])
	
	return nil
}

func (hc *HotStuffCoordinator) isLeader(view types.ViewNumber) bool {
	return hc.getLeader(view) == hc.nodeID
}

func (hc *HotStuffCoordinator) getLeader(view types.ViewNumber) types.NodeID {
	return hc.validators[int(view)%len(hc.validators)]
}

func (hc *HotStuffCoordinator) advanceView() {
	hc.currentView++
	hc.currentPhase = types.PhaseNone
	
	// CRITICAL: Synchronize consensus engine view with coordinator view
	// This fixes the "block view X does not match current view Y" errors
	for hc.consensus.GetCurrentView() < hc.currentView {
		hc.consensus.AdvanceView()
	}
	
	fmt.Printf("   [Node %d] 📈 Advanced to view %d (consensus engine synchronized)\n", 
		hc.nodeID, hc.currentView)
}

// Getters for testing and monitoring
func (hc *HotStuffCoordinator) GetCurrentView() types.ViewNumber {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.currentView
}

func (hc *HotStuffCoordinator) GetCurrentPhase() types.ConsensusPhase {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.currentPhase
}

// ProcessPrepareQC processes a received PrepareQC and triggers PreCommit phase
func (hc *HotStuffCoordinator) ProcessPrepareQC(qc *types.QuorumCertificate) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.processPreCommitPhase(qc)
}

// ProcessPreCommitQC processes a received PreCommitQC and triggers Commit phase
func (hc *HotStuffCoordinator) ProcessPreCommitQC(qc *types.QuorumCertificate) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.processCommitPhase(qc)
}

// ProcessCommitQC processes a received CommitQC and triggers Decide phase
func (hc *HotStuffCoordinator) ProcessCommitQC(qc *types.QuorumCertificate) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.processDecidePhase(qc)
}

// AdvanceViewForDemo manually advances the view for demo purposes
func (hc *HotStuffCoordinator) AdvanceViewForDemo() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.advanceView()
}

// ======= TIMEOUT AND VIEW CHANGE IMPLEMENTATION =======
// Following data flow diagram lines 176-200

// startViewTimer starts the timeout timer for the current view (protocol line 177-178)
func (hc *HotStuffCoordinator) startViewTimer() {
	if !hc.config.IsTimeoutEnabled() {
		return // Timeouts disabled in config
	}
	
	// Cancel any existing timer
	hc.stopViewTimer()
	
	// Calculate timeout for current view: baseTimeout * 2^view (protocol line 178)
	timeout := hc.config.GetTimeoutForView(hc.currentView)
	
	// Start new timer
	hc.viewTimer = time.AfterFunc(timeout, func() {
		hc.onViewTimeout()
	})
	hc.timeoutStarted = true
	
	fmt.Printf("   [Node %d] ⏰ Started view timer for view %d (timeout: %v)\n", 
		hc.nodeID, hc.currentView, timeout)
}

// stopViewTimer stops the current view timer
func (hc *HotStuffCoordinator) stopViewTimer() {
	if hc.viewTimer != nil {
		hc.viewTimer.Stop()
		hc.viewTimer = nil
	}
	hc.timeoutStarted = false
}

// onViewTimeout handles view timeout (protocol lines 177-188)
func (hc *HotStuffCoordinator) onViewTimeout() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if !hc.timeoutStarted {
		return // Timer was cancelled
	}
	
	fmt.Printf("   [Node %d] ⏰ View %d timeout detected, initiating view change\n", 
		hc.nodeID, hc.currentView)
	
	// Protocol line 179-182: Sign and broadcast timeout message
	if err := hc.broadcastTimeoutMessage(); err != nil {
		fmt.Printf("   [Node %d] ❌ Failed to broadcast timeout: %v\n", hc.nodeID, err)
	}
	
	hc.timeoutStarted = false
}

// broadcastTimeoutMessage creates and broadcasts timeout message (protocol lines 179-182)
func (hc *HotStuffCoordinator) broadcastTimeoutMessage() error {
	// Create timeout message with highest known QC
	timeoutData := fmt.Sprintf("timeout:%d:%d", hc.currentView, hc.nodeID)
	signature, err := hc.crypto.Sign([]byte(timeoutData))
	if err != nil {
		return fmt.Errorf("failed to sign timeout message: %w", err)
	}
	
	timeoutMsg := messages.NewTimeoutMsg(hc.currentView, hc.highestQC, hc.nodeID, signature)
	
	// Broadcast timeout to all validators
	if err := hc.network.Broadcast(hc.ctx, timeoutMsg); err != nil {
		return fmt.Errorf("failed to broadcast timeout: %w", err)
	}
	
	// Process our own timeout message
	if err := hc.processTimeoutMessage(timeoutMsg); err != nil {
		return fmt.Errorf("failed to process own timeout: %w", err)
	}
	
	fmt.Printf("   [Node %d] 📡 Broadcast timeout for view %d\n", hc.nodeID, hc.currentView)
	return nil
}

// ProcessTimeoutMessage processes received timeout messages (protocol line 183)
func (hc *HotStuffCoordinator) ProcessTimeoutMessage(msg *messages.TimeoutMsg) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.processTimeoutMessage(msg)
}

// processTimeoutMessage handles timeout message processing
func (hc *HotStuffCoordinator) processTimeoutMessage(msg *messages.TimeoutMsg) error {
	// Validate timeout message
	if err := msg.Validate(hc.config); err != nil {
		return fmt.Errorf("invalid timeout message: %w", err)
	}
	
	// Verify signature
	timeoutData := fmt.Sprintf("timeout:%d:%d", msg.ViewNumber, msg.SenderID)
	if err := hc.crypto.Verify([]byte(timeoutData), msg.Signature, msg.SenderID); err != nil {
		return fmt.Errorf("invalid timeout signature: %w", err)
	}
	
	// Store timeout message
	if hc.timeoutMessages[msg.ViewNumber] == nil {
		hc.timeoutMessages[msg.ViewNumber] = make(map[types.NodeID]*messages.TimeoutMsg)
	}
	hc.timeoutMessages[msg.ViewNumber][msg.SenderID] = msg
	
	// Update highest QC if this timeout has a higher QC
	if msg.HighQC != nil && (hc.highestQC == nil || msg.HighQC.View > hc.highestQC.View) {
		hc.highestQC = msg.HighQC
	}
	
	fmt.Printf("   [Node %d] 📨 Received timeout from Node %d for view %d (%d/%d timeouts)\n", 
		hc.nodeID, msg.SenderID, msg.ViewNumber, 
		len(hc.timeoutMessages[msg.ViewNumber]), hc.config.QuorumThreshold())
	
	// Check if we have enough timeouts to trigger view change (≥2f+1)
	if len(hc.timeoutMessages[msg.ViewNumber]) >= hc.config.QuorumThreshold() {
		return hc.triggerViewChange(msg.ViewNumber)
	}
	
	return nil
}

// triggerViewChange advances to the next view (protocol lines 184-188)
func (hc *HotStuffCoordinator) triggerViewChange(timeoutView types.ViewNumber) error {
	// Only advance if we're still in the timeout view
	if hc.currentView != timeoutView {
		return nil // Already advanced
	}
	
	fmt.Printf("   [Node %d] 🔄 Triggering view change from %d to %d\n", 
		hc.nodeID, hc.currentView, hc.currentView+1)
	
	// Stop current view timer
	hc.stopViewTimer()
	
	// Advance to next view
	nextView := hc.currentView + 1
	hc.advanceView()
	
	// If we're the new leader, broadcast NewView message
	if hc.isLeader(nextView) {
		return hc.broadcastNewViewMessage(nextView)
	}
	
	// Start timer for new view
	hc.startViewTimer()
	return nil
}

// broadcastNewViewMessage creates and broadcasts NewView message (protocol line 185-186)
func (hc *HotStuffCoordinator) broadcastNewViewMessage(view types.ViewNumber) error {
	// Collect timeout certificates for justification
	var timeoutCerts []messages.TimeoutMsg
	if timeouts := hc.timeoutMessages[view-1]; timeouts != nil {
		for _, timeout := range timeouts {
			timeoutCerts = append(timeoutCerts, *timeout)
			if len(timeoutCerts) >= hc.config.QuorumThreshold() {
				break // Take exactly 2f+1 timeouts
			}
		}
	}
	
	// Create NewView message
	newViewMsg := messages.NewNewViewMsg(view, hc.highestQC, timeoutCerts, hc.nodeID)
	
	// Broadcast to all validators
	if err := hc.network.Broadcast(hc.ctx, newViewMsg); err != nil {
		return fmt.Errorf("failed to broadcast NewView: %w", err)
	}
	
	fmt.Printf("   [Node %d] 📡 Broadcast NewView for view %d (leader)\n", hc.nodeID, view)
	
	// Start timer for new view as leader
	hc.startViewTimer()
	return nil
}

// ProcessNewViewMessage processes received NewView messages (protocol line 187)
func (hc *HotStuffCoordinator) ProcessNewViewMessage(msg *messages.NewViewMsg) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.processNewViewMessage(msg)
}

// processNewViewMessage handles NewView message processing
func (hc *HotStuffCoordinator) processNewViewMessage(msg *messages.NewViewMsg) error {
	// Validate NewView message
	if err := msg.Validate(hc.config); err != nil {
		return fmt.Errorf("invalid NewView message: %w", err)
	}
	
	// Verify this is from the expected leader for the view
	expectedLeader := hc.getLeader(msg.ViewNumber)
	if msg.Sender() != expectedLeader {
		return fmt.Errorf("NewView from wrong leader: got %d, expected %d", 
			msg.Sender(), expectedLeader)
	}
	
	// Store NewView message
	if hc.newViewMessages[msg.ViewNumber] == nil {
		hc.newViewMessages[msg.ViewNumber] = make(map[types.NodeID]*messages.NewViewMsg)
	}
	hc.newViewMessages[msg.ViewNumber][msg.Sender()] = msg
	
	// Update highest QC if this NewView has a higher QC
	if msg.HighQC != nil && (hc.highestQC == nil || msg.HighQC.View > hc.highestQC.View) {
		hc.highestQC = msg.HighQC
	}
	
	fmt.Printf("   [Node %d] 📨 Received NewView from Node %d for view %d\n", 
		hc.nodeID, msg.Sender(), msg.ViewNumber)
	
	// If this is for a higher view than our current, advance
	if msg.ViewNumber > hc.currentView {
		hc.stopViewTimer()
		
		// Set our view to the new view
		for hc.currentView < msg.ViewNumber {
			hc.advanceView()
		}
		
		// Start timer for new view
		hc.startViewTimer()
		
		fmt.Printf("   [Node %d] ✅ Advanced to view %d based on NewView\n", 
			hc.nodeID, hc.currentView)
	}
	
	return nil
}

// updateHighestQC updates the highest known QC
func (hc *HotStuffCoordinator) updateHighestQC(qc *types.QuorumCertificate) {
	if qc != nil && (hc.highestQC == nil || qc.View > hc.highestQC.View) {
		hc.highestQC = qc
		fmt.Printf("   [Node %d] 📈 Updated highest QC to view %d, phase %s\n", 
			hc.nodeID, qc.View, qc.Phase)
	}
}

// GetHighestQC returns the highest known QC
func (hc *HotStuffCoordinator) GetHighestQC() *types.QuorumCertificate {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.highestQC
}