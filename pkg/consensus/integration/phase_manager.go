package integration

import (
	"fmt"
	"sync"

	"btc-federation/pkg/consensus/engine"
	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/types"
	"btc-federation/pkg/consensus/crypto"
	"btc-federation/pkg/consensus/network"
	"context"
)

// PhaseManager coordinates state transitions between consensus phases.
// It manages the three-phase commit protocol of HotStuff: Prepare, PreCommit, and Commit.
type PhaseManager struct {
	currentPhase types.ConsensusPhase
	consensus    *engine.HotStuffConsensus
	viewManager  *ViewManager
	
	// Infrastructure for HotStuff protocol
	network network.NetworkInterface
	crypto  crypto.CryptoInterface
	
	// Mutex for thread-safe operations
	mu sync.RWMutex
	
	// Current proposal being processed
	currentProposal *messages.ProposalMsg
	
	// Vote collection for QC formation (following lines 77-106)
	currentVotes map[types.BlockHash][]*types.Vote
	
	// Phase transition callbacks
	onPhaseChange func(oldPhase, newPhase types.ConsensusPhase)
}

// NewPhaseManager creates a new phase manager with infrastructure.
func NewPhaseManager(consensus *engine.HotStuffConsensus, viewManager *ViewManager, network network.NetworkInterface, crypto crypto.CryptoInterface) *PhaseManager {
	return &PhaseManager{
		currentPhase: types.PhaseNone,
		consensus:    consensus,
		viewManager:  viewManager,
		network:      network,
		crypto:       crypto,
		currentVotes: make(map[types.BlockHash][]*types.Vote),
	}
}

// GetCurrentPhase returns the current consensus phase.
func (pm *PhaseManager) GetCurrentPhase() types.ConsensusPhase {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.currentPhase
}

// SetPhaseChangeCallback sets a callback function that will be called on phase transitions.
func (pm *PhaseManager) SetPhaseChangeCallback(callback func(oldPhase, newPhase types.ConsensusPhase)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onPhaseChange = callback
}

// ProcessPrepare handles the prepare phase following HotStuff protocol lines 34-54.
func (pm *PhaseManager) ProcessPrepare(proposal *messages.ProposalMsg) (*types.Vote, error) {
	if proposal == nil {
		return nil, fmt.Errorf("proposal cannot be nil")
	}
	
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Store current proposal
	pm.currentProposal = proposal
	
	// Advance to prepare phase
	if err := pm.advancePhase(types.PhasePrepare); err != nil {
		return nil, fmt.Errorf("failed to advance to prepare phase: %w", err)
	}
	
	// Follow HotStuff validation path (lines 38-46):
	// Line 38: Validate view number
	// Line 39-42: Check parent exists and validate
	// Line 43: Ensure parent is highestQC.block or descendant of locked block  
	// Line 44-45: Verify proposal signature
	// Line 46: Apply safety rules
	// Line 47-50: Add block to tree and store
	
	// Process proposal through consensus engine (this handles all validation)
	vote, err := pm.consensus.ProcessProposal(proposal.Block)
	if err != nil {
		return nil, fmt.Errorf("failed to process proposal: %w", err)
	}
	
	// Line 51-54: Sign prepare vote and return for broadcasting
	return vote, nil
}

// CollectVote handles vote collection and QC formation following lines 77-106
func (pm *PhaseManager) CollectVote(vote *types.Vote) (*types.QuorumCertificate, error) {
	if vote == nil {
		return nil, fmt.Errorf("vote cannot be nil")
	}
	
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Line 78: Receive vote
	// Line 82-83: Verify vote signature
	voteData := fmt.Sprintf("%x:%d:%s:%d", vote.BlockHash, vote.View, vote.Phase, vote.Voter)
	if err := pm.crypto.Verify([]byte(voteData), vote.Signature, vote.Voter); err != nil {
		return nil, fmt.Errorf("invalid vote signature: %w", err)
	}
	
	// Store vote
	pm.currentVotes[vote.BlockHash] = append(pm.currentVotes[vote.BlockHash], vote)
	votes := pm.currentVotes[vote.BlockHash]
	
	// Line 80-81: Check vote threshold (≥2f+1)
	requiredVotes := (len(pm.viewManager.leaderElection.validators)*2)/3 + 1 // 2f+1
	if len(votes) < requiredVotes {
		return nil, nil // Not enough votes yet
	}
	
	// Line 84: Form Quorum Certificate
	votesSlice := make([]types.Vote, requiredVotes)
	for i := 0; i < requiredVotes; i++ {
		votesSlice[i] = *votes[i]
	}
	
	qc := &types.QuorumCertificate{
		BlockHash: vote.BlockHash,
		View:      vote.View,
		Phase:     vote.Phase,
		Votes:     votesSlice, // Take exactly 2f+1 votes
	}
	
	// Line 85-87: Store QC (fsync to disk)
	_, err := pm.consensus.ProcessVote(vote)
	if err != nil {
		return nil, fmt.Errorf("failed to process vote in consensus: %w", err)
	}
	
	// Clear votes for this block hash
	delete(pm.currentVotes, vote.BlockHash)
	
	return qc, nil
}

// BroadcastQC broadcasts a quorum certificate following lines 88-89
// Note: In production, this would use a dedicated QC message type
func (pm *PhaseManager) BroadcastQC(ctx context.Context, qc *types.QuorumCertificate) error {
	if qc == nil {
		return fmt.Errorf("quorum certificate cannot be nil")
	}
	
	// For now, QCs are propagated through the consensus engine's ProcessQuorumCertificate
	// In a full implementation, this would use a dedicated QC broadcast message
	return nil
}

// ProcessPreCommit handles the precommit phase when a prepare QC is formed.
func (pm *PhaseManager) ProcessPreCommit(qc *types.QuorumCertificate) error {
	if qc == nil {
		return fmt.Errorf("quorum certificate cannot be nil")
	}
	
	if qc.Phase != types.PhasePrepare {
		return fmt.Errorf("expected prepare QC, got %s QC", qc.Phase)
	}
	
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Advance to precommit phase
	if err := pm.advancePhase(types.PhasePreCommit); err != nil {
		return fmt.Errorf("failed to advance to precommit phase: %w", err)
	}
	
	// Create precommit vote for the same block
	precommitVote := types.NewVote(
		qc.BlockHash,
		pm.viewManager.GetCurrentView(),
		types.PhasePreCommit,
		pm.consensus.GetNodeID(),
		[]byte("mock_precommit_signature"),
	)
	
	// Process the precommit vote through consensus
	_, err := pm.consensus.ProcessVote(precommitVote)
	if err != nil {
		return fmt.Errorf("failed to process precommit vote: %w", err)
	}
	
	return nil
}

// ProcessCommit handles the commit phase when a precommit QC is formed.
func (pm *PhaseManager) ProcessCommit(qc *types.QuorumCertificate) error {
	if qc == nil {
		return fmt.Errorf("quorum certificate cannot be nil")
	}
	
	if qc.Phase != types.PhasePreCommit {
		return fmt.Errorf("expected precommit QC, got %s QC", qc.Phase)
	}
	
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Advance to commit phase
	if err := pm.advancePhase(types.PhaseCommit); err != nil {
		return fmt.Errorf("failed to advance to commit phase: %w", err)
	}
	
	// Create commit vote for the same block
	commitVote := types.NewVote(
		qc.BlockHash,
		pm.viewManager.GetCurrentView(),
		types.PhaseCommit,
		pm.consensus.GetNodeID(),
		[]byte("mock_commit_signature"),
	)
	
	// Process the commit vote through consensus
	_, err := pm.consensus.ProcessVote(commitVote)
	if err != nil {
		return fmt.Errorf("failed to process commit vote: %w", err)
	}
	
	return nil
}

// ProcessQuorumCertificate processes a newly formed quorum certificate and triggers appropriate phase transitions.
func (pm *PhaseManager) ProcessQuorumCertificate(qc *types.QuorumCertificate) error {
	if qc == nil {
		return fmt.Errorf("quorum certificate cannot be nil")
	}
	
	switch qc.Phase {
	case types.PhasePrepare:
		return pm.ProcessPreCommit(qc)
	case types.PhasePreCommit:
		return pm.ProcessCommit(qc)
	case types.PhaseCommit:
		return pm.finalizeCommit(qc)
	default:
		return fmt.Errorf("unknown QC phase: %s", qc.Phase)
	}
}

// finalizeCommit handles the final commit when a commit QC is formed.
func (pm *PhaseManager) finalizeCommit(qc *types.QuorumCertificate) error {
	// Block is now committed, reset to prepare for next round
	if err := pm.advancePhase(types.PhaseNone); err != nil {
		return fmt.Errorf("failed to reset phase after commit: %w", err)
	}
	
	// Clear current proposal
	pm.currentProposal = nil
	
	// Advance to next view for the next round of consensus
	pm.viewManager.AdvanceView()
	
	return nil
}

// AdvancePhase manually advances to a new phase (primarily for testing).
func (pm *PhaseManager) AdvancePhase(newPhase types.ConsensusPhase) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.advancePhase(newPhase)
}

// advancePhase internal implementation of phase advancement (must be called with mutex held).
func (pm *PhaseManager) advancePhase(newPhase types.ConsensusPhase) error {
	if !newPhase.IsValid() {
		return fmt.Errorf("invalid phase: %s", newPhase)
	}
	
	oldPhase := pm.currentPhase
	pm.currentPhase = newPhase
	
	// Call phase change callback if set
	if pm.onPhaseChange != nil {
		pm.onPhaseChange(oldPhase, newPhase)
	}
	
	return nil
}

// GetCurrentProposal returns the current proposal being processed.
func (pm *PhaseManager) GetCurrentProposal() *messages.ProposalMsg {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.currentProposal
}

// IsInPhase returns true if the phase manager is currently in the specified phase.
func (pm *PhaseManager) IsInPhase(phase types.ConsensusPhase) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.currentPhase == phase
}

// Reset resets the phase manager to initial state.
func (pm *PhaseManager) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	oldPhase := pm.currentPhase
	pm.currentPhase = types.PhaseNone
	pm.currentProposal = nil
	
	// Call phase change callback if set
	if pm.onPhaseChange != nil {
		pm.onPhaseChange(oldPhase, types.PhaseNone)
	}
}

// String returns a string representation of the phase manager state.
func (pm *PhaseManager) String() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	proposalStr := "nil"
	if pm.currentProposal != nil {
		proposalStr = fmt.Sprintf("Block(Hash=%x, View=%d)", 
			pm.currentProposal.Block.Hash, pm.currentProposal.Block.View)
	}
	
	return fmt.Sprintf("PhaseManager{Phase: %s, Proposal: %s}", 
		pm.currentPhase, proposalStr)
}