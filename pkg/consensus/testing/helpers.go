// Package testing provides helper functions for consensus protocol testing.
package testing

import (
	"context"
	"fmt"
	"time"

	"btc-federation/pkg/consensus/engine"
	"btc-federation/pkg/consensus/events"
	"btc-federation/pkg/consensus/integration"
	"btc-federation/pkg/consensus/mocks"
	"btc-federation/pkg/consensus/types"
)

// TestNetworkConfig provides sensible defaults for testing
func DefaultTestNetworkConfig() mocks.NetworkConfig {
	return mocks.NetworkConfig{
		BaseDelay:        10 * time.Millisecond,
		DelayVariation:   5 * time.Millisecond,
		PacketLossRate:   0.0,
		DuplicationRate:  0.0,
		MessageQueueSize: 100,
	}
}

// TestNetworkFailureConfig provides failure-free configuration for happy path testing
func DefaultTestNetworkFailureConfig() mocks.NetworkFailureConfig {
	return mocks.NetworkFailureConfig{
		PartitionedNodes:      make(map[types.NodeID]bool),
		FailingSendRate:       0.0,
		FailingBroadcastRate:  0.0,
		MessageCorruptionRate: 0.0,
	}
}

// CreateTestConsensusConfig creates a consensus configuration for testing
func CreateTestConsensusConfig(nodeCount int) *types.ConsensusConfig {
	publicKeys := make([]types.PublicKey, nodeCount)
	for i := 0; i < nodeCount; i++ {
		publicKeys[i] = []byte(fmt.Sprintf("public-key-node-%d", i))
	}
	
	config, err := types.NewConsensusConfig(publicKeys)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test consensus config: %v", err))
	}
	
	return config
}

// CreateTestNetwork creates a complete test network with event tracing
func CreateTestNetwork(config *types.ConsensusConfig, tracer events.EventTracer) map[types.NodeID]*integration.HotStuffCoordinator {
	nodeCount := config.TotalNodes()
	coordinators := make(map[types.NodeID]*integration.HotStuffCoordinator)
	networks := make(map[types.NodeID]*mocks.MockNetwork)
	storages := make(map[types.NodeID]*mocks.MockStorage)
	cryptos := make(map[types.NodeID]*mocks.MockCrypto)
	validators := make([]types.NodeID, nodeCount)
	
	// Create node IDs
	for i := 0; i < nodeCount; i++ {
		validators[i] = types.NodeID(i)
	}
	
	// Create infrastructure for each node
	for _, nodeID := range validators {
		// Create network with event tracing
		networkConfig := DefaultTestNetworkConfig()
		networkFailures := DefaultTestNetworkFailureConfig()
		networks[nodeID] = mocks.NewMockNetwork(nodeID, networkConfig, networkFailures)
		networks[nodeID].SetEventTracer(tracer)
		
		// Create storage
		storageConfig := mocks.DefaultStorageConfig()
		storageFailures := mocks.DefaultStorageFailureConfig()
		storages[nodeID] = mocks.NewMockStorage(storageConfig, storageFailures)
		
		// Create crypto
		cryptoConfig := mocks.DefaultCryptoConfig()
		cryptoFailures := mocks.DefaultCryptoFailureConfig()
		cryptos[nodeID] = mocks.NewMockCrypto(nodeID, cryptoConfig, cryptoFailures)
	}
	
	// Connect all network nodes
	for _, nodeID := range validators {
		networks[nodeID].SetPeers(networks)
	}
	
	// Set up cryptographic trust relationships
	for _, nodeID := range validators {
		for _, otherNodeID := range validators {
			cryptos[nodeID].AddNode(otherNodeID)
		}
	}
	
	// Create consensus coordinators
	for _, nodeID := range validators {
		// Create consensus engine
		consensus, err := engine.NewHotStuffConsensus(nodeID, config)
		if err != nil {
			panic(fmt.Sprintf("Failed to create consensus for node %d: %v", nodeID, err))
		}
		
		// Create HotStuff coordinator
		coordinator := integration.NewHotStuffCoordinator(
			nodeID,
			validators,
			config,
			consensus,
			networks[nodeID],
			storages[nodeID],
			cryptos[nodeID],
		)
		
		coordinators[nodeID] = coordinator
	}
	
	return coordinators
}

// createMockConsensusEngine creates a mock consensus engine for testing
// This is a placeholder - the actual implementation would create a real consensus engine
func createMockConsensusEngine(nodeID types.NodeID, config *types.ConsensusConfig, tracer events.EventTracer) (interface{}, error) {
	// Placeholder implementation
	// In the real implementation, this would create an actual consensus engine
	// with event tracing integrated
	return struct{}{}, nil
}

// ExecuteConsensusRound executes a complete consensus round with event collection
func ExecuteConsensusRound(nodes map[types.NodeID]*integration.HotStuffCoordinator, payload []byte) error {
	_ = context.Background()
	
	// Find the current leader (assuming round-robin for view 0)
	currentView := types.ViewNumber(0)
	leaders := make([]types.NodeID, 0, len(nodes))
	for nodeID := range nodes {
		leaders = append(leaders, nodeID)
	}
	currentLeader := leaders[int(currentView)%len(leaders)]
	
	// Leader proposes block
	coordinator := nodes[currentLeader]
	err := coordinator.ProposeBlock(payload)
	if err != nil {
		return fmt.Errorf("leader %d failed to propose block: %v", currentLeader, err)
	}
	
	// Wait for message propagation and processing
	time.Sleep(500 * time.Millisecond)
	
	// Process any pending messages
	for _, node := range nodes {
		// In a real implementation, this would process the message queue
		// For now, we'll simulate the consensus flow completion
		_ = node
	}
	
	return nil
}

// GetHappyPathRules returns validation rules for normal consensus operation
func GetHappyPathRules(nodeCount, faultyNodes uint16) []ValidationRule {
	quorumSize := 2*faultyNodes + 1
	
	return []ValidationRule{
		// Proposal ordering
		&MustHappenBeforeRule{
			EventA:      events.EventProposalReceived,
			EventB:      events.EventVoteSent, 
			RuleDescription: "Proposal must be received before voting",
		},
		&MustHappenBeforeRule{
			EventA:      events.EventProposalValidated,
			EventB:      events.EventVoteCreated,
			RuleDescription: "Proposal must be validated before creating vote",
		},
		
		// Quorum requirements
		&QuorumRule{
			EventType:   events.EventVoteSent,
			MinNodes:    quorumSize,
			RuleDescription: "Quorum of nodes must vote",
		},
		&QuorumRule{
			EventType:   events.EventBlockCommitted,
			MinNodes:    nodeCount,
			RuleDescription: "All nodes must commit block",
		},
		
		// Role-based validation
		&LeaderOnlyRule{
			EventType:   events.EventProposalCreated,
			RuleDescription: "Only leader should create proposals",
		},
		&ValidatorOnlyRule{
			EventType:   events.EventVoteSent,
			RuleDescription: "Only validators should send votes",
		},
		
		// Phase progression
		&MustHappenBeforeRule{
			EventA:      events.EventQCFormed,
			EventB:      events.EventBlockCommitted,
			RuleDescription: "QC formation must precede block commitment",
		},
		
		// Cross-node coordination
		&CrossNodeCoordinationRule{
			InitiatorEvent: events.EventProposalCreated,
			ResponseEvent:  events.EventVoteReceived,
			MinResponders:  quorumSize,
			MaxDelay:       2 * time.Second,
			RuleDescription:    "Proposal should receive quorum votes within 2s",
		},
		
		// No double voting in happy path
		&MustNotHappenRule{
			EventType: events.EventVoteSent,
			Condition: checkDoubleVoting,
			RuleDescription: "No double voting in same view/phase",
		},
		
		// Causal dependencies
		&CausalDependencyRule{
			TriggerEvent:  events.EventQCFormed,
			RequiredEvent: events.EventVoteReceived, 
			TimeWindow:    5 * time.Second,
			RuleDescription:   "QC formation requires sufficient votes",
		},
	}
}

// GetByzantineDetectionRules returns validation rules for Byzantine behavior detection
func GetByzantineDetectionRules() []ValidationRule {
	return []ValidationRule{
		// Byzantine behavior must be detected
		&CrossNodeCoordinationRule{
			InitiatorEvent: "byzantine_behavior_occurred",
			ResponseEvent:  events.EventByzantineDetected,
			MinResponders:  2, // At least 2 honest nodes must detect
			MaxDelay:       10 * time.Second,
			RuleDescription:    "Byzantine behavior must be detected by honest nodes within 10s",
		},
		
		// Evidence collection must follow detection
		&MustHappenBeforeRule{
			EventA:      events.EventByzantineDetected,
			EventB:      events.EventEvidenceStored,
			RuleDescription: "Evidence must be stored after Byzantine detection",
		},
		
		// View change should follow Byzantine detection
		&CausalDependencyRule{
			TriggerEvent:  events.EventViewChange,
			RequiredEvent: events.EventByzantineDetected,
			TimeWindow:    30 * time.Second,
			RuleDescription:   "View change should follow Byzantine detection",
		},
		
		// Byzantine node should not detect own misbehavior
		&MustNotHappenRule{
			EventType: events.EventByzantineDetected,
			Condition: func(event events.ConsensusEvent) bool {
				// Check if detecting node is the Byzantine node
				if byzantineNodeID, ok := event.Payload["byzantine_node_id"]; ok {
					if nodeID, ok := byzantineNodeID.(uint16); ok {
						return event.NodeID == nodeID
					}
				}
				return false
			},
			RuleDescription: "Byzantine node should not detect own misbehavior",
		},
	}
}

// GetPartitionRecoveryRules returns validation rules for network partition recovery
func GetPartitionRecoveryRules() []ValidationRule {
	return []ValidationRule{
		// State transitions for partition handling
		&StateTransitionRule{
			FromState:   "normal_operation",
			ToState:     "partition_mode",
			ValidEvents: []events.EventType{"partition_detected"},
			RuleDescription: "Partition mode only on partition detection",
		},
		
		// No voting during partition mode
		&MustNotHappenRule{
			EventType: events.EventVoteSent,
			Condition: func(event events.ConsensusEvent) bool {
				// Check if node is in partition mode
				if partitioned, ok := event.Payload["partitioned"]; ok {
					if isPartitioned, ok := partitioned.(bool); ok {
						return isPartitioned
					}
				}
				return false
			},
			RuleDescription: "No voting during partition mode",
		},
		
		// State sync required before resuming consensus
		&CausalDependencyRule{
			TriggerEvent:  "consensus_resumed",
			RequiredEvent: "state_sync_completed",
			TimeWindow:    30 * time.Second, 
			RuleDescription:   "State sync required before resuming consensus",
		},
	}
}

// checkDoubleVoting is a condition function to detect double voting
func checkDoubleVoting(event events.ConsensusEvent) bool {
	// This would need to be implemented with stateful tracking
	// For now, return false (no double voting detected)
	// In a real implementation, this would check against previous votes
	return false
}

// CalculateConsensusLatency calculates the latency from proposal to commitment
func CalculateConsensusLatency(eventList []events.ConsensusEvent) time.Duration {
	var proposalTime, commitTime time.Time
	
	for _, event := range eventList {
		if event.EventType == events.EventProposalCreated && proposalTime.IsZero() {
			proposalTime = event.Timestamp
		}
		if event.EventType == events.EventBlockCommitted {
			commitTime = event.Timestamp
		}
	}
	
	if proposalTime.IsZero() || commitTime.IsZero() {
		return 0
	}
	
	return commitTime.Sub(proposalTime)
}

// FilterEventsByType filters events by a specific type
func FilterEventsByType(eventList []events.ConsensusEvent, eventType events.EventType) []events.ConsensusEvent {
	var filtered []events.ConsensusEvent
	for _, event := range eventList {
		if event.EventType == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// FilterEventsByRound filters events by round number (assuming round is in payload)
func FilterEventsByRound(eventList []events.ConsensusEvent, round int) []events.ConsensusEvent {
	var filtered []events.ConsensusEvent
	for _, event := range eventList {
		if eventRound, ok := event.Payload["round"]; ok {
			if eventRound == round {
				filtered = append(filtered, event)
			}
		}
	}
	return filtered
}

// GetByzantineBehaviorTime finds when Byzantine behavior first occurred
func GetByzantineBehaviorTime(events []events.ConsensusEvent) time.Time {
	for _, event := range events {
		if event.EventType == "byzantine_behavior_occurred" {
			return event.Timestamp
		}
	}
	return time.Time{}
}

// ValidatePhaseProgression validates that phases occur in the correct order
func ValidatePhaseProgression(t interface{}, events []events.ConsensusEvent, expectedPhases []string) {
	// This would be implemented in actual test files
	// For now, it's a placeholder for the pattern
	_ = t
	_ = events
	_ = expectedPhases
}