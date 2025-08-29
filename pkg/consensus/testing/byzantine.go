//go:build testing
// +build testing

// Package testing provides Byzantine behavior simulation for consensus protocol testing.
// This code is only compiled when building with the 'testing' build tag,
// ensuring zero impact on production builds.
package testing

import (
	"fmt"
	"time"

	"btc-federation/pkg/consensus/events"
	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/types"
)

// ByzantineBehavior defines the types of malicious behaviors that can be injected for testing
type ByzantineBehavior string

const (
	// Vote-based attacks
	DoubleVoteMode         ByzantineBehavior = "double_vote"           // Vote for multiple blocks in same phase
	InvalidVoteMode        ByzantineBehavior = "invalid_vote"          // Send malformed votes
	SelectiveVotingMode    ByzantineBehavior = "selective_voting"      // Only vote for specific proposals
	VoteWithholdingMode    ByzantineBehavior = "vote_withholding"      // Refuse to vote on valid proposals
	PhaseConfusionMode     ByzantineBehavior = "phase_confusion"       // Vote in wrong consensus phase
	
	// Proposal-based attacks
	ConflictingProposalMode ByzantineBehavior = "conflicting_proposal" // Send different proposals to different nodes
	InvalidProposalMode     ByzantineBehavior = "invalid_proposal"      // Send malformed proposals
	SelectiveProposalMode   ByzantineBehavior = "selective_proposal"    // Send proposals only to subset of nodes
	DelayedProposalMode     ByzantineBehavior = "delayed_proposal"      // Deliberately delay proposals
	
	// QC-based attacks
	InvalidQCMode          ByzantineBehavior = "invalid_qc"            // Create QCs with forged signatures
	InsufficientStakeMode  ByzantineBehavior = "insufficient_stake"    // QC with insufficient voting power
	DuplicateValidatorsMode ByzantineBehavior = "duplicate_validators"  // Include duplicate validators in QC
	
	// Network-based attacks  
	MessageWithholdingMode ByzantineBehavior = "message_withholding"   // Selectively drop messages
	MessageReplayMode      ByzantineBehavior = "message_replay"        // Replay old messages
	MessageReorderingMode  ByzantineBehavior = "message_reordering"    // Deliver messages out of order
	TimingAttackMode       ByzantineBehavior = "timing_attack"         // Manipulate message timing
	FloodingAttackMode     ByzantineBehavior = "flooding_attack"       // Overwhelm nodes with messages
	
	// View change attacks
	FalseTimeoutMode       ByzantineBehavior = "false_timeout"         // Trigger unnecessary view changes
	ViewConfusionMode      ByzantineBehavior = "view_confusion"        // Send messages for wrong views
	LeaderDisruptionMode   ByzantineBehavior = "leader_disruption"     // Interfere with leader election
	StaleMessageMode       ByzantineBehavior = "stale_message"         // Use messages from previous views
	
	// Resource attacks
	ResourceExhaustionMode ByzantineBehavior = "resource_exhaustion"   // Attempt to exhaust node resources
	StateCorruptionMode    ByzantineBehavior = "state_corruption"      // Try to corrupt consensus state
	
	// Coordinated attacks
	CoordinatedAttackMode  ByzantineBehavior = "coordinated_attack"    // Multiple Byzantine nodes working together
	AdaptiveAttackMode     ByzantineBehavior = "adaptive_attack"       // Change attack strategy dynamically
)

// ByzantineNode wraps a consensus node with malicious behavior capabilities.
// This wrapper is only available in testing builds.
type ByzantineNode struct {
	nodeID        types.NodeID
	byzantineMode ByzantineBehavior
	eventTracer   events.EventTracer
	
	// Attack state
	enabled       bool
	conflictingVotes map[uint64][]types.Vote // View -> conflicting votes
	delayedMessages  []DelayedMessage
	replayMessages   []messages.ConsensusMessage
	
	// Attack configuration
	attackProbability    float64 // Probability of attacking (0.0-1.0)
	maxConflictingVotes  int     // Maximum conflicting votes per view
	messageDelayRange    time.Duration
	replayWindowSize     int
}

// DelayedMessage represents a message that should be delivered later
type DelayedMessage struct {
	Message    messages.ConsensusMessage
	Recipient  types.NodeID
	DeliverAt  time.Time
}

// NewByzantineNode creates a new Byzantine node wrapper for testing
func NewByzantineNode(nodeID types.NodeID, tracer events.EventTracer) *ByzantineNode {
	return &ByzantineNode{
		nodeID:              nodeID,
		eventTracer:         tracer,
		enabled:             false,
		conflictingVotes:    make(map[uint64][]types.Vote),
		delayedMessages:     make([]DelayedMessage, 0),
		replayMessages:      make([]messages.ConsensusMessage, 0, 100),
		attackProbability:   1.0, // Always attack when enabled
		maxConflictingVotes: 3,
		messageDelayRange:   5 * time.Second,
		replayWindowSize:    50,
	}
}

// EnableByzantineBehavior activates a specific type of malicious behavior
func (bn *ByzantineNode) EnableByzantineBehavior(behavior ByzantineBehavior) {
	bn.byzantineMode = behavior
	bn.enabled = true
	
	// Record Byzantine behavior activation
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_enabled", 
			events.EventPayload{
				"behavior_type": string(behavior),
				"node_id":      bn.nodeID,
				"timestamp":    time.Now(),
			})
	}
}

// DisableByzantineBehavior deactivates malicious behavior
func (bn *ByzantineNode) DisableByzantineBehavior() {
	bn.enabled = false
	
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_disabled",
			events.EventPayload{
				"previous_behavior": string(bn.byzantineMode),
				"node_id":          bn.nodeID,
				"timestamp":        time.Now(),
			})
	}
}

// IsEnabled returns whether Byzantine behavior is currently active
func (bn *ByzantineNode) IsEnabled() bool {
	return bn.enabled
}

// GetBehaviorType returns the current Byzantine behavior type
func (bn *ByzantineNode) GetBehaviorType() ByzantineBehavior {
	return bn.byzantineMode
}

// ShouldAttack determines if an attack should be executed based on probability
func (bn *ByzantineNode) ShouldAttack() bool {
	if !bn.enabled {
		return false
	}
	// For testing, we typically want deterministic behavior
	return true
}

// ExecuteDoubleVote creates conflicting votes for the same view/phase
func (bn *ByzantineNode) ExecuteDoubleVote(view types.ViewNumber, phase types.ConsensusPhase, originalBlockHash types.BlockHash) []types.Vote {
	if !bn.ShouldAttack() || bn.byzantineMode != DoubleVoteMode {
		return nil
	}
	
	// Record Byzantine behavior occurrence
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_occurred",
			events.EventPayload{
				"behavior_type": "double_vote",
				"view":         view,
				"phase":        phase,
				"node_id":      bn.nodeID,
			})
	}
	
	// Create conflicting votes
	conflictingVotes := []types.Vote{
		{
			BlockHash: originalBlockHash,
			View:      view,
			Phase:     phase,
			Voter:     bn.nodeID,
			Signature: []byte(fmt.Sprintf("vote1_%d_%d_%d", bn.nodeID, view, phase)),
		},
		{
			BlockHash: [32]byte{0x99, 0x99, 0x99}, // Different block hash
			View:      view,
			Phase:     phase,
			Voter:     bn.nodeID,
			Signature: []byte(fmt.Sprintf("vote2_%d_%d_%d", bn.nodeID, view, phase)),
		},
	}
	
	// Store for later reference
	bn.conflictingVotes[uint64(view)] = conflictingVotes
	
	return conflictingVotes
}

// ExecuteConflictingProposal creates different proposals for different recipients
func (bn *ByzantineNode) ExecuteConflictingProposal(view types.ViewNumber, recipients []types.NodeID) map[types.NodeID]*types.Block {
	if !bn.ShouldAttack() || bn.byzantineMode != ConflictingProposalMode {
		return nil
	}
	
	// Record Byzantine behavior
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_occurred",
			events.EventPayload{
				"behavior_type": "conflicting_proposal",
				"view":         view,
				"recipients":   recipients,
				"node_id":      bn.nodeID,
			})
	}
	
	conflictingBlocks := make(map[types.NodeID]*types.Block)
	
	for i, recipient := range recipients {
		// Create different block for each recipient
		block := &types.Block{
			View:     view,
			Height:   types.Height(view),
			Proposer: bn.nodeID,
			Payload:  []byte(fmt.Sprintf("conflicting_payload_%d_%d", recipient, i)),
		}
		
		// Calculate hash for the block
		hash := [32]byte{}
		copy(hash[:], fmt.Sprintf("hash_%d_%d", recipient, i))
		block.Hash = hash
		
		conflictingBlocks[recipient] = block
	}
	
	return conflictingBlocks
}

// ExecuteInvalidQC creates a QC with forged or invalid signatures
func (bn *ByzantineNode) ExecuteInvalidQC(view types.ViewNumber, blockHash types.BlockHash, phase types.ConsensusPhase) *types.QuorumCertificate {
	if !bn.ShouldAttack() || bn.byzantineMode != InvalidQCMode {
		return nil
	}
	
	// Record Byzantine behavior
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_occurred",
			events.EventPayload{
				"behavior_type": "invalid_qc",
				"view":         view,
				"phase":        phase,
				"node_id":      bn.nodeID,
			})
	}
	
	// Create QC with forged signatures
	invalidVotes := []types.Vote{}
	
	// Create fake votes from other nodes (forged signatures)
	for i := uint16(0); i < 3; i++ { // Create enough for quorum
		fakeVote := types.Vote{
			BlockHash: blockHash,
			View:      view,
			Phase:     phase,
			Voter:     types.NodeID(i),
			Signature: []byte(fmt.Sprintf("forged_signature_%d_%d", i, time.Now().UnixNano())),
		}
		invalidVotes = append(invalidVotes, fakeVote)
	}
	
	// Create invalid QC
	invalidQC := &types.QuorumCertificate{
		BlockHash: blockHash,
		View:      view,
		Phase:     phase,
		Votes:     invalidVotes,
		// VoterBitmap would be set based on the fake votes
	}
	
	return invalidQC
}

// ExecuteMessageWithholding selectively drops messages
func (bn *ByzantineNode) ExecuteMessageWithholding(message messages.ConsensusMessage, recipient types.NodeID) bool {
	if !bn.ShouldAttack() || bn.byzantineMode != MessageWithholdingMode {
		return false // Don't drop message
	}
	
	// Record Byzantine behavior
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_occurred",
			events.EventPayload{
				"behavior_type": "message_withholding",
				"message_type": fmt.Sprintf("%T", message),
				"recipient":    recipient,
				"node_id":      bn.nodeID,
			})
	}
	
	// Drop messages to specific nodes or message types
	return true // Drop the message
}

// ExecuteMessageReplay replays old messages
func (bn *ByzantineNode) ExecuteMessageReplay() []messages.ConsensusMessage {
	if !bn.ShouldAttack() || bn.byzantineMode != MessageReplayMode {
		return nil
	}
	
	if len(bn.replayMessages) == 0 {
		return nil
	}
	
	// Record Byzantine behavior
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_occurred",
			events.EventPayload{
				"behavior_type":     "message_replay",
				"replayed_messages": len(bn.replayMessages),
				"node_id":          bn.nodeID,
			})
	}
	
	// Return copies of stored messages for replay
	replayedMessages := make([]messages.ConsensusMessage, len(bn.replayMessages))
	copy(replayedMessages, bn.replayMessages)
	
	return replayedMessages
}

// StoreMessageForReplay adds a message to the replay buffer
func (bn *ByzantineNode) StoreMessageForReplay(message messages.ConsensusMessage) {
	if len(bn.replayMessages) >= bn.replayWindowSize {
		// Remove oldest message
		bn.replayMessages = bn.replayMessages[1:]
	}
	bn.replayMessages = append(bn.replayMessages, message)
}

// ExecuteTimingAttack delays message delivery
func (bn *ByzantineNode) ExecuteTimingAttack(message messages.ConsensusMessage, recipient types.NodeID) *DelayedMessage {
	if !bn.ShouldAttack() || bn.byzantineMode != TimingAttackMode {
		return nil
	}
	
	// Record Byzantine behavior
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_occurred",
			events.EventPayload{
				"behavior_type": "timing_attack",
				"message_type": fmt.Sprintf("%T", message),
				"recipient":    recipient,
				"delay":        bn.messageDelayRange,
				"node_id":      bn.nodeID,
			})
	}
	
	delayedMessage := &DelayedMessage{
		Message:   message,
		Recipient: recipient,
		DeliverAt: time.Now().Add(bn.messageDelayRange),
	}
	
	bn.delayedMessages = append(bn.delayedMessages, *delayedMessage)
	
	return delayedMessage
}

// GetDelayedMessages returns and clears any messages ready for delivery
func (bn *ByzantineNode) GetDelayedMessages() []DelayedMessage {
	now := time.Now()
	var readyMessages []DelayedMessage
	var pendingMessages []DelayedMessage
	
	for _, msg := range bn.delayedMessages {
		if now.After(msg.DeliverAt) {
			readyMessages = append(readyMessages, msg)
		} else {
			pendingMessages = append(pendingMessages, msg)
		}
	}
	
	bn.delayedMessages = pendingMessages
	return readyMessages
}

// ExecuteFalseTimeout triggers unnecessary view changes
func (bn *ByzantineNode) ExecuteFalseTimeout(currentView types.ViewNumber) bool {
	if !bn.ShouldAttack() || bn.byzantineMode != FalseTimeoutMode {
		return false
	}
	
	// Record Byzantine behavior
	if bn.eventTracer != nil {
		bn.eventTracer.RecordEvent(uint16(bn.nodeID), "byzantine_behavior_occurred",
			events.EventPayload{
				"behavior_type": "false_timeout",
				"current_view": currentView,
				"node_id":      bn.nodeID,
			})
	}
	
	return true // Trigger false timeout
}

// SetAttackProbability configures the probability of executing attacks
func (bn *ByzantineNode) SetAttackProbability(probability float64) {
	if probability < 0.0 {
		probability = 0.0
	} else if probability > 1.0 {
		probability = 1.0
	}
	bn.attackProbability = probability
}

// GetAttackStatistics returns statistics about executed attacks
func (bn *ByzantineNode) GetAttackStatistics() map[string]interface{} {
	return map[string]interface{}{
		"node_id":             bn.nodeID,
		"behavior_type":       string(bn.byzantineMode),
		"enabled":             bn.enabled,
		"conflicting_votes":   len(bn.conflictingVotes),
		"delayed_messages":    len(bn.delayedMessages),
		"replay_buffer_size":  len(bn.replayMessages),
		"attack_probability":  bn.attackProbability,
	}
}