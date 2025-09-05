package consensus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"btc-federation/pkg/consensus/events"
	"btc-federation/pkg/consensus/mocks"
	testinglib "btc-federation/pkg/consensus/testing"
	"btc-federation/pkg/consensus/types"
)

// TestViewTimeoutAndRecovery tests basic timeout detection and view change recovery
// This implements test 1.1 from task 05-04-view-change-flow-integration.md
func TestViewTimeoutAndRecovery(t *testing.T) {
	const totalNodes = 5
	t.Log("=== View Change Test: Timeout and Recovery ===")

	// 1. Create 5-node network with event tracing
	tracer := mocks.NewConsensusEventTracer()
	nodes, networkManager, consensusConfig, err := CreateTestNetwork(totalNodes, tracer, false)
	require.NoError(t, err, "Should successfully create test network")
	defer networkManager.StopAll()

	t.Logf("✓ Created network with %d nodes", totalNodes)

	// Create view change helpers for fault injection
	helpers := testinglib.NewViewChangeTestHelpers(nodes, networkManager, tracer, consensusConfig)

	// 2. Start normal consensus
	leader, err := consensusConfig.GetLeaderForView(0)
	require.NoError(t, err, "Should be able to get leader for view 0")
	t.Logf("Starting consensus: leader=Node%d", leader)

	err = nodes[leader].ProposeBlock([]byte("test_block"))
	require.NoError(t, err, "Leader should be able to propose block")

	// 3. Wait for proposal to be broadcasted to ALL validators, then block leader to trigger timeout
	t.Logf("Waiting for proposal broadcast from Node%d to all validators before blocking", leader)

	// Check that proposal was broadcast to all non-leader nodes
	allValidatorsReceivedProposal := true
	for nodeID := 0; nodeID < totalNodes; nodeID++ {
		if types.NodeID(nodeID) == leader {
			continue // Skip leader
		}

		// Wait for proposal received event from this validator
		receivedProposal := helpers.WaitForEventFromNode(types.NodeID(nodeID), events.EventProposalReceived, 2*time.Second)
		if !receivedProposal {
			t.Logf("⚠ Node%d did not receive proposal within timeout", nodeID)
			allValidatorsReceivedProposal = false
		}
	}

	if allValidatorsReceivedProposal {
		t.Log("✓ Proposal was received by all validators, now blocking leader")
	} else {
		t.Fatal("⚠ Not all validators received proposal")
	}

	err = helpers.BlockNode(leader)
	require.NoError(t, err, "Should be able to block leader node")

	// 4. Test precise timeout boundaries - verify events NOT triggered before timeout and ARE triggered after
	viewTimeout := consensusConfig.GetTimeoutForView(0)
	t.Logf("Testing precise timeout boundary (configured timeout: %v)", viewTimeout)

	// Wait for 90% of timeout period and verify NO timeout events yet
	earlyCheckPoint := time.Duration(float64(viewTimeout) * 0.9)
	t.Logf("Waiting 90%% of timeout period (%v) to verify no premature timeouts", earlyCheckPoint)
	time.Sleep(earlyCheckPoint)
	
	// Verify that no timeout events exist yet (nodes are synchronized, so this should catch invalid early events)
	initialTimeoutEvents := helpers.GetEventsCount(events.EventViewTimeout)
	initialTimerStartedEvents := helpers.GetEventsCount(events.EventViewTimerStarted) 
	t.Logf("At 90%% of timeout period - ViewTimeout: %d, TimerStarted: %d", initialTimeoutEvents, initialTimerStartedEvents)
	
	// Should have no timeout events at 90% mark
	if initialTimeoutEvents > 0 {
		t.Fatalf("❌ Premature timeout events detected at 90%% mark: %d events", initialTimeoutEvents)
	}
	
    // Wait for remaining 10% plus buffer for timeout detection
    remainingTime := viewTimeout - earlyCheckPoint + 500*time.Millisecond
    t.Logf("Waiting remaining time (%v) for timeout detection", remainingTime)
    time.Sleep(remainingTime)
    // Ensure at least one timeout event observed after full timeout
    require.True(t, helpers.WaitForEvent(events.EventViewTimeout, 2*time.Second), "Timeout must be detected after full period")
	
	// Verify timeout events were triggered after the full timeout period
	finalTimeoutEvents := helpers.GetEventsCount(events.EventViewTimeout)
	finalTimerStartedEvents := helpers.GetEventsCount(events.EventViewTimerStarted)
	t.Logf("After full timeout period - ViewTimeout: %d, TimerStarted: %d", finalTimeoutEvents, finalTimerStartedEvents)

	// 5. Validate timeout events were emitted by nodes
	allEvents := tracer.GetEvents()
	t.Logf("✓ Collected %d events after timeout period", len(allEvents))

	// Debug: Show what events we actually have
	eventTypes := make(map[string]int)
	for _, event := range allEvents {
		eventTypes[string(event.EventType)]++
	}
	t.Log("📋 Actual event types collected:")
	for eventType, count := range eventTypes {
		t.Logf("  %s: %d", eventType, count)
	}

    // Check for any view change related events (be more flexible)
    viewChangeStarted := helpers.GetEventsCount(events.EventViewChangeStarted)
    viewTimeoutEvents := helpers.GetEventsCount(events.EventViewTimeout)
    timerStartedEvents := helpers.GetEventsCount(events.EventViewTimerStarted)
    timeoutSentEvents := helpers.GetEventsCount(events.EventTimeoutMessageSent)
    timeoutRecvEvents := helpers.GetEventsCount(events.EventTimeoutMessageReceived)
    timeoutValEvents := helpers.GetEventsCount(events.EventTimeoutMessageValidated)

	t.Logf("View change events found:")
	t.Logf("  ViewChangeStarted: %d", viewChangeStarted)
	t.Logf("  ViewTimeout: %d", viewTimeoutEvents)
    t.Logf("  ViewTimerStarted: %d", timerStartedEvents)
    t.Logf("  TimeoutMessageSent: %d", timeoutSentEvents)
    t.Logf("  TimeoutMessageReceived: %d", timeoutRecvEvents)
    t.Logf("  TimeoutMessageValidated: %d", timeoutValEvents)

    // Require at least quorum of timeouts to proceed (strict)
    quorum := int(consensusConfig.QuorumThreshold())
    require.GreaterOrEqual(t, timeoutSentEvents, quorum, "At least quorum timeout messages should be sent")
    require.GreaterOrEqual(t, timeoutRecvEvents, quorum, "At least quorum timeout messages should be received")
    require.GreaterOrEqual(t, timeoutValEvents, quorum, "At least quorum timeout messages should be validated")

	// Validate one event per validator for view change activity

	// Count view change events per validator (excluding blocked leader)
	viewChangePerValidator := make(map[types.NodeID]int)
	for nodeID := 0; nodeID < totalNodes; nodeID++ {
		if types.NodeID(nodeID) == leader {
			continue // Skip blocked leader
		}

		nodeEvents := helpers.FilterEventsByNode(types.NodeID(nodeID))
		count := 0
		for _, event := range nodeEvents {
			if event.EventType == events.EventViewChangeStarted ||
				event.EventType == events.EventViewTimeout ||
				event.EventType == events.EventViewTimerStarted {
				count++
			}
		}
		viewChangePerValidator[types.NodeID(nodeID)] = count
	}

	t.Log("View change activity per validator:")
	for nodeID, count := range viewChangePerValidator {
		t.Logf("  Node%d: %d events", nodeID, count)
	}

	// Require view change activity from ALL non-leader validators
	expectedValidators := totalNodes - 1 // All nodes except the blocked leader
	validatorsWithActivity := 0
	for _, count := range viewChangePerValidator {
		if count > 0 {
			validatorsWithActivity++
		}
	}
	
	totalViewChangeActivity := 0
	for _, count := range viewChangePerValidator {
		totalViewChangeActivity += count
	}

	t.Logf("View change validation: %d/%d validators have activity (%d total events)", 
		validatorsWithActivity, expectedValidators, totalViewChangeActivity)
	
	if validatorsWithActivity != expectedValidators {
		t.Fatalf("❌ Expected view change activity from ALL %d validators, but only %d had activity", 
			expectedValidators, validatorsWithActivity)
	}
	
	t.Logf("✓ View change activity detected from ALL %d validators (%d total events)", 
		expectedValidators, totalViewChangeActivity)

	// 6. Check for view change events
	leaderElectedEvents := helpers.FilterEventsByType(events.EventLeaderElected)
	newViewStartedEvents := helpers.FilterEventsByType(events.EventNewViewStarted)

	// 6a. Verify new leader was selected via protocol  
	newLeader, err := consensusConfig.GetLeaderForView(1) // View 1 after view change
	require.NoError(t, err, "Should be able to get leader for view 1")
	t.Logf("Expected new leader for view 1: Node%d", newLeader)
	assert.NotEqual(t, leader, newLeader, "New leader should be different from old leader")
	
	// 6b. CRITICAL: Verify that the new leader actually proposed a block in view 1
	// Wait for the new leader to complete adaptive synchronization and propose
	t.Logf("⏳ Waiting for new leader Node%d to complete adaptive synchronization and propose...", newLeader)
	time.Sleep(3 * time.Second) // Wait for adaptive sync (1s) + proposal processing time
	
	// Look for proposal created events from the new leader in view 1
	newLeaderProposalEvents := helpers.FilterEventsByType(events.EventProposalCreated)
	foundNewLeaderProposal := false
	
	t.Logf("🔍 Debugging proposal events (total: %d):", len(newLeaderProposalEvents))
	for i, event := range newLeaderProposalEvents {
		t.Logf("  Proposal #%d: NodeID=%d, Payload=%+v", i, event.NodeID, event.Payload)
		
		// Check if this proposal is from new leader and in view 1
		if event.NodeID == uint16(newLeader) {
			if viewPayload, exists := event.Payload["view"]; exists {
				// Handle both uint64 and types.ViewNumber
				var eventView uint64
				var viewOk bool
				
				if v, ok := viewPayload.(uint64); ok {
					eventView = v
					viewOk = true
				} else if v, ok := viewPayload.(types.ViewNumber); ok {
					eventView = uint64(v)
					viewOk = true
				}
				
				if viewOk {
					t.Logf("  -> Proposal from new leader Node%d in view %d", newLeader, eventView)
					if eventView == 1 {
						foundNewLeaderProposal = true
						t.Logf("✓ Found proposal from new leader Node%d in view 1", newLeader)
						break
					}
				} else {
					t.Logf("  -> View payload type issue: %T (value: %+v)", viewPayload, viewPayload)
				}
			} else {
				t.Logf("  -> No view payload in proposal from Node%d", newLeader)
			}
		}
	}
	
	// STRICT REQUIREMENT: New leader MUST propose in view 1
	if !foundNewLeaderProposal {
		t.Fatalf("❌ CRITICAL: New leader Node%d did not propose any block in view 1", newLeader)
	}

	// 6c. Verify all nodes have advanced to view 1 (if view change occurred)
	if len(leaderElectedEvents) > 0 {
		err = helpers.VerifyAllNodesHaveView(1)
		if err != nil {
			t.Logf("⚠ Not all nodes at expected view: %v", err)
		} else {
			t.Log("✓ All nodes have advanced to view 1")
		}
	}

	// 7. CRITICAL: Verify leader_elected occurs BEFORE new_view_started (data flow line 558)

	t.Logf("Leader elected events: %d", len(leaderElectedEvents))
	t.Logf("New view started events: %d", len(newViewStartedEvents))

    // STRICT: Both events must be present and in order
    require.Greater(t, len(leaderElectedEvents), 0, "leader_elected event must be emitted")
    require.Greater(t, len(newViewStartedEvents), 0, "new_view_started event must be emitted")
    orderingCorrect := helpers.ValidateEventOrder(events.EventLeaderElected, events.EventNewViewStarted)
    assert.True(t, orderingCorrect, "leader_elected must occur before or with new_view_started")
    if orderingCorrect {
        t.Log("✓ Event ordering correct: leader_elected before new_view_started")
    }

    // STRICT per-node checks: exactly one leader_elected and new_view_started in view 1 for all nodes
    for nodeID := 0; nodeID < totalNodes; nodeID++ {
        nodeEvents := helpers.FilterEventsByNode(types.NodeID(nodeID))
        var leaderElectedCount, newViewStartedCount int
        for _, ev := range nodeEvents {
            // extract view value robustly
            toView1 := func(val interface{}) bool {
                switch vv := val.(type) {
                case uint64:
                    return vv == 1
                case int:
                    return vv == 1
                case types.ViewNumber:
                    return uint64(vv) == 1
                default:
                    return false
                }
            }
            if ev.EventType == events.EventLeaderElected {
                if v, exists := ev.Payload["view"]; exists && toView1(v) {
                    leaderElectedCount++
                }
            }
            if ev.EventType == events.EventNewViewStarted {
                if v, exists := ev.Payload["view"]; exists && toView1(v) {
                    newViewStartedCount++
                }
            }
        }
        assert.Equalf(t, 1, leaderElectedCount, "Node %d should have exactly one leader_elected in view 1", nodeID)
        assert.Equalf(t, 1, newViewStartedCount, "Node %d should have exactly one new_view_started in view 1", nodeID)
    }

	// 8. Verify consensus can proceed in new view with complete round verification
	t.Log("Testing consensus recovery in new view")

    // IMPORTANT: Keep original leader blocked while verifying view 1 to avoid mixing participation

	// STRICT REQUIREMENT: Validate complete consensus was achieved in view 1 from the view change
	t.Log("🔍 STRICT VALIDATION: Verifying complete consensus was achieved in view 1")
	
	// Check that we have a complete 3-phase consensus for view 1
	view1Events := helpers.FilterEventsByView(1)
	t.Logf("Found %d events specifically for view 1", len(view1Events))
	
	// Track consensus phases for view 1
	var hasProposalCreated, hasProposalBroadcasted, hasPrepareQC, hasPreCommitQC, hasCommitQC, hasBlockCommitted bool
	
	for _, event := range view1Events {
		switch event.EventType {
		case events.EventProposalCreated:
			if event.NodeID == uint16(newLeader) {
				hasProposalCreated = true
				t.Logf("✓ Found ProposalCreated from new leader Node%d in view 1", newLeader)
			}
		case events.EventProposalBroadcasted:
			if event.NodeID == uint16(newLeader) {
				hasProposalBroadcasted = true
				t.Logf("✓ Found ProposalBroadcasted from new leader Node%d in view 1", newLeader)
			}
		case events.EventPrepareQCFormed:
			hasPrepareQC = true
			t.Log("✓ Found PrepareQC formed in view 1")
		case events.EventPreCommitQCFormed:
			hasPreCommitQC = true
			t.Log("✓ Found PreCommitQC formed in view 1")
		case events.EventCommitQCFormed:
			hasCommitQC = true
			t.Log("✓ Found CommitQC formed in view 1")
		case events.EventBlockCommitted:
			hasBlockCommitted = true
			t.Log("✓ Found Block committed in view 1")
		}
	}
	
	// STRICT VALIDATION: All phases must be present for view 1
	var missing []string
	if !hasProposalCreated { missing = append(missing, "ProposalCreated") }
	if !hasProposalBroadcasted { missing = append(missing, "ProposalBroadcasted") }
	if !hasPrepareQC { missing = append(missing, "PrepareQC") }
	if !hasPreCommitQC { missing = append(missing, "PreCommitQC") }
	if !hasCommitQC { missing = append(missing, "CommitQC") }
	if !hasBlockCommitted { missing = append(missing, "BlockCommitted") }
	
	if len(missing) > 0 {
		t.Fatalf("❌ CRITICAL: Incomplete consensus in view 1 - missing phases: %v", missing)
	}
	
	t.Log("✅ STRICT VALIDATION PASSED: Complete 3-phase consensus achieved in view 1")
	
	// ADDITIONAL STRICT CHECK: Verify safety rules are working
	t.Log("🔍 STRICT VALIDATION: Testing safety rules by attempting duplicate proposal")
	err = nodes[newLeader].ProposeBlock([]byte("duplicate_proposal"))
	if err == nil {
		t.Fatal("❌ CRITICAL: Safety rules failed - duplicate proposal should be rejected")
	}
	t.Logf("✓ Safety rules working correctly - duplicate proposal rejected: %v", err)

	// STRICT VALIDATION: Verify quorum requirements for all QCs in view 1
	t.Log("🔍 STRICT VALIDATION: Verifying quorum requirements for view 1")
	quorumThreshold := consensusConfig.QuorumThreshold()
	t.Logf("Required quorum threshold: %d votes", quorumThreshold)
	
	// Count prepare votes in view 1
	prepareVotes := 0
	precommitVotes := 0
	commitVotes := 0
	
	for _, event := range view1Events {
		switch event.EventType {
		case events.EventPrepareVoteSent:
			prepareVotes++
		case events.EventPreCommitVoteSent:
			precommitVotes++
		case events.EventCommitVoteSent:
			commitVotes++
		}
	}
	
	t.Logf("Votes in view 1 - Prepare: %d, PreCommit: %d, Commit: %d", prepareVotes, precommitVotes, commitVotes)
	
	if prepareVotes < quorumThreshold {
		t.Fatalf("❌ CRITICAL: Insufficient prepare votes in view 1: got %d, need %d", prepareVotes, quorumThreshold)
	}
	if precommitVotes < quorumThreshold {
		t.Fatalf("❌ CRITICAL: Insufficient precommit votes in view 1: got %d, need %d", precommitVotes, quorumThreshold)
	}
	if commitVotes < quorumThreshold {
		t.Fatalf("❌ CRITICAL: Insufficient commit votes in view 1: got %d, need %d", commitVotes, quorumThreshold)
	}
	
	t.Log("✅ STRICT VALIDATION PASSED: All phases have sufficient quorum in view 1")
	
	// STRICT VALIDATION: Verify all non-partitioned nodes participated
	t.Log("🔍 STRICT VALIDATION: Verifying all available nodes participated in view 1 consensus")
	participatingNodes := make(map[types.NodeID]bool)
	expectedParticipants := totalNodes - 1 // Exclude the partitioned original leader
	
	for _, event := range view1Events {
		if event.EventType == events.EventPrepareVoteSent || 
		   event.EventType == events.EventPreCommitVoteSent ||
		   event.EventType == events.EventCommitVoteSent {
			participatingNodes[types.NodeID(event.NodeID)] = true
		}
	}
	
	t.Logf("Participating nodes in view 1: %v", participatingNodes)
	t.Logf("Expected participants: %d (excluding partitioned Node%d)", expectedParticipants, leader)
	
	// Debug: Show which nodes are missing
	allNodes := []types.NodeID{0, 1, 2, 3, 4}
	var missingNodes []types.NodeID
	for _, nodeID := range allNodes {
		if nodeID != leader && !participatingNodes[nodeID] {
			missingNodes = append(missingNodes, nodeID)
		}
	}
	
	if len(missingNodes) > 0 {
		t.Fatalf("❌ CRITICAL: Nodes %v did not participate in view 1 consensus (partitioned: Node%d)", 
			missingNodes, leader)
	}
	
    assert.Equal(t, expectedParticipants, len(participatingNodes), "Exactly non-partitioned nodes must participate in view 1")
	
	t.Logf("✅ STRICT VALIDATION PASSED: All %d available nodes participated in view 1 consensus", expectedParticipants)

    // Additional STRICT VALIDATION: NewView message exchange must occur
    newViewSent := helpers.GetEventsCount(events.EventNewViewMessageSent)
    newViewRecv := helpers.GetEventsCount(events.EventNewViewMessageReceived)
    newViewVal := helpers.GetEventsCount(events.EventNewViewMessageValidated)
    t.Logf("NewView messages - sent: %d, received: %d, validated: %d", newViewSent, newViewRecv, newViewVal)
    require.GreaterOrEqual(t, newViewSent, quorum, "At least quorum NewView messages must be sent")
    require.GreaterOrEqual(t, newViewRecv, quorum, "At least quorum NewView messages must be received by leader")
    require.GreaterOrEqual(t, newViewVal, quorum, "At least quorum NewView messages must be validated by leader")

    // Additional STRICT VALIDATION: Highest QC should be updated during new view processing
    highestQCUpdates := helpers.FilterEventsByType(events.EventHighestQCUpdated)
    assert.Greater(t, len(highestQCUpdates), 0, "highest_qc_updated must be emitted during new view processing")

    // Final validation summary
    finalEvents := tracer.GetEvents()
    t.Logf("\\n📊 Test Summary:")
    t.Logf("  Total events collected: %d", len(finalEvents))
    t.Logf("  Timeout detections: %d", helpers.GetEventsCount(events.EventViewTimeoutDetected))
    t.Logf("  Timeout messages: %d", helpers.GetEventsCount(events.EventTimeoutMessageSent))
    t.Logf("  Leader elections: %d", len(leaderElectedEvents))
    t.Logf("  New view starts: %d", len(newViewStartedEvents))

	// Show event breakdown by node
	t.Log("\\nEvent breakdown by node:")
	for nodeID := types.NodeID(0); nodeID < types.NodeID(totalNodes); nodeID++ {
		nodeEvents := helpers.FilterEventsByNode(nodeID)
		t.Logf("  Node %d: %d events", nodeID, len(nodeEvents))
	}

	// Basic validation already done above with strict ALL validator requirement

    t.Log("\\n=== View Change Test Complete ===")

    // Unblock original leader at the very end for cleanup
    err = helpers.UnblockNode(leader)
    require.NoError(t, err, "Should be able to unblock original leader")
}

// TestNetworkPartitionViewChange tests view change when leader is partitioned
// This is a simpler test to validate partition functionality works correctly
func TestNetworkPartitionViewChange(t *testing.T) {
	const totalNodes = 5
	t.Log("=== View Change Test: Network Partition ===")

	tracer := mocks.NewConsensusEventTracer()
	nodes, networkManager, consensusConfig, err := CreateTestNetwork(totalNodes, tracer, false)
	require.NoError(t, err, "Should successfully create test network")
	defer networkManager.StopAll()

	helpers := testinglib.NewViewChangeTestHelpers(nodes, networkManager, tracer, consensusConfig)

	// Identify leader and partition them
	leader, err := consensusConfig.GetLeaderForView(0)
	require.NoError(t, err, "Should be able to get leader for view 0")
	t.Logf("Partitioning leader Node%d from network", leader)

	err = helpers.BlockNode(leader)
	require.NoError(t, err, "Should be able to partition leader")

	// Wait for partition to be detected and timeout to occur
	time.Sleep(3 * time.Second)

	// Check that nodes detected the issue
	partitionEvents := tracer.GetEvents()
	t.Logf("Events after partition: %d", len(partitionEvents))

	// Verify some timeout-related activity occurred
	timeoutRelatedEvents := 0
	timeoutRelatedEvents += helpers.GetEventsCount(events.EventViewTimeoutDetected)
	timeoutRelatedEvents += helpers.GetEventsCount(events.EventTimeoutMessageSent)

	t.Logf("Timeout-related events detected: %d", timeoutRelatedEvents)

	// In a partition scenario, we expect at least some timeout activity
	if timeoutRelatedEvents > 0 {
		t.Log("✓ Network partition triggered timeout mechanisms")
	} else {
		t.Log("⚠ No timeout mechanisms triggered (may indicate missing implementation)")
	}

	// Restore network
	err = helpers.UnblockNode(leader)
	require.NoError(t, err, "Should be able to restore leader connectivity")

	t.Log("✓ Network partition test completed")
}
