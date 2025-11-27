//go:build consensus_testing

package integration

import (
	"btc-federation/pkg/consensus/engine"
	"btc-federation/pkg/consensus/events"
	"btc-federation/pkg/consensus/messages"
	"btc-federation/pkg/consensus/types"
)

// ForceRecoverState is a test helper that simulates state sync recovery.
// It updates the coordinator's state to match a recovered state from peers.
// This method is only available in test builds (//go:build testing).
//
// Parameters:
//   - targetView: The view number to sync to
//   - highestQC: The highest QC from the source node (may be nil for early views)
//   - lockedQC: The locked QC from the source node (may be nil)
func (hc *HotStuffCoordinator) ForceRecoverState(
	targetView types.ViewNumber,
	highestQC *types.QuorumCertificate,
	lockedQC *types.QuorumCertificate,
) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	fromView := hc.currentView

	// Emit state sync started event
	if hc.eventTracer != nil {
		hc.eventTracer.RecordEvent(uint16(hc.nodeID), events.EventStateSyncRequested,
			events.EventPayload{
				"from_view": fromView,
				"to_view":   targetView,
				"node_id":   hc.nodeID,
			})
	}

	// Update coordinator view directly
	hc.currentView = targetView

	// Update highestQC
	hc.highestQC = highestQC

	// Update engine view by advancing to target
	// The engine's view tracks separately, so we need to sync it
	for hc.consensus.GetCurrentView() < targetView {
		hc.consensus.AdvanceView()
	}

	// CRITICAL: Update lockedQC on engine to preserve safety property
	// Uses EngineTestable interface to bypass phase validation
	if lockedQC != nil {
		if testable, ok := interface{}(hc.consensus).(engine.EngineTestable); ok {
			testable.ForceSetLockedQC(lockedQC)
		}
	}

	// Clear stale vote collections for all blocks
	hc.prepareVotes = make(map[types.BlockHash]map[types.NodeID]*types.Vote)
	hc.preCommitVotes = make(map[types.BlockHash]map[types.NodeID]*types.Vote)
	hc.commitVotes = make(map[types.BlockHash]map[types.NodeID]*types.Vote)

	// Clear stale QC collections
	hc.prepareQCs = make(map[types.BlockHash]*types.QuorumCertificate)
	hc.preCommitQCs = make(map[types.BlockHash]*types.QuorumCertificate)
	hc.commitQCs = make(map[types.BlockHash]*types.QuorumCertificate)

	// Clear processed phase tracking
	hc.processedPhases = make(map[types.BlockHash]map[types.ConsensusPhase]bool)
	hc.processedCommitQCs = make(map[types.BlockHash]bool)

	// Clear timeout and new view message collections
	hc.timeoutMessages = make(map[types.ViewNumber]map[types.NodeID]*messages.TimeoutMsg)
	hc.newViewMessages = make(map[types.ViewNumber]map[types.NodeID]*messages.NewViewMsg)

	// Reset phase to allow new proposals
	hc.currentPhase = types.PhaseNone

	// Stop any existing timer and restart for new view
	hc.stopViewTimer()
	hc.startViewTimer()

	// Emit completion event
	if hc.eventTracer != nil {
		hc.eventTracer.RecordEvent(uint16(hc.nodeID), events.EventStateSyncCompleted,
			events.EventPayload{
				"view":           targetView,
				"from_view":      fromView,
				"has_highest_qc": highestQC != nil,
				"has_locked_qc":  lockedQC != nil,
				"node_id":        hc.nodeID,
			})
	}

	hc.logger.Info().
		Uint64("from_view", uint64(fromView)).
		Uint64("to_view", uint64(targetView)).
		Bool("has_highest_qc", highestQC != nil).
		Bool("has_locked_qc", lockedQC != nil).
		Msg("State sync recovery completed")

	return nil
}

// Verify HotStuffCoordinator implements CoordinatorTestable at compile time
var _ CoordinatorTestable = (*HotStuffCoordinator)(nil)
